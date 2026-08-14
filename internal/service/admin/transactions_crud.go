package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (s Service) UpdateTransaction(ctx context.Context, seq int, update UpdateTransaction) (UpdateTransactionResult, error) {
	if seq <= 0 {
		return UpdateTransactionResult{}, fmt.Errorf("%w: seq required", ErrInvalidInput)
	}
	existing, err := s.getTransaction(ctx, seq)
	if err != nil {
		return UpdateTransactionResult{}, err
	}

	sets := []string{}
	args := []any{}
	fields := []string{}
	next := existing

	if update.TradeTime != nil {
		if len(*update.TradeTime) > maxTxTimeLen {
			return UpdateTransactionResult{}, fmt.Errorf("%w: trade_time too long", ErrInvalidInput)
		}
		next.TradeTime = *update.TradeTime
		sets = append(sets, "trade_time = ?")
		args = append(args, *update.TradeTime)
		fields = append(fields, "trade_time")
	}
	if update.ConfirmDate != nil {
		if len(*update.ConfirmDate) > maxTxTimeLen {
			return UpdateTransactionResult{}, fmt.Errorf("%w: confirm_date too long", ErrInvalidInput)
		}
		next.ConfirmDate = *update.ConfirmDate
		sets = append(sets, "confirm_date = ?")
		args = append(args, nullStringArg(*update.ConfirmDate))
		fields = append(fields, "confirm_date")
	}
	if update.TradeType != nil {
		if len(*update.TradeType) > maxTxTradeTypeLen {
			return UpdateTransactionResult{}, fmt.Errorf("%w: trade_type too long", ErrInvalidInput)
		}
		next.TradeType = *update.TradeType
		sets = append(sets, "trade_type = ?")
		args = append(args, nullStringArg(*update.TradeType))
		fields = append(fields, "trade_type")
	}
	if update.Direction != nil {
		if !validTransactionDirections[*update.Direction] {
			return UpdateTransactionResult{}, fmt.Errorf("%w: direction must be one of: buy, sell, dividend", ErrInvalidInput)
		}
		next.Direction = *update.Direction
		sets = append(sets, "direction = ?")
		args = append(args, *update.Direction)
		fields = append(fields, "direction")
	}
	if update.ConfirmAmount != nil {
		if *update.ConfirmAmount <= 0 {
			return UpdateTransactionResult{}, fmt.Errorf("%w: confirm_amount must be positive", ErrInvalidInput)
		}
		if *update.ConfirmAmount > maxTxMoney {
			return UpdateTransactionResult{}, fmt.Errorf("%w: confirm_amount too large", ErrInvalidInput)
		}
		next.ConfirmAmount = *update.ConfirmAmount
		sets = append(sets, "confirm_amount = ?")
		args = append(args, *update.ConfirmAmount)
		fields = append(fields, "confirm_amount")
	}
	if update.ConfirmShare != nil {
		if (next.Direction == "buy" || next.Direction == "sell") && *update.ConfirmShare <= 0 {
			return UpdateTransactionResult{}, fmt.Errorf("%w: confirm_share must be positive for buy/sell", ErrInvalidInput)
		}
		if *update.ConfirmShare > maxTxMoney {
			return UpdateTransactionResult{}, fmt.Errorf("%w: confirm_share too large", ErrInvalidInput)
		}
		next.ConfirmShare = *update.ConfirmShare
		sets = append(sets, "confirm_share = ?")
		args = append(args, *update.ConfirmShare)
		fields = append(fields, "confirm_share")
	}
	// Direction change to buy/sell with non-positive shares is invalid.
	if (next.Direction == "buy" || next.Direction == "sell") && next.ConfirmShare <= 0 {
		return UpdateTransactionResult{}, fmt.Errorf("%w: confirm_share must be positive for buy/sell", ErrInvalidInput)
	}
	if update.Fee != nil {
		if *update.Fee < 0 {
			return UpdateTransactionResult{}, fmt.Errorf("%w: fee must be non-negative", ErrInvalidInput)
		}
		if *update.Fee > maxTxMoney {
			return UpdateTransactionResult{}, fmt.Errorf("%w: fee too large", ErrInvalidInput)
		}
		next.Fee = *update.Fee
		sets = append(sets, "fee = ?")
		args = append(args, *update.Fee)
		fields = append(fields, "fee")
	}
	if update.FundCode != nil {
		next.FundCode = NormalizeSecurityCode(*update.FundCode)
		if next.FundCode == "" || len(next.FundCode) > maxTxFundCodeLen {
			return UpdateTransactionResult{}, fmt.Errorf("%w: fund_code invalid", ErrInvalidInput)
		}
		sets = append(sets, "fund_code = ?")
		args = append(args, next.FundCode)
		fields = append(fields, "fund_code")
	}
	if len(sets) == 0 {
		return UpdateTransactionResult{}, fmt.Errorf("%w: no valid fields to update", ErrInvalidInput)
	}

	sets = append(sets, "signed_cash_flow = ?", "signed_share_change = ?", "settlement_days = ?")
	args = append(args,
		signedCashFlow(next.Direction, next.ConfirmAmount, next.Fee, nil),
		signedShareChange(next.Direction, next.ConfirmShare, nil),
		CalcSettlementDays(next.TradeTime, next.ConfirmDate),
		seq,
	)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UpdateTransactionResult{}, fmt.Errorf("begin update transaction: %w", err)
	}
	defer tx.Rollback()

	query := "UPDATE transactions SET " + strings.Join(sets, ", ") + " WHERE seq = ?"
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return UpdateTransactionResult{}, fmt.Errorf("update transaction: %w", err)
	}
	if err := s.recalcSnapshotTx(ctx, tx, next.FundCode); err != nil {
		return UpdateTransactionResult{}, err
	}
	if next.FundCode != existing.FundCode {
		if err := s.recalcSnapshotTx(ctx, tx, existing.FundCode); err != nil {
			return UpdateTransactionResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return UpdateTransactionResult{}, fmt.Errorf("commit update transaction: %w", err)
	}

	return UpdateTransactionResult{OK: true, Updated: UpdatedTransactionFields{Seq: seq, Fields: fields}}, nil
}

