package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/audit"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

// recordingExecutionSink captures execution audit events in memory and can be
// made to fail or panic so the side-channel guarantees can be asserted.
type recordingExecutionSink struct {
	events  []audit.ExecutionEvent
	err     error
	panicOn bool
}

func (r *recordingExecutionSink) RecordExecution(_ context.Context, event audit.ExecutionEvent) error {
	if r.panicOn {
		panic("audit sink panic")
	}
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, event)
	return nil
}

func newExecutionAuditServer(t *testing.T, db *sql.DB, role agenttools.Role, sink ExecutionAuditSink, nav NavCrawler) *Server {
	t.Helper()
	portfolio := portfoliosvc.NewService(db)
	admin := adminsvc.NewServiceWithDriver(db, "sqlite")
	var confirmations confirmationConsumer
	if role == agenttools.RoleOperator {
		confirmations = allowConfirmationConsumer{}
	}
	server, err := NewServer(ServerDeps{
		Portfolio:      &portfolio,
		Admin:          &admin,
		AgentOps:       confirmations,
		Nav:            nav,
		ExecutionAudit: sink,
		Role:           role,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return server
}

// assertExecutionEventSafe verifies that an execution event cannot carry raw
// error text, storage internals, paths, or URLs into the audit trail.
func assertExecutionEventSafe(t *testing.T, event audit.ExecutionEvent) {
	t.Helper()
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal execution event: %v", err)
	}
	raw := string(encoded)
	for _, marker := range []string{"SQLSTATE", "42P01", "sqlite", "postgres", "dial tcp", "://", ":\\"} {
		if strings.Contains(raw, marker) {
			t.Fatalf("execution event leaks %q: %s", marker, raw)
		}
	}
	if !event.Status.IsValid() {
		t.Fatalf("event status %q is not in the closed set", event.Status)
	}
	if event.ErrorCategory != "" && !event.ErrorCategory.IsValid() {
		t.Fatalf("event category %q is not in the closed set", event.ErrorCategory)
	}
	if event.Status == audit.ExecutionOK && event.ErrorCategory != "" {
		t.Fatalf("ok event carries error category %q", event.ErrorCategory)
	}
	if event.DurationMs < 0 {
		t.Fatalf("event duration %d, want >= 0", event.DurationMs)
	}
}

func TestExecutionAuditRecordsOKForAuditedTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	sink := &recordingExecutionSink{}
	server := newExecutionAuditServer(t, db, agenttools.RoleOperator, sink, &fakeNavCrawler{})

	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"audit-ok"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"crawl_nav","arguments":{"fund_code":"019173"}}`),
	})
	if resp.Error != nil {
		t.Fatalf("crawl_nav error = %#v", resp.Error)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %d, want 1", len(sink.events))
	}
	event := sink.events[0]
	if event.Tool != "crawl_nav" || event.Status != audit.ExecutionOK {
		t.Fatalf("event = %#v, want ok for crawl_nav", event)
	}
	if event.ErrorCategory != "" {
		t.Fatalf("ErrorCategory = %q, want empty for ok outcome", event.ErrorCategory)
	}
	if event.RecordedAt == "" {
		t.Fatalf("RecordedAt is empty")
	}
	assertExecutionEventSafe(t, event)
}

func TestExecutionAuditRecordsErroredInternalCategory(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	sink := &recordingExecutionSink{}
	server := newExecutionAuditServer(t, db, agenttools.RoleOperator, sink, nil)

	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"audit-errored"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"crawl_nav","arguments":{"fund_code":"019173"}}`),
	})
	if resp.Error == nil || resp.Error.Code != -32000 {
		t.Fatalf("response = %#v, want -32000", resp)
	}
	if !strings.Contains(resp.Error.Message, "nav crawler is not configured") {
		t.Fatalf("response = %#v, want crawler unconfigured error", resp)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %d, want 1", len(sink.events))
	}
	event := sink.events[0]
	if event.Tool != "crawl_nav" || event.Status != audit.ExecutionErrored {
		t.Fatalf("event = %#v, want errored for crawl_nav", event)
	}
	if event.ErrorCategory != audit.ExecutionCategoryInternal {
		t.Fatalf("ErrorCategory = %q, want internal", event.ErrorCategory)
	}
	assertExecutionEventSafe(t, event)
}

