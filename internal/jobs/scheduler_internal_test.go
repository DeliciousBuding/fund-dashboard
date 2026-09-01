package jobs

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/chinatime"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	_ "modernc.org/sqlite"
)

type stubDCARunner struct {
	calls int
	last  portfoliosvc.RunDCAAutoInvestInput
	err   error
}

func (s *stubDCARunner) RunDCAAutoInvest(_ context.Context, in portfoliosvc.RunDCAAutoInvestInput) (portfoliosvc.RunDCAAutoInvestResult, error) {
	s.calls++
	s.last = in
	if s.err != nil {
		return portfoliosvc.RunDCAAutoInvestResult{}, s.err
	}
	return portfoliosvc.RunDCAAutoInvestResult{OK: true, Executed: 2, Skipped: 1}, nil
}

func TestTickWeekday20RunsDCAMaterialization(t *testing.T) {
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
	stub := &stubDCARunner{}
	s := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(stub)
	now := time.Date(2026, 7, 15, 20, 3, 0, 0, chinatime.Loc) // Wednesday
	s.tick(now)
	if stub.calls != 1 {
		t.Fatalf("dca calls=%d want 1", stub.calls)
	}
	if stub.last.AsOf != "2026-07-15" || stub.last.DryRun {
		t.Fatalf("input=%+v", stub.last)
	}
	if stub.last.PortfolioID != 1 {
		t.Fatalf("portfolio_id=%d", stub.last.PortfolioID)
	}
}

func TestTickWeekday20OncePerDay(t *testing.T) {
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
	stub := &stubDCARunner{}
	s := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(stub)
	day := time.Date(2026, 7, 15, 20, 0, 0, 0, chinatime.Loc)
	s.tick(day)
	s.tick(day.Add(5 * time.Minute))
	s.tick(day.Add(55 * time.Minute))
	if stub.calls != 1 {
		t.Fatalf("dca calls=%d want 1 for same day window", stub.calls)
	}
	// next weekday
	next := time.Date(2026, 7, 16, 20, 0, 0, 0, chinatime.Loc) // Thursday
	s.tick(next)
	if stub.calls != 2 {
		t.Fatalf("dca calls=%d want 2 after next day", stub.calls)
	}
	if stub.last.AsOf != "2026-07-16" {
		t.Fatalf("as_of=%s", stub.last.AsOf)
	}
}

func TestTickNonWindowDoesNotRunDCA(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stub := &stubDCARunner{}
	s := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(stub)
	now := time.Date(2026, 7, 15, 19, 0, 0, 0, chinatime.Loc)
	s.tick(now)
	if stub.calls != 0 {
		t.Fatalf("dca calls=%d want 0", stub.calls)
	}
}

// TestTickDailyWindowRefreshesPriceAndSignalsDCAWeekdaysOnly guards the
// availability fix: the 20:00 window must run on every calendar day (QDII
// publishes T+2), while DCA materialization — a financial decision — stays
// weekday-only.
func TestTickDailyWindowRefreshesPriceAndSignalsDCAWeekdaysOnly(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, held_shares REAL, security_type TEXT, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, security_type TEXT)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL)`,
		`CREATE TABLE crawl_log (fund_code TEXT PRIMARY KEY, source TEXT, rows_added INTEGER, latest_date TEXT, status TEXT, crawled_at TEXT)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	stub := &stubDCARunner{}
	s := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(stub)

	// Saturday 20:00 — price window runs, DCA must NOT materialize.
	saturday := time.Date(2026, 7, 18, 20, 1, 0, 0, chinatime.Loc)
	s.tick(saturday)
	if stub.calls != 0 {
		t.Fatalf("dca calls on saturday=%d want 0", stub.calls)
	}
	if got := s.lastRun["price_dca"]; got != "2026-07-18" {
		t.Fatalf("price_dca lastRun=%q want 2026-07-18 (saturday window should claim+run)", got)
	}

	// Sunday 20:00 — same: price runs, DCA skipped.
	sunday := time.Date(2026, 7, 19, 20, 1, 0, 0, chinatime.Loc)
	s.tick(sunday)
	if stub.calls != 0 {
		t.Fatalf("dca calls on sunday=%d want 0", stub.calls)
	}

	// Monday (weekday) 20:00 — price runs, DCA materializes.
	monday := time.Date(2026, 7, 20, 20, 1, 0, 0, chinatime.Loc)
	s.tick(monday)
	if stub.calls != 1 {
		t.Fatalf("dca calls on monday=%d want 1", stub.calls)
	}
	if stub.last.AsOf != "2026-07-20" {
		t.Fatalf("as_of=%s want 2026-07-20", stub.last.AsOf)
	}
}

