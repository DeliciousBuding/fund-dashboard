package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type xirrCashflow struct {
	Amount float64
	Years  float64
}

type xirrTransaction struct {
	Amount    float64
	Direction string
	TradeTime time.Time
	Fee       float64
}

func (s Service) GetFundXIRR(ctx context.Context, code string, portfolioID int) (XIRRReport, error) {
	pid := clampPortfolioID(portfolioID)

	identity, err := s.queryFundIdentity(ctx, code)
	if err != nil {
		return XIRRReport{}, err
	}
	report := XIRRReport{
		Code:             code,
		SecurityType:     "fund",
		Market:           "",
		DecisionBoundary: "facts_only",
	}
	if identity != nil {
		report.SecurityType = identity.SecurityType
		report.Market = identity.Market
	}

	// Cashflows remain fund-wide (transactions table is not portfolio-scoped).
	// Terminal market value uses portfolio snapshot so multi-portfolio positions stay consistent.
	transactions, err := s.queryXIRRTransactions(ctx, code)
	if err != nil {
		return XIRRReport{}, err
	}
	if len(transactions) < 2 {
		message := "not enough cashflows (need >=2 buy/sell/dividend records)"
		report.Message = &message
		return report, nil
	}

	currentValue, err := s.queryCurrentMarketValue(ctx, code, pid)
	if err != nil {
		return XIRRReport{}, err
	}
	value := calcXIRR(buildXIRRCashflows(transactions, currentValue))
	if value == nil {
		message := "not enough cashflows (need both inflow and outflow)"
		report.Message = &message
		return report, nil
	}
	percent := round2(*value * 100)
	report.XIRRPct = &percent
	return report, nil
}

func (s Service) GetPortfolioXIRR(ctx context.Context, portfolioID int) (PortfolioXIRRReport, error) {
	portfolioID = clampPortfolioID(portfolioID)
	report := PortfolioXIRRReport{
		PortfolioID:      portfolioID,
		DecisionBoundary: "facts_only",
	}

	transactions, err := s.queryPortfolioXIRRTransactions(ctx, portfolioID)
	if err != nil {
		return PortfolioXIRRReport{}, err
	}
	currentValue, err := s.queryPortfolioMarketValue(ctx, portfolioID)
	if err != nil {
		return PortfolioXIRRReport{}, err
	}
	report.CurrentPortfolioValue = round2(currentValue)
	if len(transactions) < 2 {
		message := "not enough cashflows (need >=2 buy/sell/dividend records)"
		report.Message = &message
		return report, nil
	}

	value := calcXIRR(buildXIRRCashflows(transactions, currentValue))
	if value == nil {
		message := "not enough cashflows (need both inflow and outflow)"
		report.Message = &message
		return report, nil
	}
	percent := round2(*value * 100)
	report.XIRRPct = &percent
	return report, nil
}

func (s Service) queryXIRRTransactions(ctx context.Context, code string) ([]xirrTransaction, error) {
	const maxXIRRTx = 20000
	rows, err := s.db.QueryContext(ctx, `
		SELECT confirm_amount, direction, trade_time, fee
		FROM transactions
		WHERE fund_code = ?
		  AND direction IN ('buy', 'sell', 'dividend')
		ORDER BY trade_time
		LIMIT ?
	`, code, maxXIRRTx)
	if err != nil {
		return nil, fmt.Errorf("query xirr transactions: %w", err)
	}
	defer rows.Close()
	return scanXIRRTransactions(rows)
}

func (s Service) queryPortfolioXIRRTransactions(ctx context.Context, portfolioID int) ([]xirrTransaction, error) {
	const maxPortfolioXIRRTx = 50000
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.confirm_amount, t.direction, t.trade_time, t.fee
		FROM transactions t
		JOIN portfolio_snapshot ps ON ps.fund_code = t.fund_code AND COALESCE(ps.portfolio_id, 1) = ?
		WHERE t.direction IN ('buy', 'sell', 'dividend')
		ORDER BY t.trade_time
		LIMIT ?
	`, portfolioID, maxPortfolioXIRRTx)
	if err != nil {
		return nil, fmt.Errorf("query portfolio xirr transactions: %w", err)
	}
	defer rows.Close()
	return scanXIRRTransactions(rows)
}

func (s Service) queryCurrentMarketValue(ctx context.Context, code string, portfolioID int) (float64, error) {
	portfolioID = clampPortfolioID(portfolioID)

	// Prefer portfolio_snapshot for the active portfolio (held_shares already dust-filtered).
	var snapShares sql.NullFloat64
	var snapNAV sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
		SELECT held_shares, latest_nav
		FROM portfolio_snapshot
		WHERE fund_code = ?
		  AND COALESCE(portfolio_id, 1) = ?
	`, code, portfolioID).Scan(&snapShares, &snapNAV)
	if err == nil && snapShares.Valid && snapShares.Float64 > 0.001 && snapNAV.Valid {
		return snapShares.Float64 * snapNAV.Float64, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("query xirr snapshot value: %w", err)
	}

	// Fallback: fund-wide transaction sum × latest NAV (legacy single-portfolio DBs).
	var shares sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `
		SELECT SUM(signed_share_change)
		FROM transactions
		WHERE fund_code = ?
	`, code).Scan(&shares); err != nil {
		return 0, fmt.Errorf("query xirr shares: %w", err)
	}
	if !shares.Valid || shares.Float64 <= 0.001 {
		return 0, nil
	}

	var latestNAV sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `
		SELECT unit_nav
		FROM nav_history
		WHERE fund_code = ?
		ORDER BY date DESC
		LIMIT 1
	`, code).Scan(&latestNAV); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("query xirr latest nav: %w", err)
	}
	if !latestNAV.Valid {
		return 0, nil
	}
	return shares.Float64 * latestNAV.Float64, nil
}

func (s Service) queryPortfolioMarketValue(ctx context.Context, portfolioID int) (float64, error) {
	var total sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `
		SELECT SUM(held_shares * latest_nav)
		FROM portfolio_snapshot
		WHERE held_shares > 0.001
		  AND COALESCE(portfolio_id, 1) = ?
	`, portfolioID).Scan(&total); err != nil {
		return 0, fmt.Errorf("query portfolio xirr market value: %w", err)
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Float64, nil
}
