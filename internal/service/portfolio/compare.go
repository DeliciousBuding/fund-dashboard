package portfolio

import (
	"context"
	"fmt"
	"math"
)

type FundCompareResult struct {
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	Market           string   `json:"market"`
	XIRR             *float64 `json:"xirr"`
	Volatility       *float64 `json:"volatility"`
	Sharpe           *float64 `json:"sharpe"`
	MaxDrawdown      *float64 `json:"max_drawdown"`
	Calmar           *float64 `json:"calmar"`
	DecisionBoundary string   `json:"decision_boundary"`
}

type compareNavPoint struct {
	UnitNAV float64
}

func (s Service) CompareFunds(ctx context.Context, codes []string, portfolioID int) ([]FundCompareResult, error) {
	pid := clampPortfolioID(portfolioID)

	const maxCompareCodes = 8
	if len(codes) > maxCompareCodes {
		codes = codes[:maxCompareCodes]
	}
	results := make([]FundCompareResult, 0, len(codes))
	for _, code := range codes {
		result := FundCompareResult{
			Code:             code,
			Name:             code,
			DecisionBoundary: "facts_only",
		}
		identity, err := s.queryFundIdentity(ctx, code)
		if err != nil {
			return nil, err
		}
		if identity == nil {
			results = append(results, result)
			continue
		}
		if identity.Name != nil {
			result.Name = *identity.Name
		}
		result.Market = identity.Market

		xirr, err := s.GetFundXIRR(ctx, code, pid)
		if err != nil {
			return nil, err
		}
		result.XIRR = xirr.XIRRPct

		navs, err := s.queryCompareNavHistory(ctx, code)
		if err != nil {
			return nil, err
		}
		result.Volatility = calcCompareVolatilityPct(navs)
		result.MaxDrawdown = calcCompareMaxDrawdownPct(navs)
		if result.XIRR != nil && result.Volatility != nil && *result.Volatility > 0.001 {
			value := round4(*result.XIRR / *result.Volatility)
			result.Sharpe = &value
		}
		if result.XIRR != nil && result.MaxDrawdown != nil && *result.MaxDrawdown > 0.01 {
			value := round4(*result.XIRR / *result.MaxDrawdown)
			result.Calmar = &value
		}
		results = append(results, result)
	}
	return results, nil
}

func (s Service) queryCompareNavHistory(ctx context.Context, code string) ([]compareNavPoint, error) {
	// Cap points for volatility/drawdown calc memory (#219).
	const maxCompareNavPoints = 5000
	rows, err := s.db.QueryContext(ctx, `
		SELECT unit_nav FROM (
			SELECT unit_nav, date
			FROM nav_history
			WHERE fund_code = ?
				AND unit_nav > 0
			ORDER BY date DESC
			LIMIT ?
		) t
		ORDER BY date ASC
	`, code, maxCompareNavPoints)
	if err != nil {
		return nil, fmt.Errorf("query compare nav history: %w", err)
	}
	defer rows.Close()

	var points []compareNavPoint
	for rows.Next() {
		var nav float64
		if err := rows.Scan(&nav); err != nil {
			return nil, fmt.Errorf("scan compare nav history: %w", err)
		}
		points = append(points, compareNavPoint{UnitNAV: nav})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("compare nav history rows: %w", err)
	}
	return points, nil
}

func calcCompareVolatilityPct(navs []compareNavPoint) *float64 {
	if len(navs) < 10 {
		return nil
	}
	returns := make([]float64, 0, len(navs)-1)
	for i := 1; i < len(navs); i++ {
		if navs[i-1].UnitNAV <= 0 || navs[i].UnitNAV <= 0 {
			continue
		}
		returns = append(returns, math.Log(navs[i].UnitNAV/navs[i-1].UnitNAV))
	}
	if len(returns) < 2 {
		return nil
	}
	mean := 0.0
	for _, value := range returns {
		mean += value
	}
	mean /= float64(len(returns))
	variance := 0.0
	for _, value := range returns {
		diff := value - mean
		variance += diff * diff
	}
	variance /= float64(len(returns) - 1)
	value := round2(math.Sqrt(variance) * math.Sqrt(252) * 100)
	return &value
}

func calcCompareMaxDrawdownPct(navs []compareNavPoint) *float64 {
	if len(navs) == 0 {
		return nil
	}
	peak := navs[0].UnitNAV
	maxDrawdown := 0.0
	for _, point := range navs {
		if point.UnitNAV > peak {
			peak = point.UnitNAV
		}
		if peak > 0 {
			drawdown := (peak - point.UnitNAV) / peak
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
			}
		}
	}
	value := round2(maxDrawdown * 100)
	return &value
}
