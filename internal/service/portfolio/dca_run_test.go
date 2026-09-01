package portfolio

import (
	"context"
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
