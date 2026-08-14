package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

)

func TestAdminDashboardReportsReadOnlySystemAndDataState(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(testCfg(), WithDB(db))

	result := doJSONRequest(t, router, http.MethodGet, "/api/admin/dashboard", nil, http.StatusOK)
	if result["ok"] != true {
		t.Fatalf("ok = %v, want true; body=%s", result["ok"], toJSONString(t, result))
	}
	if result["decision_boundary"] != "read_only" {
		t.Fatalf("decision_boundary = %v, want read_only", result["decision_boundary"])
	}
	if _, ok := result["timestamp"].(string); !ok {
		t.Fatalf("timestamp = %#v, want string", result["timestamp"])
	}
	if result["response_ms"].(float64) < 0 {
		t.Fatalf("response_ms = %v, want non-negative", result["response_ms"])
	}

	system := result["system"].(map[string]any)
	if system["uptime_sec"].(float64) < 0 {
		t.Fatalf("uptime_sec = %v, want non-negative", system["uptime_sec"])
	}
	if system["uptime_human"] == "" {
		t.Fatalf("uptime_human = %v, want non-empty", system["uptime_human"])
	}
	memory := system["memory"].(map[string]any)
	if memory["rss_mb"].(float64) <= 0 {
		t.Fatalf("rss_mb = %v, want positive", memory["rss_mb"])
	}
	if system["go_version"] == "" || system["platform"] == "" {
		t.Fatalf("system = %#v, want Go version and platform", system)
	}
	if system["build_version"] != "test" {
		t.Fatalf("build_version = %#v, want test (FUND_VERSION / cfg.Version)", system["build_version"])
	}

	database := result["database"].(map[string]any)
	if database["size_bytes"].(float64) <= 0 {
		t.Fatalf("database size_bytes = %v, want positive", database["size_bytes"])
	}

	crawler := result["crawler"].(map[string]any)
	if crawler["nav_total"].(float64) != 2 {
		t.Fatalf("crawler nav_total = %v, want 2", crawler["nav_total"])
	}
	if crawler["success_rate_pct"].(float64) < 0 {
		t.Fatalf("crawler success_rate_pct = %v, want non-negative", crawler["success_rate_pct"])
	}

	state := result["state"].(map[string]any)
	if state["transaction_count"].(float64) != 2 {
		t.Fatalf("transaction_count = %v, want 2", state["transaction_count"])
	}
	if state["last_transaction"] != "2026-06-01T09:00:00Z" {
		t.Fatalf("last_transaction = %v, want fixture timestamp", state["last_transaction"])
	}
	if state["last_nav_date"] != "2026-06-18" {
		t.Fatalf("last_nav_date = %v, want fixture date", state["last_nav_date"])
	}
	if state["held_funds"].(float64) != 2 || state["nav_records"].(float64) != 2 || state["nav_funds"].(float64) != 2 {
		t.Fatalf("state = %s, want held/nav counts 2", toJSONString(t, state))
	}
	if state["securities_total"].(float64) != 2 {
		t.Fatalf("securities_total = %v, want 2", state["securities_total"])
	}
	if state["anomaly_count"].(float64) != 0 {
		t.Fatalf("anomaly_count = %v, want 0", state["anomaly_count"])
	}
}


func TestOpsDashboardRequiresEdgeKeyAndReturnsGoVersion(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db))

	// Missing edge key → 401
	req := httptest.NewRequest(http.MethodGet, "/api/ops/dashboard", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("no edge key status = %d, want 401; body=%s", res.Code, res.Body.String())
	}

	// With edge key → 200 and go_version
	req = httptest.NewRequest(http.MethodGet, "/api/ops/dashboard", nil)
	req.Header.Set(edgeKeyHeader, testEdgeKey)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("edge key status = %d, want 200; body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"go_version"`) {
		t.Fatalf("ops dashboard missing go_version: %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"build_version":"test"`) {
		t.Fatalf("ops dashboard missing build_version: %s", res.Body.String())
	}
	if strings.Contains(res.Body.String(), `"node_version"`) {
		t.Fatalf("ops dashboard should not expose node_version: %s", res.Body.String())
	}
}
