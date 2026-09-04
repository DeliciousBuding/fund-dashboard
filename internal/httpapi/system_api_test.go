package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
)

// 工作台系统 API(design 06 §2.6)端点矩阵:
// 200(带 session)、401(无 session)、403(session 但缺 CSRF 头)。

func TestSystemReadEndpointsServeWithSession(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()

	router := newAuthedRouter(t, testCfg(), db, WithDBDriver("sqlite"))

	status := doJSONRequest(t, router, http.MethodGet, "/api/system/status", nil, http.StatusOK)
	if status["version"] != "test" || status["db_driver"] != "sqlite" || status["go_version"] == nil {
		t.Fatalf("status = %s", toJSONString(t, status))
	}
	if status["uptime_sec"].(float64) < 0 {
		t.Fatalf("uptime_sec = %v", status["uptime_sec"])
	}
	if status["freshness"].(map[string]any)["health"] == nil {
		t.Fatalf("status freshness.health missing: %s", toJSONString(t, status))
	}

	// 无 scheduler 接线 → 空任务清单而不是错误。
	jobsResp := doJSONRequest(t, router, http.MethodGet, "/api/system/jobs", nil, http.StatusOK)
	if jobsList, ok := jobsResp["jobs"].([]any); !ok || len(jobsList) != 0 {
		t.Fatalf("jobs = %s, want empty list", toJSONString(t, jobsResp))
	}

	integrity := doJSONRequest(t, router, http.MethodGet, "/api/system/integrity", nil, http.StatusOK)
	if integrity["overall"] == nil {
		t.Fatalf("integrity = %s, want overall", toJSONString(t, integrity))
	}

	audit := doJSONRequest(t, router, http.MethodGet, "/api/system/audit", nil, http.StatusOK)
	if _, ok := audit["events"].([]any); !ok {
		t.Fatalf("audit = %s, want events array", toJSONString(t, audit))
	}
	// limit 上限 500:超大 limit 不报错。
	doJSONRequest(t, router, http.MethodGet, "/api/system/audit?limit=99999", nil, http.StatusOK)

	agent := doJSONRequest(t, router, http.MethodGet, "/api/system/agent", nil, http.StatusOK)
	if agent["endpoint"] != "/mcp" {
		t.Fatalf("agent endpoint = %v", agent["endpoint"])
	}
	if agent["tools"].(map[string]any)["total_tools"].(float64) <= 0 {
		t.Fatalf("tools summary empty: %s", toJSONString(t, agent))
	}
	keys := agent["keys"].(map[string]any)
	if keys["mcp_api_key"] != "已配置" { // 只回显配置状态，绝不泄漏 key 片段
		t.Fatalf("mcp_api_key mask = %v", keys["mcp_api_key"])
	}
	if keys["public_mcp_key"] != "已配置" {
		t.Fatalf("public_mcp_key mask = %v", keys["public_mcp_key"])
	}

	// 写端点(authedRouter 已带 CSRF 头)。
	verify := doJSONRequest(t, router, http.MethodPost, "/api/system/verify", nil, http.StatusOK)
	if verify["overall"] == nil {
		t.Fatalf("verify = %s", toJSONString(t, verify))
	}
}

func TestSystemWriteEndpointsWithNavAndHoldings(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()

	router := newAuthedRouter(t, testCfg(), db, WithNavCrawler(&stubNav{}), WithHoldingsCrawler(&stubHoldings{}))

	nav := doJSONRequest(t, router, http.MethodPost, "/api/system/crawl-nav", nil, http.StatusOK)
	if nav["status"] != "complete" {
		t.Fatalf("crawl-nav = %s", toJSONString(t, nav))
	}
	holdings := doJSONRequest(t, router, http.MethodPost, "/api/system/crawl-holdings", nil, http.StatusOK)
	if holdings["status"] != "complete" {
		t.Fatalf("crawl-holdings = %s", toJSONString(t, holdings))
	}
}

func TestSystemEndpointsRequireSession(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()

	router := NewRouter(testCfg(), WithDB(db), WithAuth(newTestAuthService(t, db)), WithNavCrawler(&stubNav{}))

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/system/status"},
		{http.MethodGet, "/api/system/jobs"},
		{http.MethodGet, "/api/system/integrity"},
		{http.MethodGet, "/api/system/audit"},
		{http.MethodGet, "/api/system/agent"},
		{http.MethodPost, "/api/system/verify"},
		{http.MethodPost, "/api/system/crawl-nav"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without session = %d, want 401; body=%s", tc.method, tc.path, res.Code, res.Body.String())
		}
	}
}

func TestSystemWriteRequiresCSRFHeader(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()
	svc := newTestAuthService(t, db)
	token := loginTestUser(t, svc)
	router := NewRouter(testCfg(), WithDB(db), WithAuth(svc), WithNavCrawler(&stubNav{}))

	// 有 session 但缺 CSRF 头 → 403 csrf_header_required。
	req := httptest.NewRequest(http.MethodPost, "/api/system/verify", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("verify without CSRF = %d, want 403; body=%s", res.Code, res.Body.String())
	}
}

