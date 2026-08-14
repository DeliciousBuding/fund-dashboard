package admin

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeImportTransactionFieldClamps(t *testing.T) {
	amt := 100.0
	fee := 0.0
	share := 10.0
	base := ImportTransaction{
		FundCode:      "019173",
		TradeTime:     "2026-01-01",
		Direction:     "buy",
		ConfirmAmount: &amt,
		ConfirmShare:  &share,
		Fee:           &fee,
		OrderID:       "OID1",
		FundName:      "Fund",
		TradeType:     "买入",
	}
	if _, err := normalizeImportTransaction(base, 1, 0); err != nil {
		t.Fatalf("valid import: %v", err)
	}

	// order_id too long
	longOID := base
	longOID.OrderID = strings.Repeat("x", maxTxOrderIDLen+1)
	if _, err := normalizeImportTransaction(longOID, 1, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("order_id: want ErrInvalidInput, got %v", err)
	}

	// fund_name too long
	longName := base
	longName.FundName = strings.Repeat("名", maxTxFundNameLen+1)
	if _, err := normalizeImportTransaction(longName, 1, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("fund_name: want ErrInvalidInput, got %v", err)
	}

	// amount too large
	big := 1e9 + 1
	bigAmt := base
	bigAmt.ConfirmAmount = &big
	if _, err := normalizeImportTransaction(bigAmt, 1, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("confirm_amount: want ErrInvalidInput, got %v", err)
	}

	// trade_time too long
	longTime := base
	longTime.TradeTime = strings.Repeat("2", maxTxTimeLen+1)
	if _, err := normalizeImportTransaction(longTime, 1, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("trade_time: want ErrInvalidInput, got %v", err)
	}

	// signed override magnitude
	huge := maxTxMoney + 1
	signed := base
	signed.SignedCashFlow = &huge
	if _, err := normalizeImportTransaction(signed, 1, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("signed_cash_flow: want ErrInvalidInput, got %v", err)
	}
}
