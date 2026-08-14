package portfolio

import (
	"context"
	"database/sql"
	"fmt"
)

func (s Service) GetFundDetail(ctx context.Context, code string, portfolioID int) (*FundDetail, error) {
	portfolioID = clampPortfolioID(portfolioID)
	identity, err := s.queryFundIdentity(ctx, code)
	if err != nil {
		return nil, err
	}
	if identity == nil {
		return nil, nil
	}

	position, err := s.queryFundDetailPosition(ctx, code, portfolioID)
	if err != nil {
		return nil, err
	}
	navCount, lastNAV, err := s.queryFundNavSummary(ctx, code)
	if err != nil {
		return nil, err
	}
	transactions, err := s.queryFundTransactions(ctx, code)
	if err != nil {
		return nil, err
	}
	if len(transactions) == 0 {
		return &FundDetail{
			Code:         code,
			Name:         identity.Name,
			Type:         identity.Type,
			SecurityType: identity.SecurityType,
			Market:       identity.Market,
			Position:     position,
			NAVCount:     navCount,
			LastNAVDate:  lastNAV,
		}, nil
	}

	return &FundDetail{
		Code:             code,
		Name:             identity.Name,
		Type:             identity.Type,
		SecurityType:     identity.SecurityType,
		Market:           identity.Market,
		Position:         position,
		NAVCount:         navCount,
		LastNAVDate:      lastNAV,
		TransactionCount: len(transactions),
		Transactions:     transactions,
	}, nil
}

func (s Service) GetNavHistory(ctx context.Context, code string, limit int) (NavHistoryReport, error) {
	if limit <= 0 {
		limit = 200
	}
	// Hard cap public ?limit= to bound DB/response cost (#201).
	const maxNavLimit = 2000
	if limit > maxNavLimit {
		limit = maxNavLimit
	}
	identity, err := s.queryFundIdentity(ctx, code)
	if err != nil {
		return NavHistoryReport{}, err
	}
	report := NavHistoryReport{
		Code:             code,
		SecurityType:     "fund",
		Market:           "",
		DecisionBoundary: "facts_only",
	}
	if identity != nil {
		report.SecurityType = identity.SecurityType
		report.Market = identity.Market
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT date, unit_nav, daily_change_pct, COALESCE(security_type, 'fund')
		FROM nav_history
		WHERE fund_code = ?
		ORDER BY date DESC
		LIMIT ?
	`, code, limit)
	if err != nil {
		return NavHistoryReport{}, fmt.Errorf("query nav history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var point NavHistoryPoint
		var change sql.NullFloat64
		if err := rows.Scan(&point.Date, &point.UnitNAV, &change, &point.SecurityType); err != nil {
			return NavHistoryReport{}, fmt.Errorf("scan nav history point: %w", err)
		}
		point.DailyChangePct = nullableFloat64Ptr(change)
		report.Data = append(report.Data, point)
	}
	if err := rows.Err(); err != nil {
		return NavHistoryReport{}, fmt.Errorf("nav history rows: %w", err)
	}
	return report, nil
}

func (s Service) queryFundIdentity(ctx context.Context, code string) (*fundIdentity, error) {
	var name sql.NullString
	var fundType sql.NullString
	var securityType sql.NullString
	var market sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT fund_name, fund_type, security_type, market
		FROM fund_details
		WHERE fund_code = ?
	`, code).Scan(&name, &fundType, &securityType, &market)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query fund identity: %w", err)
	}
	if name.Valid {
		name.String = clampPortfolioText(name.String, 200)
	}
	if fundType.Valid {
		fundType.String = clampPortfolioText(fundType.String, 64)
	}
	if securityType.Valid {
		securityType.String = clampPortfolioText(securityType.String, 32)
	}
	if market.Valid {
		market.String = clampPortfolioText(market.String, 32)
	}
	return &fundIdentity{
		Name:         nullableStringValuePtr(name),
		Type:         nullableStringValuePtr(fundType),
		SecurityType: nullableStringValue(securityType, "fund"),
		Market:       nullableStringValue(market, ""),
	}, nil
}

