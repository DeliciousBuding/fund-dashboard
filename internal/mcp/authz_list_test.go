package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
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

// TestListToolsHidesConfirmationGatedToolsWithoutAgentOps covers the deployment
// shape where FUND_AGENT_OPS_ENABLED is unset: app.go never builds the agentops
// service, httpapi passes a nil AgentOps, and claimWriteConfirmation then fails
// every gated call closed with tool_denied: confirmation_service_unavailable.
// A tool that can never succeed must not be advertised, but the operator must
// keep the maintenance tools that still distinguish it from the analyst.
func TestListToolsHidesConfirmationGatedToolsWithoutAgentOps(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()

	portfolio := portfoliosvc.NewService(db)
	admin := adminsvc.NewServiceWithDriver(db, "sqlite")
	// AgentOps deliberately omitted: a true nil interface, as in production with
	// FUND_AGENT_OPS_ENABLED unset.
	unwired, err := NewServer(ServerDeps{Portfolio: &portfolio, Admin: &admin, Role: agenttools.RoleOperator})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	names := listToolNames(t, unwired)

	for _, gated := range []string{
		"add_transaction", "import_transactions", "delete_transaction", "update_transaction",
		"add_fund", "delete_fund", "update_fund", "add_security", "adjust_position",
		"upsert_dca_plan", "disable_dca_plan", "run_dca_auto_invest", "mark_source_event",
		"check_alerts", "generate_report",
	} {
		if names[gated] {
			t.Fatalf("operator tools/list advertises confirmation-gated %q although AgentOps is unwired", gated)
		}
	}
	for _, executable := range []string{
		"get_portfolio_summary", "crawl_nav", "recalculate_snapshot",
		"crawl_fund_holdings", "get_investment_source_brief",
	} {
		if !names[executable] {
			t.Fatalf("operator tools/list lost executable tool %q", executable)
		}
	}

	wired := listToolNames(t, newMCPServerWithRole(t, db, agenttools.RoleOperator))
	if !wired["add_transaction"] {
		t.Fatal("operator tools/list lost add_transaction even though AgentOps is wired")
	}
	if len(names) >= len(wired) {
		t.Fatalf("unwired operator surface (%d) must be smaller than the wired one (%d)", len(names), len(wired))
	}
}
