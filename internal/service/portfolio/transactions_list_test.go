package portfolio

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/repository/sqlitedb"
)

// Legacy ledger DBs (pre-PG-schema SQLite) may lack anomaly / settlement_days /
// portfolio_id on transactions. ListTransactions must probe and adapt instead of
// erroring — the ledger stays the SSOT and is never ALTERed by a read service.
func TestListTransactionsAdaptsToLegacyShape(t *testing.T) {
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE transactions (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id TEXT,
			trade_time TEXT,
			confirm_date TEXT,
			trade_type TEXT,
			direction TEXT,
			fund_code TEXT,
			fund_name TEXT,
			confirm_amount REAL,
			confirm_share REAL,
			fee REAL
		)`,
		`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee)
			VALUES
			('o1','2026-06-01T10:00:00+08:00','2026-06-02','用户买入','buy','019173','Test Fund',120,100,0.1),
			('o2','2026-06-02T10:00:00+08:00','2026-06-03','用户卖出','sell','019173','Test Fund',60,50,0.1)`,
	} {
		if _, err := db.ExecContext(context.Background(), q); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}

	svc := NewService(db)
	res, err := svc.ListTransactions(context.Background(), ListTransactionsOptions{PortfolioID: 1})
	if err != nil {
		t.Fatalf("list transactions on legacy shape: %v", err)
	}
	if res.Total != 2 || len(res.Transactions) != 2 {
		t.Fatalf("result = %+v, want 2 rows", res)
	}
	first := res.Transactions[0]
	if first.TradeTime == nil || *first.TradeTime != "2026-06-02T10:00:00+08:00" {
		t.Fatalf("newest first ordering broken: %+v", first)
	}
	if first.Anomaly != nil || first.SettlementDays != nil || first.PortfolioID != nil {
		t.Fatalf("missing legacy columns must stay null: %+v", first)
	}
	if first.Fee == nil || *first.Fee != 0.1 {
		t.Fatalf("fee = %v, want 0.1", first.Fee)
	}

	filtered, err := svc.ListTransactions(context.Background(), ListTransactionsOptions{Direction: "sell"})
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}
	if filtered.Total != 1 {
		t.Fatalf("direction filter total = %d, want 1", filtered.Total)
	}
}

func TestListTransactionsFullShapeFilterAndPagination(t *testing.T) {
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE transactions (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id TEXT,
			trade_time TEXT,
			confirm_date TEXT,
			trade_type TEXT,
			direction TEXT,
			fund_code TEXT,
			fund_name TEXT,
			confirm_amount REAL,
			confirm_share REAL,
			fee REAL,
			anomaly TEXT,
			settlement_days INTEGER,
			portfolio_id INTEGER
		)`,
		`INSERT INTO transactions (order_id, trade_time, direction, fund_code, fund_name, confirm_amount, portfolio_id)
			VALUES
			('a1','2026-06-01T10:00:00+08:00','buy','019173','Alpha',100,1),
			('a2','2026-06-02T10:00:00+08:00','buy','019173','Alpha',100,2),
			('b1','2026-06-03T10:00:00+08:00','buy','AAPL','Apple',300,1)`,
	} {
		if _, err := db.ExecContext(context.Background(), q); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}

	svc := NewService(db)
	res, err := svc.ListTransactions(context.Background(), ListTransactionsOptions{PortfolioID: 1})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if res.Total != 2 {
		t.Fatalf("portfolio filter total = %d, want 2", res.Total)
	}

	searched, err := svc.ListTransactions(context.Background(), ListTransactionsOptions{Search: "apple"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if searched.Total != 1 || searched.Transactions[0].FundCode != "AAPL" {
		t.Fatalf("search result = %+v, want 1 AAPL row", searched)
	}
	if searched.Transactions[0].PortfolioID == nil || *searched.Transactions[0].PortfolioID != 1 {
		t.Fatalf("portfolio_id = %v, want 1", searched.Transactions[0].PortfolioID)
	}

	page, err := svc.ListTransactions(context.Background(), ListTransactionsOptions{Limit: 1, Offset: 2})
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if page.Total != 3 || len(page.Transactions) != 1 || page.Transactions[0].OrderID == nil || *page.Transactions[0].OrderID != "a1" {
		t.Fatalf("pagination result = %+v, want oldest row a1", page)
	}
}
