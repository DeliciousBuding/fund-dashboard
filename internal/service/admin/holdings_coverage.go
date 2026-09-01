package admin

import (
	"context"
	"fmt"
)

const holdingsCoverageApplicableExpr = `
		fd.security_type = 'fund'
		AND COALESCE(fd.fund_type, '') NOT LIKE '%债券%'
		AND (
			COALESCE(fd.fund_type, '') LIKE '%QDII%'
			OR COALESCE(fd.fund_type, '') LIKE '%股票%'
			OR COALESCE(fd.fund_type, '') LIKE '%指数%'
			OR COALESCE(fd.fund_type, '') LIKE '%ETF%'
			OR COALESCE(fd.fund_type, '') LIKE '%混合%'
		)
	`

// HoldingsCoverageReport is the read-only holdings penetration coverage view.
type HoldingsCoverageReport struct {
	TotalFunds            int                      `json:"total_funds"`
	ApplicableFunds       int                      `json:"applicable_funds"`
	FundsWithHoldings     int                      `json:"funds_with_holdings"`
	CoveragePct           float64                  `json:"coverage_pct"`
	ApplicableCoveragePct float64                  `json:"applicable_coverage_pct"`
	TotalCoveragePct      float64                  `json:"total_coverage_pct"`
	SourceMissingFunds    []HoldingsCoverageFund   `json:"source_missing_funds"`
	NotApplicableFunds    []HoldingsCoverageFund   `json:"not_applicable_funds"`
	ByFundType            []HoldingsCoverageByType `json:"by_fund_type"`
	DecisionBoundary      string                   `json:"decision_boundary"`
}

type HoldingsCoverageFund struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	FundType string `json:"fund_type"`
}

type HoldingsCoverageByType struct {
	FundType     string  `json:"fund_type"`
	Total        int     `json:"total"`
	Applicable   int     `json:"applicable"`
	WithHoldings int     `json:"with_holdings"`
	CoveragePct  float64 `json:"coverage_pct"`
}

func (s Service) GetHoldingsCoverage(ctx context.Context, portfolioID int) (HoldingsCoverageReport, error) {
	portfolioID = clampPortfolioID(portfolioID)

	var total int
	var applicable int
	var withHoldings int
	summaryQuery := fmt.Sprintf(`
		SELECT
			COUNT(DISTINCT ps.fund_code),
			COUNT(DISTINCT CASE WHEN %s THEN ps.fund_code END),
			COUNT(DISTINCT CASE WHEN %s AND fh.fund_code IS NOT NULL THEN ps.fund_code END)
		FROM portfolio_snapshot ps
		LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
		LEFT JOIN fund_holdings fh ON fh.fund_code = ps.fund_code
		WHERE ps.held_shares > 0.001
		  AND COALESCE(ps.portfolio_id, 1) = ?
	`, holdingsCoverageApplicableExpr, holdingsCoverageApplicableExpr)
	if err := s.db.QueryRowContext(ctx, summaryQuery, portfolioID).Scan(&total, &applicable, &withHoldings); err != nil {
		return HoldingsCoverageReport{}, fmt.Errorf("holdings coverage summary: %w", err)
	}

	byType, err := s.queryHoldingsCoverageByType(ctx, portfolioID)
	if err != nil {
		return HoldingsCoverageReport{}, err
	}
	sourceMissing, err := s.queryHoldingsCoverageFunds(ctx, portfolioID, true)
	if err != nil {
		return HoldingsCoverageReport{}, err
	}
	notApplicable, err := s.queryHoldingsCoverageFunds(ctx, portfolioID, false)
	if err != nil {
		return HoldingsCoverageReport{}, err
	}

	applicableCoveragePct := coveragePct(withHoldings, applicable)
	return HoldingsCoverageReport{
		TotalFunds:            total,
		ApplicableFunds:       applicable,
		FundsWithHoldings:     withHoldings,
		CoveragePct:           applicableCoveragePct,
		ApplicableCoveragePct: applicableCoveragePct,
		TotalCoveragePct:      coveragePct(withHoldings, total),
		SourceMissingFunds:    sourceMissing,
		NotApplicableFunds:    notApplicable,
		ByFundType:            byType,
		DecisionBoundary:      "read_only",
	}, nil
}

