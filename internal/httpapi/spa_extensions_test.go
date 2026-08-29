package httpapi

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

// SPA 写扩展 + 读扩展（/api/transactions、/api/dca/*、/api/securities、
// /api/portfolio/adjust-position、/api/reports、/api/alerts）的端到端矩阵。

var spaExtensionFixtureExtra = []string{
	// 共享 fixture 的 fund_details/transactions 是裁剪版；扩展读写路径需要完整列。
	`ALTER TABLE fund_details ADD COLUMN currency TEXT DEFAULT ''`,
	`ALTER TABLE fund_details ADD COLUMN exchange TEXT DEFAULT ''`,
	`ALTER TABLE fund_details ADD COLUMN source TEXT DEFAULT ''`,
	`ALTER TABLE transactions ADD COLUMN anomaly TEXT`,
	`ALTER TABLE transactions ADD COLUMN portfolio_id INTEGER DEFAULT 1`,
	`CREATE TABLE portfolio_definitions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`INSERT INTO portfolio_definitions (id, name, description) VALUES (1, 'default', 'Default portfolio')`,
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
	`INSERT INTO dca_plans (id, fund_code, fund_name, amount, frequency, weekday_mask, trade_type, portfolio_id, start_date, active)
		VALUES (1, '019173', '纳斯达克100指数(QDII)C', 25, 'weekday', '1,3,5', '定投买入', 1, '2026-06-01', 1)`,
}

func openSPAExtensionFixture(t *testing.T) *sql.DB {
	t.Helper()
	stmts := append(append([]string(nil), portfolioHTTPFixtureStatements...), spaExtensionFixtureExtra...)
	return testutil.OpenTempDBWithSchema(t, stmts)
}

func TestSPAReadExtensionsServeLedgerPlansAndAlerts(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()

	router := newAuthedRouter(t, testCfg(), db)

	tx := doJSONRequest(t, router, http.MethodGet, "/api/transactions", nil, http.StatusOK)
	if tx["total"].(float64) != 2 {
		t.Fatalf("transactions total = %v, want 2: %s", tx["total"], toJSONString(t, tx))
	}
	filtered := doJSONRequest(t, router, http.MethodGet, "/api/transactions?fund_code=019173", nil, http.StatusOK)
	if filtered["total"].(float64) != 1 {
		t.Fatalf("filtered transactions total = %v, want 1: %s", filtered["total"], toJSONString(t, filtered))
	}
	if strings.Contains(toJSONString(t, filtered), "AAPL") {
		t.Fatalf("fund_code filter leaked other rows: %s", toJSONString(t, filtered))
	}

	plans := doJSONRequest(t, router, http.MethodGet, "/api/dca/plans", nil, http.StatusOK)
	if len(plans["plans"].([]any)) != 1 {
		t.Fatalf("dca plans = %s, want 1 plan", toJSONString(t, plans))
	}
	active := doJSONRequest(t, router, http.MethodGet, "/api/dca/plans?active=true", nil, http.StatusOK)
	if len(active["plans"].([]any)) != 1 {
		t.Fatalf("active dca plans = %s, want 1 plan", toJSONString(t, active))
	}

	alerts := doJSONRequest(t, router, http.MethodGet, "/api/alerts", nil, http.StatusOK)
	if alerts["ok"] != true {
		t.Fatalf("alerts response = %s, want ok", toJSONString(t, alerts))
	}

	freshness := doJSONRequest(t, router, http.MethodGet, "/api/freshness", nil, http.StatusOK)
	if freshness["health"] == nil || freshness["decision_boundary"] == nil {
		t.Fatalf("freshness response = %s, want health + decision_boundary", toJSONString(t, freshness))
	}
}

func TestSPAWriteExtensionsDCAPlanLifecycle(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()

	router := newAuthedRouter(t, testCfg(), db)

	created := doJSONRequest(t, router, http.MethodPost, "/api/dca/plans", map[string]any{
		"fund_code":  "AAPL",
		"amount":     100,
		"frequency":  "weekday",
		"start_date": "2026-06-10",
	}, http.StatusOK)
	if created["ok"] != true {
		t.Fatalf("upsert dca plan = %s, want ok", toJSONString(t, created))
	}
	plan := created["plan"].(map[string]any)
	planID := int(plan["id"].(float64))
	if planID == 0 {
		t.Fatalf("created plan id = 0: %s", toJSONString(t, created))
	}
	if plan["source"] != "web" {
		t.Fatalf("browser-created plan source = %v, want web（不沿用 service 的 mcp 默认）", plan["source"])
	}

	// handler 层校验：缺 fund_code → 400
	bad := doJSONRequest(t, router, http.MethodPost, "/api/dca/plans", map[string]any{
		"amount": 100,
	}, http.StatusBadRequest)
	if bad["error"] == nil {
		t.Fatalf("validation error response = %s", toJSONString(t, bad))
	}

	// service 层校验消息透传：amount 超上限是 service 独有的校验（handler 只拦 <= 0）。
	invalid := doJSONRequest(t, router, http.MethodPost, "/api/dca/plans", map[string]any{
		"fund_code": "AAPL",
		"amount":    2_000_000,
	}, http.StatusBadRequest)
	if invalid["error"] != "amount max 1000000" {
		t.Fatalf("service validation should pass through as 400 message, got: %s", toJSONString(t, invalid))
	}

	disabled := doJSONRequest(t, router, http.MethodPost, "/api/dca/plans/"+itoa(planID)+"/disable", nil, http.StatusOK)
	if disabled["ok"] != true || disabled["updated"] != true {
		t.Fatalf("disable dca plan = %s", toJSONString(t, disabled))
	}
}

func TestSPAWriteExtensionsSecuritiesAndAdjustPosition(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()

	router := newAuthedRouter(t, testCfg(), db)

	created := doJSONRequest(t, router, http.MethodPost, "/api/securities", map[string]any{
		"code":          "MSFT",
		"name":          "Microsoft Corporation",
		"security_type": "stock",
		"market":        "US",
	}, http.StatusOK)
	if created["ok"] != true || created["created"] != true {
		t.Fatalf("upsert security = %s", toJSONString(t, created))
	}

	// handler 层校验：缺 name → 400
	doJSONRequest(t, router, http.MethodPost, "/api/securities", map[string]any{
		"code": "NOPE",
	}, http.StatusBadRequest)

	adjusted := doJSONRequest(t, router, http.MethodPost, "/api/portfolio/adjust-position", map[string]any{
		"code":   "MSFT",
		"shares": 5,
		"reason": "manual sync",
	}, http.StatusOK)
	if adjusted["ok"] != true || adjusted["shares"].(float64) != 5 {
		t.Fatalf("adjust position = %s", toJSONString(t, adjusted))
	}

	// handler 层校验：负份额 → 400
	doJSONRequest(t, router, http.MethodPost, "/api/portfolio/adjust-position", map[string]any{
		"code":   "MSFT",
		"shares": -1,
	}, http.StatusBadRequest)

	deleted := doJSONRequest(t, router, http.MethodDelete, "/api/securities/MSFT", nil, http.StatusOK)
	if deleted["ok"] != true || deleted["deleted"] != true {
		t.Fatalf("delete security = %s", toJSONString(t, deleted))
	}
	// 再删一次：已不存在 → deleted=false（幂等）
	again := doJSONRequest(t, router, http.MethodDelete, "/api/securities/MSFT", nil, http.StatusOK)
	if again["deleted"] != false {
		t.Fatalf("second delete = %s, want deleted=false", toJSONString(t, again))
	}
}

func TestSPAWriteExtensionsReportsAndDCARun(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()

	router := newAuthedRouter(t, testCfg(), db)

	report := doJSONRequest(t, router, http.MethodPost, "/api/reports", map[string]any{
		"title": "月度报告",
	}, http.StatusOK)
	if report["ok"] != true || report["decision_boundary"] != "facts_only" {
		t.Fatalf("generate report = %s", toJSONString(t, report))
	}
	if !strings.HasPrefix(report["report_id"].(string), "rpt-1-") {
		t.Fatalf("report_id = %v, want rpt-1-*", report["report_id"])
	}

	run := doJSONRequest(t, router, http.MethodPost, "/api/dca/run", map[string]any{
		"dry_run": true,
	}, http.StatusOK)
	if run["ok"] != true || run["dry_run"] != true {
		t.Fatalf("dca dry run = %s", toJSONString(t, run))
	}
}

func TestSPAExtensionsRequireSession(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()

	// 不注入 session：裸 router，所有扩展端点必须 401/403。
	router := NewRouter(testCfg(), WithDB(db), WithAuth(newTestAuthService(t, db)))

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/transactions"},
		{http.MethodGet, "/api/dca/plans"},
		{http.MethodGet, "/api/alerts"},
		{http.MethodGet, "/api/freshness"},
		{http.MethodPost, "/api/dca/plans"},
		{http.MethodPost, "/api/dca/plans/1/disable"},
		{http.MethodPost, "/api/dca/run"},
		{http.MethodPost, "/api/securities"},
		{http.MethodDelete, "/api/securities/019173"},
		{http.MethodPost, "/api/portfolio/adjust-position"},
		{http.MethodPost, "/api/reports"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code == http.StatusOK || res.Code == http.StatusCreated {
			t.Fatalf("%s %s without session returned %d, want 401/403", tc.method, tc.path, res.Code)
		}
	}
}
