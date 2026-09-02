package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	db "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

func TestConsumeConfirmationUnavailableNoPanic(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	// Explicitly nil AgentOps (true interface nil).
	portfolio := portfoliosvc.NewService(db)
	admin := adminsvc.NewServiceWithDriver(db, "sqlite")
	server, err := NewServer(ServerDeps{Portfolio: &portfolio, Admin: &admin, AgentOps: nil, Role: agenttools.RoleOperator})
	if err != nil {
		t.Fatal(err)
	}
	// Missing confirmation params.
	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"check_alerts","arguments":{}}`),
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "confirmation") {
		t.Fatalf("missing conf error = %#v", resp.Error)
	}
	// Fake confirmation must not panic when AgentOps is unavailable.
	resp = server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"check_alerts","arguments":{"confirmation_id":1,"confirmation_token":"fake"}}`),
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "confirmation_service_unavailable") {
		t.Fatalf("fake conf error = %#v, want confirmation_service_unavailable", resp.Error)
	}
}

func TestServerListsRegistryToolsInMCPShape(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	// Operator sees full implemented surface; analyst is role-filtered (see TestServerListsToolsRoleFiltered).
	server := newMCPServerWithRole(t, db, agenttools.RoleOperator)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	})

	if response.Error != nil {
		t.Fatalf("tools/list error = %#v", response.Error)
	}
	result := decodeResult(t, response)
	tools := result["tools"].([]any)
	// tools/list only advertises implemented tools (not the full 44 registry matrix).
	if len(tools) < 20 || len(tools) > 44 {
		t.Fatalf("tools length = %d, want implemented subset (20-44 full)", len(tools))
	}
	body := toJSONString(t, result)
	if !strings.Contains(body, `"name":"get_portfolio_summary"`) {
		t.Fatalf("tools/list missing get_portfolio_summary: %s", body)
	}
	if !strings.Contains(body, `"name":"crawl_nav"`) {
		t.Fatalf("tools/list missing crawl_nav: %s", body)
	}
	if !strings.Contains(body, `"name":"recalculate_snapshot"`) {
		t.Fatalf("tools/list missing recalculate_snapshot: %s", body)
	}
	if !strings.Contains(body, `"name":"crawl_fund_holdings"`) {
		t.Fatalf("tools/list missing crawl_fund_holdings: %s", body)
	}
	if !strings.Contains(body, `"name":"upsert_dca_plan"`) {
		t.Fatalf("tools/list missing upsert_dca_plan: %s", body)
	}
	if !strings.Contains(body, `"name":"disable_dca_plan"`) {
		t.Fatalf("tools/list missing disable_dca_plan: %s", body)
	}
	if !strings.Contains(body, `"name":"add_fund"`) {
		t.Fatalf("tools/list missing add_fund: %s", body)
	}
	if !strings.Contains(body, `"name":"add_security"`) {
		t.Fatalf("tools/list missing add_security: %s", body)
	}
	if !strings.Contains(body, `"name":"update_fund"`) {
		t.Fatalf("tools/list missing update_fund: %s", body)
	}
	if !strings.Contains(body, `"name":"delete_fund"`) {
		t.Fatalf("tools/list missing delete_fund: %s", body)
	}
	if !strings.Contains(body, `"name":"adjust_position"`) {
		t.Fatalf("tools/list missing adjust_position: %s", body)
	}
	if !strings.Contains(body, `"name":"check_alerts"`) {
		t.Fatalf("tools/list missing check_alerts: %s", body)
	}
	if !strings.Contains(body, `"name":"run_dca_auto_invest"`) {
		t.Fatalf("tools/list missing run_dca_auto_invest: %s", body)
	}
	if !strings.Contains(body, `"name":"generate_report"`) {
		t.Fatalf("tools/list missing generate_report: %s", body)
	}
	if !strings.Contains(body, `"name":"mark_source_event"`) {
		t.Fatalf("tools/list missing mark_source_event: %s", body)
	}
	if !strings.Contains(body, `"readOnlyHint":true`) {
		t.Fatalf("tools/list missing MCP camelCase annotations: %s", body)
	}
	// Full registry parity: no registry-only phantoms expected at 44.
	if len(tools) < 44 {
		t.Fatalf("tools length = %d, want >= 44 full registry", len(tools))
	}
}

func TestServerCallsReadOnlyPortfolioSummaryTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"call-1"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_portfolio_summary","arguments":{"portfolio_id":1}}`),
	})

	if response.Error != nil {
		t.Fatalf("tools/call error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	summary := payload["summary"].(map[string]any)
	if summary["total_transactions"].(float64) != 2 {
		t.Fatalf("summary total_transactions = %v, want 2; text=%s", summary["total_transactions"], text)
	}
	if !strings.Contains(text, `"holdings"`) || !strings.Contains(text, `"AAPL"`) {
		t.Fatalf("summary text missing holdings facts: %s", text)
	}
	if strings.Contains(text, "建议买入") || strings.Contains(text, "加仓") {
		t.Fatalf("summary text contains advice language: %s", text)
	}
}

func TestServerCallsCorePortfolioReadTools(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	cases := []struct {
		name     string
		params   string
		contains []string
	}{
		{
			name:     "get_portfolio_allocation",
			params:   `{"name":"get_portfolio_allocation","arguments":{"portfolio_id":1}}`,
			contains: []string{`"total_value"`, `"risk_flags"`, `"by_security_type"`},
		},
		{
			name:     "get_investment_source_brief",
			params:   `{"name":"get_investment_source_brief","arguments":{"portfolio_id":1,"limit":3}}`,
			contains: []string{`"decision_boundary": "source_queries_only"`, `"queries"`, `"source_targets"`},
		},
	}

	for _, tc := range cases {
		response := server.Handle(context.Background(), Request{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"read-tool"`),
			Method:  "tools/call",
			Params:  json.RawMessage(tc.params),
		})
		if response.Error != nil {
			t.Fatalf("%s error = %#v", tc.name, response.Error)
		}
		text := firstTextContent(t, response)
		for _, want := range tc.contains {
			if !strings.Contains(text, want) {
				t.Fatalf("%s text missing %s: %s", tc.name, want, text)
			}
		}
	}
}

func TestServerCallsListPortfoliosTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"list-portfolios"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_portfolios","arguments":{}}`),
	})

	if response.Error != nil {
		t.Fatalf("list_portfolios error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	var portfolios []map[string]any
	if err := json.Unmarshal([]byte(text), &portfolios); err != nil {
		t.Fatalf("decode list_portfolios payload: %v; text=%s", err, text)
	}
	if len(portfolios) != 1 {
		t.Fatalf("portfolios length = %d, want 1; text=%s", len(portfolios), text)
	}
	if portfolios[0]["id"].(float64) != 1 ||
		portfolios[0]["name"] != "default" ||
		portfolios[0]["description"] != "Default portfolio" {
		t.Fatalf("portfolio row = %#v, want default portfolio", portfolios[0])
	}
	if strings.Contains(text, "建议买入") || strings.Contains(text, "加仓") || strings.Contains(text, "backup") {
		t.Fatalf("list_portfolios should contain only read-only portfolio facts: %s", text)
	}
}

func TestServerCallsListDCAPlansTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"list-dca-plans"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"list_dca_plans","arguments":{"portfolio_id":1,"active_only":true}}`),
	})

	if response.Error != nil {
		t.Fatalf("list_dca_plans error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["count"].(float64) != 1 ||
		payload["decision_boundary"] != "facts_only" ||
		payload["side_effects"] != "none" {
		t.Fatalf("list_dca_plans envelope wrong: %s", text)
	}
	plans := payload["plans"].([]any)
	if len(plans) != 1 {
		t.Fatalf("plans length = %d, want one active plan; text=%s", len(plans), text)
	}
	plan := plans[0].(map[string]any)
	if plan["id"].(float64) != 1 ||
		plan["fund_code"] != "019173" ||
		plan["amount"].(float64) != 25 ||
		plan["weekday_mask"] != "1,3,5" ||
		plan["active"].(float64) != 1 {
		t.Fatalf("plan = %#v, want fixture DCA rule facts; text=%s", plan, text)
	}
	if strings.Contains(text, "run_dca_auto_invest") ||
		strings.Contains(text, "backup") ||
		strings.Contains(text, "建议买入") ||
		strings.Contains(text, "加仓") {
		t.Fatalf("list_dca_plans should be facts-only plan rules: %s", text)
	}
}

func TestServerCallsPortfolioTimelineTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"portfolio-timeline"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_portfolio_timeline","arguments":{"portfolio_id":1}}`),
	})

	if response.Error != nil {
		t.Fatalf("get_portfolio_timeline error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["count"].(float64) != 3 {
		t.Fatalf("count = %v, want 3; text=%s", payload["count"], text)
	}
	if payload["first"] != "2026-06-18" || payload["last"] != "2026-06-20" {
		t.Fatalf("first/last wrong: %s", text)
	}
	data := payload["data"].([]any)
	first := data[0].(map[string]any)
	if first["date"] != "2026-06-18" ||
		first["total_value"].(float64) != 530 ||
		first["total_cost"].(float64) != 420 ||
		first["pnl"].(float64) != 110 {
		t.Fatalf("first timeline point wrong: %#v; text=%s", first, text)
	}
	if payload["decision_boundary"] != "facts_only" ||
		strings.Contains(text, "crawl_nav") ||
		strings.Contains(text, "建议买入") ||
		strings.Contains(text, "加仓") {
		t.Fatalf("timeline should be facts-only with no refresh/advice language: %s", text)
	}
}

func TestServerCallsPortfolioPenetrationTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"portfolio-penetration"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_portfolio_penetration","arguments":{"portfolio_id":1,"limit":5,"sort_by":"market_value"}}`),
	})

	if response.Error != nil {
		t.Fatalf("get_portfolio_penetration error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["decision_boundary"] != "facts_only" {
		t.Fatalf("decision_boundary = %v, want facts_only; text=%s", payload["decision_boundary"], text)
	}
	if payload["total_portfolio_value_cny"].(float64) != 150 {
		t.Fatalf("total_portfolio_value_cny = %v, want 150; text=%s", payload["total_portfolio_value_cny"], text)
	}
	if payload["stocks_found"].(float64) != 1 || payload["funds_with_holdings"].(float64) != 1 {
		t.Fatalf("coverage counts wrong: %s", text)
	}
	penetration := payload["penetration"].([]any)
	if len(penetration) != 1 {
		t.Fatalf("penetration length = %d, want 1; text=%s", len(penetration), text)
	}
	nvda := penetration[0].(map[string]any)
	if nvda["stock_code"] != "NVDA" ||
		nvda["sector"] != "Semiconductors" ||
		nvda["estimated_market_value_cny"].(float64) != 12.75 ||
		nvda["penetration_pct"].(float64) != 8.5 {
		t.Fatalf("NVDA penetration wrong: %#v; text=%s", nvda, text)
	}
	sectors := payload["by_sector"].([]any)
	if len(sectors) != 1 {
		t.Fatalf("by_sector length = %d, want 1; text=%s", len(sectors), text)
	}
	sector := sectors[0].(map[string]any)
	if sector["sector"] != "Semiconductors" ||
		sector["total_exposure_cny"].(float64) != 12.75 ||
		sector["penetration_pct"].(float64) != 8.5 ||
		sector["stock_count"].(float64) != 1 {
		t.Fatalf("sector aggregation wrong: %#v; text=%s", sector, text)
	}
	unavailable := payload["unavailable_funds"].([]any)
	if len(unavailable) != 0 {
		t.Fatalf("unavailable_funds = %#v, want empty; text=%s", unavailable, text)
	}
	if strings.Contains(text, "crawl_fund_holdings") ||
		strings.Contains(text, "backup") ||
		strings.Contains(text, "建议买入") ||
		strings.Contains(text, "加仓") {
		t.Fatalf("penetration should be facts-only with no refresh/advice language: %s", text)
	}
}

func TestServerCallsCompareFundsTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
			VALUES ('CMP1', 'Compare One', 'test', 'fund', 'US');
		INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
			VALUES
			('CMP1-BUY', '2025-06-01T00:00:00Z', '2025-06-02', '用户买入', 'buy', 'CMP1', 'Compare One', 100, 100, 0, -100, 100, 1),
			('CMP1-DIV', '2026-06-01T00:00:00Z', '2026-06-02', '现金分红', 'dividend', 'CMP1', 'Compare One', 10, 0, 0, 10, 0, 1);
		INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type)
			VALUES
			('CMP1', '2026-06-01', 1.00, 0, 'fund'),
			('CMP1', '2026-06-02', 1.10, 0, 'fund'),
			('CMP1', '2026-06-03', 1.20, 0, 'fund'),
			('CMP1', '2026-06-04', 1.00, 0, 'fund'),
			('CMP1', '2026-06-05', 1.30, 0, 'fund'),
			('CMP1', '2026-06-06', 1.40, 0, 'fund'),
			('CMP1', '2026-06-07', 1.50, 0, 'fund'),
			('CMP1', '2026-06-08', 1.60, 0, 'fund'),
			('CMP1', '2026-06-09', 1.70, 0, 'fund'),
			('CMP1', '2026-06-10', 1.80, 0, 'fund');
	`); err != nil {
		t.Fatalf("seed compare fixture: %v", err)
	}
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"compare-funds"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"compare_funds","arguments":{"codes":["cmp1","aapl"]}}`),
	})

	if response.Error != nil {
		t.Fatalf("compare_funds error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["decision_boundary"] != "facts_only" || payload["side_effects"] != "none" {
		t.Fatalf("boundary fields wrong: %s", text)
	}
	funds := payload["funds"].([]any)
	if len(funds) != 2 {
		t.Fatalf("funds length = %d, want 2; text=%s", len(funds), text)
	}
	cmp := funds[0].(map[string]any)
	if cmp["code"] != "CMP1" ||
		cmp["name"] != "Compare One" ||
		cmp["xirr"].(float64) != 90 ||
		cmp["volatility"].(float64) != 178.99 ||
		cmp["max_drawdown"].(float64) != 16.67 ||
		cmp["sharpe"].(float64) != 0.5028 ||
		cmp["calmar"].(float64) != 5.3989 {
		t.Fatalf("CMP1 comparison wrong: %#v; text=%s", cmp, text)
	}
	aapl := funds[1].(map[string]any)
	if aapl["code"] != "AAPL" || aapl["market"] != "US" {
		t.Fatalf("AAPL identity wrong or zero-padded: %#v; text=%s", aapl, text)
	}
	if strings.Contains(text, "crawl_nav") ||
		strings.Contains(text, "建议买入") ||
		strings.Contains(text, "加仓") ||
		strings.Contains(text, "recommendation") {
		t.Fatalf("compare_funds should be facts-only with no refresh/advice language: %s", text)
	}
}

func TestServerCallsComputeDCAAmountTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"compute-dca"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"compute_dca_amount","arguments":{"fund_code":"aapl","base_amount":100,"mode":"change_pct"}}`),
	})

	if response.Error != nil {
		t.Fatalf("compute_dca_amount error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["fund_code"] != "AAPL" ||
		payload["security_type"] != "stock" ||
		payload["market"] != "US" ||
		payload["mode"] != "change_pct" ||
		payload["base_amount"].(float64) != 100 ||
		payload["latest_nav"].(float64) != 190 ||
		payload["change_pct"].(float64) != 6.5 ||
		payload["dca_rate"].(float64) != 0.5 ||
		payload["actual_amount"].(float64) != 50 ||
		payload["signal"] != "rally_control" ||
		payload["range"] != "rally_control" ||
		payload["decision_boundary"] != "facts_only" ||
		payload["side_effects"] != "none" {
		t.Fatalf("compute_dca_amount payload wrong: %s", text)
	}
	if strings.Contains(text, "0AAPL") ||
		strings.Contains(text, "broker") ||
		strings.Contains(text, "backup") ||
		strings.Contains(text, "建议买入") {
		t.Fatalf("compute_dca_amount should be facts-only simulation: %s", text)
	}
}

func TestServerCallsRunBacktestTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
			VALUES ('BT1', 'Backtest Fund', 'test', 'fund', 'CN');
		INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type)
			VALUES
			('BT1', '2026-01-01', 1.0, 0, 'fund'),
			('BT1', '2026-01-15', 1.2, 0, 'fund'),
			('BT1', '2026-02-01', 2.0, 0, 'fund'),
			('BT1', '2026-03-01', 1.5, 0, 'fund');
	`); err != nil {
		t.Fatalf("seed backtest navs: %v", err)
	}
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"run-backtest"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"run_backtest","arguments":{"fund_code":"bt1","strategy":"dca","start_date":"2026-01-01","base_amount":100}}`),
	})

	if response.Error != nil {
		t.Fatalf("run_backtest error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["fund_code"] != "BT1" ||
		payload["strategy"] != "dca" ||
		payload["total_invested"].(float64) != 300 ||
		payload["final_value"].(float64) != 25 ||
		payload["decision_boundary"] != "facts_only" ||
		payload["side_effects"] != "none" {
		t.Fatalf("run_backtest payload wrong: %s", text)
	}
	trades := payload["trades"].([]any)
	timeline := payload["timeline"].([]any)
	if len(trades) != 3 || len(timeline) != 4 {
		t.Fatalf("trades/timeline length = %d/%d, want 3/4; text=%s", len(trades), len(timeline), text)
	}
	if strings.Contains(text, "broker") ||
		strings.Contains(text, "backup") ||
		strings.Contains(text, "建议买入") ||
		strings.Contains(text, "run_dca_auto_invest") {
		t.Fatalf("run_backtest should be facts-only simulation: %s", text)
	}
}

func TestServerCompareFundsRejectsBlankCodes(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"compare-funds-blank"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"compare_funds","arguments":{"codes":[" ","\t"]}}`),
	})

	if response.Error != nil {
		t.Fatalf("compare_funds error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["error"] != "codes required" {
		t.Fatalf("blank compare_funds response = %s, want codes required error", text)
	}
}

func TestServerCallsHarnessSourceEventsAndFullDashboard(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	cases := []struct {
		name     string
		params   string
		contains []string
	}{
		{
			name:     "get_investment_harness_snapshot",
			params:   `{"name":"get_investment_harness_snapshot","arguments":{"portfolio_id":1}}`,
			contains: []string{`"decision_boundary": "facts_only"`, `"agent_permissions"`, `"backup_producer"`},
		},
		{
			name:     "get_source_events",
			params:   `{"name":"get_source_events","arguments":{"limit":5,"show_read":true}}`,
			contains: []string{`"decision_boundary": "facts_only"`, `"count": 1`, `"is_read": false`, `"AAPL market update"`},
		},
		{
			name:     "get_full_dashboard",
			params:   `{"name":"get_full_dashboard","arguments":{"portfolio_id":1,"source_limit":3,"event_limit":5}}`,
			contains: []string{`"summary"`, `"harness_snapshot"`, `"allocation"`, `"source_brief"`, `"source_events"`, `"agent_context"`},
		},
	}

	for _, tc := range cases {
		response := server.Handle(context.Background(), Request{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"agent-pack"`),
			Method:  "tools/call",
			Params:  json.RawMessage(tc.params),
		})
		if response.Error != nil {
			t.Fatalf("%s error = %#v", tc.name, response.Error)
		}
		text := firstTextContent(t, response)
		for _, want := range tc.contains {
			if !strings.Contains(text, want) {
				t.Fatalf("%s text missing %s: %s", tc.name, want, text)
			}
		}
	}
}

func TestServerCallsDataFreshnessTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"freshness"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_data_freshness","arguments":{}}`),
	})

	if response.Error != nil {
		t.Fatalf("get_data_freshness error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["decision_boundary"] != "read_only" {
		t.Fatalf("decision_boundary = %v, want read_only; text=%s", payload["decision_boundary"], text)
	}
	if payload["health"] == "" {
		t.Fatalf("health empty: %s", text)
	}
	if !strings.Contains(text, `"missing_nav_securities"`) ||
		!strings.Contains(text, `"watchlist_missing_nav_securities"`) ||
		!strings.Contains(text, `"stale_nav_securities"`) ||
		!strings.Contains(text, `"stale_detail"`) {
		t.Fatalf("freshness text missing agent-facing sections: %s", text)
	}
	if strings.Contains(text, "broker") || strings.Contains(text, "backup_producer") || strings.Contains(text, "建议买入") {
		t.Fatalf("freshness text contains forbidden execution/advice language: %s", text)
	}
	// #74: only force crawl_nav when stale/missing held NAV exists.
	requires, _ := payload["recommended_maintenance_requires_run"].(bool)
	stale, _ := payload["stale_detail"].([]any)
	missing, _ := payload["missing_nav_securities"].([]any)
	need := len(stale) > 0 || len(missing) > 0
	if requires != need {
		t.Fatalf("recommended_maintenance_requires_run=%v want %v (stale=%d missing=%d): %s", requires, need, len(stale), len(missing), text)
	}
	if need {
		if payload["recommended_maintenance_tool"] != "crawl_nav" {
			t.Fatalf("tool=%v want crawl_nav when stale/missing: %s", payload["recommended_maintenance_tool"], text)
		}
		args, _ := payload["recommended_maintenance_args"].(map[string]any)
		if args == nil || args["stale_only"] != true {
			t.Fatalf("recommended_maintenance_args=%v want {stale_only:true}: %s", payload["recommended_maintenance_args"], text)
		}
		codes, _ := payload["recommended_codes"].([]any)
		if len(codes) == 0 {
			t.Fatalf("recommended_codes empty when need crawl: %s", text)
		}
	} else if payload["recommended_maintenance_tool"] != nil {
		t.Fatalf("tool=%v want null when healthy: %s", payload["recommended_maintenance_tool"], text)
	}
}

func TestServerCallsVerifyDataTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"verify"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"verify_data","arguments":{}}`),
	})

	if response.Error != nil {
		t.Fatalf("verify_data error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["decision_boundary"] != "facts_only" {
		t.Fatalf("decision_boundary = %v, want facts_only; text=%s", payload["decision_boundary"], text)
	}
	if payload["healthy"] != true {
		t.Fatalf("healthy = %v, want true for fixture; text=%s", payload["healthy"], text)
	}
	if !strings.Contains(text, `"issues"`) ||
		!strings.Contains(text, `"all clear"`) ||
		!strings.Contains(text, `"securities_without_nav"`) ||
		!strings.Contains(text, `"negative_positions"`) ||
		!strings.Contains(text, `"missing_settlement_count"`) {
		t.Fatalf("verify_data text missing data-quality sections: %s", text)
	}
	if strings.Contains(text, "db-repair") || strings.Contains(text, "db-restore") || strings.Contains(text, "backup") || strings.Contains(text, "trade") {
		t.Fatalf("verify_data text contains forbidden action language: %s", text)
	}
}

func TestServerCallsFundStatusToolAndPreservesTicker(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"fund-status"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_fund_status","arguments":{"code":"aapl"}}`),
	})

	if response.Error != nil {
		t.Fatalf("get_fund_status error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["code"] != "AAPL" {
		t.Fatalf("code = %v, want AAPL; text=%s", payload["code"], text)
	}
	if payload["name"] != "Apple Inc." || payload["security_type"] != "stock" || payload["market"] != "US" {
		t.Fatalf("identity fields wrong: %s", text)
	}
	if !strings.Contains(text, `"transactions"`) ||
		!strings.Contains(text, `"nav"`) ||
		!strings.Contains(text, `"position"`) ||
		!strings.Contains(text, `"trading"`) ||
		!strings.Contains(text, `"decision_boundary": "facts_only"`) {
		t.Fatalf("fund status text missing agent-facing sections: %s", text)
	}
	position := payload["position"].(map[string]any)
	if position["shares"].(float64) != 2 || position["value"].(float64) != 380 {
		t.Fatalf("position = %#v, want fixture shares/value; text=%s", position, text)
	}
	if strings.Contains(text, "0AAPL") || strings.Contains(text, "建议买入") || strings.Contains(text, "加仓") {
		t.Fatalf("fund status text contains padding or advice language: %s", text)
	}
}

func TestServerCallsSystemStatusTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"system-status"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_system_status","arguments":{}}`),
	})

	if response.Error != nil {
		t.Fatalf("get_system_status error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["ok"] != true {
		t.Fatalf("ok = %v, want true; text=%s", payload["ok"], text)
	}
	if payload["decision_boundary"] != "read_only" {
		t.Fatalf("decision_boundary = %v, want read_only; text=%s", payload["decision_boundary"], text)
	}
	if payload["side_effects"] != "none" {
		t.Fatalf("side_effects = %v, want none; text=%s", payload["side_effects"], text)
	}
	if payload["uptime_sec"].(float64) < 0 {
		t.Fatalf("uptime_sec = %v, want non-negative; text=%s", payload["uptime_sec"], text)
	}
	if payload["server_time"] == "" {
		t.Fatalf("server_time empty; text=%s", text)
	}
	if !strings.Contains(text, `"transactions"`) ||
		!strings.Contains(text, `"nav"`) ||
		!strings.Contains(text, `"portfolio"`) ||
		!strings.Contains(text, `"securities"`) ||
		!strings.Contains(text, `"anomalies"`) ||
		!strings.Contains(text, `"market_schedule"`) {
		t.Fatalf("system status text missing agent-facing sections: %s", text)
	}
	if strings.Contains(text, "backup_producer") || strings.Contains(text, "broker") || strings.Contains(text, "建议买入") || strings.Contains(text, "加仓") {
		t.Fatalf("system status text contains forbidden execution/advice language: %s", text)
	}
}

func TestServerCallsFundDetailAndNavHistoryTools(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	detailResponse := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"fund-detail"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_fund_detail","arguments":{"code":"aapl"}}`),
	})

	if detailResponse.Error != nil {
		t.Fatalf("get_fund_detail error = %#v", detailResponse.Error)
	}
	detailText := firstTextContent(t, detailResponse)
	detail := decodeTextPayload(t, detailText)
	if detail["code"] != "AAPL" || detail["name"] != "Apple Inc." {
		t.Fatalf("detail identity wrong: %s", detailText)
	}
	if detail["security_type"] != "stock" || detail["market"] != "US" {
		t.Fatalf("detail type/market wrong: %s", detailText)
	}
	position := detail["position"].(map[string]any)
	if position["shares"].(float64) != 2 || position["market_value"].(float64) != 380 {
		t.Fatalf("detail position = %#v, want shares=2 market_value=380; text=%s", position, detailText)
	}
	if detail["transaction_count"].(float64) != 1 {
		t.Fatalf("transaction_count = %v, want 1; text=%s", detail["transaction_count"], detailText)
	}
	if !strings.Contains(detailText, `"transactions"`) ||
		!strings.Contains(detailText, `"decision_boundary": "facts_only"`) {
		t.Fatalf("detail missing agent-facing facts-only sections: %s", detailText)
	}
	if strings.Contains(detailText, "0AAPL") || strings.Contains(detailText, "建议买入") || strings.Contains(detailText, "加仓") {
		t.Fatalf("detail text contains padding or advice language: %s", detailText)
	}

	navResponse := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"nav-history"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_nav_history","arguments":{"code":"aapl","limit":1}}`),
	})

	if navResponse.Error != nil {
		t.Fatalf("get_nav_history error = %#v", navResponse.Error)
	}
	navText := firstTextContent(t, navResponse)
	nav := decodeTextPayload(t, navText)
	if nav["code"] != "AAPL" || nav["security_type"] != "stock" || nav["market"] != "US" {
		t.Fatalf("nav identity wrong: %s", navText)
	}
	data := nav["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("nav data length = %d, want 1; text=%s", len(data), navText)
	}
	first := data[0].(map[string]any)
	if first["date"] != "2026-06-18" || first["unit_nav"].(float64) != 190 || first["daily_change_pct"].(float64) != 6.5 {
		t.Fatalf("nav point = %#v, want fixture values; text=%s", first, navText)
	}
	if nav["decision_boundary"] != "facts_only" || strings.Contains(navText, "crawl_nav") || strings.Contains(navText, "建议买入") {
		t.Fatalf("nav history should be facts-only with no refresh/advice language: %s", navText)
	}
}

func TestServerCallsSearchFundsAndDrawdownTools(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	searchResponse := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"search"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"search_funds","arguments":{"query":"apple"}}`),
	})

	if searchResponse.Error != nil {
		t.Fatalf("search_funds error = %#v", searchResponse.Error)
	}
	searchText := firstTextContent(t, searchResponse)
	var searchRows []map[string]any
	if err := json.Unmarshal([]byte(searchText), &searchRows); err != nil {
		t.Fatalf("decode search rows: %v; text=%s", err, searchText)
	}
	if len(searchRows) != 1 {
		t.Fatalf("search rows length = %d, want 1; text=%s", len(searchRows), searchText)
	}
	if searchRows[0]["code"] != "AAPL" || searchRows[0]["security_type"] != "stock" || searchRows[0]["market"] != "US" {
		t.Fatalf("search row identity wrong: %#v", searchRows[0])
	}
	if searchRows[0]["held_shares"].(float64) != 2 || searchRows[0]["current_value"].(float64) != 380 {
		t.Fatalf("search row portfolio facts wrong: %#v", searchRows[0])
	}
	if strings.Contains(searchText, "建议买入") || strings.Contains(searchText, "加仓") {
		t.Fatalf("search text contains advice language: %s", searchText)
	}

	drawdownResponse := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"drawdown"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_fund_drawdown","arguments":{"code":"19173"}}`),
	})

	if drawdownResponse.Error != nil {
		t.Fatalf("get_fund_drawdown error = %#v", drawdownResponse.Error)
	}
	drawdownText := firstTextContent(t, drawdownResponse)
	drawdown := decodeTextPayload(t, drawdownText)
	if drawdown["code"] != "019173" || drawdown["security_type"] != "fund" || drawdown["market"] != "CN" {
		t.Fatalf("drawdown identity wrong: %s", drawdownText)
	}
	if drawdown["max_drawdown_pct"].(float64) != 20 {
		t.Fatalf("max_drawdown_pct = %v, want 20; text=%s", drawdown["max_drawdown_pct"], drawdownText)
	}
	if drawdown["peak_date"] != "2026-06-18" || drawdown["trough_date"] != "2026-06-19" {
		t.Fatalf("drawdown dates wrong: %s", drawdownText)
	}
	if drawdown["decision_boundary"] != "facts_only" ||
		strings.Contains(drawdownText, "crawl_nav") ||
		strings.Contains(drawdownText, "建议买入") ||
		strings.Contains(drawdownText, "加仓") {
		t.Fatalf("drawdown should be facts-only with no refresh/advice language: %s", drawdownText)
	}
}

func TestServerCallsSearchStocksTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"search-stocks"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"search_stocks","arguments":{"query":"t","market":"all","limit":5}}`),
	})

	if response.Error != nil {
		t.Fatalf("search_stocks error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["query"] != "t" ||
		payload["market_filter"] != "all" ||
		payload["count"].(float64) != 2 ||
		payload["decision_boundary"] != "facts_only" ||
		payload["side_effects"] != "none" ||
		payload["external_fetch"] != "not_performed" {
		t.Fatalf("search_stocks payload wrong: %s", text)
	}
	results := payload["results"].([]any)
	first := results[0].(map[string]any)
	second := results[1].(map[string]any)
	if first["code"] != "00700" || first["market"] != "HK" || first["source"] != "local_profile" {
		t.Fatalf("first result = %#v, want Tencent local profile", first)
	}
	if second["code"] != "MSFT" || second["market"] != "US" || second["sector"] != "Technology" {
		t.Fatalf("second result = %#v, want MSFT local profile", second)
	}
	if strings.Contains(text, "broker") ||
		strings.Contains(text, "backup") ||
		strings.Contains(text, "建议买入") ||
		strings.Contains(text, "external_fetch_performed") {
		t.Fatalf("search_stocks should stay facts-only and local-read-only: %s", text)
	}
}

func TestServerCallsGetUSStockTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"us-stock"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_us_stock","arguments":{"symbol":"aapl","range":"1y","include_history":true}}`),
	})

	if response.Error != nil {
		t.Fatalf("get_us_stock error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["symbol"] != "AAPL" ||
		payload["decision_boundary"] != "facts_only" ||
		payload["side_effects"] != "none" ||
		payload["external_fetch"] != "not_performed" {
		t.Fatalf("get_us_stock payload wrong: %s", text)
	}
	quote := payload["quote"].(map[string]any)
	if quote["name"] != "Apple Inc." ||
		quote["price"].(float64) != 198.25 ||
		quote["currency"] != "USD" ||
		quote["market_time"] != "2026-06-18 20:00:00" {
		t.Fatalf("quote = %#v, want cached AAPL quote", quote)
	}
	history := payload["history"].(map[string]any)
	if history["range"] != "1y" ||
		history["count"].(float64) != 2 ||
		history["first_date"] != "2026-06-17" ||
		history["last_date"] != "2026-06-18" {
		t.Fatalf("history = %#v, want cached history summary", history)
	}
	profile := payload["profile"].(map[string]any)
	if profile["sector"] != "Technology" || profile["industry"] != "Consumer Electronics" {
		t.Fatalf("profile = %#v, want cached profile", profile)
	}
	if strings.Contains(text, "broker") ||
		strings.Contains(text, "backup") ||
		strings.Contains(text, "建议买入") ||
		strings.Contains(text, "external_fetch_performed") {
		t.Fatalf("get_us_stock should stay facts-only and local-read-only: %s", text)
	}
}

func TestServerCallsGetMarketIndicesTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	response := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"market-indices"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_market_indices","arguments":{}}`),
	})

	if response.Error != nil {
		t.Fatalf("get_market_indices error = %#v", response.Error)
	}
	text := firstTextContent(t, response)
	payload := decodeTextPayload(t, text)
	if payload["count"].(float64) != 2 ||
		payload["decision_boundary"] != "facts_only" ||
		payload["side_effects"] != "none" ||
		payload["external_fetch"] != "not_performed" {
		t.Fatalf("get_market_indices payload wrong: %s", text)
	}
	indices := payload["indices"].(map[string]any)
	gspc := indices["^GSPC"].(map[string]any)
	ndx := indices["^NDX"].(map[string]any)
	if gspc["name"] != "标普500" ||
		gspc["price"].(float64) != 5600.5 ||
		gspc["change_pct"].(float64) != 0.42 {
		t.Fatalf("GSPC = %#v, want cached index row", gspc)
	}
	if ndx["name"] != "纳斯达克100" || ndx["market"] != "US" {
		t.Fatalf("NDX = %#v, want cached index row", ndx)
	}
	if strings.Contains(text, "broker") ||
		strings.Contains(text, "backup") ||
		strings.Contains(text, "建议买入") ||
		strings.Contains(text, "external_fetch_performed") {
		t.Fatalf("get_market_indices should stay facts-only and cache-read-only: %s", text)
	}
}

func TestServerCallsFundXirrTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	for _, stmt := range []string{
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
			VALUES ('XIRR1', 'XIRR Test Asset', 'test', 'stock', 'US')`,
		`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
			VALUES
			('XIRR-BUY', '2025-06-01T00:00:00Z', '2025-06-02', '用户买入', 'buy', 'XIRR1', 'XIRR Test Asset', 100, 100, 0, -100, 100, 1),
			('XIRR-DIV', '2026-06-01T00:00:00Z', '2026-06-02', '现金分红', 'dividend', 'XIRR1', 'XIRR Test Asset', 10, 0, 0, 10, 0, 1)`,
		`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type)
			VALUES ('XIRR1', '2026-06-01', 1.2, 0, 'stock')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec xirr fixture: %v", err)
		}
	}
	server := newMCPServer(t, db)

	xirrResponse := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"fund-xirr"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_fund_xirr","arguments":{"code":"xirr1"}}`),
	})

	if xirrResponse.Error != nil {
		t.Fatalf("get_fund_xirr error = %#v", xirrResponse.Error)
	}
	xirrText := firstTextContent(t, xirrResponse)
	xirr := decodeTextPayload(t, xirrText)
	if xirr["code"] != "XIRR1" || xirr["security_type"] != "stock" || xirr["market"] != "US" {
		t.Fatalf("xirr identity wrong: %s", xirrText)
	}
	if xirr["xirr_pct"].(float64) != 30 {
		t.Fatalf("xirr_pct = %v, want 30; text=%s", xirr["xirr_pct"], xirrText)
	}
	if xirr["message"] != nil {
		t.Fatalf("message = %v, want nil; text=%s", xirr["message"], xirrText)
	}
	if xirr["decision_boundary"] != "facts_only" ||
		strings.Contains(xirrText, "建议买入") ||
		strings.Contains(xirrText, "加仓") {
		t.Fatalf("xirr should be facts-only with no advice language: %s", xirrText)
	}
}

func TestServerCallsPortfolioXirrTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	for _, stmt := range []string{
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
			VALUES ('PXIRR1', 'Portfolio XIRR Test Asset', 'test', 'stock', 'US')`,
		`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
			VALUES
			('PXIRR-BUY', '2025-06-01T00:00:00Z', '2025-06-02', '用户买入', 'buy', 'PXIRR1', 'Portfolio XIRR Test Asset', 100, 100, 0, -100, 100, 1),
			('PXIRR-DIV', '2026-06-01T00:00:00Z', '2026-06-02', '现金分红', 'dividend', 'PXIRR1', 'Portfolio XIRR Test Asset', 10, 0, 0, 10, 0, 1)`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, market, portfolio_id)
			VALUES ('PXIRR1', 'Portfolio XIRR Test Asset', 100, -100, 1.2, 120, 20, 20, 'stock', 'US', 2)`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec portfolio xirr fixture: %v", err)
		}
	}
	server := newMCPServer(t, db)

	xirrResponse := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"portfolio-xirr"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_portfolio_xirr","arguments":{"portfolio_id":2}}`),
	})

	if xirrResponse.Error != nil {
		t.Fatalf("get_portfolio_xirr error = %#v", xirrResponse.Error)
	}
	xirrText := firstTextContent(t, xirrResponse)
	xirr := decodeTextPayload(t, xirrText)
	if xirr["portfolio_id"].(float64) != 2 {
		t.Fatalf("portfolio_id = %v, want 2; text=%s", xirr["portfolio_id"], xirrText)
	}
	if xirr["xirr_pct"].(float64) != 30 {
		t.Fatalf("xirr_pct = %v, want 30; text=%s", xirr["xirr_pct"], xirrText)
	}
	if xirr["current_portfolio_value"].(float64) != 120 {
		t.Fatalf("current_portfolio_value = %v, want 120; text=%s", xirr["current_portfolio_value"], xirrText)
	}
	if xirr["decision_boundary"] != "facts_only" ||
		strings.Contains(xirrText, "建议买入") ||
		strings.Contains(xirrText, "加仓") {
		t.Fatalf("portfolio xirr should be facts-only with no advice language: %s", xirrText)
	}
}

