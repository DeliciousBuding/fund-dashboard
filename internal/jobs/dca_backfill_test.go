package jobs

import (
	"context"
	"database/sql"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/chinatime"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	_ "modernc.org/sqlite"
)

// recordingDCARunner captures every backfill call (order included), unlike
// stubDCARunner which only keeps the last input.
type recordingDCARunner struct {
	mu     sync.Mutex
	asOfs  []string
	inputs []portfoliosvc.RunDCAAutoInvestInput
}

func (r *recordingDCARunner) RunDCAAutoInvest(_ context.Context, in portfoliosvc.RunDCAAutoInvestInput) (portfoliosvc.RunDCAAutoInvestResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asOfs = append(r.asOfs, in.AsOf)
	r.inputs = append(r.inputs, in)
	return portfoliosvc.RunDCAAutoInvestResult{OK: true, Skipped: 1}, nil
}

func (r *recordingDCARunner) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.asOfs)
}

func newBackfillTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	// crawl_log backs the durable once-per-day claim (__sched_dca_backfill).
	if _, err := db.Exec(`CREATE TABLE crawl_log (fund_code TEXT PRIMARY KEY, source TEXT, rows_added INTEGER, latest_date TEXT, status TEXT, crawled_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	return db
}

// wantLookback returns the expected replay dates for a tick at now: the 7 CST
// natural days before today, oldest first, today excluded.
func wantLookback(now time.Time) []string {
	now = now.In(chinatime.Loc)
	out := make([]string, 0, dcaBackfillLookbackDays)
	for i := dcaBackfillLookbackDays; i >= 1; i-- {
		out = append(out, now.AddDate(0, 0, -i).Format("2006-01-02"))
	}
	return out
}

func checkBackfillInputs(t *testing.T, r *recordingDCARunner, from int, now time.Time) {
	t.Helper()
	want := wantLookback(now)
	r.mu.Lock()
	defer r.mu.Unlock()
	got := slices.Clone(r.asOfs)[from:]
	if !slices.Equal(got, want) {
		t.Fatalf("backfill as_ofs=%v want %v (oldest-first 7 days, today excluded)", got, want)
	}
	for _, in := range r.inputs[from:] {
		if in.PortfolioID != 1 || in.DryRun {
			t.Fatalf("input=%+v want portfolio 1, dry_run=false", in)
		}
	}
}

// The morning band tick replays exactly the 7 days before today (CST), oldest
// first — the window is bounded, so due dates older than 7 days are never
// requested (no unbounded backtracking).
func TestDCABackfillWindowReplaysLookbackDays(t *testing.T) {
	db := newBackfillTestDB(t)
	r := &recordingDCARunner{}
	s := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(r)
	now := time.Date(2026, 7, 15, 8, 3, 0, 0, chinatime.Loc) // Wednesday
	s.tick(now)
	if r.calls() != dcaBackfillLookbackDays {
		t.Fatalf("calls=%d want %d", r.calls(), dcaBackfillLookbackDays)
	}
	checkBackfillInputs(t, r, 0, now)
	// Durable claim row exists for the day.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM crawl_log WHERE fund_code='__sched_dca_backfill' AND latest_date='2026-07-15'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("durable claim rows=%d want 1", n)
	}
}

// One backfill pass per CST day: later ticks in the band are claimed in-memory,
// a fresh scheduler (process restart) is blocked by the durable crawl_log claim,
// and the next day claims a new window.
func TestDCABackfillWindowOncePerDayAndDurable(t *testing.T) {
	db := newBackfillTestDB(t)
	r := &recordingDCARunner{}
	s := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(r)
	day := time.Date(2026, 7, 15, 6, 5, 0, 0, chinatime.Loc)
	s.tick(day)
	s.tick(day.Add(3 * time.Hour + 50 * time.Minute)) // 09:55, still in band
	if r.calls() != dcaBackfillLookbackDays {
		t.Fatalf("calls after same-day ticks=%d want %d", r.calls(), dcaBackfillLookbackDays)
	}
	// Process restart same day: durable claim blocks the replay.
	s2 := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(r)
	s2.tick(day.Add(time.Hour))
	if r.calls() != dcaBackfillLookbackDays {
		t.Fatalf("calls after restart same day=%d want %d (durable claim must block)", r.calls(), dcaBackfillLookbackDays)
	}
	// Next day: a fresh 7-day window (shifted by one).
	next := time.Date(2026, 7, 16, 8, 0, 0, 0, chinatime.Loc)
	s2.tick(next)
	if r.calls() != 2*dcaBackfillLookbackDays {
		t.Fatalf("calls after next day=%d want %d", r.calls(), 2*dcaBackfillLookbackDays)
	}
	checkBackfillInputs(t, r, dcaBackfillLookbackDays, next)
}

// The backfill is a financial decision like the 20:00 materialization: the
// trigger never runs on Saturday/Sunday (a missed Friday is replayed Monday,
// still inside the 7-day window).
func TestDCABackfillWindowSkipsWeekends(t *testing.T) {
	db := newBackfillTestDB(t)
	r := &recordingDCARunner{}
	s := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(r)
	s.tick(time.Date(2026, 7, 18, 7, 10, 0, 0, chinatime.Loc)) // Saturday
	s.tick(time.Date(2026, 7, 19, 7, 10, 0, 0, chinatime.Loc)) // Sunday
	if r.calls() != 0 {
		t.Fatalf("weekend calls=%d want 0", r.calls())
	}
}

// Regression guard for the normal path: the morning backfill never touches
// today — the 20:00 window still materializes today exactly once, unchanged.
func TestDCABackfillDoesNotTouchTodayWindow(t *testing.T) {
	db := newBackfillTestDB(t)
	r := &recordingDCARunner{}
	s := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(r)
	morning := time.Date(2026, 7, 15, 8, 3, 0, 0, chinatime.Loc)
	s.tick(morning)
	evening := time.Date(2026, 7, 15, 20, 3, 0, 0, chinatime.Loc)
	s.tick(evening)
	if r.calls() != dcaBackfillLookbackDays+1 {
		t.Fatalf("calls=%d want %d (7 backfill + 1 today)", r.calls(), dcaBackfillLookbackDays+1)
	}
	last := r.inputs[len(r.inputs)-1]
	if last.AsOf != "2026-07-15" {
		t.Fatalf("today window as_of=%q want 2026-07-15 (unchanged behavior)", last.AsOf)
	}
	for _, in := range r.inputs[:dcaBackfillLookbackDays] {
		if in.AsOf >= "2026-07-15" {
			t.Fatalf("backfill leaked today into replay: %+v", in)
		}
	}
}

// The startup catch-up also replays missed due dates (once per CST day via the
// startup_refresh claim), so a process that was down during a 20:00 window
// compensates right after restart.
func TestStartupCatchUpRunsDCABackfill(t *testing.T) {
	db := newBackfillTestDB(t)
	for _, q := range []string{
		`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, held_shares REAL, security_type TEXT, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, security_type TEXT)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	r := &recordingDCARunner{}
	s := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(r)
	s.startupRefresh = func(context.Context) (int, int, error) { return 0, 0, nil }
	now := time.Date(2026, 7, 17, 8, 0, 0, 0, chinatime.Loc) // Friday
	s.runStartupCatchUp(now)
	if r.calls() != dcaBackfillLookbackDays {
		t.Fatalf("startup backfill calls=%d want %d", r.calls(), dcaBackfillLookbackDays)
	}
	checkBackfillInputs(t, r, 0, now)
	// Same-day second start: the startup_refresh claim skips the whole catch-up.
	s.runStartupCatchUp(now.Add(time.Hour))
	if r.calls() != dcaBackfillLookbackDays {
		t.Fatalf("calls after same-day restart=%d want %d", r.calls(), dcaBackfillLookbackDays)
	}
	// Weekend restart: catch-up claims but the backfill weekday-gates.
	sun := NewScheduler(NewPriceRefresher(db), db).WithDCARunner(r)
	sun.startupRefresh = func(context.Context) (int, int, error) { return 0, 0, nil }
	sun.runStartupCatchUp(time.Date(2026, 7, 19, 9, 0, 0, 0, chinatime.Loc))
	if r.calls() != dcaBackfillLookbackDays {
		t.Fatalf("calls after weekend restart=%d want 0 additional", r.calls())
	}
}

