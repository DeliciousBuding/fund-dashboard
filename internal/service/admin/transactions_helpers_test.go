package admin

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func ptrFloat(v float64) *float64 { return &v }

func TestNormalizeImportTransactionTable(t *testing.T) {
	long := func(n int, ch string) string { return strings.Repeat(ch, n) }

	cases := []struct {
		name    string
		item    ImportTransaction
		wantErr string
		check   func(t *testing.T, got ImportTransaction)
	}{
		{
			name: "buy valid preserves normalized code and order id",
			item: ImportTransaction{
				FundCode: "019173", TradeTime: "2026-01-01", ConfirmDate: "2026-01-03",
				Direction: "buy", ConfirmAmount: ptrFloat(100), ConfirmShare: ptrFloat(10),
				Fee: ptrFloat(1), OrderID: "OID1", FundName: "Fund", TradeType: "buy",
			},
			check: func(t *testing.T, got ImportTransaction) {
				if got.FundCode != "019173" || got.OrderID != "OID1" {
					t.Fatalf("got fund_code=%q order_id=%q", got.FundCode, got.OrderID)
				}
			},
		},
		{
			name: "security_code fallback",
			item: ImportTransaction{
				SecurityCode: "019173", TradeTime: "2026-01-01", Direction: "buy",
				ConfirmAmount: ptrFloat(100), ConfirmShare: ptrFloat(10), Fee: ptrFloat(0),
			},
			check: func(t *testing.T, got ImportTransaction) {
				if got.FundCode != "019173" {
					t.Fatalf("fund_code = %q, want 019173", got.FundCode)
				}
			},
		},
		{
			name: "numeric shorthand pads to six digits",
			item: ImportTransaction{
				FundCode: "19173", TradeTime: "2026-01-01", Direction: "buy",
				ConfirmAmount: ptrFloat(100), ConfirmShare: ptrFloat(10), Fee: ptrFloat(0),
			},
			check: func(t *testing.T, got ImportTransaction) {
				if got.FundCode != "019173" {
					t.Fatalf("fund_code = %q, want 019173", got.FundCode)
				}
			},
		},
		{
			name: "dividend allows zero share",
			item: ImportTransaction{
				FundCode: "019173", TradeTime: "2026-01-01", Direction: "dividend",
				ConfirmAmount: ptrFloat(5), ConfirmShare: ptrFloat(0), Fee: ptrFloat(0),
			},
		},
		{
			name: "dividend allows nil share",
			item: ImportTransaction{
				FundCode: "019173", TradeTime: "2026-01-01", Direction: "dividend",
				ConfirmAmount: ptrFloat(5), Fee: ptrFloat(0),
			},
		},
		{
			name: "amount boundary at max allowed",
			item: ImportTransaction{
				FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy",
				ConfirmAmount: ptrFloat(maxTxMoney), ConfirmShare: ptrFloat(maxTxMoney), Fee: ptrFloat(maxTxMoney),
			},
		},
		{
			name: "signed overrides boundary allowed",
			item: ImportTransaction{
				FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy",
				ConfirmAmount: ptrFloat(100), ConfirmShare: ptrFloat(10), Fee: ptrFloat(0),
				SignedCashFlow: ptrFloat(maxTxMoney), SignedShareChange: ptrFloat(-maxTxMoney),
			},
		},
		{
			name: "empty confirm date allowed",
			item: ImportTransaction{
				FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy",
				ConfirmAmount: ptrFloat(100), ConfirmShare: ptrFloat(10), Fee: ptrFloat(0),
			},
		},
		{
			name: "empty order id autogenerates",
			item: ImportTransaction{
				FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy",
				ConfirmAmount: ptrFloat(100), ConfirmShare: ptrFloat(10), Fee: ptrFloat(0),
			},
			check: func(t *testing.T, got ImportTransaction) {
				if !strings.HasPrefix(got.OrderID, "go_import_") {
					t.Fatalf("order_id = %q, want go_import_ prefix", got.OrderID)
				}
			},
		},
		{
			name:    "missing fund code",
			item:    ImportTransaction{TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "fund_code is required",
		},
		{
			name:    "fund code too long",
			item:    ImportTransaction{FundCode: long(maxTxFundCodeLen+1, "A"), TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "fund_code too long",
		},
		{
			name:    "missing trade time",
			item:    ImportTransaction{FundCode: "019173", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "trade_time is required",
		},
		{
			name:    "trade time too long",
			item:    ImportTransaction{FundCode: "019173", TradeTime: long(maxTxTimeLen+1, "2"), Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "trade_time too long",
		},
		{
			name:    "confirm date too long",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", ConfirmDate: long(maxTxTimeLen+1, "2"), Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "confirm_date too long",
		},
		{
			name:    "direction empty",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "direction",
		},
		{
			name:    "direction uppercase",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "BUY", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "direction",
		},
		{
			name:    "direction unknown",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "transfer", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "direction",
		},
		{
			name:    "nil amount",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "confirm_amount must be positive",
		},
		{
			name:    "zero amount",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(0), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "confirm_amount must be positive",
		},
		{
			name:    "negative amount",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(-1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "confirm_amount must be positive",
		},
		{
			name:    "amount too large",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(maxTxMoney + 1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "confirm_amount too large",
		},
		{
			name:    "nil fee",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1)},
			wantErr: "fee must be non-negative",
		},
		{
			name:    "negative fee",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(-0.01)},
			wantErr: "fee must be non-negative",
		},
		{
			name:    "fee too large",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(maxTxMoney + 1)},
			wantErr: "fee too large",
		},
		{
			name:    "buy nil share",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), Fee: ptrFloat(0)},
			wantErr: "confirm_share must be positive",
		},
		{
			name:    "buy zero share",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(0), Fee: ptrFloat(0)},
			wantErr: "confirm_share must be positive",
		},
		{
			name:    "sell negative share",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "sell", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(-5), Fee: ptrFloat(0)},
			wantErr: "confirm_share must be positive",
		},
		{
			name:    "buy share too large",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(maxTxMoney + 1), Fee: ptrFloat(0)},
			wantErr: "confirm_share too large",
		},
		{
			name:    "dividend share too large",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "dividend", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(maxTxMoney + 1), Fee: ptrFloat(0)},
			wantErr: "confirm_share too large",
		},
		{
			name:    "order id too long",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0), OrderID: long(maxTxOrderIDLen+1, "x")},
			wantErr: "order_id too long",
		},
		{
			name:    "fund name too long",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0), FundName: long(maxTxFundNameLen+1, "名")},
			wantErr: "fund_name too long",
		},
		{
			name:    "trade type too long",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0), TradeType: long(maxTxTradeTypeLen+1, "t")},
			wantErr: "trade_type too long",
		},
		{
			name:    "signed cash flow above",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0), SignedCashFlow: ptrFloat(maxTxMoney + 1)},
			wantErr: "signed_cash_flow too large",
		},
		{
			name:    "signed cash flow below",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0), SignedCashFlow: ptrFloat(-maxTxMoney - 1)},
			wantErr: "signed_cash_flow too large",
		},
		{
			name:    "signed share change above",
			item:    ImportTransaction{FundCode: "019173", TradeTime: "2026-01-01", Direction: "buy", ConfirmAmount: ptrFloat(1), ConfirmShare: ptrFloat(1), Fee: ptrFloat(0), SignedShareChange: ptrFloat(maxTxMoney + 1)},
			wantErr: "signed_share_change too large",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeImportTransaction(tc.item, 1, 0)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("error = %v, want wrapped ErrInvalidInput", err)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

func TestSignedCashFlowTable(t *testing.T) {
	cases := []struct {
		name      string
		direction string
		amount    float64
		fee       float64
		provided  *float64
		want      float64
	}{
		{"buy adds fee", "buy", 100, 1, nil, -101},
		{"sell subtracts fee", "sell", 100, 1, nil, 99},
		{"dividend ignores fee", "dividend", 100, 1, nil, 100},
		{"unknown direction returns amount", "redeem", 100, 1, nil, 100},
		{"negative fee treated as zero", "buy", 100, -5, nil, -100},
		{"provided override wins", "buy", 100, 1, ptrFloat(42), 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := signedCashFlow(tc.direction, tc.amount, tc.fee, tc.provided); got != tc.want {
				t.Fatalf("signedCashFlow = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSignedShareChangeTable(t *testing.T) {
	cases := []struct {
		name      string
		direction string
		share     float64
		provided  *float64
		want      float64
	}{
		{"buy positive", "buy", 10, nil, 10},
		{"sell negative", "sell", 10, nil, -10},
		{"dividend zero", "dividend", 10, nil, 0},
		{"unknown negative", "redeem", 10, nil, -10},
		{"provided override wins", "buy", 10, ptrFloat(7), 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := signedShareChange(tc.direction, tc.share, tc.provided); got != tc.want {
				t.Fatalf("signedShareChange = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTransactionArgHelpers(t *testing.T) {
	if got := floatArgOrZero(nil); got != 0 {
		t.Fatalf("floatArgOrZero(nil) = %v, want 0", got)
	}
	if got := floatArgOrZero(ptrFloat(3.5)); got != 3.5 {
		t.Fatalf("floatArgOrZero = %v, want 3.5", got)
	}
	if got := nullStringArg(""); got != nil {
		t.Fatalf("nullStringArg(\"\") = %v, want nil", got)
	}
	if got := nullStringArg("x"); got != "x" {
		t.Fatalf("nullStringArg(\"x\") = %v, want x", got)
	}
	if got := nullStringValue(sql.NullString{}); got != "" {
		t.Fatalf("nullStringValue invalid = %q, want empty", got)
	}
	if got := nullStringValue(sql.NullString{String: "y", Valid: true}); got != "y" {
		t.Fatalf("nullStringValue valid = %q, want y", got)
	}
}

func TestCalcSettlementDaysTable(t *testing.T) {
	cases := []struct {
		name        string
		tradeTime   string
		confirmDate string
		want        int
	}{
		{"two calendar days", "2026-01-01", "2026-01-03", 2},
		{"same day zero", "2026-01-01", "2026-01-01", 0},
		{"confirm before trade zero", "2026-01-03", "2026-01-01", 0},
		{"invalid trade zero", "not-a-date", "2026-01-03", 0},
		{"invalid confirm zero", "2026-01-01", "2026-13-40", 0},
		{"empty confirm zero", "2026-01-01", "", 0},
		{"full timestamp prefix", "2026-01-01", "2026-01-03T12:00:00Z", 2},
		{"month boundary", "2026-02-28", "2026-03-01", 1},
		{"leap day", "2024-02-28", "2024-02-29", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CalcSettlementDays(tc.tradeTime, tc.confirmDate); got != tc.want {
				t.Fatalf("CalcSettlementDays = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseYMDTable(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"2026-01-01", true},
		{"", false},
		{"2026-1-1", false},
		{"2026-13-01", false},
		{"2026-02-30", false},
		{"2026-01-01T12:00:00Z", true},
	}
	for _, tc := range cases {
		if _, ok := parseYMD(tc.in); ok != tc.want {
			t.Fatalf("parseYMD(%q) ok = %v, want %v", tc.in, ok, tc.want)
		}
	}
}
