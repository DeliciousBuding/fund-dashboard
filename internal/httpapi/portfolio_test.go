package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

func TestPortfolioAgentRoutesExposeFactsOnlySurfaces(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(testCfg(), WithDB(db))

	harness := doJSONRequest(t, router, http.MethodGet, "/api/portfolio/harness", nil, http.StatusOK)
	if harness["decision_boundary"] != "facts_only" {
		t.Fatalf("harness decision_boundary = %v, want facts_only", harness["decision_boundary"])
	}
	if harness["agent_permissions"] == nil {
		t.Fatalf("harness missing agent_permissions: %#v", harness)
	}
	if !strings.Contains(toJSONString(t, harness), "backup_producer") {
		t.Fatalf("harness missing backup_producer disabled boundary: %s", toJSONString(t, harness))
	}

	brief := doJSONRequest(t, router, http.MethodGet, "/api/portfolio/source-brief?limit=4", nil, http.StatusOK)
	if brief["decision_boundary"] != "source_queries_only" {
		t.Fatalf("source brief decision_boundary = %v, want source_queries_only", brief["decision_boundary"])
	}
	if !strings.Contains(toJSONString(t, brief), "DSA search providers") {
		t.Fatalf("source brief missing DSA search providers: %s", toJSONString(t, brief))
	}
	if strings.Contains(toJSONString(t, brief), "建议扣款") || strings.Contains(toJSONString(t, brief), "加仓") {
		t.Fatalf("source brief contains decision language: %s", toJSONString(t, brief))
	}

	events := doJSONRequest(t, router, http.MethodGet, "/api/portfolio/source-events?show_read=true", nil, http.StatusOK)
	if events["decision_boundary"] != "facts_only" {
		t.Fatalf("source events decision_boundary = %v, want facts_only", events["decision_boundary"])
	}
	if events["count"].(float64) != 1 {
		t.Fatalf("source events count = %v, want 1", events["count"])
	}
	if !strings.Contains(toJSONString(t, events), `"is_read":false`) {
		t.Fatalf("source events should convert is_read to bool: %s", toJSONString(t, events))
	}

	contextPack := doJSONRequest(t, router, http.MethodGet, "/api/portfolio/agent-context?source_limit=4&event_limit=10", nil, http.StatusOK)
	if contextPack["schema_version"] != "agent-context-pack-v1" {
		t.Fatalf("agent context schema_version = %v, want agent-context-pack-v1", contextPack["schema_version"])
	}
	if contextPack["decision_boundary"] != "facts_only" {
		t.Fatalf("agent context decision_boundary = %v, want facts_only", contextPack["decision_boundary"])
	}
	if !strings.Contains(toJSONString(t, contextPack), "backup_producer") {
		t.Fatalf("agent context missing backup disabled boundary: %s", toJSONString(t, contextPack))
	}
	if !strings.Contains(toJSONString(t, contextPack), "stored_events_summary") {
		t.Fatalf("agent context missing source event summary: %s", toJSONString(t, contextPack))
	}
	// Public agent-context must not advertise write tools (#65).
	body := toJSONString(t, contextPack)
	for _, banned := range []string{`"add_transaction"`, `"delete_fund"`, `"crawl_nav"`, `"run_dca_auto_invest"`} {
		if strings.Contains(body, banned) {
			t.Fatalf("public agent-context leaked %s: %s", banned, body)
		}
	}
	// harness also public-filtered
	hbody := toJSONString(t, harness)
	for _, banned := range []string{`"add_transaction"`, `"crawl_nav"`} {
		if strings.Contains(hbody, banned) {
			t.Fatalf("public harness leaked %s: %s", banned, hbody)
		}
	}
}

