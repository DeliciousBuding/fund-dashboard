package admin

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var validTransactionDirections = map[string]bool{
	"buy":      true,
	"sell":     true,
	"dividend": true,
}

var ErrInvalidInput = errors.New("invalid input")
var ErrNotFound = errors.New("not found")

type ImportTransaction struct {
	OrderID           string   `json:"order_id"`
	FundCode          string   `json:"fund_code"`
	SecurityCode      string   `json:"security_code"`
	FundName          string   `json:"fund_name"`
	TradeTime         string   `json:"trade_time"`
	ConfirmDate       string   `json:"confirm_date"`
	TradeType         string   `json:"trade_type"`
	Direction         string   `json:"direction"`
	ConfirmAmount     *float64 `json:"confirm_amount"`
	ConfirmShare      *float64 `json:"confirm_share"`
	Fee               *float64 `json:"fee"`
	SignedCashFlow    *float64 `json:"signed_cash_flow"`
	SignedShareChange *float64 `json:"signed_share_change"`
}

type ImportTransactionsResult struct {
	OK            bool `json:"ok"`
	Imported      int  `json:"imported"`
	Total         int  `json:"total"`
	AffectedFunds int  `json:"affected_funds"`
}

type UpdateTransaction struct {
	TradeTime     *string  `json:"trade_time"`
	ConfirmDate   *string  `json:"confirm_date"`
	TradeType     *string  `json:"trade_type"`
	Direction     *string  `json:"direction"`
	ConfirmAmount *float64 `json:"confirm_amount"`
	ConfirmShare  *float64 `json:"confirm_share"`
	Fee           *float64 `json:"fee"`
	FundCode      *string  `json:"fund_code"`
}

type UpdateTransactionResult struct {
	OK      bool                     `json:"ok"`
	Updated UpdatedTransactionFields `json:"updated"`
}

type UpdatedTransactionFields struct {
	Seq    int      `json:"seq"`
	Fields []string `json:"fields"`
}

type DeleteTransactionResult struct {
	OK      bool               `json:"ok"`
	Deleted DeletedTransaction `json:"deleted"`
}

type DeletedTransaction struct {
	Seq       int     `json:"seq"`
	FundCode  string  `json:"fund_code"`
	Direction string  `json:"direction"`
	Amount    float64 `json:"amount"`
}

type transactionRow struct {
	Seq           int
	TradeTime     string
	ConfirmDate   string
	TradeType     string
	Direction     string
	FundCode      string
	FundName      string
	ConfirmAmount float64
	ConfirmShare  float64
	Fee           float64
}

func (s Service) ImportTransactions(ctx context.Context, transactions []ImportTransaction) (ImportTransactionsResult, error) {
	if len(transactions) == 0 {
		return ImportTransactionsResult{}, fmt.Errorf("%w: transactions array is required", ErrInvalidInput)
	}
	// Bound import size for admin/SPA/MCP write paths (#214).
	const maxImportRows = 5000
	if len(transactions) > maxImportRows {
		return ImportTransactionsResult{}, fmt.Errorf("%w: transactions max %d rows", ErrInvalidInput, maxImportRows)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportTransactionsResult{}, fmt.Errorf("begin import transactions: %w", err)
	}
	defer tx.Rollback()

	// Portable idempotency: SQLite may have UNIQUE(order_id); PG production schema does not.
	// WHERE NOT EXISTS works on both without requiring ON CONFLICT target.
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO transactions
		(order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name,
		 confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM transactions WHERE order_id = ? AND fund_code = ?)
	`)
	if err != nil {
		return ImportTransactionsResult{}, fmt.Errorf("prepare import transactions: %w", err)
	}
	defer stmt.Close()

	imported := 0
	affected := map[string]bool{}
	now := time.Now().UTC().UnixNano()
	for i, item := range transactions {
		normalized, err := normalizeImportTransaction(item, now, i)
		if err != nil {
			return ImportTransactionsResult{}, err
		}
		result, err := stmt.ExecContext(ctx,
			normalized.OrderID,
			normalized.TradeTime,
			nullStringArg(normalized.ConfirmDate),
			nullStringArg(normalized.TradeType),
			normalized.Direction,
			normalized.FundCode,
			nullStringArg(normalized.FundName),
			*normalized.ConfirmAmount,
			floatArgOrZero(normalized.ConfirmShare),
			*normalized.Fee,
			signedCashFlow(normalized.Direction, *normalized.ConfirmAmount, *normalized.Fee, normalized.SignedCashFlow),
			signedShareChange(normalized.Direction, floatArgOrZero(normalized.ConfirmShare), normalized.SignedShareChange),
			CalcSettlementDays(normalized.TradeTime, normalized.ConfirmDate),
			normalized.OrderID,  // NOT EXISTS guard
			normalized.FundCode, // NOT EXISTS guard (conversion shares order_id)
		)
		if err != nil {
			return ImportTransactionsResult{}, fmt.Errorf("insert transaction %d: %w", i, err)
		}
		if rows, _ := result.RowsAffected(); rows > 0 {
			imported++
			affected[normalized.FundCode] = true
		}
	}

	for code := range affected {
		if err := s.recalcSnapshotTx(ctx, tx, code); err != nil {
			return ImportTransactionsResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ImportTransactionsResult{}, fmt.Errorf("commit import transactions: %w", err)
	}

	return ImportTransactionsResult{OK: true, Imported: imported, Total: len(transactions), AffectedFunds: len(affected)}, nil
}

