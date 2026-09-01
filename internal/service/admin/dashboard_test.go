package admin

import (
	"context"
	"testing"
	"time"
)

func TestDashboardNAVFreshUsesStalePriceWindowNot24h(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// Two held funds: one with NAV 2 calendar days ago (fresh under stalePriceDays=4),
	// one with NAV 10 days ago (stale).
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES
			('FRESH2', 'Fresh Fund', 'fund'),
			('OLD2', 'Old Fund', 'fund');
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES
			('FRESH2', 'Fresh Fund', 10, -10, 1, 10, 0, 0, 'fund', 1),
			('OLD2', 'Old Fund', 5, -5, 1, 5, 0, 0, 'fund', 1);
		INSERT INTO nav_history (fund_code, date, unit_nav) VALUES
			('FRESH2', '2026-07-15', 1.0),
			('OLD2', '2026-07-01', 1.0);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := NewServiceWithDriver(db, "sqlite")
	report, err := svc.GetDashboard(context.Background(), now.Add(-time.Hour), now)
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}
	if report.Crawler.FreshWindowDays != stalePriceDays {
		t.Fatalf("FreshWindowDays=%d want %d", report.Crawler.FreshWindowDays, stalePriceDays)
	}
	// Strict 24h wall-clock from 2026-07-17 would only count dates >= 2026-07-16 → 0.
	// Window of 4 calendar days counts dates >= 2026-07-14 → FRESH2 only.
	if report.Crawler.NAVFresh != 1 {
		t.Fatalf("NAVFresh=%d want 1 (fund T+1 window)", report.Crawler.NAVFresh)
	}
	if report.Crawler.NAVFresh24H != report.Crawler.NAVFresh {
		t.Fatalf("legacy NAVFresh24H=%d want alias of NAVFresh=%d", report.Crawler.NAVFresh24H, report.Crawler.NAVFresh)
	}
	if report.Crawler.SuccessRatePct != 50 {
		t.Fatalf("SuccessRatePct=%v want 50 (1/2 held)", report.Crawler.SuccessRatePct)
	}
}