func TestServerRejectsUnknownAndWriteToolsWithoutExecuting(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServer(t, db)

	unknown := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"missing_tool","arguments":{}}`),
	})
	if unknown.Error == nil || !strings.Contains(unknown.Error.Message, string(agenttools.DenyUnknownTool)) {
		t.Fatalf("unknown response = %#v, want unknown_tool error", unknown)
	}

	write := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"add_transaction","arguments":{"fund_code":"AAPL","amount":1}}`),
	})
	if write.Error == nil {
		t.Fatalf("write response error = nil, want denial")
	}
	if !strings.Contains(write.Error.Message, string(agenttools.DenyScope)) {
		t.Fatalf("write response = %#v, want read/external-context scope denial", write)
	}

	var txCount int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM transactions").Scan(&txCount); err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	if txCount != 2 {
		t.Fatalf("transactions count = %d, want unchanged 2", txCount)
	}
}

func TestServerCallsConfirmedTransactionWriteTools(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServerWithRole(t, db, agenttools.RoleOperator)

	add := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"add-tx"`),
		Method:  "tools/call",
		Params: json.RawMessage(`{"name":"add_transaction","arguments":{
			"confirmation_id":1,
			"confirmation_token":"test-token",
			"order_id":"MCP-GO-ADD",
			"fund_code":"aapl",
			"trade_time":"2026-06-03T09:00:00Z",
			"confirm_date":"2026-06-05",
			"trade_type":"用户买入",
			"direction":"buy",
			"confirm_amount":198.25,
			"confirm_share":1,
			"fee":0
		}}`),
	})
	if add.Error != nil {
		t.Fatalf("add_transaction error = %#v", add.Error)
	}
	addPayload := decodeTextPayload(t, firstTextContent(t, add))
	if addPayload["ok"] != true ||
		addPayload["imported"].(float64) != 1 ||
		addPayload["affected_funds"].(float64) != 1 ||
		addPayload["decision_boundary"] != "facts_only" {
		t.Fatalf("add_transaction payload = %s", firstTextContent(t, add))
	}

	var seq int
	var settlement int
	if err := db.QueryRowContext(context.Background(), `
		SELECT seq, settlement_days
		FROM transactions
		WHERE order_id = 'MCP-GO-ADD'
	`).Scan(&seq, &settlement); err != nil {
		t.Fatalf("query added MCP transaction: %v", err)
	}
	if settlement != 2 {
		t.Fatalf("settlement_days = %d, want 2", settlement)
	}

	update := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"update-tx"`),
		Method:  "tools/call",
		Params: json.RawMessage(`{"name":"update_transaction","arguments":{
			"confirmation_id":1,
			"confirmation_token":"test-token",
			"seq":` + strconv.Itoa(seq) + `,
			"direction":"sell",
			"confirm_amount":200,
			"confirm_share":1,
			"confirm_date":"2026-06-06"
		}}`),
	})
	if update.Error != nil {
		t.Fatalf("update_transaction error = %#v", update.Error)
	}
	updatePayload := decodeTextPayload(t, firstTextContent(t, update))
	if updatePayload["ok"] != true || updatePayload["decision_boundary"] != "facts_only" {
		t.Fatalf("update_transaction payload = %s", firstTextContent(t, update))
	}

	var signedCash float64
	var signedShare float64
	if err := db.QueryRowContext(context.Background(), `
		SELECT signed_cash_flow, signed_share_change, settlement_days
		FROM transactions
		WHERE seq = ?
	`, seq).Scan(&signedCash, &signedShare, &settlement); err != nil {
		t.Fatalf("query updated MCP transaction: %v", err)
	}
	if signedCash != 200 || signedShare != -1 || settlement != 3 {
		t.Fatalf("updated derived fields = cash %.2f share %.2f settlement %d", signedCash, signedShare, settlement)
	}

	del := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"delete-tx"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"delete_transaction","arguments":{"confirmation_id":1,"confirmation_token":"test-token","seq":` + strconv.Itoa(seq) + `}}`),
	})
	if del.Error != nil {
		t.Fatalf("delete_transaction error = %#v", del.Error)
	}
	delPayload := decodeTextPayload(t, firstTextContent(t, del))
	if delPayload["ok"] != true || delPayload["decision_boundary"] != "facts_only" {
		t.Fatalf("delete_transaction payload = %s", firstTextContent(t, del))
	}

	var count int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM transactions WHERE seq = ?", seq).Scan(&count); err != nil {
		t.Fatalf("count deleted MCP transaction: %v", err)
	}
	if count != 0 {
		t.Fatalf("deleted MCP transaction count = %d, want 0", count)
	}
}

func TestServerSecuritiesWriteToolsConfirmationGated(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	// analyst / no agentops: write must fail closed
	analyst := newMCPServer(t, db)
	denied := analyst.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"deny-add-fund"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"add_fund","arguments":{"fund_code":"ZZ9999","fund_name":"Zed"}}`),
	})
	if denied.Error == nil {
		t.Fatalf("expected denial for unconfirmed add_fund")
	}
	// operator with allowConfirmationConsumer
	op := newMCPServerWithRole(t, db, agenttools.RoleOperator)
	add := op.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"add-fund"`),
		Method:  "tools/call",
		Params: json.RawMessage(`{"name":"add_fund","arguments":{
			"confirmation_id":1,
			"confirmation_token":"test-token",
			"fund_code":"ZZ9999",
			"fund_name":"Zed Fund",
			"fund_type":"test"
		}}`),
	})
	if add.Error != nil {
		t.Fatalf("add_fund error = %#v", add.Error)
	}
	addPayload := decodeTextPayload(t, firstTextContent(t, add))
	if addPayload["ok"] != true || addPayload["created"] != true {
		t.Fatalf("add_fund payload = %s", firstTextContent(t, add))
	}
	upd := op.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"update-fund"`),
		Method:  "tools/call",
		Params: json.RawMessage(`{"name":"update_fund","arguments":{
			"confirmation_id":1,
			"confirmation_token":"test-token",
			"fund_code":"ZZ9999",
			"fund_name":"Zed Renamed"
		}}`),
	})
	if upd.Error != nil {
		t.Fatalf("update_fund error = %#v", upd.Error)
	}
	stock := op.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"add-security"`),
		Method:  "tools/call",
		Params: json.RawMessage(`{"name":"add_security","arguments":{
			"confirmation_id":1,
			"confirmation_token":"test-token",
			"code":"TSLA",
			"name":"Tesla Inc.",
			"security_type":"stock",
			"market":"US"
		}}`),
	})
	if stock.Error != nil {
		t.Fatalf("add_security error = %#v", stock.Error)
	}
	del := op.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"delete-fund"`),
		Method:  "tools/call",
		Params: json.RawMessage(`{"name":"delete_fund","arguments":{
			"confirmation_id":1,
			"confirmation_token":"test-token",
			"fund_code":"ZZ9999"
		}}`),
	})
	if del.Error != nil {
		t.Fatalf("delete_fund error = %#v", del.Error)
	}
	delPayload := decodeTextPayload(t, firstTextContent(t, del))
	if delPayload["ok"] != true || delPayload["deleted"] != true {
		t.Fatalf("delete_fund payload = %s", firstTextContent(t, del))
	}
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM fund_details WHERE fund_code='ZZ9999'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("ZZ9999 still present after delete")
	}
}

