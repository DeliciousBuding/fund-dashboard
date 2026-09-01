package portfolio

import (
	"context"

	"path/filepath"

	"testing"

	db "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
)

func TestUpsertAndDeleteSecurity(t *testing.T) {

	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})

	if err != nil {

		t.Fatal(err)

	}

	defer db.Close()

	for _, q := range []string{

		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT, fund_type TEXT, security_type TEXT, market TEXT, currency TEXT, exchange TEXT, source TEXT)`,

		`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, fund_name TEXT, held_shares REAL, total_cost REAL, latest_nav REAL, current_value REAL, unrealized_pnl REAL, pnl_pct REAL, security_type TEXT, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,

		`CREATE TABLE transactions (seq INTEGER PRIMARY KEY AUTOINCREMENT, fund_code TEXT)`,

		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL, PRIMARY KEY(fund_code, date))`,

		`CREATE TABLE fund_holdings (fund_code TEXT, stock_code TEXT, stock_name TEXT, weight_pct REAL, shares REAL, market_value REAL, report_date TEXT, PRIMARY KEY(fund_code, stock_code, report_date))`,

		`CREATE TABLE dca_plans (id INTEGER PRIMARY KEY AUTOINCREMENT, fund_code TEXT)`,

		`CREATE TABLE dca_plan_executions (id INTEGER PRIMARY KEY AUTOINCREMENT, plan_id INTEGER, fund_code TEXT, trade_date TEXT, amount REAL, status TEXT, order_id TEXT, created_at TEXT, updated_at TEXT)`,

		`CREATE TABLE summary_by_fund (fund_code TEXT PRIMARY KEY, fund_name TEXT, total_shares REAL, total_cost REAL, tx_count INTEGER)`,

		`CREATE TABLE crawl_log (fund_code TEXT, source TEXT, rows_added INTEGER, latest_date TEXT, status TEXT, crawled_at TEXT)`,

		`CREATE TABLE source_events (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT, related_security_code TEXT)`,
	} {

		if _, err := db.Exec(q); err != nil {

			t.Fatalf("%s: %v", q, err)

		}

	}

	svc := NewService(db)

	res, err := svc.UpsertSecurity(context.Background(), UpsertSecurityInput{Code: "019173", Name: "Test Fund", SecurityType: "fund"})

	if err != nil || !res.Created || res.Security.Name != "Test Fund" {

		t.Fatalf("create=%+v err=%v", res, err)

	}

	res, err = svc.UpsertSecurity(context.Background(), UpsertSecurityInput{Code: "019173", Name: "Renamed"})

	if err != nil || res.Created || res.Security.Name != "Renamed" {

		t.Fatalf("update=%+v err=%v", res, err)

	}

	for _, q := range []string{

		`INSERT INTO transactions(fund_code) VALUES ('019173')`,

		`INSERT INTO dca_plan_executions(plan_id, fund_code, trade_date, amount, status, order_id, created_at, updated_at)

			VALUES (1,'019173','2026-07-15',100,'executed','DCA-1-20260715','2026-07-15','2026-07-15')`,

		`INSERT INTO summary_by_fund(fund_code, fund_name, total_shares, total_cost, tx_count) VALUES ('019173','Test',1,1,1)`,

		`INSERT INTO crawl_log(fund_code, source, rows_added, latest_date, status, crawled_at) VALUES ('019173','eastmoney',1,'2026-07-15','ok','2026-07-15')`,

		`INSERT INTO source_events(title, related_security_code) VALUES ('note','019173')`,
	} {

		if _, err := db.Exec(q); err != nil {

			t.Fatal(err)

		}

	}

	del, err := svc.DeleteSecurity(context.Background(), "019173")

	if err != nil || !del.Deleted {

		t.Fatalf("delete=%+v err=%v", del, err)

	}

	for _, q := range []string{

		`SELECT COUNT(*) FROM transactions WHERE fund_code='019173'`,

		`SELECT COUNT(*) FROM dca_plan_executions WHERE fund_code='019173'`,

		`SELECT COUNT(*) FROM summary_by_fund WHERE fund_code='019173'`,

		`SELECT COUNT(*) FROM crawl_log WHERE fund_code='019173'`,

		`SELECT COUNT(*) FROM source_events WHERE related_security_code='019173'`,
	} {

		var n int

		if err := db.QueryRow(q).Scan(&n); err != nil {

			t.Fatal(err)

		}

		if n != 0 {

			t.Fatalf("%s => %d, want 0", q, n)

		}

	}

}

func TestDeleteUSSecurityCascadesStockCache(t *testing.T) {

	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})

	if err != nil {

		t.Fatal(err)

	}

	defer db.Close()

	for _, q := range []string{

		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT, fund_type TEXT, security_type TEXT, market TEXT, currency TEXT, exchange TEXT, source TEXT)`,

		`CREATE TABLE stock_realtime (code TEXT, market TEXT, name TEXT, price REAL, PRIMARY KEY(code, market))`,

		`CREATE TABLE stock_kline_cache (code TEXT, market TEXT, period TEXT, date TEXT, close REAL, PRIMARY KEY(code, market, period, date))`,

		`CREATE TABLE stock_profile (code TEXT PRIMARY KEY, name TEXT)`,

		`INSERT INTO fund_details (fund_code, fund_name, security_type, market) VALUES ('AAPL','Apple','stock','US')`,

		`INSERT INTO stock_realtime(code, market, name, price) VALUES ('AAPL','US','Apple',190)`,

		`INSERT INTO stock_kline_cache(code, market, period, date, close) VALUES ('AAPL','US','daily','2026-07-15',190)`,

		`INSERT INTO stock_profile(code, name) VALUES ('AAPL','Apple')`,
	} {

		if _, err := db.Exec(q); err != nil {

			t.Fatalf("%s: %v", q, err)

		}

	}

	svc := NewService(db)

	del, err := svc.DeleteSecurity(context.Background(), "AAPL")

	if err != nil || !del.Deleted {

		t.Fatalf("delete=%+v err=%v", del, err)

	}

	for _, q := range []string{

		`SELECT COUNT(*) FROM stock_realtime WHERE code='AAPL'`,

		`SELECT COUNT(*) FROM stock_kline_cache WHERE code='AAPL'`,

		`SELECT COUNT(*) FROM stock_profile WHERE code='AAPL'`,

		`SELECT COUNT(*) FROM fund_details WHERE fund_code='AAPL'`,
	} {

		var n int

		if err := db.QueryRow(q).Scan(&n); err != nil {

			t.Fatal(err)

		}

		if n != 0 {

			t.Fatalf("%s => %d, want 0", q, n)

		}

	}

}