func (s Service) queryHoldingsCoverageByType(ctx context.Context, portfolioID int) ([]HoldingsCoverageByType, error) {
	query := fmt.Sprintf(`
		SELECT
			COALESCE(fd.fund_type, '未分类'),
			COUNT(DISTINCT ps.fund_code),
			COUNT(DISTINCT CASE WHEN %s THEN ps.fund_code END),
			COUNT(DISTINCT CASE WHEN %s AND fh.fund_code IS NOT NULL THEN ps.fund_code END)
		FROM portfolio_snapshot ps
		LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
		LEFT JOIN fund_holdings fh ON fh.fund_code = ps.fund_code
		WHERE ps.held_shares > 0.001
		  AND COALESCE(ps.portfolio_id, 1) = ?
		GROUP BY fd.fund_type
		ORDER BY COUNT(DISTINCT ps.fund_code) DESC, COALESCE(fd.fund_type, '未分类')
	`, holdingsCoverageApplicableExpr, holdingsCoverageApplicableExpr)
	rows, err := s.db.QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("holdings coverage by type: %w", err)
	}
	defer rows.Close()

	items := []HoldingsCoverageByType{}
	for rows.Next() {
		var item HoldingsCoverageByType
		if err := rows.Scan(&item.FundType, &item.Total, &item.Applicable, &item.WithHoldings); err != nil {
			return nil, fmt.Errorf("scan holdings coverage by type: %w", err)
		}
		item.FundType = clampAdminText(item.FundType, 64)
		item.CoveragePct = coveragePct(item.WithHoldings, item.Applicable)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("holdings coverage by type rows: %w", err)
	}
	return items, nil
}

func (s Service) queryHoldingsCoverageFunds(ctx context.Context, portfolioID int, sourceMissing bool) ([]HoldingsCoverageFund, error) {
	condition := fmt.Sprintf("NOT (%s)", holdingsCoverageApplicableExpr)
	groupHaving := "GROUP BY ps.fund_code, fd.fund_name, ps.fund_name, fd.fund_type"
	if sourceMissing {
		condition = holdingsCoverageApplicableExpr
		groupHaving = "GROUP BY ps.fund_code, fd.fund_name, ps.fund_name, fd.fund_type HAVING COUNT(fh.stock_code) = 0"
	}
	query := fmt.Sprintf(`
		SELECT
			ps.fund_code,
			COALESCE(fd.fund_name, ps.fund_name, ps.fund_code),
			COALESCE(fd.fund_type, '未分类')
		FROM portfolio_snapshot ps
		LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
		LEFT JOIN fund_holdings fh ON fh.fund_code = ps.fund_code
		WHERE ps.held_shares > 0.001
		  AND COALESCE(ps.portfolio_id, 1) = ?
		  AND %s
		%s
		ORDER BY ps.fund_code
		LIMIT ?
	`, condition, groupHaving)
	rows, err := s.db.QueryContext(ctx, query, portfolioID, adminListMaxRows)
	if err != nil {
		return nil, fmt.Errorf("holdings coverage funds: %w", err)
	}
	defer rows.Close()

	items := []HoldingsCoverageFund{}
	for rows.Next() {
		var item HoldingsCoverageFund
		if err := rows.Scan(&item.Code, &item.Name, &item.FundType); err != nil {
			return nil, fmt.Errorf("scan holdings coverage fund: %w", err)
		}
		item.Code = clampAdminText(item.Code, 32)
		item.Name = clampAdminText(item.Name, 200)
		item.FundType = clampAdminText(item.FundType, 64)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("holdings coverage fund rows: %w", err)
	}
	return items, nil
}

func coveragePct(numerator int, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(int(float64(numerator)/float64(denominator)*1000+0.5)) / 10
}