func TestPortfolioDashboardRoutesExposeFrontendCompatibleFacts(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE portfolio_definitions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT INTO portfolio_definitions (id, name, description) VALUES
			(1, 'default', 'Default portfolio'),
			(2, 'satellite', 'Satellite sleeve');
	`); err != nil {
		t.Fatalf("seed portfolio definitions: %v", err)
	}

	router := NewRouter(testCfg(), WithDB(db))

	summary := doJSONRequest(t, router, http.MethodGet, "/api/portfolio", nil, http.StatusOK)
	if summary["total_tx"].(float64) != 2 ||
		summary["unique_funds"].(float64) != 1 ||
		summary["unique_stocks"].(float64) != 1 ||
		summary["held_funds"].(float64) != 2 ||
		summary["unrealized_pnl"].(float64) != 110 {
		t.Fatalf("summary response = %s", toJSONString(t, summary))
	}
	if summary["last_nav_date"] != "2026-06-18" {
		t.Fatalf("last_nav_date = %v, want 2026-06-18", summary["last_nav_date"])
	}
	byType := summary["by_security_type"].([]any)
	if len(byType) != 2 {
		t.Fatalf("by_security_type length = %d, want fund and stock rows: %s", len(byType), toJSONString(t, summary))
	}
	if strings.Contains(toJSONString(t, summary), "TotalTx") {
		t.Fatalf("summary should use frontend snake_case fields: %s", toJSONString(t, summary))
	}

	timeline := doJSONArrayRequest(t, router, http.MethodGet, "/api/portfolio/timeline", http.StatusOK)
	if len(timeline) != 1 ||
		timeline[0]["date"] != "2026-06-18" ||
		timeline[0]["total_value"].(float64) != 530 ||
		timeline[0]["total_cost"].(float64) != 420 {
		t.Fatalf("timeline response = %s", toJSONString(t, timeline))
	}

	xirr := doJSONRequest(t, router, http.MethodGet, "/api/portfolio/xirr", nil, http.StatusOK)
	if _, ok := xirr["xirr"]; !ok {
		t.Fatalf("xirr response missing frontend xirr field: %s", toJSONString(t, xirr))
	}
	if xirr["decision_boundary"] != nil || xirr["current_portfolio_value"] != nil {
		t.Fatalf("xirr route should keep frontend-compatible envelope only: %s", toJSONString(t, xirr))
	}

	portfolios := doJSONArrayRequest(t, router, http.MethodGet, "/api/portfolio/portfolios", http.StatusOK)
	if len(portfolios) != 2 ||
		portfolios[0]["id"].(float64) != 1 ||
		portfolios[0]["name"] != "default" ||
		portfolios[1]["id"].(float64) != 2 {
		t.Fatalf("portfolios response = %s", toJSONString(t, portfolios))
	}

	allPayloads := toJSONString(t, map[string]any{
		"summary":    summary,
		"timeline":   timeline,
		"xirr":       xirr,
		"portfolios": portfolios,
	})
	if strings.Contains(allPayloads, "backup") ||
		strings.Contains(allPayloads, "建议买入") ||
		strings.Contains(allPayloads, "external_fetch_performed") {
		t.Fatalf("dashboard REST routes should stay read-only facts: %s", allPayloads)
	}
}

func TestPortfolioPenetrationRouteExposesFrontendCompatibleProjection(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(testCfg(), WithDB(db))

	penetration := doJSONRequest(t, router, http.MethodGet, "/api/portfolio/penetration", nil, http.StatusOK)
	if penetration["total_portfolio_value"].(float64) != 150 ||
		penetration["equity_fund_count"].(float64) != 1 ||
		penetration["unique_stocks"].(float64) != 1 {
		t.Fatalf("penetration summary = %s", toJSONString(t, penetration))
	}
	rows := penetration["penetration"].([]any)
	if len(rows) != 1 {
		t.Fatalf("penetration rows length = %d, want 1: %s", len(rows), toJSONString(t, penetration))
	}
	nvda := rows[0].(map[string]any)
	if nvda["stock_code"] != "NVDA" ||
		nvda["stock_name"] != "NVIDIA" ||
		nvda["total_exposure_cny"].(float64) != 12.75 ||
		nvda["weight_pct"].(float64) != 8.5 {
		t.Fatalf("NVDA row = %#v", nvda)
	}
	funds := nvda["held_by_funds"].([]any)
	if len(funds) != 1 {
		t.Fatalf("held_by_funds length = %d, want 1: %#v", len(funds), funds)
	}
	fund := funds[0].(map[string]any)
	if fund["fund_code"] != "019173" ||
		fund["fund_name"] != "纳斯达克100指数(QDII)C" ||
		fund["weight_pct"].(float64) != 8.5 ||
		fund["fund_value_cny"].(float64) != 150 {
		t.Fatalf("held_by_funds[0] = %#v", fund)
	}
	if strings.Contains(toJSONString(t, penetration), "backup") ||
		strings.Contains(toJSONString(t, penetration), "建议买入") {
		t.Fatalf("penetration route should stay facts-only: %s", toJSONString(t, penetration))
	}
}

func TestPortfolioSourceEventsWriteFeedbackRoutes(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(testCfg(), WithDB(db))

	created := doJSONRequest(t, router, http.MethodPost, "/api/portfolio/source-events", map[string]any{
		"title":                 "Apple 发布新财报",
		"source":                "websearch",
		"related_security_code": "AAPL",
	}, http.StatusCreated)
	if created["id"].(float64) == 0 {
		t.Fatalf("created id = %v, want inserted id", created["id"])
	}

	id := int(created["id"].(float64))
	patched := doJSONRequest(t, router, http.MethodPatch, "/api/portfolio/source-events/"+itoa(id), map[string]any{
		"is_read":   true,
		"is_useful": true,
	}, http.StatusOK)
	if patched["ok"] != true {
		t.Fatalf("patch response = %#v, want ok true", patched)
	}

	unread := doJSONRequest(t, router, http.MethodGet, "/api/portfolio/source-events", nil, http.StatusOK)
	if unread["count"].(float64) != 1 {
		t.Fatalf("unread count = %v, want existing unread fixture only", unread["count"])
	}

	all := doJSONRequest(t, router, http.MethodGet, "/api/portfolio/source-events?show_read=true&code=AAPL", nil, http.StatusOK)
	if all["count"].(float64) != 2 || !strings.Contains(toJSONString(t, all), `"is_useful":true`) {
		t.Fatalf("filtered all events = %s, want both AAPL events including marked useful event", toJSONString(t, all))
	}
}

func openPortfolioHTTPFixture(t *testing.T) *sql.DB {
	t.Helper()
	return testutil.OpenTempDBWithSchema(t, portfolioHTTPFixtureStatements)
}

func doJSONRequest(t *testing.T, handler http.Handler, method string, path string, body any, wantStatus int) map[string]any {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.HasPrefix(path, "/api/admin/") || path == "/mcp" ||
		strings.HasPrefix(path, "/api/agent/confirmations") ||
		strings.HasPrefix(path, "/api/agent/tools") {
		req.Header.Set("Authorization", "Bearer "+testAdminKey)
	}
	if strings.HasPrefix(path, "/api/transactions/") ||
		(strings.HasPrefix(path, "/api/portfolio/source-events") && (method == http.MethodPost || method == http.MethodPatch || method == http.MethodPut || method == http.MethodDelete)) {
		req.Header.Set(edgeKeyHeader, testEdgeKey)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, res.Code, wantStatus, res.Body.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response JSON: %v; body=%s", err, res.Body.String())
	}
	return decoded
}

func doJSONArrayRequest(t *testing.T, handler http.Handler, method string, path string, wantStatus int) []map[string]any {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, res.Code, wantStatus, res.Body.String())
	}
	var decoded []map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response JSON array: %v; body=%s", err, res.Body.String())
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

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

var portfolioHTTPFixtureStatements = []string{
	`CREATE TABLE fund_details (
		fund_code TEXT PRIMARY KEY,
		fund_name TEXT,
		fund_type TEXT,
		security_type TEXT DEFAULT 'fund',
		market TEXT DEFAULT ''
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
			portfolio_id INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (fund_code, portfolio_id)
		)`,
	`CREATE TABLE nav_history (
		fund_code TEXT,
		date TEXT,
		unit_nav REAL,
		daily_change_pct REAL DEFAULT 0,
		security_type TEXT DEFAULT 'fund'
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
	`CREATE TABLE fund_holdings (
		fund_code TEXT,
		stock_code TEXT,
		stock_name TEXT,
		weight_pct REAL,
		shares REAL,
		market_value REAL,
		report_date TEXT,
		PRIMARY KEY (fund_code, stock_code, report_date)
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
	`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES
		('019173', '纳斯达克100指数(QDII)C', 'QDII-股票', 'fund', 'CN'),
		('AAPL', 'Apple Inc.', '科技股', 'stock', 'US')`,
	`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES
		('019173', '纳斯达克100指数(QDII)C', 100, -120, 1.5, 150, 30, 25, 'fund', 1),
		('AAPL', 'Apple Inc.', 2, -300, 190, 380, 80, 26.67, 'stock', 1)`,
	`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type) VALUES
		('019173', '2026-06-18', 1.5, -4.2, 'fund'),
		('AAPL', '2026-06-18', 190, 6.5, 'stock')`,
	`INSERT INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date) VALUES
		('019173', 'NVDA', 'NVIDIA', 8.5, 100, 12000, '2026-03-31')`,
	`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
		VALUES
		('TX001', '2026-06-01T09:00:00Z', '2026-06-02', '用户买入', 'buy', '019173', '纳斯达克100指数(QDII)C', 120, 100, 0.1, -120, 100, 2),
		('TX002', '2026-06-01T09:00:00Z', '2026-06-02', '用户买入', 'buy', 'AAPL', 'Apple Inc.', 300, 2, 0.1, -300, 2, 2)`,
	`INSERT INTO source_events (title, source, snippet, query, related_security_code, related_security_name, fetched_at, created_at) VALUES
		('Market update', 'websearch', 'Markets moved...', 'AAPL market update', 'AAPL', 'Apple Inc.', '2026-06-18 10:00:00', '2026-06-18 10:00:00')`,
}
