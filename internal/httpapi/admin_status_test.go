package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

)

func TestAdminStatusReportsReadOnlyDiagnostics(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(testCfg(), WithDB(db))

	result := doJSONRequest(t, router, http.MethodGet, "/api/admin/status", nil, http.StatusOK)
	if v, ok := result["ok"]; !ok || v != true {
		t.Fatalf("ok = %v, want true; body=%s", v, toJSONString(t, result))
	}
	if result["decision_boundary"] != "read_only" {
		t.Fatalf("decision_boundary = %v, want read_only", result["decision_boundary"])
	}
	if result["uptime_sec"].(float64) < 0 {
		t.Fatalf("uptime_sec = %v, want non-negative", result["uptime_sec"])
	}
	if v, ok := result["response_ms"].(float64); !ok || v < 0 {
		t.Fatalf("response_ms = %v, want non-negative; body=%s", result["response_ms"], toJSONString(t, result))
	}

	transactions := result["transactions"].(map[string]any)
	if transactions["count"].(float64) != 2 {
		t.Fatalf("transaction count = %v, want 2", transactions["count"])
	}
	if transactions["last"] != "2026-06-01T09:00:00Z" {
		t.Fatalf("transaction last = %v, want fixture timestamp", transactions["last"])
	}

	nav := result["nav"].(map[string]any)
	if nav["count"].(float64) != 2 || nav["funds"].(float64) != 2 {
		t.Fatalf("nav = %#v, want count/funds 2", nav)
	}
	if nav["first"] != "2026-06-18" || nav["last"] != "2026-06-18" {
		t.Fatalf("nav range = %#v, want fixture date", nav)
	}

	portfolio := result["portfolio"].(map[string]any)
	if portfolio["held_funds"].(float64) != 2 {
		t.Fatalf("held_funds = %v, want 2", portfolio["held_funds"])
	}

	securities := result["securities"].(map[string]any)
	if securities["total"].(float64) != 2 || securities["funds"].(float64) != 1 || securities["stocks"].(float64) != 1 {
		t.Fatalf("securities = %#v, want total=2 funds=1 stocks=1", securities)
	}

	anomalies := result["anomalies"].(map[string]any)
	if anomalies["count"].(float64) != 0 {
		t.Fatalf("anomalies count = %v, want 0", anomalies["count"])
	}
	if strings.Contains(toJSONString(t, result), "buy") || strings.Contains(toJSONString(t, result), "sell") {
		t.Fatalf("admin status must stay diagnostic-only, got %s", toJSONString(t, result))
	}
}

func TestAdminStatusByCodeReportsNumericFundAndPreservesTicker(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE fund_status (
		fund_code TEXT PRIMARY KEY,
		purchase_status TEXT,
		redemption_status TEXT
	)`); err != nil {
		t.Fatalf("create fund_status fixture: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO fund_status (fund_code, purchase_status, redemption_status)
		VALUES ('019173', 'open', 'open')`); err != nil {
		t.Fatalf("insert fund_status fixture: %v", err)
	}

	router := NewRouter(testCfg(), WithDB(db))

	fund := doJSONRequest(t, router, http.MethodGet, "/api/admin/status/19173", nil, http.StatusOK)
	if fund["code"] != "019173" {
		t.Fatalf("fund code = %v, want 019173", fund["code"])
	}
	if fund["name"] != "纳斯达克100指数(QDII)C" {
		t.Fatalf("fund name = %v, want fixture name", fund["name"])
	}
	if fund["security_type"] != "fund" || fund["market"] != "CN" {
		t.Fatalf("fund identity = %s, want fund/CN", toJSONString(t, fund))
	}
	if fund["transactions"].(map[string]any)["n"].(float64) != 1 {
		t.Fatalf("fund transactions = %s, want n=1", toJSONString(t, fund["transactions"]))
	}
	if fund["nav"].(map[string]any)["n"].(float64) != 1 {
		t.Fatalf("fund nav = %s, want n=1", toJSONString(t, fund["nav"]))
	}
	if fund["position"].(map[string]any)["held_shares"].(float64) != 100 {
		t.Fatalf("fund position = %s, want held_shares=100", toJSONString(t, fund["position"]))
	}
	if fund["trading"].(map[string]any)["purchase_status"] != "open" {
		t.Fatalf("fund trading = %s, want purchase_status=open", toJSONString(t, fund["trading"]))
	}

	stock := doJSONRequest(t, router, http.MethodGet, "/api/admin/status/aapl", nil, http.StatusOK)
	if stock["code"] != "AAPL" {
		t.Fatalf("stock code = %v, want AAPL", stock["code"])
	}
	if stock["name"] != "Apple Inc." {
		t.Fatalf("stock name = %v, want Apple Inc.", stock["name"])
	}
	if stock["security_type"] != "stock" || stock["market"] != "US" {
		t.Fatalf("stock identity = %s, want stock/US", toJSONString(t, stock))
	}
	if stock["position"].(map[string]any)["held_shares"].(float64) != 2 {
		t.Fatalf("stock position = %s, want held_shares=2", toJSONString(t, stock["position"]))
	}
	if strings.Contains(stock["code"].(string), "0AAPL") {
		t.Fatalf("ticker should not be zero-padded: %s", stock["code"])
	}
}
