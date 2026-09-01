package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/audit"
	"github.com/DeliciousBuding/fund-dashboard/internal/confirmations"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/agentstate"
)

// End-to-end: the MCP route wires the agentops service as ExecutionAuditSink,
// so a denied tools/call persists an event_type "execution" audit row with
// closed-set attribution — no schema migration required.
func TestMCPRoutePersistsExecutionAuditForDeniedTool(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	auditRepo := agentstate.NewAuditEventRepository(db)
	if err := auditRepo.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure audit schema: %v", err)
	}
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	manager, err := confirmations.NewManager([]byte("test-secret"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	agentOps := agentops.NewService(agentops.ServiceDeps{
		Registry:         registry,
		Confirmations:    manager,
		ConfirmationRepo: agentstate.NewConfirmationRepository(db),
		AuditRepo:        auditRepo,
	})

	router := NewRouter(testCfg(), WithDB(db), WithAgentOps(agentOps))

	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "e2e-exec-audit",
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "add_transaction",
			"arguments": map[string]any{"caller": "e2e-analyst"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testPublicMCPKey)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want JSON-RPC envelope", res.Code, res.Body.String())
	}
	var envelope struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Error == nil {
		t.Fatalf("expected tool denial for analyst write tool, got %s", res.Body.String())
	}

	events, err := auditRepo.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("audit List: %v", err)
	}
	var execution *audit.Event
	for i := range events {
		if events[i].EventType == "execution" {
			execution = &events[i]
			break
		}
	}
	if execution == nil {
		t.Fatalf("no execution audit row persisted; events=%d", len(events))
	}
	if execution.Tool != "add_transaction" || execution.Status != audit.StatusResult {
		t.Fatalf("execution row = %#v", execution)
	}
	if execution.Caller != "e2e-analyst" {
		t.Fatalf("Caller = %q, want the explicit caller arg", execution.Caller)
	}
	if execution.ResultSummary["kind"] != "execution" ||
		execution.ResultSummary["execution_status"] != string(audit.ExecutionErrored) {
		t.Fatalf("result summary = %#v", execution.ResultSummary)
	}
	if category, _ := execution.ResultSummary["error_category"].(string); !audit.ExecutionErrorCategory(category).IsValid() {
		t.Fatalf("error_category %q not in closed set", category)
	}
}
