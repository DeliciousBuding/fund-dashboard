package jobs_test

import (
	"context"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
)

func TestRefreshSecurity_FiltersOlderPoints(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO nav_history (date, fund_code, unit_nav, daily_change_pct, security_type)
		VALUES
			('2026-07-01', '019173', 1.20, 0, 'fund'),
			('2026-07-02', '019173', 1.21, 0.8, 'fund')
	`); err != nil {
		t.Fatal(err)
	}

	stub := &stubPriceSource{
		points: []datasource.PricePoint{
			{Date: "2026-07-01", Price: 1.20, ChangePct: 0},
			{Date: "2026-07-02", Price: 1.21, ChangePct: 0.8},
			{Date: "2026-07-03", Price: 1.25, ChangePct: 3.3},
		},
	}
	refresher := jobs.NewPriceRefresher(db, jobs.WithSource(datasource.TypeFund, stub))
	res, err := refresher.RefreshSecurity(context.Background(), "019173", datasource.TypeFund)
	if err != nil {
		t.Fatal(err)
	}
	// Only the new day is inserted; same-day point may re-upsert as 0 added.
	if res.Added != 1 {
		t.Fatalf("added=%d want 1", res.Added)
	}
	if res.Latest != "2026-07-03" {
		t.Fatalf("latest=%s want 2026-07-03", res.Latest)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nav_history WHERE fund_code='019173'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("rows=%d want 3", count)
	}
}

func TestRefreshStaleHeld_SkipsFresh(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// Freshness queries need trade_time on transactions.
	if _, err := db.Exec(`ALTER TABLE transactions ADD COLUMN trade_time TEXT`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE transactions SET trade_time = '2099-01-01 10:00:00'`); err != nil {
		t.Fatal(err)
	}

	// Fresh NAV for held 019173 (far-future so always fresh vs wall clock).
	if _, err := db.Exec(`
		INSERT INTO nav_history (date, fund_code, unit_nav, daily_change_pct, security_type)
		VALUES ('2099-01-01', '019173', 1.5, 0, 'fund')
	`); err != nil {
		t.Fatal(err)
	}
	// Missing NAV second held security.
	if _, err := db.Exec(`
		INSERT INTO fund_details VALUES ('000001', '华夏成长', 'fund');
		INSERT INTO portfolio_snapshot
			(fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
			VALUES ('000001', '华夏成长', 10, -100, 0, 0, 0, 0, 'fund', 1);
	`); err != nil {
		t.Fatal(err)
	}

	var fetched []string
	stub := &recordingSource{points: []datasource.PricePoint{
		{Date: "2099-01-02", Price: 2.0, ChangePct: 1},
	}, onCode: func(code string) { fetched = append(fetched, code) }}
	refresher := jobs.NewPriceRefresher(db,
		jobs.WithDBDriver("sqlite"),
		jobs.WithSource(datasource.TypeFund, stub),
	)
	securities, _, err := refresher.RefreshStaleHeld(context.Background())
	if err != nil {
		t.Fatalf("RefreshStaleHeld: %v", err)
	}
	if securities != 1 {
		t.Fatalf("securities=%d want 1 (only stale/missing)", securities)
	}
	if len(fetched) != 1 || fetched[0] != "000001" {
		t.Fatalf("fetched=%v want only 000001", fetched)
	}
}

type recordingSource struct {
	points []datasource.PricePoint
	onCode func(string)
	err    error
}

func (s *recordingSource) FetchHistory(_ context.Context, code string) ([]datasource.PricePoint, error) {
	if s.onCode != nil {
		s.onCode(code)
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.points, nil
}

func (s *recordingSource) FetchMeta(_ context.Context, _ string) (*datasource.FundMeta, error) {
	return nil, s.err
}
