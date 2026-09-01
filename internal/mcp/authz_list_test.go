package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
)

func listToolNames(t *testing.T, server *Server) map[string]bool {
	t.Helper()
	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"tools"`),
		Method:  "tools/list",
	})
	if resp.Error != nil {
		t.Fatalf("tools/list response = %#v", resp)
	}
	decoded := decodeResult(t, resp)
	rawTools, ok := decoded["tools"].([]any)
	if !ok {
		t.Fatalf("tools = %#v, want array", decoded["tools"])
	}
	names := make(map[string]bool, len(rawTools))
	for _, raw := range rawTools {
		tool, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("tool entry = %#v, want object", raw)
		}
		name, ok := tool["name"].(string)
		if !ok || name == "" {
			t.Fatalf("tool name = %#v, want non-empty string", tool["name"])
		}
		names[name] = true
	}
	return names
}

func TestListToolsFiltersByRoleAndConfirmation(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()

	analyst := listToolNames(t, newMCPServer(t, db))
	if !analyst["get_portfolio_summary"] {
		t.Fatal("analyst tools/list missing read tool get_portfolio_summary")
	}
	if analyst["crawl_nav"] {
		t.Fatal("analyst tools/list advertises maintenance tool crawl_nav")
	}
	if analyst["add_transaction"] {
		t.Fatal("analyst tools/list advertises write tool add_transaction")
	}
	if analyst["check_alerts"] {
		t.Fatal("analyst tools/list advertises confirmation-gated check_alerts")
	}

	operator := listToolNames(t, newMCPServerWithRole(t, db, agenttools.RoleOperator))
	if !operator["get_portfolio_summary"] || !operator["crawl_nav"] || !operator["add_transaction"] {
		t.Fatalf("operator tools/list missing tools: %v", operator)
	}
}