func TestSystemJobsReportSchedulerRuntime(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()

	fake := func() []jobs.JobStatus {
		return []jobs.JobStatus{
			{Name: "price_dca", Schedule: "daily 20:00 CST", LastRun: 123456, LastError: "", NextRun: 789012},
			{Name: "wal", Schedule: "daily 03:00 CST", LastRun: 0, NextRun: 456789},
		}
	}
	router := newAuthedRouter(t, testCfg(), db, WithJobStatus(fake))
	resp := doJSONRequest(t, router, http.MethodGet, "/api/system/jobs", nil, http.StatusOK)
	list := resp["jobs"].([]any)
	if len(list) != 2 {
		t.Fatalf("jobs = %s", toJSONString(t, resp))
	}
	first := list[0].(map[string]any)
	if first["name"] != "price_dca" || first["last_run"].(float64) != 123456 || first["next_run"].(float64) != 789012 {
		t.Fatalf("first job = %s", toJSONString(t, first))
	}
}

func TestSystemAuditMergesAuthAndAgentTimeline(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()

	// agent_audit_events 表 + 一行(Set schema identically to EnsureSchema)。
	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE agent_audit_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL, caller TEXT NOT NULL, tool TEXT NOT NULL,
			event_type TEXT NOT NULL, status TEXT NOT NULL, scope TEXT NOT NULL,
			permission TEXT NOT NULL, risk_level TEXT NOT NULL,
			redacted_args_json TEXT NOT NULL, result_summary_json TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`); err != nil {
		t.Fatalf("create agent_audit_events: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO agent_audit_events (request_id, caller, tool, event_type, status, scope,
			permission, risk_level, redacted_args_json, result_summary_json, created_at)
		VALUES ('r1', 'cli', 'portfolio.summary', 'port_read', 'result', 'read', 'allowed', 'low', '{}', '{}', ?)
	`, now); err != nil {
		t.Fatalf("insert agent event: %v", err)
	}

	// 手动记录一条 auth 事件(service 层直写——handler 层事件由 auth 端点测试覆盖)。
	svc := newTestAuthService(t, db)
	svc.RecordAuthEvent(context.Background(), "login_ok", "203.0.113.7", "test-agent", "")

	router := newAuthedRouter(t, testCfg(), db, WithDBDriver("sqlite"))
	audit := doJSONRequest(t, router, http.MethodGet, "/api/system/audit?limit=50", nil, http.StatusOK)
	events := audit["events"].([]any)
	if len(events) != 2 {
		t.Fatalf("events = %s, want 2 rows", toJSONString(t, audit))
	}
	// 时间倒序:auth 事件 ts = now(新),agent 事件 ts = now(新)但秒级可能相同;
	// 无严格顺序断言,只验证两类都出现。
	seen := map[string]bool{}
	for _, raw := range events {
		ev := raw.(map[string]any)
		seen[ev["kind"].(string)] = true
		if ev["ts"].(float64) <= 0 {
			t.Fatalf("entry ts <= 0: %s", toJSONString(t, ev))
		}
	}
	if !seen["auth"] || !seen["agent"] {
		t.Fatalf("timeline missing kinds: %v", seen)
	}
}

// TestSystemStatusReportsDBSizeForResolvedSQLiteDriver pins the driver
// source-of-truth: FUND_DB_DRIVER is optional, so cfg.DBDriver is empty in a
// default deployment while the router has already resolved the driver to
// "sqlite". Gating db_size_bytes on the raw config value made the endpoint
// report db_driver=sqlite and never emit the size; the gate must use the
// resolved driver.
func TestSystemStatusReportsDBSizeForResolvedSQLiteDriver(t *testing.T) {
	db := openSPAExtensionFixture(t)
	defer db.Close()

	// No FUND_DB_PATH configured -> the optional field stays absent.
	withoutPath := doJSONRequest(t, newAuthedRouter(t, testCfg(), db, WithDBDriver("sqlite")),
		http.MethodGet, "/api/system/status", nil, http.StatusOK)
	if _, present := withoutPath["db_size_bytes"]; present {
		t.Fatalf("db_size_bytes present without FUND_DB_PATH: %s", toJSONString(t, withoutPath))
	}
	if withoutPath["db_driver"] != "sqlite" {
		t.Fatalf("db_driver = %v, want sqlite", withoutPath["db_driver"])
	}

	dbFile := filepath.Join(t.TempDir(), "fund.db")
	if err := os.WriteFile(dbFile, make([]byte, 4096), 0o600); err != nil {
		t.Fatalf("write stub db file: %v", err)
	}
	cfg := testCfg() // DBDriver deliberately left empty, as in a default deployment
	cfg.DBPath = dbFile
	withPath := doJSONRequest(t, newAuthedRouter(t, cfg, db, WithDBDriver("sqlite")),
		http.MethodGet, "/api/system/status", nil, http.StatusOK)
	size, present := withPath["db_size_bytes"].(float64)
	if !present {
		t.Fatalf("db_size_bytes missing with a resolved sqlite driver: %s", toJSONString(t, withPath))
	}
	if size != 4096 {
		t.Fatalf("db_size_bytes = %v, want 4096", size)
	}
}
