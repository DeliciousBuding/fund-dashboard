package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

)

func TestAdminFreshnessReportsHeldWatchlistAndStalePriceGaps(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	for _, stmt := range []string{
		"DELETE FROM nav_history",
		"DELETE FROM portfolio_snapshot",
		"DELETE FROM fund_details",
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES
			('HELD1', 'Held Missing NAV', '股票型', 'fund', 'CN'),
			('WATCH1', 'Watch Missing NAV', 'ETF', 'stock', 'SH'),
			('STALE1', 'Stale Fund', '指数型', 'fund', 'CN'),
			('FRESH1', 'Fresh Fund', '指数型', 'fund', 'CN')`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES
			('HELD1', 'Held Missing NAV', 10, -10, 1, 10, 0, 0, 'fund', 1),
			('STALE1', 'Stale Fund', 10, -10, 1, 10, 0, 0, 'fund', 1),
			('FRESH1', 'Fresh Fund', 10, -10, 1, 10, 0, 0, 'fund', 1)`,
		`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type) VALUES
			('STALE1', date('now', '-10 day'), 1.0, 0, 'fund'),
			('FRESH1', date('now'), 1.0, 0, 'fund')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec fixture statement %q: %v", stmt, err)
		}
	}

	router := NewRouter(testCfg(), WithDB(db))

	result := doJSONRequest(t, router, http.MethodGet, "/api/admin/freshness", nil, http.StatusOK)
	if result["decision_boundary"] != "read_only" {
		t.Fatalf("decision_boundary = %v, want read_only", result["decision_boundary"])
	}
	if result["health"] != "degraded" {
		t.Fatalf("health = %v, want degraded; body=%s", result["health"], toJSONString(t, result))
	}
	if result["last_nav_date"] == "" {
		t.Fatalf("last_nav_date = %v, want non-empty", result["last_nav_date"])
	}
	if result["anomaly_count"].(float64) != 0 {
		t.Fatalf("anomaly_count = %v, want 0 without anomaly column", result["anomaly_count"])
	}

	body := toJSONString(t, result)
	missingNAV := result["missing_nav_securities"].([]any)
	if !jsonArrayContainsCode(missingNAV, "HELD1") {
		t.Fatalf("missing held NAV gap in %s", body)
	}
	if jsonArrayContainsCode(missingNAV, "WATCH1") {
		t.Fatalf("watchlist security leaked into held missing list: %s", body)
	}
	if !jsonArrayContainsCode(result["watchlist_missing_nav_securities"].([]any), "WATCH1") {
		t.Fatalf("missing watchlist NAV gap in %s", body)
	}
	if !jsonArrayContainsCode(result["stale_securities"].([]any), "STALE1") || !strings.Contains(body, `"stale_days":10`) {
		t.Fatalf("missing stale security evidence in %s", body)
	}
	if !strings.Contains(body, "crawl_nav") {
		t.Fatalf("actionable guidance should name crawl_nav without executing it: %s", body)
	}
	if strings.Contains(body, "backup") || strings.Contains(body, "trade") {
		t.Fatalf("freshness response should stay diagnostic-only: %s", body)
	}
}

func jsonArrayContainsCode(items []any, code string) bool {
	for _, item := range items {
		row, ok := item.(map[string]any)
		if ok && row["code"] == code {
			return true
		}
	}
	return false
}
