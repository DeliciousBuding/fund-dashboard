package jobs_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
	_ "modernc.org/sqlite"
)

// stubPriceSource returns canned history for testing.
type stubPriceSource struct {
	points []datasource.PricePoint
	meta   *datasource.FundMeta
	err    error
}

func (s *stubPriceSource) FetchHistory(_ context.Context, _ string) ([]datasource.PricePoint, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.points, nil
}

func (s *stubPriceSource) FetchMeta(_ context.Context, _ string) (*datasource.FundMeta, error) {
	return s.meta, s.err
}

// testDB creates an in-memory SQLite database with production-shaped tables.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE nav_history (
			date TEXT, fund_code TEXT, unit_nav REAL, daily_change_pct REAL DEFAULT 0,
			security_type TEXT DEFAULT 'fund',
			PRIMARY KEY (date, fund_code)
		)`,
		`CREATE TABLE portfolio_snapshot (
			fund_code TEXT NOT NULL,
			fund_name TEXT,
			held_shares REAL DEFAULT 0,
			total_cost REAL DEFAULT 0,
			latest_nav REAL,
			current_value REAL DEFAULT 0,
			unrealized_pnl REAL DEFAULT 0,
			pnl_pct REAL DEFAULT 0,
			security_type TEXT DEFAULT 'fund',
			portfolio_id INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (fund_code, portfolio_id)
		)`,
		`CREATE TABLE fund_details (
			fund_code TEXT PRIMARY KEY,
			fund_name TEXT,
			security_type TEXT DEFAULT 'fund'
		)`,
		`CREATE TABLE transactions (
			seq INTEGER PRIMARY KEY,
			fund_code TEXT,
			fund_name TEXT,
			signed_share_change REAL DEFAULT 0,
			signed_cash_flow REAL DEFAULT 0
		)`,
		`INSERT INTO fund_details VALUES ('019173', '纳斯达克100', 'fund')`,
		// cost is negative cash (buys spend money); shares positive
		`INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow)
			VALUES ('019173', '纳斯达克100', 100, -120)`,
		`INSERT INTO portfolio_snapshot
			(fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
			VALUES ('019173', '纳斯达克100', 100, -120, 0, 0, 0, 0, 'fund', 1)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	return db
}

func TestRefreshSecurity_PersistsPoints(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	stub := &stubPriceSource{
		points: []datasource.PricePoint{
			{Date: "2026-07-01", Price: 1.2345, ChangePct: 0.50},
			{Date: "2026-07-02", Price: 1.2400, ChangePct: 0.45},
		},
	}

	refresher := jobs.NewPriceRefresher(db,
		jobs.WithSource(datasource.TypeFund, stub),
	)

	result, err := refresher.RefreshSecurity(context.Background(), "019173", datasource.TypeFund)
	if err != nil {
		t.Fatalf("RefreshSecurity: %v", err)
	}
	if result.Added != 2 {
		t.Fatalf("expected 2 added, got %d", result.Added)
	}
	if result.Latest != "2026-07-02" {
		t.Fatalf("expected latest 2026-07-02, got %s", result.Latest)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM nav_history WHERE fund_code='019173'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows in nav_history, got %d", count)
	}

	// Snapshot must move to production-shaped latest_nav/current_value.
	var latestNAV, currentValue, heldShares, totalCost float64
	if err := db.QueryRow(`
		SELECT latest_nav, current_value, held_shares, total_cost
		FROM portfolio_snapshot WHERE fund_code='019173'
	`).Scan(&latestNAV, &currentValue, &heldShares, &totalCost); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if latestNAV != 1.2400 {
		t.Fatalf("latest_nav = %v, want 1.24", latestNAV)
	}
	if heldShares != 100 {
		t.Fatalf("held_shares = %v, want 100", heldShares)
	}
	if totalCost != -120 {
		t.Fatalf("total_cost = %v, want -120", totalCost)
	}
	wantValue := 100 * 1.2400
	if currentValue != wantValue {
		t.Fatalf("current_value = %v, want %v", currentValue, wantValue)
	}
}

func TestRefreshSecurity_IdempotentUpsert(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	stub := &stubPriceSource{
		points: []datasource.PricePoint{
			{Date: "2026-07-01", Price: 1.5000, ChangePct: 0.10},
		},
	}

	refresher := jobs.NewPriceRefresher(db,
		jobs.WithSource(datasource.TypeFund, stub),
	)

	first, err := refresher.RefreshSecurity(context.Background(), "019173", datasource.TypeFund)
	if err != nil {
		t.Fatal(err)
	}
	if first.Added != 1 {
		t.Fatalf("first added = %d, want 1", first.Added)
	}

	result, err := refresher.RefreshSecurity(context.Background(), "019173", datasource.TypeFund)
	if err != nil {
		t.Fatal(err)
	}
	// Unchanged history must not count as new rows (#87).
	if result.Added != 0 {
		t.Fatalf("second added = %d, want 0 for no-op upsert", result.Added)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM nav_history WHERE fund_code='019173'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row after idempotent upsert, got %d", count)
	}
}

func TestRefreshAllHeld_UsesHeldSnapshot(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	stub := &stubPriceSource{
		points: []datasource.PricePoint{
			{Date: "2026-07-03", Price: 2.0, ChangePct: 1.0},
		},
	}
	refresher := jobs.NewPriceRefresher(db, jobs.WithSource(datasource.TypeFund, stub))

	securities, added, err := refresher.RefreshAllHeld(context.Background())
	if err != nil {
		t.Fatalf("RefreshAllHeld: %v", err)
	}
	if securities != 1 {
		t.Fatalf("securities = %d, want 1", securities)
	}
	if added < 1 {
		t.Fatalf("added = %d, want >=1", added)
	}
}

func TestRecalcSnapshot_ZerosDustHeldShares(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// Fully sold fund: buy 100 + sell 100 leaves float residue in SUM.
	// Use near-zero signed_share_change to simulate production dust (~1e-15).
	if _, err := db.Exec(`DELETE FROM transactions WHERE fund_code = '019173'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM portfolio_snapshot WHERE fund_code = '019173'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow)
		VALUES
			('019173', '纳斯达克100', 100, -120),
			('019173', '纳斯达克100', -99.99999999999999, 130)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO nav_history (date, fund_code, unit_nav, daily_change_pct, security_type)
		VALUES ('2026-07-10', '019173', 1.5, 0, 'fund')
	`); err != nil {
		t.Fatal(err)
	}

	if err := jobs.RecalcSnapshot(context.Background(), db, "019173"); err != nil {
		t.Fatalf("RecalcSnapshot: %v", err)
	}

	var held, currentValue, unrealized, pnlPct float64
	if err := db.QueryRow(`
		SELECT held_shares, current_value, unrealized_pnl, pnl_pct FROM portfolio_snapshot WHERE fund_code = '019173'
	`).Scan(&held, &currentValue, &unrealized, &pnlPct); err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if held != 0 {
		t.Fatalf("held_shares = %v, want 0 after dust clamp (#90)", held)
	}
	if currentValue != 0 {
		t.Fatalf("current_value = %v, want 0 when held is dust", currentValue)
	}
	if unrealized != 0 || pnlPct != 0 {
		t.Fatalf("closed position unrealized/pnl = %v/%v, want 0", unrealized, pnlPct)
	}
}
