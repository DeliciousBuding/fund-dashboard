package admin

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func seedTransaction(t *testing.T, db *sql.DB, orderID, fundCode, direction string, amount, share, fee float64) int {
	t.Helper()
	res, err := db.ExecContext(context.Background(), `
		INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name,
			confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
		VALUES (?, '2026-01-01', '2026-01-03', 'manual', ?, ?, 'Fund', ?, ?, ?, 0, 0, 0)
	`, orderID, direction, fundCode, amount, share, fee)
	if err != nil {
		t.Fatalf("seed transaction: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return int(id)
}

func queryTransaction(t *testing.T, db *sql.DB, seq int) (amount, share, fee, cash, shareChange float64, settlement int, direction, fundCode string) {
	t.Helper()
	err := db.QueryRowContext(context.Background(), `
		SELECT confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days, direction, fund_code
		FROM transactions WHERE seq = ?
	`, seq).Scan(&amount, &share, &fee, &cash, &shareChange, &settlement, &direction, &fundCode)
	if err != nil {
		t.Fatalf("query transaction %d: %v", seq, err)
	}
	return
}

func countTransactions(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM transactions`).Scan(&n); err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	return n
}

func TestImportTransactionsRejectsEmptyAndOversizedBatch(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	if _, err := svc.ImportTransactions(context.Background(), nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty batch: want ErrInvalidInput, got %v", err)
	}

	big := make([]ImportTransaction, maxImportRowsForTest)
	for i := range big {
		big[i] = ImportTransaction{FundCode: "019173"}
	}
	if _, err := svc.ImportTransactions(context.Background(), big); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("oversized batch: want ErrInvalidInput, got %v", err)
	}
}

const maxImportRowsForTest = 5001

func TestImportTransactionsHappyPathComputesSignedFlows(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	amt, share, fee := 100.0, 10.0, 1.0
	res, err := svc.ImportTransactions(context.Background(), []ImportTransaction{
		{OrderID: "A1", FundCode: "019173", TradeTime: "2026-01-01", ConfirmDate: "2026-01-03", Direction: "buy", ConfirmAmount: &amt, ConfirmShare: &share, Fee: &fee},
		{OrderID: "A2", FundCode: "019174", TradeTime: "2026-01-02", ConfirmDate: "2026-01-04", Direction: "sell", ConfirmAmount: &amt, ConfirmShare: &share, Fee: &fee},
	})
	if err != nil {
		t.Fatalf("ImportTransactions: %v", err)
	}
	if !res.OK || res.Imported != 2 || res.Total != 2 || res.AffectedFunds != 2 {
		t.Fatalf("result = %+v, want OK imported=2 total=2 affected=2", res)
	}

	var seq int
	if err := db.QueryRowContext(context.Background(), `SELECT seq FROM transactions WHERE order_id = 'A1'`).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	a, s, f, cash, sc, days, dir, code := queryTransaction(t, db, seq)
	if a != 100 || s != 10 || f != 1 || cash != -101 || sc != 10 || days != 2 || dir != "buy" || code != "019173" {
		t.Fatalf("buy row = amount %v share %v fee %v cash %v shareChange %v days %d dir %q code %q", a, s, f, cash, sc, days, dir, code)
	}

	if err := db.QueryRowContext(context.Background(), `SELECT seq FROM transactions WHERE order_id = 'A2'`).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	a, s, f, cash, sc, days, dir, code = queryTransaction(t, db, seq)
	if a != 100 || s != 10 || f != 1 || cash != 99 || sc != -10 || days != 2 || dir != "sell" || code != "019174" {
		t.Fatalf("sell row = amount %v share %v fee %v cash %v shareChange %v days %d dir %q code %q", a, s, f, cash, sc, days, dir, code)
	}
}

func TestImportTransactionsDividendStoresZeroShare(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	amt, fee := 5.0, 0.0
	res, err := svc.ImportTransactions(context.Background(), []ImportTransaction{
		{OrderID: "D1", FundCode: "019173", TradeTime: "2026-01-01", Direction: "dividend", ConfirmAmount: &amt, Fee: &fee},
	})
	if err != nil {
		t.Fatalf("ImportTransactions dividend: %v", err)
	}
	if !res.OK || res.Imported != 1 {
		t.Fatalf("result = %+v", res)
	}
	var share, cash, shareChange float64
	if err := db.QueryRowContext(context.Background(), `
		SELECT confirm_share, signed_cash_flow, signed_share_change FROM transactions WHERE order_id = 'D1'
	`).Scan(&share, &cash, &shareChange); err != nil {
		t.Fatal(err)
	}
	if share != 0 || cash != 5 || shareChange != 0 {
		t.Fatalf("dividend row share=%v cash=%v shareChange=%v", share, cash, shareChange)
	}
}

func TestImportTransactionsDedupByIdempotency(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	amt, share, fee := 100.0, 10.0, 1.0
	item := ImportTransaction{OrderID: "DUP1", FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: &amt, ConfirmShare: &share, Fee: &fee}

	res, err := svc.ImportTransactions(context.Background(), []ImportTransaction{item, item})
	if err != nil {
		t.Fatalf("import batch: %v", err)
	}
	if res.Imported != 1 || res.Total != 2 || res.AffectedFunds != 1 {
		t.Fatalf("batch dedup = %+v, want imported=1 total=2 affected=1", res)
	}

	res, err = svc.ImportTransactions(context.Background(), []ImportTransaction{item})
	if err != nil {
		t.Fatalf("import repeat: %v", err)
	}
	if res.Imported != 0 {
		t.Fatalf("repeat import = %+v, want imported=0", res)
	}
	if n := countTransactions(t, db); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
}

func TestImportTransactionsSameOrderDifferentFundBothImported(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	amt, share, fee := 100.0, 10.0, 1.0
	res, err := svc.ImportTransactions(context.Background(), []ImportTransaction{
		{OrderID: "SHARED", FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: &amt, ConfirmShare: &share, Fee: &fee},
		{OrderID: "SHARED", FundCode: "019174", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: &amt, ConfirmShare: &share, Fee: &fee},
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Imported != 2 || res.AffectedFunds != 2 {
		t.Fatalf("result = %+v, want imported=2 affected=2 (guard keys on order_id+fund_code)", res)
	}
}

func TestImportTransactionsInvalidRowRollsBackWholeBatch(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	amt, share, fee := 100.0, 10.0, 1.0
	_, err := svc.ImportTransactions(context.Background(), []ImportTransaction{
		{OrderID: "OK1", FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: &amt, ConfirmShare: &share, Fee: &fee},
		{OrderID: "BAD1", FundCode: "019174", TradeTime: "2026-01-01", Direction: "sell", ConfirmAmount: &amt, Fee: &fee},
	})
	if err == nil || !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput for invalid sell row, got %v", err)
	}
	if n := countTransactions(t, db); n != 0 {
		t.Fatalf("rows after failed batch = %d, want 0 (atomic rollback)", n)
	}
}

func TestUpdateTransactionAppliesFieldsAndOrder(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	seq := seedTransaction(t, db, "U1", "019173", "buy", 100, 10, 1)
	tradeTime := "2026-02-01"
	fee := 5.0
	res, err := svc.UpdateTransaction(context.Background(), seq, UpdateTransaction{TradeTime: &tradeTime, Fee: &fee})
	if err != nil {
		t.Fatalf("UpdateTransaction: %v", err)
	}
	if !res.OK || res.Updated.Seq != seq {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Updated.Fields) != 2 || res.Updated.Fields[0] != "trade_time" || res.Updated.Fields[1] != "fee" {
		t.Fatalf("Fields = %v, want [trade_time fee]", res.Updated.Fields)
	}

	amount, share, gotFee, cash, _, days, dir, code := queryTransaction(t, db, seq)
	if amount != 100 || share != 10 || gotFee != 5 || cash != -105 || days != 0 || dir != "buy" || code != "019173" {
		t.Fatalf("updated row = amount %v share %v fee %v cash %v days %d dir %q code %q", amount, share, gotFee, cash, days, dir, code)
	}
}

func TestUpdateTransactionDirectionFlipsSignedShare(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	seq := seedTransaction(t, db, "U2", "019173", "buy", 100, 10, 1)
	direction := "sell"
	if _, err := svc.UpdateTransaction(context.Background(), seq, UpdateTransaction{Direction: &direction}); err != nil {
		t.Fatalf("UpdateTransaction: %v", err)
	}
	_, _, _, cash, shareChange, _, _, _ := queryTransaction(t, db, seq)
	if shareChange != -10 {
		t.Fatalf("signed_share_change = %v, want -10 after buy->sell", shareChange)
	}
	if cash != 99 {
		t.Fatalf("signed_cash_flow = %v, want 99 after buy->sell", cash)
	}
}

func TestUpdateTransactionMovesFundCodeAndRecalcsBoth(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	seq := seedTransaction(t, db, "U3", "019173", "buy", 100, 10, 1)
	fundCode := "019174"
	res, err := svc.UpdateTransaction(context.Background(), seq, UpdateTransaction{FundCode: &fundCode})
	if err != nil {
		t.Fatalf("UpdateTransaction: %v", err)
	}
	if !res.OK {
		t.Fatalf("result = %+v", res)
	}
	_, _, _, _, _, _, _, code := queryTransaction(t, db, seq)
	if code != "019174" {
		t.Fatalf("fund_code = %q, want 019174", code)
	}
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM portfolio_snapshot WHERE fund_code = '019174'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("portfolio_snapshot rows for moved fund = %d, want 1 (recalc side effect)", n)
	}
}

func TestUpdateTransactionInvalidInputs(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	seq := seedTransaction(t, db, "U4", "019173", "buy", 100, 10, 1)

	cases := []struct {
		name    string
		update  UpdateTransaction
		wantErr string
	}{
		{"seq zero", UpdateTransaction{}, "seq required"},
		{"no fields", UpdateTransaction{ConfirmDate: nil}, "no valid fields"},
		{"bad direction", UpdateTransaction{Direction: strp("BUY")}, "direction"},
		{"amount zero", UpdateTransaction{ConfirmAmount: ptrFloat(0)}, "confirm_amount must be positive"},
		{"amount negative", UpdateTransaction{ConfirmAmount: ptrFloat(-1)}, "confirm_amount must be positive"},
		{"amount too large", UpdateTransaction{ConfirmAmount: ptrFloat(maxTxMoney + 1)}, "confirm_amount too large"},
		{"share zero on buy", UpdateTransaction{ConfirmShare: ptrFloat(0)}, "confirm_share must be positive"},
		{"share too large", UpdateTransaction{ConfirmShare: ptrFloat(maxTxMoney + 1)}, "confirm_share too large"},
		{"fee negative", UpdateTransaction{Fee: ptrFloat(-0.01)}, "fee must be non-negative"},
		{"fee too large", UpdateTransaction{Fee: ptrFloat(maxTxMoney + 1)}, "fee too large"},
		{"fund code empty", UpdateTransaction{FundCode: strp("")}, "fund_code invalid"},
		{"fund code too long", UpdateTransaction{FundCode: strp(strings.Repeat("A", maxTxFundCodeLen+1))}, "fund_code invalid"},
		{"trade time too long", UpdateTransaction{TradeTime: strp(strings.Repeat("2", maxTxTimeLen+1))}, "trade_time too long"},
		{"confirm date too long", UpdateTransaction{ConfirmDate: strp(strings.Repeat("2", maxTxTimeLen+1))}, "confirm_date too long"},
		{"trade type too long", UpdateTransaction{TradeType: strp(strings.Repeat("t", maxTxTradeTypeLen+1))}, "trade_type too long"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "seq zero" {
				_, err := svc.UpdateTransaction(context.Background(), 0, tc.update)
				assertInvalidInput(t, err, tc.wantErr)
				return
			}
			_, err := svc.UpdateTransaction(context.Background(), seq, tc.update)
			assertInvalidInput(t, err, tc.wantErr)
		})
	}

	// Failed updates must not mutate the stored row.
	amount, share, fee, cash, shareChange, _, _, _ := queryTransaction(t, db, seq)
	if amount != 100 || share != 10 || fee != 1 || cash != 0 || shareChange != 0 {
		t.Fatalf("row mutated after failed updates: amount %v share %v fee %v cash %v shareChange %v", amount, share, fee, cash, shareChange)
	}
}

func TestUpdateTransactionRejectsDirectionChangeToBuyWithoutShares(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	// Dividend rows legitimately carry zero shares; flipping to buy must be rejected.
	seq := seedTransaction(t, db, "U5", "019173", "dividend", 5, 0, 0)
	direction := "buy"
	_, err := svc.UpdateTransaction(context.Background(), seq, UpdateTransaction{Direction: &direction})
	if err == nil || !errors.Is(err, ErrInvalidInput) || !strings.Contains(err.Error(), "confirm_share") {
		t.Fatalf("want confirm_share validation, got %v", err)
	}
}

func TestUpdateTransactionNotFound(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	fee := 1.0
	if _, err := svc.UpdateTransaction(context.Background(), 999, UpdateTransaction{Fee: &fee}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestDeleteTransaction(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	seq := seedTransaction(t, db, "DEL1", "019173", "buy", 100, 10, 1)
	res, err := svc.DeleteTransaction(context.Background(), seq)
	if err != nil {
		t.Fatalf("DeleteTransaction: %v", err)
	}
	if !res.OK || res.Deleted.Seq != seq || res.Deleted.FundCode != "019173" || res.Deleted.Direction != "buy" || res.Deleted.Amount != 100 {
		t.Fatalf("deleted = %+v", res.Deleted)
	}
	if n := countTransactions(t, db); n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}

	if _, err := svc.DeleteTransaction(context.Background(), seq); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: want ErrNotFound, got %v", err)
	}
	if _, err := svc.DeleteTransaction(context.Background(), 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("seq 0: want ErrInvalidInput, got %v", err)
	}
}

func TestDeleteTransactionRecalcsSnapshotToZero(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	seq := seedTransaction(t, db, "DEL2", "019173", "buy", 100, 10, 1)
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
		VALUES ('019173', 'Fund', 10, -100, 1, 10, 0, 0, 'fund', 1)
	`); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.DeleteTransaction(context.Background(), seq); err != nil {
		t.Fatalf("DeleteTransaction: %v", err)
	}
	var held float64
	if err := db.QueryRowContext(context.Background(), `SELECT held_shares FROM portfolio_snapshot WHERE fund_code = '019173'`).Scan(&held); err != nil {
		t.Fatal(err)
	}
	if held != 0 {
		t.Fatalf("held_shares after delete = %v, want 0 (recalc side effect)", held)
	}
}

func strp(s string) *string { return &s }

func assertInvalidInput(t *testing.T, err error, wantSubstring string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", wantSubstring)
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("error = %v, want wrapped ErrInvalidInput", err)
	}
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Fatalf("error = %q, want substring %q", err.Error(), wantSubstring)
	}
}
