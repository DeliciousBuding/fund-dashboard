package jobs

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
	_ "modernc.org/sqlite"
)

func TestNavUpsertConflictTarget(t *testing.T) {
	cases := []struct {
		driver string
		want   string
	}{
		{"sqlite", "(fund_code, date)"},
		{"", "(fund_code, date)"},
		{"pg", "(date, fund_code)"},
		{"PG", "(date, fund_code)"},
	}
	for _, tc := range cases {
		if got := navUpsertConflictTarget(tc.driver); got != tc.want {
			t.Fatalf("navUpsertConflictTarget(%q) = %q, want %q", tc.driver, got, tc.want)
		}
	}
}

type failingPriceSource struct{}

func (failingPriceSource) FetchHistory(context.Context, string) ([]datasource.PricePoint, error) {
	return nil, errors.New("boom")
}

func (failingPriceSource) FetchMeta(context.Context, string) (*datasource.FundMeta, error) {
	return nil, nil
}

func TestRefreshCodesSurfacesErrorWhenAllFail(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, held_shares REAL, security_type TEXT, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, security_type TEXT)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL)`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, security_type, portfolio_id) VALUES ('F1', 1, 'fund', 1)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	r := NewPriceRefresher(db, WithSource(datasource.TypeFund, failingPriceSource{}))
	securities, added, err := r.RefreshCodes(context.Background(), []string{"F1"})
	if err == nil {
		t.Fatalf("RefreshCodes all-failed: err=nil securities=%d added=%d, want error", securities, added)
	}
	if securities != 0 {
		t.Fatalf("securities=%d, want 0", securities)
	}
}

func TestTickNilIndicesRefresherDoesNotPanic(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, held_shares REAL, security_type TEXT, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, security_type TEXT)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	s := NewScheduler(NewPriceRefresher(db), db).
		WithDCARunner(nil).
		WithMarketIndicesRefresher(nil)
	now := time.Date(2026, 7, 15, 20, 3, 0, 0, cst) // Wednesday 20:00 window
	s.tick(now)
	for _, entry := range s.StatusSnapshot() {
		if entry.Name == "price_dca" && entry.LastError != "" {
			t.Fatalf("price_dca last_error = %q, want empty (no held rows, indices disabled, no DCA)", entry.LastError)
		}
	}
}

type failingHoldingsSource struct{}

func (failingHoldingsSource) FetchHoldings(context.Context, string, int) ([]datasource.FundHolding, error) {
	return nil, errors.New("boom")
}

func TestCrawlAllHeldSurfacesErrorWhenAllFundsFail(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, held_shares REAL, security_type TEXT, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,
		`INSERT INTO portfolio_snapshot (fund_code, held_shares, security_type, portfolio_id) VALUES ('F1', 1, 'fund', 1)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	r := &HoldingsRefresher{db: db, source: failingHoldingsSource{}}
	funds, added, err := r.CrawlAllHeld(context.Background())
	if err == nil {
		t.Fatalf("CrawlAllHeld all-failed: err=nil funds=%d added=%d, want error", funds, added)
	}
	if funds != 0 {
		t.Fatalf("funds=%d, want 0", funds)
	}
}