func TestExecutionAuditRecordsValidationCategory(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	sink := &recordingExecutionSink{}
	server := newExecutionAuditServer(t, db, agenttools.RoleOperator, sink, nil)

	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"audit-validation"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"import_transactions","arguments":{"confirmation_id":1,"confirmation_token":"x"}}`),
	})
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("response = %#v, want -32602", resp)
	}
	if !strings.Contains(resp.Error.Message, "transactions array is required") {
		t.Fatalf("response = %#v, want transactions validation", resp)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %d, want 1", len(sink.events))
	}
	event := sink.events[0]
	if event.Tool != "import_transactions" || event.Status != audit.ExecutionErrored {
		t.Fatalf("event = %#v, want errored for import_transactions", event)
	}
	if event.ErrorCategory != audit.ExecutionCategoryValidation {
		t.Fatalf("ErrorCategory = %q, want validation", event.ErrorCategory)
	}
	assertExecutionEventSafe(t, event)
}

func TestExecutionAuditRecordsPanicRecovered(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	sink := &recordingExecutionSink{}
	server := newExecutionAuditServer(t, db, agenttools.RoleOperator, sink, panickingNavCrawler{})

	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"audit-panic"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"crawl_nav","arguments":{"fund_code":"019173"}}`),
	})
	if resp.Error == nil || resp.Error.Code != -32603 || resp.Error.Message != "internal_error" {
		t.Fatalf("response = %#v, want -32603 internal_error", resp)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %d, want 1", len(sink.events))
	}
	event := sink.events[0]
	if event.Tool != "crawl_nav" || event.Status != audit.ExecutionPanicRecovered {
		t.Fatalf("event = %#v, want panic_recovered for crawl_nav", event)
	}
	if event.ErrorCategory != audit.ExecutionCategoryInternal {
		t.Fatalf("ErrorCategory = %q, want internal", event.ErrorCategory)
	}
	assertExecutionEventSafe(t, event)
}

func TestExecutionAuditRecordsGateDenialForAuditedTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	sink := &recordingExecutionSink{}
	// crawl_nav is maintenance scope, which the analyst role cannot reach.
	server := newExecutionAuditServer(t, db, agenttools.RoleAnalyst, sink, nil)

	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"audit-denied"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"crawl_nav","arguments":{"fund_code":"019173"}}`),
	})
	if resp.Error == nil || resp.Error.Code != -32001 {
		t.Fatalf("response = %#v, want -32001 denied", resp)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %d, want 1", len(sink.events))
	}
	event := sink.events[0]
	if event.Tool != "crawl_nav" || event.Status != audit.ExecutionErrored {
		t.Fatalf("event = %#v, want errored for crawl_nav", event)
	}
	if event.ErrorCategory != audit.ExecutionCategoryDenied {
		t.Fatalf("ErrorCategory = %q, want denied", event.ErrorCategory)
	}
	assertExecutionEventSafe(t, event)
}

func TestExecutionAuditSkipsToolsWithoutRecordResultPolicy(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	sink := &recordingExecutionSink{}
	server := newExecutionAuditServer(t, db, agenttools.RoleAnalyst, sink, nil)

	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"audit-skip"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_market_indices","arguments":{}}`),
	})
	if resp.Error != nil {
		t.Fatalf("get_market_indices error = %#v", resp.Error)
	}
	if len(sink.events) != 0 {
		t.Fatalf("events = %d, want 0: registry record_result=false must skip execution audit", len(sink.events))
	}
}

