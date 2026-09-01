package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// TransactionListItem is one ledger row for the /api/transactions read model
// (fund-detail transactions stay on their own narrower shape).
type TransactionListItem struct {
	Seq            int64    `json:"seq"`
	TradeTime      *string  `json:"trade_time"`
	ConfirmDate    *string  `json:"confirm_date"`
	Direction      *string  `json:"direction"`
	TradeType      *string  `json:"trade_type"`
	FundCode       string   `json:"fund_code"`
	FundName       *string  `json:"fund_name"`
	Amount         *float64 `json:"amount"`
	Shares         *float64 `json:"shares"`
	Fee            *float64 `json:"fee"`
	OrderID        *string  `json:"order_id"`
	Anomaly        *string  `json:"anomaly"`
	SettlementDays *int64   `json:"settlement_days"`
	PortfolioID    *int64   `json:"portfolio_id"`
}

type ListTransactionsOptions struct {
	PortfolioID int
	FundCode    string
	Direction   string // exact match on direction, e.g. 买/卖
	Search      string // substring on fund_name / fund_code
	Limit       int    // default 200, hard cap 5000
	Offset      int
	SortBy      string // whitelisted column id: date|fund|direction|trade_type|amount|shares|fee
	SortDesc    bool
}

type ListTransactionsResult struct {
	Transactions []TransactionListItem `json:"transactions"`
	Total        int64                 `json:"total"`
}

// transactions ledger columns that only exist on newer schemas (PG fresh
// installs, migrated SQLite). Legacy ledger DBs may lack them — probe and
// adapt instead of ALTERing the user's ledger (see buildKlineUpsert).
var transactionsOptionalColumns = []string{"anomaly", "settlement_days", "portfolio_id"}

// ListTransactions serves the ledger page: newest first, filtered and paginated.
func (s Service) ListTransactions(ctx context.Context, opts ListTransactionsOptions) (*ListTransactionsResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}
	offset := max(opts.Offset, 0)

	cols, err := s.tableColumns(ctx, "transactions")
	if err != nil {
		return nil, fmt.Errorf("probe transactions columns: %w", err)
	}
	has := func(name string) bool {
		_, found := cols[name]
		return found
	}

	selectCols := []string{
		"seq", "trade_time", "confirm_date", "direction", "trade_type",
		"fund_code", "fund_name", "confirm_amount", "confirm_share", "fee", "order_id",
	}
	for _, opt := range transactionsOptionalColumns {
		if has(opt) {
			selectCols = append(selectCols, opt)
		}
	}

	var where []string
	var args []any
	if opts.PortfolioID > 0 && has("portfolio_id") {
		where = append(where, "portfolio_id = ?")
		args = append(args, opts.PortfolioID)
	}
	if code := strings.TrimSpace(opts.FundCode); code != "" {
		where = append(where, "fund_code = ?")
		args = append(args, code)
	}
	if direction := strings.TrimSpace(opts.Direction); direction != "" {
		where = append(where, "direction = ?")
		args = append(args, direction)
	}
	if search := strings.TrimSpace(opts.Search); search != "" {
		where = append(where, "(fund_name LIKE ? OR fund_code LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	countArgs := append([]any(nil), args...)
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM transactions`+whereClause, countArgs...,
	).Scan(&total); err != nil {
		return nil, fmt.Errorf("count transactions: %w", err)
	}

	orderBy := "trade_time DESC, seq DESC"
	switch opts.SortBy {
	case "date":
		orderBy = "trade_time"
	case "fund":
		orderBy = "COALESCE(fund_name, fund_code)"
	case "direction":
		orderBy = "direction"
	case "trade_type":
		orderBy = "trade_type"
	case "amount":
		orderBy = "confirm_amount"
	case "shares":
		orderBy = "confirm_share"
	case "fee":
		orderBy = "fee"
	}
	if opts.SortBy != "" {
		if opts.SortDesc {
			orderBy += " DESC"
		} else {
			orderBy += " ASC"
		}
		orderBy += ", seq DESC"
	}
	queryArgs := append(append([]any(nil), args...), limit, offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+strings.Join(selectCols, ", ")+`
		FROM transactions`+whereClause+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	items := []TransactionListItem{}
	for rows.Next() {
		var item TransactionListItem
		var tradeTime, confirmDate, direction, tradeType, fundName, orderID, anomaly sql.NullString
		var amount, shares, fee sql.NullFloat64
		var settlement, portfolioID sql.NullInt64

		dest := []any{
			&item.Seq, &tradeTime, &confirmDate, &direction, &tradeType,
			&item.FundCode, &fundName, &amount, &shares, &fee, &orderID,
		}
		if has("anomaly") {
			dest = append(dest, &anomaly)
		}
		if has("settlement_days") {
			dest = append(dest, &settlement)
		}
		if has("portfolio_id") {
			dest = append(dest, &portfolioID)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		item.TradeTime = nullableStringValuePtr(tradeTime)
		item.ConfirmDate = dateOnlyNullablePtr(confirmDate)
		item.Direction = nullableStringValuePtr(direction)
		item.TradeType = nullableStringValuePtr(tradeType)
		item.FundName = nullableStringValuePtr(fundName)
		item.Amount = nullableFloat64Ptr(amount)
		item.Shares = nullableFloat64Ptr(shares)
		item.Fee = nullableFloat64Ptr(fee)
		item.OrderID = nullableStringValuePtr(orderID)
		item.Anomaly = nullableStringValuePtr(anomaly)
		item.SettlementDays = nullableInt64Ptr(settlement)
		item.PortfolioID = nullableInt64Ptr(portfolioID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transactions: %w", err)
	}
	return &ListTransactionsResult{Transactions: items, Total: total}, nil
}
