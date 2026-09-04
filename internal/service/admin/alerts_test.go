package admin

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	db "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
	dbpkg "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
)

func TestCheckAlertsFindsPriceAndStale(t *testing.T) {
	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	// Production schema via the real boot path (portfolio_snapshot has no
	// market column there, matching production PG).
	if err := dbpkg.EnsureSQLiteSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO fund_details (fund_code, fund_name, security_type, market) VALUES ('019173','Fund','fund','CN')`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
			VALUES ('019173','Fund',100,-100,1.0,100,0,0,'fund',1)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	// peak then trough + large daily change on stale date
	if _, err := db.Exec(`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct) VALUES ('019173', '2026-01-01', 2.0, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct) VALUES (?, ?, ?, ?)`, "019173", old, 1.0, -16.67); err != nil {
		t.Fatal(err)
	}
	svc := NewServiceWithDriver(db, "sqlite")
	res, err := svc.CheckAlerts(context.Background(), CheckAlertsInput{PriceChangePct: 5, DrawdownPct: 10, StaleDays: 4})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count == 0 {
		t.Fatalf("expected alerts, got %+v", res)
	}
	kinds := map[string]bool{}
	for _, a := range res.Alerts {
		kinds[a.Kind] = true
	}
	if !kinds["price_change"] && !kinds["stale_nav"] && !kinds["drawdown"] {
		t.Fatalf("unexpected kinds: %+v", res.Alerts)
	}
	if res.WebhookSent {
		t.Fatal("webhook must not be sent")
	}
}

func TestWeekdayMaskHit(t *testing.T) {
	if !weekdayMaskHit("1,3,5", 3) {
		t.Fatal("expected hit")
	}
	if weekdayMaskHit("1,3,5", 2) {
		t.Fatal("expected miss")
	}
	if !weekdayMaskHit("1-5", 4) {
		t.Fatal("range hit")
	}
}

// #221: alert max drawdown must use the most recent NAV window, not the earliest.
func TestMaxDrawdownPctUsesRecentWindow(t *testing.T) {
	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := dbpkg.EnsureSQLiteSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	// Earliest flat history would yield ~0% DD if we only took ASC LIMIT N.
	for i := 0; i < 150; i++ {
		d := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format("2006-01-02")
		if _, err := db.Exec(`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('019173', ?, 1.0)`, d); err != nil {
			t.Fatal(err)
		}
	}
	// Recent peak → trough (50% drawdown).
	if _, err := db.Exec(`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES
		('019173', '2026-01-01', 2.0),
		('019173', '2026-01-02', 1.0)`); err != nil {
		t.Fatal(err)
	}
	svc := NewServiceWithDriver(db, "sqlite")
	dd, peak, trough, err := svc.maxDrawdownPct(context.Background(), "019173")
	if err != nil {
		t.Fatal(err)
	}
	if dd < 49.9 || dd > 50.1 {
		t.Fatalf("maxDrawdownPct = %v, want ~50 (recent window); peak=%s trough=%s", dd, peak, trough)
	}
	if peak != "2026-01-01" || trough != "2026-01-02" {
		t.Fatalf("peak/trough = %s/%s, want 2026-01-01/2026-01-02", peak, trough)
	}
}