func TestExecutionAuditSinkFailureDoesNotAffectToolPath(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	sink := &recordingExecutionSink{err: errors.New("audit store unavailable")}
	server := newExecutionAuditServer(t, db, agenttools.RoleOperator, sink, &fakeNavCrawler{})

	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"audit-sink-fail"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"crawl_nav","arguments":{"fund_code":"019173"}}`),
	})
	if resp.Error != nil {
		t.Fatalf("tool failed when audit sink failed: %#v", resp.Error)
	}
	payload := decodeTextPayload(t, firstTextContent(t, resp))
	if payload["status"] != "complete" {
		t.Fatalf("payload = %#v, want complete tool result", payload)
	}
	if len(sink.events) != 0 {
		t.Fatalf("events = %d, want 0 for failing sink", len(sink.events))
	}
}

func TestExecutionAuditSinkPanicDoesNotAffectToolPath(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	sink := &recordingExecutionSink{panicOn: true}
	server := newExecutionAuditServer(t, db, agenttools.RoleOperator, sink, &fakeNavCrawler{})

	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"audit-sink-panic"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"crawl_nav","arguments":{"fund_code":"019173"}}`),
	})
	if resp.Error != nil {
		t.Fatalf("tool failed when audit sink panicked: %#v", resp.Error)
	}
	if len(sink.events) != 0 {
		t.Fatalf("events = %d, want 0 for panicking sink", len(sink.events))
	}
}

func TestExecutionAuditSinkFailureDoesNotAffectPanicRecovery(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	sink := &recordingExecutionSink{err: errors.New("audit store unavailable")}
	server := newExecutionAuditServer(t, db, agenttools.RoleOperator, sink, panickingNavCrawler{})

	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"audit-panic-fail"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"crawl_nav","arguments":{"fund_code":"019173"}}`),
	})
	if resp.Error == nil || resp.Error.Code != -32603 || resp.Error.Message != "internal_error" {
		t.Fatalf("response = %#v, want -32603 internal_error despite failing sink", resp)
	}
}

func TestExecutionErrorCategoryIsClosedAndNeverStoresRawError(t *testing.T) {
	windowsDrivePath := strings.Join([]string{"C:", "data", "fund.db"}, "\\")
	cases := []struct {
		name string
		code int
		msg  string
		want audit.ExecutionErrorCategory
	}{
		{"nil error", 0, "", ""},
		{"postgres sqlstate", -32000, `tool_error: ERROR: relation "transactions" does not exist (SQLSTATE 42P01)`, audit.ExecutionCategoryInternal},
		{"sqlite busy", -32000, "tool_error: sqlite: database is locked (5) (SQLITE_BUSY)", audit.ExecutionCategoryInternal},
		{"dial failure", -32000, "tool_error: dial tcp example.com:5432: connect: connection refused", audit.ExecutionCategoryInternal},
		{"windows file path", -32000, "tool_error: open " + windowsDrivePath + ": The system cannot find the path specified.", audit.ExecutionCategoryInternal},
		{"url detail", -32000, "tool_error: fetch failed: https://example.com/x", audit.ExecutionCategoryInternal},
		{"sanitized internal", -32000, "tool_error: internal_error", audit.ExecutionCategoryInternal},
		{"crawler unconfigured", -32000, "tool_error: nav crawler is not configured", audit.ExecutionCategoryInternal},
		{"short validation passthrough", -32000, "tool_error: fund not found", audit.ExecutionCategoryValidation},
		{"amount validation passthrough", -32000, "tool_error: amount must be positive", audit.ExecutionCategoryValidation},
		{"invalid params", -32602, "invalid_params: shares is required", audit.ExecutionCategoryValidation},
		{"denied", -32001, "tool_denied: scope_not_allowed", audit.ExecutionCategoryDenied},
		{"not implemented", -32601, "tool_not_implemented: get_x", audit.ExecutionCategoryNotImplemented},
		{"transport internal", -32603, "internal_error", audit.ExecutionCategoryInternal},
	}
	for _, tc := range cases {
		var err *Error
		if tc.code != 0 {
			err = &Error{Code: tc.code, Message: tc.msg}
		}
		got := executionErrorCategory(err)
		if got != tc.want {
			t.Errorf("%s: executionErrorCategory(%d, %q) = %q, want %q", tc.name, tc.code, tc.msg, got, tc.want)
		}
		if got != "" && !got.IsValid() {
			t.Errorf("%s: category %q outside closed set", tc.name, got)
		}
		if tc.msg != "" && string(got) == tc.msg {
			t.Errorf("%s: category equals raw error text", tc.name)
		}
	}
}
