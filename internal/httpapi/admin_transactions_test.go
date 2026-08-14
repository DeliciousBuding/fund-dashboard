package httpapi

import (
	"context"
	"net/http"
	"testing"
)

func TestAdminTransactionRoutesImportUpdateDelete(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(testCfg(), WithDB(db))

	imported := doJSONRequest(t, router, http.MethodPost, "/api/admin/import-transactions", map[string]any{
		"transactions": []map[string]any{
			{
				"order_id":       "WEBADD001",
				"fund_code":      "19173",
				"fund_name":      "纳斯达克100指数(QDII)C",
				"trade_time":     "2026-06-03T09:00:00Z",
				"confirm_date":   "2026-06-04",
				"trade_type":     "用户买入",
				"direction":      "buy",
				"confirm_amount": 500,
				"confirm_share":  250,
				"fee":            0,
			},
			{
				"order_id":       "WEBADD002",
				"security_code":  "aapl",
				"fund_name":      "Apple Inc.",
				"trade_time":     "2026-06-03T09:00:00Z",
				"trade_type":     "分红",
				"direction":      "dividend",
				"confirm_amount": 8.5,
				"confirm_share":  99,
				"fee":            0,
			},
		},
	}, http.StatusOK)
	if imported["ok"] != true ||
		imported["imported"].(float64) != 2 ||
		imported["total"].(float64) != 2 ||
		imported["affected_funds"].(float64) != 2 {
		t.Fatalf("import response = %s", toJSONString(t, imported))
	}

	ctx := context.Background()
	var signedCash float64
	var signedShare float64
	var settlement int
	if err := db.QueryRowContext(ctx, `
		SELECT signed_cash_flow, signed_share_change, settlement_days
		FROM transactions
		WHERE order_id = 'WEBADD001'
	`).Scan(&signedCash, &signedShare, &settlement); err != nil {
		t.Fatalf("query imported buy transaction: %v", err)
	}
	if signedCash != -500 || signedShare != 250 || settlement != 1 {
		t.Fatalf("imported buy derived fields = cash %.2f share %.2f settlement %d", signedCash, signedShare, settlement)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT signed_cash_flow, signed_share_change, settlement_days
		FROM transactions
		WHERE order_id = 'WEBADD002'
	`).Scan(&signedCash, &signedShare, &settlement); err != nil {
		t.Fatalf("query imported dividend transaction: %v", err)
	}
	if signedCash != 8.5 || signedShare != 0 || settlement != 0 {
		t.Fatalf("imported dividend derived fields = cash %.2f share %.2f settlement %d", signedCash, signedShare, settlement)
	}

	var seq int
	if err := db.QueryRowContext(ctx, "SELECT seq FROM transactions WHERE order_id = 'WEBADD001'").Scan(&seq); err != nil {
		t.Fatalf("query imported seq: %v", err)
	}
	updated := doJSONRequest(t, router, http.MethodPut, "/api/admin/transactions/"+itoa(seq), map[string]any{
		"direction":      "sell",
		"confirm_amount": 120,
		"confirm_share":  60,
		"confirm_date":   "2026-06-07",
	}, http.StatusOK)
	if updated["ok"] != true {
		t.Fatalf("update response = %s", toJSONString(t, updated))
	}
	if err := db.QueryRowContext(ctx, `
		SELECT signed_cash_flow, signed_share_change, settlement_days
		FROM transactions
		WHERE seq = ?
	`, seq).Scan(&signedCash, &signedShare, &settlement); err != nil {
		t.Fatalf("query updated transaction: %v", err)
	}
	if signedCash != 120 || signedShare != -60 || settlement != 4 {
		t.Fatalf("updated derived fields = cash %.2f share %.2f settlement %d", signedCash, signedShare, settlement)
	}

	deleted := doJSONRequest(t, router, http.MethodDelete, "/api/admin/transactions/"+itoa(seq), nil, http.StatusOK)
	if deleted["ok"] != true {
		t.Fatalf("delete response = %s", toJSONString(t, deleted))
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transactions WHERE seq = ?", seq).Scan(&count); err != nil {
		t.Fatalf("count deleted transaction: %v", err)
	}
	if count != 0 {
		t.Fatalf("deleted transaction count = %d, want 0", count)
	}
}

func TestAdminTransactionRoutesValidateInputs(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(testCfg(), WithDB(db))

	doJSONRequest(t, router, http.MethodPost, "/api/admin/import-transactions", map[string]any{
		"transactions": []map[string]any{
			{
				"fund_code":      "019173",
				"trade_time":     "2026-06-03",
				"direction":      "hold",
				"confirm_amount": 1,
				"fee":            0,
			},
		},
	}, http.StatusBadRequest)
	doJSONRequest(t, router, http.MethodPut, "/api/admin/transactions/99999", map[string]any{
		"direction": "buy",
	}, http.StatusNotFound)
	doJSONRequest(t, router, http.MethodPut, "/api/admin/transactions/1", map[string]any{}, http.StatusBadRequest)
	doJSONRequest(t, router, http.MethodDelete, "/api/admin/transactions/99999", nil, http.StatusNotFound)
}

func TestAdminTransactionImportIncludesFeeInSignedCashFlow(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(testCfg(), WithDB(db))

	// buy 100 + fee 1.5 => signed_cash_flow -101.5 (matches XIRR)
	// sell 50 - fee 0.5 => signed_cash_flow 49.5
	imported := doJSONRequest(t, router, http.MethodPost, "/api/admin/import-transactions", map[string]any{
		"transactions": []map[string]any{
			{
				"order_id":       "FEEBUY001",
				"fund_code":      "19173",
				"fund_name":      "纳斯达克100",
				"trade_time":     "2026-06-10T09:00:00Z",
				"confirm_date":   "2026-06-11",
				"trade_type":     "用户买入",
				"direction":      "buy",
				"confirm_amount": 100,
				"confirm_share":  50,
				"fee":            1.5,
			},
			{
				"order_id":       "FEESELL001",
				"fund_code":      "19173",
				"fund_name":      "纳斯达克100",
				"trade_time":     "2026-06-12T09:00:00Z",
				"confirm_date":   "2026-06-13",
				"trade_type":     "用户卖出",
				"direction":      "sell",
				"confirm_amount": 50,
				"confirm_share":  20,
				"fee":            0.5,
			},
		},
	}, http.StatusOK)
	if imported["ok"] != true || imported["imported"].(float64) != 2 {
		t.Fatalf("import = %s", toJSONString(t, imported))
	}

	var buyCash, sellCash float64
	if err := db.QueryRow(`SELECT signed_cash_flow FROM transactions WHERE order_id='FEEBUY001'`).Scan(&buyCash); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT signed_cash_flow FROM transactions WHERE order_id='FEESELL001'`).Scan(&sellCash); err != nil {
		t.Fatal(err)
	}
	if buyCash != -101.5 {
		t.Fatalf("buy signed_cash_flow = %v, want -101.5 (amount+fee)", buyCash)
	}
	if sellCash != 49.5 {
		t.Fatalf("sell signed_cash_flow = %v, want 49.5 (amount-fee)", sellCash)
	}
}
