package admin

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"
)

func (s Service) recalcSnapshotTx(ctx context.Context, tx *sql.Tx, code string) error {
	if code == "" {
		return nil
	}

	var shares sql.NullFloat64
	var cost sql.NullFloat64
	var txFundName sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT SUM(COALESCE(signed_share_change, 0)), SUM(COALESCE(signed_cash_flow, 0)), MAX(fund_name)
		FROM transactions
		WHERE fund_code = ?
	`, code).Scan(&shares, &cost, &txFundName); err != nil {
		return fmt.Errorf("recalc snapshot transactions: %w", err)
	}

	heldShares := 0.0
	totalCost := 0.0
	if shares.Valid {
		heldShares = shares.Float64
	}
	if cost.Valid {
		totalCost = cost.Float64
	}
	// Float residue after full sells (~1e-15) is not a real position (#90).
	// Align with SPA / scheduler held filters (held_shares > 0.001).
	const heldSharesDust = 0.001
	if heldShares > -heldSharesDust && heldShares < heldSharesDust {
		heldShares = 0
	}

	var detailName sql.NullString
	var securityType sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT fund_name, security_type
		FROM fund_details
		WHERE fund_code = ?
	`, code).Scan(&detailName, &securityType); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("recalc snapshot identity: %w", err)
	}

	fundName := code
	if detailName.Valid && detailName.String != "" {
		fundName = detailName.String
	} else if txFundName.Valid && txFundName.String != "" {
		fundName = txFundName.String
	}
	secType := "fund"
	if securityType.Valid && securityType.String != "" {
		secType = securityType.String
	}

	var latestNAV sql.NullFloat64
	if err := tx.QueryRowContext(ctx, `
		SELECT unit_nav
		FROM nav_history
		WHERE fund_code = ?
		ORDER BY date DESC
		LIMIT 1
	`, code).Scan(&latestNAV); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("recalc snapshot latest nav: %w", err)
	}

	currentValue := 0.0
	if latestNAV.Valid {
		currentValue = heldShares * latestNAV.Float64
	}
	unrealized := currentValue + totalCost
	pnlPct := 0.0
	if totalCost != 0 {
		denominator := totalCost
		if denominator < 0 {
			denominator = -denominator
		}
		pnlPct = unrealized / denominator * 100
	}

	if heldShares == 0 {
		currentValue = 0
		unrealized = 0
		pnlPct = 0
	}

	// Preserve existing portfolio_id on update; default new rows to portfolio 1.
	// SQLite: PRIMARY KEY(fund_code) only — avoid ON CONFLICT(fund_code, portfolio_id).
	portfolioID := int64(1)
	err := tx.QueryRowContext(ctx, `
		SELECT portfolio_id FROM portfolio_snapshot
		WHERE fund_code = ?
		ORDER BY portfolio_id
		LIMIT 1
	`, code).Scan(&portfolioID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("recalc snapshot read portfolio_id: %w", err)
	}
	if err == sql.ErrNoRows || portfolioID <= 0 {
		portfolioID = 1
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE portfolio_snapshot SET
			fund_name = ?, held_shares = ?, total_cost = ?, latest_nav = ?,
			current_value = ?, unrealized_pnl = ?, pnl_pct = ?, security_type = ?
		WHERE fund_code = ? AND COALESCE(portfolio_id, 1) = ?
	`, fundName, heldShares, totalCost, nullableFloatArg(latestNAV), currentValue, unrealized, pnlPct, secType, code, portfolioID)
	if err != nil {
		return fmt.Errorf("recalc snapshot update: %w", err)
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return fmt.Errorf("recalc snapshot rows affected: %w", raErr)
	}
	if n == 0 {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO portfolio_snapshot
				(fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, code, fundName, heldShares, totalCost, nullableFloatArg(latestNAV), currentValue, unrealized, pnlPct, secType, portfolioID)
		if err != nil {
			return fmt.Errorf("recalc snapshot insert: %w", err)
		}
	}
	return nil
}

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
	code = NormalizeSecurityCode(code)
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

func nullableFloatArg(value sql.NullFloat64) any {
	if !value.Valid {
		return nil
	}
	return value.Float64
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
