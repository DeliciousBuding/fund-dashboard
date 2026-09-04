package admin

import (
	"context"
	"strings"
	"testing"
)

func TestFreshnessActionableBranches(t *testing.T) {
	cases := []struct {
		name           string
		stale, missing int
		wantContains   string
	}{
		{"stale wins", 1, 5, "crawl_nav"},
		{"missing only", 0, 2, "添加 2 只持仓证券的价格数据"},
		{"all healthy", 0, 0, "数据新鲜度正常"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := freshnessActionable(tc.stale, tc.missing)
			if !strings.Contains(got, tc.wantContains) {
				t.Fatalf("actionable = %q, want substring %q", got, tc.wantContains)
			}
		})
	}
}

func TestFreshnessHealthBranches(t *testing.T) {
	cases := []struct {
		name           string
		stale, missing int
		want           string
	}{
		{"all healthy fresh", 0, 0, "fresh"},
		{"one stale degraded", 1, 0, "degraded"},
		{"three stale degraded", 3, 0, "degraded"},
		{"four stale stale", 4, 0, "stale"},
		{"missing only degraded", 0, 1, "degraded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := freshnessHealth(tc.stale, tc.missing); got != tc.want {
				t.Fatalf("freshnessHealth(%d, %d) = %q, want %q", tc.stale, tc.missing, got, tc.want)
			}
		})
	}
}

func TestQueryAnomalyCountWithAnomalyColumn(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// transactions.anomaly ships in the production schema (EnsureSQLiteSchema
	// fixture), so the historical ALTER TABLE backfill is no longer needed here.
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO transactions (order_id, trade_time, direction, fund_code, anomaly) VALUES
			('A1', '2026-01-01', 'buy', '019173', 'bad date'),
			('A2', '2026-01-02', 'sell', '019174', 'suspicious'),
			('A3', '2026-01-03', 'buy', '019175', NULL)
	`); err != nil {
		t.Fatalf("seed anomalies: %v", err)
	}

	svc := NewServiceWithDriver(db, "sqlite")
	count, err := svc.queryAnomalyCount(context.Background())
	if err != nil {
		t.Fatalf("queryAnomalyCount: %v", err)
	}
	if count != 2 {
		t.Fatalf("anomaly count = %d, want 2 (NULL rows excluded)", count)
	}

	report, err := svc.GetFreshness(context.Background())
	if err != nil {
		t.Fatalf("GetFreshness: %v", err)
	}
	if report.AnomalyCount != 2 {
		t.Fatalf("report AnomalyCount = %d, want 2", report.AnomalyCount)
	}
}

func TestQueryStaleSecuritiesThresholdBoundary(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// Two held funds: NAV exactly 4 days old (not stale, HAVING > 4) and 5 days old (stale).
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES
			('EDGE4', 'Edge Fund', 'fund'),
			('EDGE5', 'Edge Fund 5', 'fund');
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES
			('EDGE4', 'Edge Fund', 10, -10, 1, 10, 0, 0, 'fund', 1),
			('EDGE5', 'Edge Fund 5', 10, -10, 1, 10, 0, 0, 'fund', 1);
		INSERT INTO nav_history (fund_code, date, unit_nav) VALUES
			('EDGE4', date('now', '-4 days'), 1.0),
			('EDGE5', date('now', '-5 days'), 1.0);
	`); err != nil {
		t.Fatalf("seed boundary funds: %v", err)
	}

	svc := NewServiceWithDriver(db, "sqlite")
	stale, err := svc.queryStaleSecurities(context.Background())
	if err != nil {
		t.Fatalf("queryStaleSecurities: %v", err)
	}
	byCode := map[string]StaleSecurity{}
	for _, s := range stale {
		byCode[s.Code] = s
	}
	if _, ok := byCode["EDGE4"]; ok {
		t.Fatalf("EDGE4 (4 days) reported stale: %+v", byCode["EDGE4"])
	}
	item, ok := byCode["EDGE5"]
	if !ok {
		t.Fatalf("EDGE5 (5 days) missing from stale list: %+v", stale)
	}
	if item.StaleDays != 5 {
		t.Fatalf("EDGE5 StaleDays = %d, want 5", item.StaleDays)
	}
}

func TestQueryMissingNAVSecuritiesWatchlistBranch(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// Watchlist fund (no held position), held fund without NAV, and a fund with NAV.
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES
			('WATCH1', 'Watch Fund', 'fund'),
			('HELD1', 'Held Fund', 'fund'),
			('OK1', 'OK Fund', 'fund');
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES
			('HELD1', 'Held Fund', 10, -10, 1, 10, 0, 0, 'fund', 1),
			('OK1', 'OK Fund', 10, -10, 1, 10, 0, 0, 'fund', 1);
		INSERT INTO nav_history (fund_code, date, unit_nav) VALUES
			('OK1', date('now'), 1.0);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := NewServiceWithDriver(db, "sqlite")
	held, err := svc.queryMissingNAVSecurities(context.Background(), true)
	if err != nil {
		t.Fatalf("held missing: %v", err)
	}
	if len(held) != 1 || held[0].Code != "HELD1" {
		t.Fatalf("held missing = %+v, want [HELD1]", held)
	}

	watch, err := svc.queryMissingNAVSecurities(context.Background(), false)
	if err != nil {
		t.Fatalf("watchlist missing: %v", err)
	}
	if len(watch) != 1 || watch[0].Code != "WATCH1" {
		t.Fatalf("watchlist missing = %+v, want [WATCH1]", watch)
	}
}

func TestDateOnlyPtrTruncates(t *testing.T) {
	full := "2026-01-01T12:00:00Z"
	if got := dateOnlyPtr(&full); got == nil || *got != "2026-01-01" {
		t.Fatalf("dateOnlyPtr(full) = %v, want 2026-01-01", got)
	}
	short := "2026-01-01"
	if got := dateOnlyPtr(&short); got == nil || *got != "2026-01-01" {
		t.Fatalf("dateOnlyPtr(short) = %v, want 2026-01-01", got)
	}
	if got := dateOnlyPtr(nil); got != nil {
		t.Fatalf("dateOnlyPtr(nil) = %v, want nil", got)
	}
}
