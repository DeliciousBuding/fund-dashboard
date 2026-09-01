package admin

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Field / amount ceilings for import + update write paths (#222).
const (
	maxTxOrderIDLen   = 128
	maxTxFundNameLen  = 200
	maxTxTradeTypeLen = 64
	maxTxTimeLen      = 40
	maxTxFundCodeLen  = 32
	maxTxMoney        = 1e9
)

func normalizeImportTransaction(item ImportTransaction, now int64, index int) (ImportTransaction, error) {
	code := item.FundCode
	if code == "" {
		code = item.SecurityCode
	}
	// Reject oversized codes before normalization: NormalizeSecurityCode truncates
	// long codes to 32 chars, which would silently merge distinct securities on
	// the import write path. Keep the post-normalize bound as a defensive guard.
	trimmed := strings.TrimSpace(code)
	if len(trimmed) > maxTxFundCodeLen {
		return ImportTransaction{}, fmt.Errorf("%w: fund_code too long", ErrInvalidInput)
	}
	code = NormalizeSecurityCode(trimmed)
	if code == "" {
		return ImportTransaction{}, fmt.Errorf("%w: fund_code is required", ErrInvalidInput)
	}
	if len(code) > maxTxFundCodeLen {
		return ImportTransaction{}, fmt.Errorf("%w: fund_code too long", ErrInvalidInput)
	}
	if item.TradeTime == "" {
		return ImportTransaction{}, fmt.Errorf("%w: trade_time is required", ErrInvalidInput)
	}
	if len(item.TradeTime) > maxTxTimeLen {
		return ImportTransaction{}, fmt.Errorf("%w: trade_time too long", ErrInvalidInput)
	}
	if item.ConfirmDate != "" && len(item.ConfirmDate) > maxTxTimeLen {
		return ImportTransaction{}, fmt.Errorf("%w: confirm_date too long", ErrInvalidInput)
	}
	if !validTransactionDirections[item.Direction] {
		return ImportTransaction{}, fmt.Errorf("%w: direction must be one of: buy, sell, dividend", ErrInvalidInput)
	}
	if item.ConfirmAmount == nil || *item.ConfirmAmount <= 0 {
		return ImportTransaction{}, fmt.Errorf("%w: confirm_amount must be positive", ErrInvalidInput)
	}
	if *item.ConfirmAmount > maxTxMoney {
		return ImportTransaction{}, fmt.Errorf("%w: confirm_amount too large", ErrInvalidInput)
	}
	if item.Fee == nil || *item.Fee < 0 {
		return ImportTransaction{}, fmt.Errorf("%w: fee must be non-negative", ErrInvalidInput)
	}
	if *item.Fee > maxTxMoney {
		return ImportTransaction{}, fmt.Errorf("%w: fee too large", ErrInvalidInput)
	}
	// buy/sell must move shares; dividend may keep confirm_share 0 (#201).
	if item.Direction == "buy" || item.Direction == "sell" {
		share := 0.0
		if item.ConfirmShare != nil {
			share = *item.ConfirmShare
		}
		if share <= 0 {
			return ImportTransaction{}, fmt.Errorf("%w: confirm_share must be positive for buy/sell", ErrInvalidInput)
		}
		if share > maxTxMoney {
			return ImportTransaction{}, fmt.Errorf("%w: confirm_share too large", ErrInvalidInput)
		}
	} else if item.ConfirmShare != nil && *item.ConfirmShare > maxTxMoney {
		return ImportTransaction{}, fmt.Errorf("%w: confirm_share too large", ErrInvalidInput)
	}
	if len(item.OrderID) > maxTxOrderIDLen {
		return ImportTransaction{}, fmt.Errorf("%w: order_id too long", ErrInvalidInput)
	}
	if len(item.FundName) > maxTxFundNameLen {
		return ImportTransaction{}, fmt.Errorf("%w: fund_name too long", ErrInvalidInput)
	}
	if len(item.TradeType) > maxTxTradeTypeLen {
		return ImportTransaction{}, fmt.Errorf("%w: trade_type too long", ErrInvalidInput)
	}
	if item.SignedCashFlow != nil {
		v := *item.SignedCashFlow
		if v > maxTxMoney || v < -maxTxMoney {
			return ImportTransaction{}, fmt.Errorf("%w: signed_cash_flow too large", ErrInvalidInput)
		}
	}
	if item.SignedShareChange != nil {
		v := *item.SignedShareChange
		if v > maxTxMoney || v < -maxTxMoney {
			return ImportTransaction{}, fmt.Errorf("%w: signed_share_change too large", ErrInvalidInput)
		}
	}
	item.FundCode = code
	if item.OrderID == "" {
		item.OrderID = fmt.Sprintf("go_import_%d_%d", now, index)
	}
	return item, nil
}

// signedCashFlow matches XIRR cashflow convention:
// buy = -(amount+fee), sell = amount-fee, dividend = amount.
// Explicit provided values win (import/backfill escape hatch).
func signedCashFlow(direction string, amount float64, fee float64, provided *float64) float64 {
	if provided != nil {
		return *provided
	}
	if fee < 0 {
		fee = 0
	}
	switch direction {
	case "buy":
		return -(amount + fee)
	case "sell":
		return amount - fee
	default: // dividend and unknown
		return amount
	}
}

func signedShareChange(direction string, share float64, provided *float64) float64 {
	if provided != nil {
		return *provided
	}
	if direction == "dividend" {
		return 0
	}
	if direction == "buy" {
		return share
	}
	return -share
}

func floatArgOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func nullStringArg(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func CalcSettlementDays(tradeTime string, confirmDate string) int {
	trade, ok := parseYMD(tradeTime)
	if !ok {
		return 0
	}
	confirm, ok := parseYMD(confirmDate)
	if !ok {
		return 0
	}
	days := int(confirm.Sub(trade).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

var ymdPattern = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)

func parseYMD(value string) (time.Time, bool) {
	if len(value) < 10 {
		return time.Time{}, false
	}
	date := value[:10]
	if !ymdPattern.MatchString(date) {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
