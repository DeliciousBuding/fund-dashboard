package admin

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/chinatime"
)

func seedAlertHeldFund(t *testing.T, db *sql.DB, code, name, market string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO fund_details (fund_code, fund_name, security_type, market) VALUES (?, ?, 'fund', ?)`, code, name, market); err != nil {
		t.Fatalf("seed fund_details: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
		VALUES (?, ?, 10, -10, 1, 10, 0, 0, 'fund', 1)`, code, name); err != nil {
		t.Fatalf("seed portfolio_snapshot: %v", err)
	}
}

func TestCheckAlertsDefaultsAndPortfolioClamp(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	res, err := svc.CheckAlerts(context.Background(), CheckAlertsInput{PriceChangePct: 0, DrawdownPct: -1, StaleDays: 0, PortfolioID: 5000})
	if err != nil {
		t.Fatalf("CheckAlerts: %v", err)
	}
	if !res.OK {
		t.Fatalf("OK = false")
	}
	if res.PriceChangePct != 5 || res.DrawdownPct != 10 || res.StaleDays != 4 {
		t.Fatalf("defaults = %v/%v/%d, want 5/10/4", res.PriceChangePct, res.DrawdownPct, res.StaleDays)
	}
	if res.PortfolioID != 1000 {
		t.Fatalf("PortfolioID = %d, want clamped 1000", res.PortfolioID)
	}
	if res.DecisionBoundary != "facts_only" || res.SideEffects != "none" || res.WebhookSent {
		t.Fatalf("boundary = %q side_effects = %q webhook = %v", res.DecisionBoundary, res.SideEffects, res.WebhookSent)
	}
	if res.CheckedAt == "" {
		t.Fatal("CheckedAt empty")
	}
	if res.Count != 0 || len(res.Alerts) != 0 {
		t.Fatalf("count = %d alerts = %v, want none on empty db", res.Count, res.Alerts)
	}
}

func TestCheckAlertsPriceThresholdBoundaries(t *testing.T) {
	today := time.Now().In(chinatime.Loc).Format("2006-01-02")
	cases := []struct {
		name      string
		change    float64
		threshold float64
		wantKind  string
		wantSev   string
	}{
		{"below threshold no alert", 4.99, 5, "", ""},
		{"exactly threshold medium", 5.0, 5, "price_change", "medium"},
		{"twice threshold high", 10.0, 5, "price_change", "high"},
		{"negative exactly threshold", -5.0, 5, "price_change", "medium"},
		{"negative twice threshold", -10.0, 5, "price_change", "high"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testDB(t)
			defer db.Close()
			seedAlertHeldFund(t, db, "019173", "Fund", "CN")
			if _, err := db.Exec(`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct) VALUES ('019173', ?, 1.0, ?)`, today, tc.change); err != nil {
				t.Fatal(err)
			}
			svc := NewServiceWithDriver(db, "sqlite")
			res, err := svc.CheckAlerts(context.Background(), CheckAlertsInput{PriceChangePct: tc.threshold, StaleDays: 3650})
			if err != nil {
				t.Fatal(err)
			}
			var price *AlertItem
			for i := range res.Alerts {
				if res.Alerts[i].Kind == "price_change" {
					price = &res.Alerts[i]
				}
			}
			if tc.wantKind == "" {
				if price != nil {
					t.Fatalf("unexpected price_change alert: %+v", price)
				}
				return
			}
			if price == nil {
				t.Fatalf("no price_change alert in %+v", res.Alerts)
			}
			if price.Severity != tc.wantSev {
				t.Fatalf("severity = %q, want %q", price.Severity, tc.wantSev)
			}
			if price.Value == nil || *price.Value != tc.change {
				t.Fatalf("value = %v, want %v", price.Value, tc.change)
			}
			if price.Threshold == nil || *price.Threshold != tc.threshold {
				t.Fatalf("threshold = %v, want %v", price.Threshold, tc.threshold)
			}
			if price.SecurityType != "fund" || price.Market != "CN" || price.AsOf != today {
				t.Fatalf("security_type=%q market=%q as_of=%q", price.SecurityType, price.Market, price.AsOf)
			}
		})
	}
}

