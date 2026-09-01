package portfolio

import (
	"context"
	"path/filepath"
	"testing"

	db "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
)

func TestGenerateReportAssemblesSections(t *testing.T) {
	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT, fund_type TEXT, security_type TEXT, market TEXT)`,
		`CREATE TABLE transactions (
			seq INTEGER PRIMARY KEY AUTOINCREMENT, order_id TEXT, trade_time TEXT, confirm_date TEXT,
			trade_type TEXT, direction TEXT, fund_code TEXT, fund_name TEXT, confirm_amount REAL,
			confirm_share REAL, fee REAL, signed_cash_flow REAL, signed_share_change REAL, settlement_days INTEGER
		)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL, daily_change_pct REAL, security_type TEXT)`,
		`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, fund_name TEXT, held_shares REAL, total_cost REAL, latest_nav REAL, current_value REAL, unrealized_pnl REAL, pnl_pct REAL, security_type TEXT, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,
		`CREATE TABLE dca_plans (
			id INTEGER PRIMARY KEY, fund_code TEXT, fund_name TEXT, amount REAL, frequency TEXT,
			weekday_mask TEXT, trade_type TEXT, portfolio_id INTEGER, start_date TEXT, end_date TEXT,
			active INTEGER, source TEXT, created_at TEXT, updated_at TEXT
		)`,
		`CREATE TABLE source_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT, url TEXT, source TEXT, snippet TEXT, query TEXT,
			related_security_code TEXT, related_security_name TEXT, is_read INTEGER, is_useful INTEGER,
			fetched_at TEXT, created_at TEXT
		)`,
		`CREATE TABLE fund_holdings (fund_code TEXT, stock_code TEXT, stock_name TEXT, weight_pct REAL, shares REAL, market_value REAL, report_date TEXT)`,
		`INSERT INTO fund_details VALUES ('019173','Fund','test','fund','CN')`,
		`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
			VALUES ('T1','2026-06-01T09:00:00Z','2026-06-02','用户买入','buy','019173','Fund',100,100,0,-100,100,1)`,
		`INSERT INTO portfolio_snapshot VALUES ('019173','Fund',100,-100,1.2,120,20,20,'fund',1)`,
		`INSERT INTO nav_history VALUES ('019173','2026-06-18',1.2,0,'fund')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	svc := NewService(db)
	res, err := svc.GenerateReport(context.Background(), GenerateReportInput{PortfolioID: 1, Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Format != "json" || res.Artifact != "json" || res.SideEffects != "none" {
		t.Fatalf("%+v", res)
	}
	if res.Sections["summary"] == nil || res.Sections["allocation"] == nil {
		t.Fatalf("missing sections: %+v", res.Sections)
	}
}