func TestServerAdjustAndAlertsTools(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	op := newMCPServerWithRole(t, db, agenttools.RoleOperator)
	adj := op.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"adj"`),
		Method:  "tools/call",
		Params: json.RawMessage(`{"name":"adjust_position","arguments":{
			"confirmation_id":1,"confirmation_token":"test-token",
			"fund_code":"019173","shares":80
		}}`),
	})
	if adj.Error != nil {
		t.Fatalf("adjust_position error = %#v", adj.Error)
	}
	payload := decodeTextPayload(t, firstTextContent(t, adj))
	if payload["ok"] != true || payload["shares"].(float64) != 80 {
		t.Fatalf("adjust payload = %s", firstTextContent(t, adj))
	}
	alerts := op.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"alerts"`),
		Method:  "tools/call",
		Params: json.RawMessage(`{"name":"check_alerts","arguments":{
			"confirmation_id":1,"confirmation_token":"test-token",
			"price_change_pct":1,"drawdown_pct":1,"stale_days":1
		}}`),
	})
	if alerts.Error != nil {
		t.Fatalf("check_alerts error = %#v", alerts.Error)
	}
	alertPayload := decodeTextPayload(t, firstTextContent(t, alerts))
	if alertPayload["ok"] != true || alertPayload["webhook_sent"] != false {
		t.Fatalf("alerts payload = %s", firstTextContent(t, alerts))
	}
	// fail-closed without confirmation for analyst
	analyst := newMCPServer(t, db)
	denied := analyst.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"deny-adj"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"adjust_position","arguments":{"fund_code":"019173","shares":1}}`),
	})
	if denied.Error == nil {
		t.Fatalf("expected denial")
	}
}

func TestServerRunDCAAutoInvestTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	// ensure NAV for plan fund
	if _, err := db.Exec(`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type) VALUES ('019173','2026-07-14',2.0,0,'fund')`); err != nil {
		// may already exist - ignore
		_ = err
	}
	op := newMCPServerWithRole(t, db, agenttools.RoleOperator)
	// dry_run default true
	resp := op.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"dca-run"`),
		Method:  "tools/call",
		Params: json.RawMessage(`{"name":"run_dca_auto_invest","arguments":{
			"confirmation_id":1,"confirmation_token":"test-token",
			"as_of":"2026-07-15","portfolio_id":1
		}}`),
	})
	if resp.Error != nil {
		t.Fatalf("run_dca_auto_invest error = %#v", resp.Error)
	}
	payload := decodeTextPayload(t, firstTextContent(t, resp))
	if payload["ok"] != true || payload["dry_run"] != true {
		t.Fatalf("payload = %s", firstTextContent(t, resp))
	}
	// fail-closed
	analyst := newMCPServer(t, db)
	denied := analyst.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"deny-dca"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"run_dca_auto_invest","arguments":{"as_of":"2026-07-15"}}`),
	})
	if denied.Error == nil {
		t.Fatalf("expected denial")
	}
}

func TestServerGenerateReportTool(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	op := newMCPServerWithRole(t, db, agenttools.RoleOperator)
	resp := op.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"report"`),
		Method:  "tools/call",
		Params: json.RawMessage(`{"name":"generate_report","arguments":{
			"confirmation_id":1,"confirmation_token":"test-token",
			"portfolio_id":1,"title":"Unit Report"
		}}`),
	})
	if resp.Error != nil {
		t.Fatalf("generate_report error = %#v", resp.Error)
	}
	payload := decodeTextPayload(t, firstTextContent(t, resp))
	if payload["ok"] != true || payload["format"] != "json" || payload["artifact"] != "json" {
		t.Fatalf("payload = %s", firstTextContent(t, resp))
	}
	analyst := newMCPServer(t, db)
	denied := analyst.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"deny-report"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"generate_report","arguments":{"portfolio_id":1}}`),
	})
	if denied.Error == nil {
		t.Fatalf("expected denial")
	}
}

func newMCPServer(t *testing.T, db *sql.DB) *Server {
	return newMCPServerWithRole(t, db, agenttools.RoleAnalyst)
}

func newMCPServerWithRole(t *testing.T, db *sql.DB, role agenttools.Role) *Server {
	t.Helper()
	portfolio := portfoliosvc.NewService(db)
	admin := adminsvc.NewServiceWithDriver(db, "sqlite")
	var confirmations confirmationConsumer
	if role == agenttools.RoleOperator {
		confirmations = allowConfirmationConsumer{}
	}
	// Mirror httpapi.NewRouter: the portfolio service must learn the same wiring fact
	// the MCP advertisement guard uses, otherwise harness/agent-context discovery and
	// tools/list would describe two different servers.
	portfolio.SetConfirmationFlowAvailable(confirmations != nil)
	server, err := NewServer(ServerDeps{Portfolio: &portfolio, Admin: &admin, AgentOps: confirmations, Role: role})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	return server
}

type allowConfirmationConsumer struct{}

func (allowConfirmationConsumer) ClaimConfirmation(_ context.Context, input agentops.ConsumeConfirmationInput) (agentops.ConsumedConfirmation, error) {
	return agentops.ConsumedConfirmation{ConfirmationID: input.ConfirmationID, Tool: input.Tool}, nil
}

func openMCPFixture(t *testing.T) *sql.DB {
	t.Helper()
	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatalf("open sqlite fixture: %v", err)
	}
	for _, stmt := range mcpFixtureStatements {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			db.Close()
			t.Fatalf("exec fixture statement %q: %v", stmt, err)
		}
	}
	return db
}

func decodeResult(t *testing.T, response Response) map[string]any {
	t.Helper()
	payload, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return decoded
}

func firstTextContent(t *testing.T, response Response) string {
	t.Helper()
	result := decodeResult(t, response)
	content := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("content empty: %#v", result)
	}
	first := content[0].(map[string]any)
	if first["type"] != "text" {
		t.Fatalf("content[0].type = %v, want text", first["type"])
	}
	text, ok := first["text"].(string)
	if !ok {
		t.Fatalf("content[0].text = %#v, want string", first["text"])
	}
	return text
}

func decodeTextPayload(t *testing.T, text string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("decode text payload: %v; text=%s", err, text)
	}
	return decoded
}

func toJSONString(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal value: %v", err)
	}
	return string(payload)
}

var mcpFixtureStatements = []string{
	`CREATE TABLE fund_details (
		fund_code TEXT PRIMARY KEY,
		fund_name TEXT,
		fund_type TEXT,
		security_type TEXT DEFAULT 'fund',
		market TEXT DEFAULT '',
		currency TEXT DEFAULT 'CNY',
		exchange TEXT,
		source TEXT DEFAULT 'mcp'
	)`,
	`CREATE TABLE transactions (
		seq INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id TEXT,
		trade_time TEXT,
		confirm_date TEXT,
		trade_type TEXT,
		direction TEXT,
		fund_code TEXT,
		fund_name TEXT,
		confirm_amount REAL,
		confirm_share REAL,
		fee REAL,
		signed_cash_flow REAL,
		signed_share_change REAL,
		settlement_days INTEGER
	)`,
	`CREATE TABLE nav_history (
		fund_code TEXT,
		date TEXT,
		unit_nav REAL,
		daily_change_pct REAL DEFAULT 0,
		security_type TEXT DEFAULT 'fund'
	)`,
	`CREATE TABLE portfolio_snapshot (
			fund_code TEXT NOT NULL,
		fund_name TEXT,
		held_shares REAL,
		total_cost REAL,
		latest_nav REAL,
		current_value REAL,
		unrealized_pnl REAL,
		pnl_pct REAL,
		security_type TEXT DEFAULT 'fund',
		market TEXT DEFAULT '',
			portfolio_id INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (fund_code, portfolio_id)
		)`,
	`CREATE TABLE portfolio_definitions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE fund_holdings (
		fund_code TEXT,
		stock_code TEXT,
		stock_name TEXT,
		weight_pct REAL,
		report_date TEXT
	)`,
	`CREATE TABLE sector_map (
		stock_code TEXT PRIMARY KEY,
		market TEXT,
		sector TEXT
	)`,
	`CREATE TABLE stock_profile (
		code TEXT,
		name TEXT,
		market TEXT,
		sector TEXT,
		industry TEXT,
		market_cap REAL,
		pe REAL,
		description TEXT,
		PRIMARY KEY (code, market)
	)`,
	`CREATE TABLE stock_realtime (
		code TEXT,
		market TEXT,
		name TEXT,
		price REAL,
		open REAL,
		high REAL,
		low REAL,
		change_pct REAL,
		change_amt REAL,
		volume REAL,
		amount REAL,
		turnover REAL,
		pe REAL,
		total_mv REAL,
		circ_mv REAL,
		high52 REAL,
		low52 REAL,
		currency TEXT DEFAULT '',
		updated_at TEXT DEFAULT (datetime('now')),
		PRIMARY KEY (code, market)
	)`,
	`CREATE TABLE stock_kline_cache (
		code TEXT,
		market TEXT,
		date TEXT,
		open REAL,
		close REAL,
		high REAL,
		low REAL,
		volume REAL,
		amount REAL,
		amplitude REAL,
		change_pct REAL,
		turnover_rate REAL,
		PRIMARY KEY (code, market, date)
	)`,
	`CREATE TABLE indices (
		code TEXT PRIMARY KEY,
		name TEXT,
		market TEXT,
		price REAL,
		change_pct REAL,
		change_amt REAL,
		updated_at TEXT DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE source_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		url TEXT,
		source TEXT NOT NULL DEFAULT 'websearch',
		snippet TEXT,
		query TEXT,
		related_security_code TEXT,
		related_security_name TEXT,
		is_read INTEGER DEFAULT 0,
		is_useful INTEGER DEFAULT 0,
		fetched_at TEXT NOT NULL DEFAULT (datetime('now')),
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE dca_plans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fund_code TEXT NOT NULL,
		fund_name TEXT,
		amount REAL NOT NULL,
		frequency TEXT NOT NULL DEFAULT 'weekday',
		weekday_mask TEXT NOT NULL DEFAULT '1,2,3,4,5',
		trade_type TEXT NOT NULL DEFAULT '定投买入',
		portfolio_id INTEGER NOT NULL DEFAULT 1,
		start_date TEXT NOT NULL,
		end_date TEXT,
		active INTEGER NOT NULL DEFAULT 1,
		source TEXT NOT NULL DEFAULT 'manual',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES
		('019173', '纳斯达克100指数(QDII)C', 'QDII-股票', 'fund', 'CN'),
		('AAPL', 'Apple Inc.', '科技股', 'stock', 'US')`,
	`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
		VALUES
		('TX001', '2026-06-01T09:00:00Z', '2026-06-02', '用户买入', 'buy', '019173', '纳斯达克100指数(QDII)C', 120, 100, 0.1, -120, 100, 2),
		('TX002', '2026-06-01T09:00:00Z', '2026-06-02', '用户买入', 'buy', 'AAPL', 'Apple Inc.', 300, 2, 0.1, -300, 2, 2)`,
	`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type) VALUES
		('019173', '2026-06-18', 1.5, -4.2, 'fund'),
		('019173', '2026-06-19', 1.2, -20.0, 'fund'),
		('019173', '2026-06-20', 1.4, 16.7, 'fund'),
		('AAPL', '2026-06-18', 190, 6.5, 'stock')`,
	`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, market, portfolio_id) VALUES
		('019173', '纳斯达克100指数(QDII)C', 100, -120, 1.5, 150, 30, 25, 'fund', 'CN', 1),
		('AAPL', 'Apple Inc.', 2, -300, 190, 380, 80, 26.67, 'stock', 'US', 1)`,
	`INSERT INTO portfolio_definitions (id, name, description) VALUES
		(1, 'default', 'Default portfolio')`,
	`INSERT INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, report_date) VALUES
		('019173', 'NVDA', 'NVIDIA', 8.5, '2026-03-31')`,
	`INSERT INTO sector_map (stock_code, market, sector) VALUES
		('NVDA', 'US', 'Semiconductors')`,
	`INSERT INTO stock_profile (code, name, market, sector, industry, market_cap, pe, description) VALUES
		('MSFT', 'Microsoft Corporation', 'US', 'Technology', 'Software', 3200000000000, 35.5, 'Productivity and cloud software'),
		('00700', 'Tencent Holdings', 'HK', 'Communication Services', 'Internet Content', 3800000000000, 18.2, 'Chinese internet platform'),
		('AAPL', 'Apple Inc.', 'US', 'Technology', 'Consumer Electronics', 3000000000000, 31.2, 'Consumer hardware and services')`,
	`INSERT INTO stock_realtime (code, market, name, price, open, high, low, change_pct, change_amt, volume, amount, pe, total_mv, high52, low52, currency, updated_at)
		VALUES ('AAPL', 'US', 'Apple Inc.', 198.25, 196.5, 199.0, 195.8, 1.2, 2.35, 45000000, 8900000000, 31.2, 3000000000000, 205.0, 160.0, 'USD', '2026-06-18 20:00:00')`,
	`INSERT INTO stock_kline_cache (code, market, date, open, close, high, low, volume, change_pct) VALUES
		('AAPL', 'US', '2026-06-18', 196.5, 198.25, 199.0, 195.8, 45000000, 1.2),
		('AAPL', 'US', '2026-06-17', 194.0, 195.9, 196.2, 193.7, 41000000, 0.7)`,
	`INSERT INTO indices (code, name, market, price, change_pct, change_amt, updated_at) VALUES
		('^GSPC', '标普500', 'US', 5600.5, 0.42, 23.5, '2099-01-01 12:00:00'),
		('^NDX', '纳斯达克100', 'US', 19888.2, 1.25, 245.8, '2099-01-01 12:00:00')`,
	`INSERT INTO source_events (title, source, snippet, query, related_security_code, related_security_name, fetched_at, created_at) VALUES
		('AAPL market update', 'websearch', 'Apple moved with market...', 'AAPL market update', 'AAPL', 'Apple Inc.', '2026-06-18 10:00:00', '2026-06-18 10:00:00')`,
	`INSERT INTO dca_plans (id, fund_code, fund_name, amount, frequency, weekday_mask, trade_type, portfolio_id, start_date, end_date, active, source, created_at, updated_at)
		VALUES
		(1, '019173', '纳斯达克100指数(QDII)C', 25, 'weekday', '1,3,5', '定投买入', 1, '2026-06-01', NULL, 1, 'manual', '2026-06-01 09:00:00', '2026-06-02 09:00:00'),
		(2, 'AAPL', 'Apple Inc.', 50, 'weekday', '2,4', '定投买入', 1, '2026-06-03', NULL, 0, 'manual', '2026-06-03 09:00:00', '2026-06-04 09:00:00'),
		(3, 'OTHER', 'Other Portfolio', 10, 'weekday', '1,2,3,4,5', '定投买入', 2, '2026-06-05', NULL, 1, 'manual', '2026-06-05 09:00:00', '2026-06-06 09:00:00')`,
}

func TestIsNotificationDistinguishesMissingID(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"notification without id", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, true},
		{"cancelled notification", `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}`, true},
		{"progress notification", `{"jsonrpc":"2.0","method":"notifications/progress"}`, true},
		{"numeric id request", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, false},
		{"string id request", `{"jsonrpc":"2.0","id":"abc","method":"tools/list"}`, false},
		// "id": null is a valid request id (JSON-RPC 2.0 §4.2), NOT a notification.
		{"null id is still a request", `{"jsonrpc":"2.0","id":null,"method":"initialize"}`, false},
		{"no jsonrpc but id present", `{"method":"initialize","id":5}`, false},
	}
	for _, tc := range cases {
		var request Request
		if err := json.Unmarshal([]byte(tc.body), &request); err != nil {
			t.Fatalf("%s: unmarshal %v", tc.name, err)
		}
		if got := IsNotification(request); got != tc.want {
			t.Errorf("%s: IsNotification = %v, want %v (id=%s)", tc.name, got, tc.want, request.ID)
		}
	}
}