func TestCheckAlertsDrawdownThresholdBoundaries(t *testing.T) {
	d1 := time.Now().In(chinatime.Loc).AddDate(0, 0, -2).Format("2006-01-02")
	d2 := time.Now().In(chinatime.Loc).AddDate(0, 0, -1).Format("2006-01-02")
	cases := []struct {
		name   string
		trough float64
		ddPct  float64
		wantDD float64
		want   string
	}{
		{"below threshold no alert", 1.95, 5, 2.5, ""},
		{"above threshold medium", 1.76, 10, 12, "medium"},
		{"one and a half threshold high", 1.7, 10, 15, "high"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testDB(t)
			defer db.Close()
			seedAlertHeldFund(t, db, "019173", "Fund", "")
			if _, err := db.Exec(`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct) VALUES
				('019173', ?, 2.0, 0),
				('019173', ?, ?, 0)`, d1, d2, tc.trough); err != nil {
				t.Fatal(err)
			}
			svc := NewServiceWithDriver(db, "sqlite")
			res, err := svc.CheckAlerts(context.Background(), CheckAlertsInput{DrawdownPct: tc.ddPct})
			if err != nil {
				t.Fatal(err)
			}
			var dd *AlertItem
			for i := range res.Alerts {
				if res.Alerts[i].Kind == "drawdown" {
					dd = &res.Alerts[i]
				}
			}
			if tc.want == "" {
				if dd != nil {
					t.Fatalf("unexpected drawdown alert: %+v", dd)
				}
				return
			}
			if dd == nil {
				t.Fatalf("no drawdown alert in %+v", res.Alerts)
			}
			if dd.Severity != tc.want {
				t.Fatalf("severity = %q, want %q", dd.Severity, tc.want)
			}
			if dd.Value == nil || *dd.Value < tc.wantDD-0.001 || *dd.Value > tc.wantDD+0.001 {
				t.Fatalf("value = %v, want ~%v", dd.Value, tc.wantDD)
			}
			if dd.Threshold == nil || *dd.Threshold != tc.ddPct {
				t.Fatalf("threshold = %v, want %v", dd.Threshold, tc.ddPct)
			}
		})
	}
}

func TestCheckAlertsStaleThresholdBoundaries(t *testing.T) {
	cases := []struct {
		name      string
		daysAgo   int
		staleDays int
		want      bool
		wantDays  int
	}{
		{"three days under default threshold", 3, 4, false, 0},
		{"exactly threshold stale", 4, 4, true, 4},
		{"far beyond threshold stale", 10, 4, true, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testDB(t)
			defer db.Close()
			seedAlertHeldFund(t, db, "019173", "Fund", "")
			// stale days are computed against the China market calendar in production;
			// navDate must be seeded on the same clock or the test drifts by a day
			// when the runner TZ date differs from the China date (e.g. UTC runners).
			navDate := time.Now().In(chinatime.Loc).AddDate(0, 0, -tc.daysAgo).Format("2006-01-02")
			if _, err := db.Exec(`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct) VALUES ('019173', ?, 1.0, 0)`, navDate); err != nil {
				t.Fatal(err)
			}
			svc := NewServiceWithDriver(db, "sqlite")
			res, err := svc.CheckAlerts(context.Background(), CheckAlertsInput{StaleDays: tc.staleDays})
			if err != nil {
				t.Fatal(err)
			}
			var stale *AlertItem
			for i := range res.Alerts {
				if res.Alerts[i].Kind == "stale_nav" {
					stale = &res.Alerts[i]
				}
			}
			if !tc.want {
				if stale != nil {
					t.Fatalf("unexpected stale_nav alert: %+v", stale)
				}
				return
			}
			if stale == nil {
				t.Fatalf("no stale_nav alert in %+v", res.Alerts)
			}
			if stale.Severity != "low" {
				t.Fatalf("severity = %q, want low", stale.Severity)
			}
			if stale.Value == nil || int(*stale.Value) != tc.wantDays {
				t.Fatalf("value = %v, want days %d", stale.Value, tc.wantDays)
			}
			if stale.Threshold == nil || int(*stale.Threshold) != tc.staleDays {
				t.Fatalf("threshold = %v, want %d", stale.Threshold, tc.staleDays)
			}
			if stale.AsOf != navDate {
				t.Fatalf("as_of = %q, want %q", stale.AsOf, navDate)
			}
		})
	}
}

