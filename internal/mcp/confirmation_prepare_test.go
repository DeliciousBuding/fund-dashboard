package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/confirmations"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/agentstate"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

// newOperatorServerWithFullAgentOps builds an operator MCP server whose
// confirmation prepare + claim halves are both wired to a real AgentOps service,
// matching the production httpapi wiring for an OAuth operator token.
func newOperatorServerWithFullAgentOps(t *testing.T, db *sql.DB) *Server {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 3, 10, 0, 0, time.UTC)
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	confirmationRepo := agentstate.NewConfirmationRepository(db)
	if err := confirmationRepo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure confirmation schema: %v", err)
	}
	auditRepo := agentstate.NewAuditEventRepository(db)
	if err := auditRepo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure audit schema: %v", err)
	}
	manager, err := confirmations.NewManager([]byte("test-secret"), confirmations.WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	ops := agentops.NewService(agentops.ServiceDeps{
		Registry:         registry,
		Confirmations:    manager,
		ConfirmationRepo: confirmationRepo,
		AuditRepo:        auditRepo,
		Clock:            func() time.Time { return now },
	})
	portfolio := portfoliosvc.NewService(db)
	admin := adminsvc.NewServiceWithDriver(db, "sqlite")
	server, err := NewServer(ServerDeps{
		Registry:         registry,
		Portfolio:        &portfolio,
		Admin:            &admin,
		AgentOps:         ops,
		ConfirmationPrep: ops,
		Role:             agenttools.RoleOperator,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return server
}

func TestPrepareConfirmationToolMintsCredentialThenWriteSucceeds(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newOperatorServerWithFullAgentOps(t, db)

	// The tool is advertised for an operator whose confirmation flow is wired.
	listed := listToolNames(t, server)
	if !listed["prepare_confirmation"] {
		t.Fatalf("prepare_confirmation not advertised for wired operator: %v", listed)
	}

	payload := map[string]any{
		"order_id":       "MCP-PREP-FLOW",
		"fund_code":      "aapl",
		"trade_time":     "2026-06-03T09:00:00Z",
		"confirm_date":   "2026-06-05",
		"trade_type":     "用户买入",
		"direction":      "buy",
		"confirm_amount": 198.25,
		"confirm_share":  1.0,
		"fee":            0.0,
	}
	prepParams, err := json.Marshal(map[string]any{
		"name": "prepare_confirmation",
		"arguments": map[string]any{
			"tool":    "add_transaction",
			"caller":  "mcp-prep-test",
			"payload": payload,
		},
	})
	if err != nil {
		t.Fatalf("marshal prep params: %v", err)
	}
	prepResp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0", ID: json.RawMessage(`"prep"`), Method: "tools/call", Params: prepParams,
	})
	if prepResp.Error != nil {
		t.Fatalf("prepare_confirmation error = %#v", prepResp.Error)
	}
	text := firstTextContent(t, prepResp)
	if !strings.Contains(text, `"confirmation_id"`) || !strings.Contains(text, `"token"`) {
		t.Fatalf("prepare result missing credential fields: %s", text)
	}
	payloadObj := decodeTextPayload(t, text)
	confirmationID, ok1 := payloadObj["confirmation_id"].(float64)
	confirmationToken, ok2 := payloadObj["token"].(string)
	if !ok1 || !ok2 || confirmationID <= 0 || confirmationToken == "" {
		t.Fatalf("prepare result malformed: %#v", payloadObj)
	}

	// Second half: the write tool claims the credential and executes.
	writeArgs := map[string]any{
		"confirmation_id":    int(confirmationID),
		"confirmation_token": confirmationToken,
	}
	for key, value := range payload {
		writeArgs[key] = value
	}
	writeParams, err := json.Marshal(map[string]any{"name": "add_transaction", "arguments": writeArgs})
	if err != nil {
		t.Fatalf("marshal write params: %v", err)
	}
	writeResp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0", ID: json.RawMessage(`"write"`), Method: "tools/call", Params: writeParams,
	})
	if writeResp.Error != nil {
		t.Fatalf("write after prepare error = %#v", writeResp.Error)
	}
	if !strings.Contains(firstTextContent(t, writeResp), `"side_effects"`) {
		t.Fatalf("write result missing side_effects: %s", firstTextContent(t, writeResp))
	}
}