func TestTickSaturdayRunsHoldingsPathOnce(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, held_shares REAL, security_type TEXT, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`); err != nil {
		t.Fatal(err)
	}
	s := NewScheduler(NewPriceRefresher(db), db)
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, chinatime.Loc) // Saturday
	s.tick(now)
	s.tick(now.Add(10 * time.Minute))
	// second tick same day should claim false; no panic is enough smoke
	if s.lastRun["holdings"] != "2026-07-18" {
		t.Fatalf("lastRun holdings=%q", s.lastRun["holdings"])
	}
}

func TestClaimWindow(t *testing.T) {
	s := &Scheduler{lastRun: map[string]string{}}
	if !s.claimWindow("price_dca", "2026-07-15") {
		t.Fatal("first claim should succeed")
	}
	if s.claimWindow("price_dca", "2026-07-15") {
		t.Fatal("second claim same window should fail")
	}
	if !s.claimWindow("price_dca", "2026-07-16") {
		t.Fatal("next day should succeed")
	}
}

func TestStartupCatchUpOncePerDay(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, held_shares REAL, security_type TEXT, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, security_type TEXT)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL)`,
		`CREATE TABLE crawl_log (fund_code TEXT, source TEXT, rows_added INTEGER, latest_date TEXT, status TEXT, crawled_at TEXT)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	s := NewScheduler(NewPriceRefresher(db), db)
	now := time.Date(2026, 7, 17, 8, 0, 0, 0, chinatime.Loc)
	s.runStartupCatchUp(now)
	s.runStartupCatchUp(now.Add(time.Hour))
	if s.lastRun["startup_refresh"] != "2026-07-17" {
		t.Fatalf("lastRun=%v", s.lastRun)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM crawl_log WHERE fund_code='__sched_startup_refresh' AND latest_date='2026-07-17'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("durable claims=%d want 1", n)
	}
	// new process simulation: clear memory, durable should still block
	s2 := NewScheduler(NewPriceRefresher(db), db)
	if s2.claimWindow("startup_refresh", "2026-07-17") {
		t.Fatal("second process same day should not claim startup_refresh")
	}
}

func TestDurableClaimWindowAcrossMemoryReset(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE crawl_log (fund_code TEXT, source TEXT, rows_added INTEGER, latest_date TEXT, status TEXT, crawled_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	s1 := NewScheduler(NewPriceRefresher(db), db)
	if !s1.claimWindow("price_dca", "2026-07-15") {
		t.Fatal("first claim")
	}
	s2 := NewScheduler(NewPriceRefresher(db), db)
	if s2.claimWindow("price_dca", "2026-07-15") {
		t.Fatal("durable claim should block second process")
	}
	if !s2.claimWindow("price_dca", "2026-07-16") {
		t.Fatal("next day should allow")
	}
}

func TestMultiJobDurableClaimsSameDay(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE crawl_log (fund_code TEXT PRIMARY KEY, source TEXT, rows_added INTEGER, latest_date TEXT, status TEXT, crawled_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	s := NewScheduler(NewPriceRefresher(db), db)
	if !s.claimWindow("startup_refresh", "2026-07-17") {
		t.Fatal("startup first")
	}
	if !s.claimWindow("price_dca", "2026-07-17") {
		t.Fatal("price_dca same day should also claim with distinct fund_code")
	}
	if !s.claimWindow("holdings", "2026-07-17") {
		t.Fatal("holdings same day")
	}
	// second process memory reset
	s2 := NewScheduler(NewPriceRefresher(db), db)
	if s2.claimWindow("startup_refresh", "2026-07-17") || s2.claimWindow("price_dca", "2026-07-17") {
		t.Fatal("durable should block all claimed jobs")
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM crawl_log WHERE fund_code LIKE '__sched_%'`).Scan(&n)
	if n != 3 {
		t.Fatalf("claim rows=%d want 3", n)
	}
}