func TestCheckAlertsDCADayAlerts(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	// One plan matches every weekday (always today), one has an empty mask (never).
	// Full column list: the production dca_plans table declares NOT NULL on
	// frequency/trade_type/source/created_at/updated_at without defaults.
	if _, err := db.Exec(`INSERT INTO dca_plans (fund_code, fund_name, amount, frequency, weekday_mask, trade_type, portfolio_id, start_date, active, source, created_at, updated_at) VALUES
		('019173', 'Fund A', 100, 'weekday', '1,2,3,4,5,6,7', 'buy', 1, '2026-01-01', 1, 'manual', '2026-01-01', '2026-01-01'),
		('019174', 'Fund B', 200, 'weekday', '', 'buy', 1, '2026-01-01', 1, 'manual', '2026-01-01', '2026-01-01'),
		('019175', 'Fund C', 300, 'weekday', '1,2,3,4,5,6,7', 'buy', 2, '2026-01-01', 1, 'manual', '2026-01-01', '2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	svc := NewServiceWithDriver(db, "sqlite")
	res, err := svc.CheckAlerts(context.Background(), CheckAlertsInput{})
	if err != nil {
		t.Fatal(err)
	}
	var dca []AlertItem
	for _, a := range res.Alerts {
		if a.Kind == "dca_day" {
			dca = append(dca, a)
		}
	}
	// Portfolio 1 only: Fund A fires, Fund B is masked out, Fund C is another portfolio.
	if len(dca) != 1 {
		t.Fatalf("dca_day alerts = %+v, want exactly 1", dca)
	}
	if dca[0].Code != "019173" || dca[0].Name != "Fund A" || dca[0].Severity != "info" {
		t.Fatalf("dca alert = %+v", dca[0])
	}
	if dca[0].Value == nil || *dca[0].Value != 100 {
		t.Fatalf("value = %v, want 100", dca[0].Value)
	}
	if dca[0].AsOf != time.Now().In(chinatime.Loc).Format("2006-01-02") {
		t.Fatalf("as_of = %q, want today (china loc)", dca[0].AsOf)
	}
}

func TestWeekdayMaskHitTable(t *testing.T) {
	cases := []struct {
		mask string
		day  int
		want bool
	}{
		{"1,3,5", 1, true},
		{"1,3,5", 2, false},
		{"1,3,5", 7, false},
		{"1-5", 5, true},
		{"1-5", 6, false},
		{"5-1", 3, false},
		{" 1 , 3 ", 3, true},
		{"1,,3", 3, true},
		{"", 1, false},
		{" ", 1, false},
		{"abc", 1, false},
		{"0", 0, true},
	}
	for _, tc := range cases {
		if got := weekdayMaskHit(tc.mask, tc.day); got != tc.want {
			t.Fatalf("weekdayMaskHit(%q, %d) = %v, want %v", tc.mask, tc.day, got, tc.want)
		}
	}
}

func TestMaxDrawdownPctEmptyAndSinglePoint(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	// No history at all.
	dd, peak, trough, err := svc.maxDrawdownPct(context.Background(), "019173")
	if err != nil {
		t.Fatal(err)
	}
	if dd != 0 || peak != "" || trough != "" {
		t.Fatalf("empty history = %v/%q/%q, want 0//", dd, peak, trough)
	}

	// A single point has no drawdown.
	if _, err := db.Exec(`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('019173', '2026-01-01', 1.5)`); err != nil {
		t.Fatal(err)
	}
	dd, peak, trough, err = svc.maxDrawdownPct(context.Background(), "019173")
	if err != nil {
		t.Fatal(err)
	}
	if dd != 0 {
		t.Fatalf("single point dd = %v, want 0", dd)
	}

	// Strictly increasing history (above the earlier 1.5 single point) has no
	// drawdown. Dates start on 01-02 because the production nav_history PK is
	// (fund_code, date) and the single point already occupies 01-01.
	for i := 0; i < 5; i++ {
		nav := 2.0 + float64(i)
		date := time.Date(2026, 1, 2+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		if _, err := db.Exec(`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('019173', ?, ?)`, date, nav); err != nil {
			t.Fatal(err)
		}
	}
	dd, _, _, err = svc.maxDrawdownPct(context.Background(), "019173")
	if err != nil {
		t.Fatal(err)
	}
	if dd != 0 {
		t.Fatalf("increasing history dd = %v, want 0", dd)
	}
}