func (s Service) queryFundDetailPosition(ctx context.Context, code string, portfolioID int) (FundDetailPosition, error) {
	var shares sql.NullFloat64
	var cost sql.NullFloat64
	var latestNAV sql.NullFloat64
	var value sql.NullFloat64
	var pnl sql.NullFloat64
	var pnlPct sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
		SELECT held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct
		FROM portfolio_snapshot
		WHERE fund_code = ? AND COALESCE(portfolio_id, 1) = ?
	`, code, portfolioID).Scan(&shares, &cost, &latestNAV, &value, &pnl, &pnlPct)
	if err != nil {
		if err == sql.ErrNoRows {
			return FundDetailPosition{}, nil
		}
		return FundDetailPosition{}, fmt.Errorf("query fund position: %w", err)
	}
	position := FundDetailPosition{
		CostBasis:     nullableFloat64Ptr(cost),
		LatestNAV:     nullableFloat64Ptr(latestNAV),
		MarketValue:   nullableFloat64Ptr(value),
		UnrealizedPNL: nullableFloat64Ptr(pnl),
		PNLPct:        nullableFloat64Ptr(pnlPct),
	}
	if shares.Valid {
		position.Shares = shares.Float64
	}
	return position, nil
}

func (s Service) queryFundNavSummary(ctx context.Context, code string) (int, *string, error) {
	var count int
	var last sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), MAX(date)
		FROM nav_history
		WHERE fund_code = ?
	`, code).Scan(&count, &last); err != nil {
		return 0, nil, fmt.Errorf("query fund nav summary: %w", err)
	}
	return count, dateOnlyNullablePtr(last), nil
}

func (s Service) queryFundTransactions(ctx context.Context, code string) ([]FundTransaction, error) {
	// Cap detail table load; UI shows most recent first (#220).
	const maxFundTx = 5000
	rows, err := s.db.QueryContext(ctx, `
		SELECT seq, trade_time, confirm_date, direction, trade_type, confirm_amount,
		       confirm_share, fee, settlement_days, order_id
		FROM transactions
		WHERE fund_code = ?
		ORDER BY trade_time DESC, seq DESC
		LIMIT ?
	`, code, maxFundTx)
	if err != nil {
		return nil, fmt.Errorf("query fund transactions: %w", err)
	}
	defer rows.Close()

	transactions := []FundTransaction{}
	for rows.Next() {
		var tx FundTransaction
		var tradeTime sql.NullString
		var confirmDate sql.NullString
		var direction sql.NullString
		var tradeType sql.NullString
		var amount sql.NullFloat64
		var shares sql.NullFloat64
		var fee sql.NullFloat64
		var settlement sql.NullInt64
		var orderID sql.NullString
		if err := rows.Scan(
			&tx.Seq,
			&tradeTime,
			&confirmDate,
			&direction,
			&tradeType,
			&amount,
			&shares,
			&fee,
			&settlement,
			&orderID,
		); err != nil {
			return nil, fmt.Errorf("scan fund transaction: %w", err)
		}
		tx.Time = nullableStringValuePtr(tradeTime)
		tx.ConfirmDate = dateOnlyNullablePtr(confirmDate)
		tx.Direction = nullableStringValuePtr(direction)
		tx.Type = nullableStringValuePtr(tradeType)
		tx.Amount = nullableFloat64Ptr(amount)
		tx.Shares = nullableFloat64Ptr(shares)
		tx.Fee = nullableFloat64Ptr(fee)
		tx.SettlementDays = nullableIntPtr(settlement)
		tx.OrderID = nullableStringValuePtr(orderID)
		transactions = append(transactions, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fund transaction rows: %w", err)
	}
	return transactions, nil
}