func (s Service) DeleteTransaction(ctx context.Context, seq int) (DeleteTransactionResult, error) {
	if seq <= 0 {
		return DeleteTransactionResult{}, fmt.Errorf("%w: seq required", ErrInvalidInput)
	}
	existing, err := s.getTransaction(ctx, seq)
	if err != nil {
		return DeleteTransactionResult{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return DeleteTransactionResult{}, fmt.Errorf("begin delete transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM transactions WHERE seq = ?", seq); err != nil {
		return DeleteTransactionResult{}, fmt.Errorf("delete transaction: %w", err)
	}
	if err := s.recalcSnapshotTx(ctx, tx, existing.FundCode); err != nil {
		return DeleteTransactionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return DeleteTransactionResult{}, fmt.Errorf("commit delete transaction: %w", err)
	}

	return DeleteTransactionResult{
		OK: true,
		Deleted: DeletedTransaction{
			Seq:       seq,
			FundCode:  existing.FundCode,
			Direction: existing.Direction,
			Amount:    existing.ConfirmAmount,
		},
	}, nil
}

func (s Service) getTransaction(ctx context.Context, seq int) (transactionRow, error) {
	var row transactionRow
	var orderID sql.NullString
	var tradeTime sql.NullString
	var confirmDate sql.NullString
	var tradeType sql.NullString
	var direction sql.NullString
	var fundCode sql.NullString
	var fundName sql.NullString
	var amount sql.NullFloat64
	var share sql.NullFloat64
	var fee sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
		SELECT seq, order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name,
		       confirm_amount, confirm_share, fee
		FROM transactions
		WHERE seq = ?
	`, seq).Scan(&row.Seq, &orderID, &tradeTime, &confirmDate, &tradeType, &direction, &fundCode, &fundName, &amount, &share, &fee)
	if err != nil {
		if err == sql.ErrNoRows {
			return transactionRow{}, ErrNotFound
		}
		return transactionRow{}, fmt.Errorf("query transaction: %w", err)
	}
	row.TradeTime = clampAdminText(nullStringValue(tradeTime), 40)
	row.ConfirmDate = clampAdminText(nullStringValue(confirmDate), 40)
	row.TradeType = clampAdminText(nullStringValue(tradeType), 64)
	row.Direction = clampAdminText(nullStringValue(direction), 16)
	row.FundCode = clampAdminText(nullStringValue(fundCode), 32)
	row.FundName = clampAdminText(nullStringValue(fundName), 200)
	if amount.Valid {
		row.ConfirmAmount = amount.Float64
	}
	if share.Valid {
		row.ConfirmShare = share.Float64
	}
	if fee.Valid {
		row.Fee = fee.Float64
	}
	return row, nil
}

