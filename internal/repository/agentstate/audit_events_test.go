package agentstate

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/audit"
)

func TestAuditEventRepositoryCreatesTableAndRoundTripsEvent(t *testing.T) {
	ctx := context.Background()
	db := openAgentStateFixture(t)
	defer db.Close()

	repo := NewAuditEventRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}

	event := audit.Event{
		RequestID: "req-1",
		Caller:    "hermes",
		Tool:      "add_transaction",
		EventType: "agent_tool_attempt",
		Status:    audit.StatusAttempt,
		Scope:     string(agenttools.ScopeWrite),
		Permission: string(
			agenttools.PermissionRequiresConfirmation,
		),
		RiskLevel: "high",
		RedactedArgs: map[string]any{
			"fund_code": "AAPL",
			"api_key":   audit.RedactedValue,
		},
		CreatedAt: "2026-07-07T04:20:00Z",
	}

	id, err := repo.Save(ctx, event)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if id <= 0 {
		t.Fatalf("id = %d, want positive row id", id)
	}

	got, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got == nil {
		t.Fatalf("Get returned nil, want event")
	}
	if got.RequestID != event.RequestID ||
		got.Caller != event.Caller ||
		got.Tool != event.Tool ||
		got.EventType != event.EventType ||
		got.Status != event.Status ||
		got.Scope != event.Scope ||
		got.Permission != event.Permission ||
		got.RiskLevel != event.RiskLevel ||
		got.CreatedAt != event.CreatedAt {
		t.Fatalf("event = %#v, want %#v", got, event)
	}
	if got.RedactedArgs["api_key"] != audit.RedactedValue || got.RedactedArgs["fund_code"] != "AAPL" {
		t.Fatalf("RedactedArgs = %#v, want persisted redacted args", got.RedactedArgs)
	}
	if len(got.ResultSummary) != 0 {
		t.Fatalf("ResultSummary = %#v, want empty result summary for attempt event", got.ResultSummary)
	}
}

func TestAuditEventRepositoryPersistsResultSummary(t *testing.T) {
	ctx := context.Background()
	db := openAgentStateFixture(t)
	defer db.Close()

	repo := NewAuditEventRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}
	event := audit.Event{
		RequestID: "req-2",
		Caller:    "hermes",
		Tool:      "check_alerts",
		EventType: "agent_tool_result",
		Status:    audit.StatusResult,
		Scope:     string(agenttools.ScopeMaintenance),
		Permission: string(
			agenttools.PermissionAllowed,
		),
		RiskLevel: "medium",
		ResultSummary: map[string]any{
			"status":  "ok",
			"webhook": audit.RedactedValue,
		},
		CreatedAt: "2026-07-07T04:21:00Z",
	}

	id, err := repo.Save(ctx, event)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	got, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got == nil || got.ResultSummary["webhook"] != audit.RedactedValue || got.ResultSummary["status"] != "ok" {
		t.Fatalf("ResultSummary = %#v, want persisted result summary", got)
	}
}

func TestAuditEventRepositoryDoesNotStoreRawSensitiveArgs(t *testing.T) {
	ctx := context.Background()
	db := openAgentStateFixture(t)
	defer db.Close()

	repo := NewAuditEventRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}
	if _, err := repo.Save(ctx, audit.Event{
		RequestID: "req-3",
		Caller:    "hermes",
		Tool:      "add_transaction",
		EventType: "agent_tool_attempt",
		Status:    audit.StatusAttempt,
		Scope:     string(agenttools.ScopeWrite),
		Permission: string(
			agenttools.PermissionRequiresConfirmation,
		),
		RiskLevel: "high",
		RedactedArgs: map[string]any{
			"api_key": audit.RedactedValue,
		},
		CreatedAt: "2026-07-07T04:22:00Z",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var stored string
	if err := db.QueryRowContext(ctx, `
		SELECT redacted_args_json
		FROM agent_audit_events
		WHERE request_id = ?
	`, "req-3").Scan(&stored); err != nil {
		t.Fatalf("query redacted args: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stored), &decoded); err != nil {
		t.Fatalf("stored redacted args is not JSON: %v", err)
	}
	if decoded["api_key"] != audit.RedactedValue {
		t.Fatalf("stored args = %#v, want redacted value only", decoded)
	}
}

// TestAuditEventRepositoryEnsureSchemaCreatesToolIndex locks SQLite parity with
// the PG schema index set and proves EnsureSchema is idempotent.
func TestAuditEventRepositoryEnsureSchemaCreatesToolIndex(t *testing.T) {
	ctx := context.Background()
	db := openAgentStateFixture(t)
	defer db.Close()

	repo := NewAuditEventRepository(db)
	for i := 0; i < 2; i++ {
		if err := repo.EnsureSchema(ctx); err != nil {
			t.Fatalf("EnsureSchema run %d: %v", i, err)
		}
	}
	var n int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'index' AND name = 'idx_agent_audit_events_tool'
	`).Scan(&n); err != nil {
		t.Fatalf("query index: %v", err)
	}
	if n != 1 {
		t.Fatalf("idx_agent_audit_events_tool present=%d, want 1", n)
	}
}

func TestAuditEventRepositoryGetMissingReturnsNil(t *testing.T) {
	ctx := context.Background()
	db := openAgentStateFixture(t)
	defer db.Close()

	repo := NewAuditEventRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}
	got, err := repo.Get(ctx, 404)
	if err != nil {
		t.Fatalf("Get missing returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Get missing = %#v, want nil", got)
	}
}
