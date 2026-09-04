package portfolio

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	db "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
	"github.com/DeliciousBuding/fund-dashboard/internal/snapshot"
)

func TestRunDCAAutoInvestDryRunAndExecute(t *testing.T) {
	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// pick a Wednesday (mask day 3)
	// 2026-07-15 is Wednesday
	asOf := "2026-07-15"
	if d, err := time.Parse("2006-01-02", asOf); err == nil {
		_ = d
	}
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
			VALUES (1, '019173', 'Test Fund', 100, 'weekday', '1,2,3,4,5', '定投买入', 1, '2026-01-01', NULL, 1, 'manual')`,
		`INSERT INTO nav_history VALUES ('019173', '2026-07-14', 2.0)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	svc := NewService(db)

	preview, err := svc.RunDCAAutoInvest(context.Background(), RunDCAAutoInvestInput{
		AsOf: asOf, PortfolioID: 1, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Previewed != 1 || len(preview.Items) != 1 || preview.Items[0].Status != "preview" {
		t.Fatalf("preview=%+v", preview)
	}
	if preview.Items[0].Shares == nil || *preview.Items[0].Shares != 50 {
		t.Fatalf("shares want 50 got %+v", preview.Items[0].Shares)
	}

	exec, err := svc.RunDCAAutoInvest(context.Background(), RunDCAAutoInvestInput{
		AsOf: asOf, PortfolioID: 1, DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if exec.Executed != 1 {
		t.Fatalf("executed=%+v", exec)
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM transactions WHERE order_id LIKE 'DCA-1-%'`).Scan(&n)
	if n != 1 {
		t.Fatalf("tx count %d", n)
	}
	var en int
	_ = db.QueryRow(`SELECT COUNT(*) FROM dca_plan_executions WHERE plan_id=1 AND trade_date=?`, asOf).Scan(&en)
	if en != 1 {
		t.Fatalf("executions count %d", en)
	}

	// idempotent
	again, err := svc.RunDCAAutoInvest(context.Background(), RunDCAAutoInvestInput{
		AsOf: asOf, PortfolioID: 1, DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if again.Executed != 0 || again.Skipped != 1 {
		t.Fatalf("idempotent=%+v", again)
	}
}

// newDCARunFixture builds the minimal DCA ledger schema (same inline pattern as
// TestRunDCAAutoInvestDryRunAndExecute above) with one active plan (id=1, fund
// '019173', amount 100, portfolio 1, start 2026-01-01) and a single NAV row, so
// table cases can drive RunDCAAutoInvest with past due dates the way the
// scheduler backfill does.
func newDCARunFixture(t *testing.T, mask, start, end string, hasNav bool, nav float64) *sql.DB {
	t.Helper()
	d, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "dca.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
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
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	endArg := any(nil)
	if end != "" {
		endArg = end
	}
	if _, err := d.Exec(`
		INSERT INTO dca_plans (id, fund_code, fund_name, amount, frequency, weekday_mask, trade_type, portfolio_id, start_date, end_date, active, source)
		VALUES (1, '019173', 'Test Fund', 100, 'weekday', ?, '定投买入', 1, ?, ?, 1, 'manual')
	`, mask, start, endArg); err != nil {
		t.Fatal(err)
	}
	if hasNav {
		if _, err := d.Exec(`INSERT INTO nav_history VALUES ('019173', '2026-08-28', ?)`, nav); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

// TestRunDCAAutoInvestBackfillDueDateTable pins the per-date semantics the
// scheduler 7-day backfill relies on: each replayed due date executes (or
// skips) exactly as the original 20:00 window would have, with the due date —
// not the replay date — stamped on order_id, transactions.confirm_date and the
// dca_plan_executions ledger.
func TestRunDCAAutoInvestBackfillDueDateTable(t *testing.T) {
	cases := []struct {
		name         string
		mask         string
		start        string
		end          string
		hasNav       bool
		nav          float64
		asOf         string // due date a backfill pass would pass in
		repeat       int    // replay count (>=2 exercises idempotency)
		wantStatus   string
		wantExecuted int
		wantOrderID  string // "" = no transaction expected
		wantTx       int
		wantLedger   int
	}{
		{
			name: "missed_due_date_replays_with_due_date_on_order_and_ledger",
			mask: "3", start: "2026-01-01", hasNav: true, nav: 2.0,
			asOf:       "2026-09-02", // Wednesday, mask day 3 — window was missed that day
			wantStatus: "executed", wantExecuted: 1,
			wantOrderID: "DCA-1-20260902", wantTx: 1, wantLedger: 1,
		},
		{
			name: "replayed_due_date_does_not_double_execute",
			mask: "3", start: "2026-01-01", hasNav: true, nav: 2.0,
			asOf: "2026-09-02", repeat: 2,
			wantStatus: "skipped_duplicate", wantExecuted: 0,
			wantOrderID: "DCA-1-20260902", wantTx: 1, wantLedger: 1,
		},
		{
			name: "weekday_not_in_mask_is_not_backfilled",
			mask: "3", start: "2026-01-01", hasNav: true, nav: 2.0,
			asOf:       "2026-09-03", // Thursday
			wantStatus: "skipped_not_due", wantExecuted: 0, wantTx: 0, wantLedger: 0,
		},
		{
			name: "empty_mask_weekday_semantics_preserved_on_past_date",
			mask: "", start: "2026-01-01", hasNav: true, nav: 2.0,
			asOf:       "2026-08-31", // Monday — empty mask defaults to 1-5
			wantStatus: "executed", wantExecuted: 1,
			wantOrderID: "DCA-1-20260831", wantTx: 1, wantLedger: 1,
		},
		{
			name: "empty_mask_weekend_due_date_is_not_backfilled",
			mask: "", start: "2026-01-01", hasNav: true, nav: 2.0,
			asOf:       "2026-09-05", // Saturday
			wantStatus: "skipped_not_due", wantExecuted: 0, wantTx: 0, wantLedger: 0,
		},
		{
			name: "due_date_before_plan_start_is_not_backfilled",
			mask: "3", start: "2026-09-06", hasNav: true, nav: 2.0,
			asOf:       "2026-09-02",
			wantStatus: "skipped_not_due", wantExecuted: 0, wantTx: 0, wantLedger: 0,
		},
		{
			name: "due_date_after_plan_end_is_not_backfilled",
			mask: "3", start: "2026-01-01", end: "2026-08-31", hasNav: true, nav: 2.0,
			asOf:       "2026-09-02",
			wantStatus: "skipped_not_due", wantExecuted: 0, wantTx: 0, wantLedger: 0,
		},
		{
			name: "due_date_without_nav_fails_safe",
			mask: "3", start: "2026-01-01", hasNav: false,
			asOf:       "2026-09-02",
			wantStatus: "skipped_no_nav", wantExecuted: 0, wantTx: 0, wantLedger: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := newDCARunFixture(t, tc.mask, tc.start, tc.end, tc.hasNav, tc.nav)
			svc := NewService(d)
			repeat := tc.repeat
			if repeat < 1 {
				repeat = 1
			}
			var last RunDCAAutoInvestResult
			for i := 0; i < repeat; i++ {
				res, err := svc.RunDCAAutoInvest(context.Background(), RunDCAAutoInvestInput{
					AsOf: tc.asOf, PortfolioID: 1, DryRun: false,
				})
				if err != nil {
					t.Fatal(err)
				}
				last = res
			}
			if last.Executed != tc.wantExecuted {
				t.Fatalf("executed=%d want %d (%+v)", last.Executed, tc.wantExecuted, last)
			}
			var status string
			for _, it := range last.Items {
				if it.PlanID == 1 {
					status = it.Status
				}
			}
			if status != tc.wantStatus {
				t.Fatalf("status=%q want %q (items=%+v)", status, tc.wantStatus, last.Items)
			}
			var txCount, ledgerCount int
			if err := d.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&txCount); err != nil {
				t.Fatal(err)
			}
			if err := d.QueryRow(`SELECT COUNT(*) FROM dca_plan_executions`).Scan(&ledgerCount); err != nil {
				t.Fatal(err)
			}
			if txCount != tc.wantTx || ledgerCount != tc.wantLedger {
				t.Fatalf("rows tx=%d ledger=%d want %d/%d", txCount, ledgerCount, tc.wantTx, tc.wantLedger)
			}
			if tc.wantOrderID == "" {
				return
			}
			var orderID, confirmDate, ledgerDate string
			if err := d.QueryRow(`SELECT order_id, confirm_date FROM transactions`).Scan(&orderID, &confirmDate); err != nil {
				t.Fatal(err)
			}
			if orderID != tc.wantOrderID {
				t.Fatalf("order_id=%q want %q", orderID, tc.wantOrderID)
			}
			if confirmDate != tc.asOf {
				t.Fatalf("confirm_date=%q want due date %q", confirmDate, tc.asOf)
			}
			if err := d.QueryRow(`SELECT trade_date FROM dca_plan_executions`).Scan(&ledgerDate); err != nil {
				t.Fatal(err)
			}
			if ledgerDate != tc.asOf {
				t.Fatalf("ledger trade_date=%q want due date %q", ledgerDate, tc.asOf)
			}
		})
	}
}

// TestRunDCAAutoInvestRoundsSharesTo4dp guards the execution-boundary rounding:
// 100 / 3.0 must land as 33.3333 (4dp, mirroring stored NAV precision) in the
// API item, confirm_share and signed_share_change — not the full-float
// 33.333333333333336. Preview reports the identical rounded value.
func TestRunDCAAutoInvestRoundsSharesTo4dp(t *testing.T) {
	d := newDCARunFixture(t, "1,2,3,4,5", "2026-01-01", "", true, 3.0)
	svc := NewService(d)

	preview, err := svc.RunDCAAutoInvest(context.Background(), RunDCAAutoInvestInput{
		AsOf: "2026-09-02", PortfolioID: 1, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Items) != 1 || preview.Items[0].Shares == nil {
		t.Fatalf("preview=%+v", preview)
	}
	if got := *preview.Items[0].Shares; got != 33.3333 {
		t.Fatalf("preview shares=%v want 33.3333 (4dp)", got)
	}

	if _, err := svc.RunDCAAutoInvest(context.Background(), RunDCAAutoInvestInput{
		AsOf: "2026-09-02", PortfolioID: 1, DryRun: false,
	}); err != nil {
		t.Fatal(err)
	}
	var confirmShare, signedShare float64
	if err := d.QueryRow(`SELECT confirm_share, signed_share_change FROM transactions`).Scan(&confirmShare, &signedShare); err != nil {
		t.Fatal(err)
	}
	if confirmShare != 33.3333 || signedShare != 33.3333 {
		t.Fatalf("stored shares confirm=%v signed=%v want 33.3333 both (4dp)", confirmShare, signedShare)
	}
	// 4dp rounding must not disturb the exact-division regression path (100/2=50).
	d2 := newDCARunFixture(t, "1,2,3,4,5", "2026-01-01", "", true, 2.0)
	if _, err := NewService(d2).RunDCAAutoInvest(context.Background(), RunDCAAutoInvestInput{
		AsOf: "2026-09-02", PortfolioID: 1, DryRun: false,
	}); err != nil {
		t.Fatal(err)
	}
	var exact float64
	if err := d2.QueryRow(`SELECT confirm_share FROM transactions`).Scan(&exact); err != nil {
		t.Fatal(err)
	}
	if exact != 50 {
		t.Fatalf("exact division shares=%v want 50", exact)
	}
}

func TestRecalcSnapshotLightDustClamp(t *testing.T) {
	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "dust.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT, security_type TEXT, market TEXT)`,
		`CREATE TABLE transactions (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			fund_code TEXT, fund_name TEXT, signed_share_change REAL, signed_cash_flow REAL
		)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL)`,
		`CREATE TABLE portfolio_snapshot (
			fund_code TEXT, portfolio_id INTEGER DEFAULT 1, fund_name TEXT,
			held_shares REAL, total_cost REAL, latest_nav REAL, current_value REAL,
			unrealized_pnl REAL, pnl_pct REAL, security_type TEXT,
			PRIMARY KEY (fund_code, portfolio_id)
		)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES ('019173','t','fund')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow)
		VALUES ('019173','t',100,-120), ('019173','t',-99.99999999999999,130)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('019173','2026-07-10',1.5)`); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.RecalcForPortfolio(context.Background(), db, "019173", 1, snapshot.ModeLight); err != nil {
		t.Fatal(err)
	}
	var held, value, unrealized, pnl float64
	if err := db.QueryRow(`
		SELECT held_shares, current_value, unrealized_pnl, pnl_pct FROM portfolio_snapshot
		WHERE fund_code='019173' AND portfolio_id=1
	`).Scan(&held, &value, &unrealized, &pnl); err != nil {
		t.Fatal(err)
	}
	if held != 0 {
		t.Fatalf("held_shares=%v want 0 after dust clamp", held)
	}
	if value != 0 || unrealized != 0 || pnl != 0 {
		t.Fatalf("closed dust position value/pnl=%v/%v/%v want 0", value, unrealized, pnl)
	}
}
