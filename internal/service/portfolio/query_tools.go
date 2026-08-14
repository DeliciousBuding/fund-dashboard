package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (s Service) SearchFunds(ctx context.Context, rawQuery string) ([]SearchResult, error) {
	rawQuery = strings.TrimSpace(rawQuery)
	if rawQuery == "" {
		return []SearchResult{}, nil
	}
	// Bound LIKE pattern cost (#217).
	if len(rawQuery) > 100 {
		rawQuery = rawQuery[:100]
	}
	queryValue := "%" + rawQuery + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			fd.fund_code,
			COALESCE(fd.fund_name, ''),
			COALESCE(fd.fund_type, ''),
			COALESCE(fd.security_type, 'fund'),
			COALESCE(fd.market, ''),
			ps.held_shares,
			ps.current_value,
			ps.unrealized_pnl,
			ps.pnl_pct
		FROM fund_details fd
		LEFT JOIN portfolio_snapshot ps ON fd.fund_code = ps.fund_code
		WHERE fd.fund_name LIKE ?
		   OR fd.fund_code LIKE ?
		   OR fd.fund_type LIKE ?
		   OR (fd.fund_code LIKE (? || '%') AND LENGTH(? || '') > 0)
		ORDER BY ps.held_shares DESC NULLS LAST, fd.fund_code
		LIMIT 20
	`, queryValue, queryValue, queryValue, rawQuery, rawQuery)
	if err != nil {
		return nil, fmt.Errorf("search funds: %w", err)
	}
	defer rows.Close()

	results := []SearchResult{}
	for rows.Next() {
		var item SearchResult
		var heldShares sql.NullFloat64
		var currentValue sql.NullFloat64
		var unrealizedPNL sql.NullFloat64
		var pnlPct sql.NullFloat64
		if err := rows.Scan(
			&item.Code,
			&item.Name,
			&item.Type,
			&item.SecurityType,
			&item.Market,
			&heldShares,
			&currentValue,
			&unrealizedPNL,
			&pnlPct,
		); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		if heldShares.Valid {
			item.HeldShares = heldShares.Float64
		}
		item.Code = clampPortfolioText(item.Code, 32)
		item.Name = clampPortfolioText(item.Name, 200)
		item.Type = clampPortfolioText(item.Type, 64)
		item.SecurityType = clampPortfolioText(item.SecurityType, 32)
		item.Market = clampPortfolioText(item.Market, 32)
		item.CurrentValue = nullableFloat64Ptr(currentValue)
		item.UnrealizedPNL = nullableFloat64Ptr(unrealizedPNL)
		item.PNLPct = nullableFloat64Ptr(pnlPct)
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search result rows: %w", err)
	}
	return results, nil
}

func (s Service) GetFundDrawdown(ctx context.Context, code string) (*DrawdownReport, error) {
	identity, err := s.queryFundIdentity(ctx, code)
	if err != nil {
		return nil, err
	}

	// Cap points for memory/CPU on long histories (#218): last N points in chronological order.
	const maxDrawdownPoints = 5000
	rows, err := s.db.QueryContext(ctx, `
		SELECT date, unit_nav FROM (
			SELECT date, unit_nav
			FROM nav_history
			WHERE fund_code = ?
			ORDER BY date DESC
			LIMIT ?
		) t
		ORDER BY date ASC
	`, code, maxDrawdownPoints)
	if err != nil {
		return nil, fmt.Errorf("query drawdown nav: %w", err)
	}
	defer rows.Close()

	var peak float64
	var peakDate string
	var currentPeakDate string
	var maxDrawdown float64
	var troughDate string
	hasRows := false
	for rows.Next() {
		var date string
		var nav float64
		if err := rows.Scan(&date, &nav); err != nil {
			return nil, fmt.Errorf("scan drawdown nav: %w", err)
		}
		if !hasRows {
			peak = nav
			peakDate = date
			currentPeakDate = date
			troughDate = date
			hasRows = true
			continue
		}
		if nav > peak {
			peak = nav
			currentPeakDate = date
		}
		if peak > 0 {
			drawdown := (peak - nav) / peak
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
				peakDate = currentPeakDate
				troughDate = date
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("drawdown nav rows: %w", err)
	}
	if !hasRows {
		return nil, nil
	}

	report := &DrawdownReport{
		Code:             code,
		SecurityType:     "fund",
		Market:           "",
		MaxDrawdownPct:   round2(maxDrawdown * 100),
		PeakDate:         dateOnly(peakDate),
		TroughDate:       dateOnly(troughDate),
		DecisionBoundary: "facts_only",
	}
	if identity != nil {
		report.SecurityType = identity.SecurityType
		report.Market = identity.Market
	}
	return report, nil
}