// End-to-end with the real portfolio service: a Wednesday due date missed while
// the process was down is replayed on Thursday morning with the due date stamped
// on order_id / confirm_date / ledger, exactly once — a forced second pass
// (claim bypassed, racing-tick simulation) must not double-invest.
func TestDCABackfillMaterializesMissedDueDateEndToEnd(t *testing.T) {
	db := newBackfillTestDB(t)
	for _, q := range []string{
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT, security_type TEXT, market TEXT)`,
		`CREATE TABLE dca_plans (
			id INTEGER PRIMARY KEY, fund_code TEXT, fund_name TEXT, amount REAL,
			frequency TEXT, weekday_mask TEXT, trade_type TEXT, portfolio_id INTEGER,
			start_date TEXT, end_date TEXT, active INTEGER, source TEXT
		)`,
		`CREATE TABLE transactions (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id TEXT UNIQUE,
			trade_time TEXT, confirm_date TEXT, trade_type TEXT, direction TEXT,
			fund_code TEXT, fund_name TEXT, confirm_amount REAL, confirm_share REAL,
			fee REAL, signed_cash_flow REAL, signed_share_change REAL, settlement_days INTEGER
		)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL)`,
		`CREATE TABLE dca_plan_executions (
			id INTEGER PRIMARY KEY AUTOINCREMENT, plan_id INTEGER, fund_code TEXT, trade_date TEXT,
			amount REAL, status TEXT, order_id TEXT, nav_date TEXT, nav REAL, message TEXT, created_at TEXT, updated_at TEXT
		)`,
		`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, fund_name TEXT, held_shares REAL, total_cost REAL, latest_nav REAL, current_value REAL, unrealized_pnl REAL, pnl_pct REAL, security_type TEXT, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,
		`INSERT INTO fund_details VALUES ('019173','Test Fund','fund','CN')`,
		`INSERT INTO dca_plans (id, fund_code, fund_name, amount, frequency, weekday_mask, trade_type, portfolio_id, start_date, end_date, active, source)
			VALUES (1, '019173', 'Test Fund', 100, 'weekday', '3', '定投买入', 1, '2026-01-01', NULL, 1, 'manual')`,
		`INSERT INTO nav_history VALUES ('019173', '2026-09-08', 2.5)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	// Process was down during Wednesday 2026-09-09 20:00; restarts Thursday 08:03.
	now := time.Date(2026, 9, 10, 8, 3, 0, 0, chinatime.Loc) // Thursday
	s := NewScheduler(NewPriceRefresher(db), db)              // real DCARunner
	s.tick(now)

	var orderID, confirmDate string
	var confirmShare float64
	if err := db.QueryRow(`SELECT order_id, confirm_date, confirm_share FROM transactions`).Scan(&orderID, &confirmDate, &confirmShare); err != nil {
		t.Fatalf("expected exactly one backfilled transaction: %v", err)
	}
	if orderID != "DCA-1-20260909" || confirmDate != "2026-09-09" {
		t.Fatalf("order=%s confirm_date=%s want due date 2026-09-09", orderID, confirmDate)
	}
	if confirmShare != 40 { // 100 / 2.5
		t.Fatalf("confirm_share=%v want 40", confirmShare)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM dca_plan_executions WHERE plan_id=1 AND trade_date='2026-09-09' AND order_id='DCA-1-20260909'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("ledger rows=%d want 1 (due date on ledger)", n)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM transactions WHERE order_id='DCA-1-20260910'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("today must stay with the 20:00 window, found %d rows", n)
	}

	// Racing second pass with the claim bypassed: ledger idempotency alone
	// must prevent a double invest.
	s2 := NewScheduler(NewPriceRefresher(db), db)
	if err := s2.runDCABackfill(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("transactions after forced replay=%d want 1 (idempotent)", n)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM dca_plan_executions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("ledger rows after forced replay=%d want 1 (idempotent)", n)
	}
}
