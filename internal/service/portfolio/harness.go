package portfolio

import (
	"context"
	"fmt"
	"math"
	"time"
)

// HarnessAudience selects which tool discovery surface a harness snapshot advertises.
// Public HTTP is unauthenticated → always Public. MCP operator may request Operator.
type HarnessAudience string

const (
	HarnessAudiencePublic   HarnessAudience = "public"
	HarnessAudienceOperator HarnessAudience = "operator"
)

// GetHarnessSnapshot returns the public (least-privilege) harness discovery surface.
// Prefer GetHarnessSnapshotFor when the caller role is known (MCP operator).
func (s Service) GetHarnessSnapshot(ctx context.Context, portfolioID int) (*HarnessSnapshot, error) {
	return s.GetHarnessSnapshotFor(ctx, portfolioID, HarnessAudiencePublic)
}

func (s Service) GetHarnessSnapshotFor(ctx context.Context, portfolioID int, audience HarnessAudience) (*HarnessSnapshot, error) {
	portfolioID = clampPortfolioID(portfolioID)
	if audience == "" {
		audience = HarnessAudiencePublic
	}

	allocation, err := s.GetAllocation(ctx, portfolioID)
	if err != nil {
		return nil, err
	}
	rows, err := s.harnessHoldingRows(ctx, portfolioID)
	if err != nil {
		return nil, err
	}

	holdingSignals := make([]HarnessHoldingSignal, 0, len(rows))
	for _, row := range rows {
		signal := makeHoldingSignal(row, allocation.TotalValue)
		holdingSignals = append(holdingSignals, signal)
	}

	dataQuality, err := s.harnessDataQuality(ctx, portfolioID, holdingSignals)
	if err != nil {
		return nil, err
	}

	tools, permissions, capabilities := harnessDiscovery(audience)
	recommended := filterRecommendedAgentActions(
		buildRecommendedAgentActions(
			dataQuality.StalePriceCount,
			dataQuality.MissingChangePctCount,
			dataQuality.HoldingsCoveragePct,
			allocation.RiskFlags,
		),
		tools,
	)

	return &HarnessSnapshot{
		GeneratedAt:             time.Now().UTC().Format(time.RFC3339Nano),
		DecisionBoundary:        "facts_only",
		TotalValue:              allocation.TotalValue,
		HoldingsCount:           len(holdingSignals),
		Allocation:              allocation,
		HoldingSignals:          holdingSignals,
		DataQuality:             dataQuality,
		AvailableAgentTools:     tools,
		AgentPermissions:        permissions,
		AgentCapabilities:       capabilities,
		RecommendedAgentActions: recommended,
		AgentBrief:              buildHarnessAgentBrief(len(holdingSignals), allocation, dataQuality, audience),
	}, nil
}

func (s Service) harnessHoldingRows(ctx context.Context, portfolioID int) ([]harnessHoldingRow, error) {
	// Join latest nav change once (window) instead of correlated N+1 subquery per holding.
	rows, err := s.db.QueryContext(ctx, `
		SELECT ps.fund_code, COALESCE(fd.fund_name, ps.fund_name, ps.fund_code) as fund_name,
			ps.held_shares, ps.total_cost, ps.latest_nav, ps.current_value,
			COALESCE(ps.security_type, fd.security_type, 'fund') as security_type,
			COALESCE(fd.market, '') as market,
			ln.daily_change_pct,
			ps.pnl_pct
		FROM portfolio_snapshot ps
		LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
		LEFT JOIN (
			SELECT fund_code, daily_change_pct
			FROM (
				SELECT fund_code, daily_change_pct,
					ROW_NUMBER() OVER (PARTITION BY fund_code ORDER BY date DESC) AS rn
				FROM nav_history
			) ranked
			WHERE rn = 1
		) ln ON ln.fund_code = ps.fund_code
		WHERE ps.held_shares > 0.001 AND COALESCE(ps.portfolio_id, 1) = ?
		ORDER BY COALESCE(ps.current_value, 0) DESC, ps.fund_code
		LIMIT 5000
	`, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("query harness holdings: %w", err)
	}
	defer rows.Close()

	var out []harnessHoldingRow
	for rows.Next() {
		var row harnessHoldingRow
		if err := rows.Scan(
			&row.Code,
			&row.Name,
			&row.HeldShares,
			&row.TotalCost,
			&row.LatestNAV,
			&row.CurrentValue,
			&row.SecurityType,
			&row.Market,
			&row.DailyChangePct,
			&row.PNLPct,
		); err != nil {
			return nil, fmt.Errorf("scan harness holding: %w", err)
		}
		row.Code = clampPortfolioText(row.Code, 32)
		row.Name = clampPortfolioText(row.Name, 200)
		row.SecurityType = clampPortfolioText(row.SecurityType, 32)
		row.Market = clampPortfolioText(row.Market, 32)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("harness holding rows: %w", err)
	}
	return out, nil
}

func makeHoldingSignal(row harnessHoldingRow, totalValue float64) HarnessHoldingSignal {
	latestNAV := 0.0
	if row.LatestNAV.Valid {
		latestNAV = row.LatestNAV.Float64
	}
	currentValue := 0.0
	if row.CurrentValue.Valid {
		currentValue = row.CurrentValue.Float64
	}
	var costPerShare *float64
	if row.TotalCost != 0 && row.HeldShares > 0 {
		value := round4(math.Abs(row.TotalCost) / row.HeldShares)
		costPerShare = &value
	}
	var deviationPct *float64
	if costPerShare != nil && latestNAV > 0 {
		value := round2((latestNAV - *costPerShare) / *costPerShare * 100)
		deviationPct = &value
	}
	var changePct *float64
	if row.DailyChangePct.Valid {
		value := round2(row.DailyChangePct.Float64)
		changePct = &value
	}
	weightPct := 0.0
	if totalValue > 0 {
		weightPct = round2(currentValue / totalValue * 100)
	}

	var pnlPct *float64
	if row.PNLPct.Valid {
		v := round2(row.PNLPct.Float64)
		pnlPct = &v
	}

	return HarnessHoldingSignal{
		Code:         row.Code,
		Name:         row.Name,
		SecurityType: fallbackString(row.SecurityType, "fund"),
		Market:       row.Market,
		HeldShares:   round2(row.HeldShares),
		CurrentValue: round2(currentValue),
		WeightPct:    weightPct,
		LatestNAV:    round4(latestNAV),
		CostPerShare: costPerShare,
		ChangePct:    changePct,
		DeviationPct: deviationPct,
		PNLPct:       pnlPct,
		SignalTags:   signalTags(changePct, deviationPct),
		DataPoints: SignalDataPoint{
			HasPrice:     latestNAV > 0,
			HasCostBasis: costPerShare != nil,
			HasChangePct: changePct != nil,
		},
	}
}

func signalTags(changePct *float64, deviationPct *float64) []string {
	var tags []string
	if changePct != nil {
		switch {
		case *changePct <= -5:
			tags = append(tags, "price_drop_gt_5pct")
		case *changePct >= 5:
			tags = append(tags, "price_rally_gt_5pct")
		default:
			tags = append(tags, "price_range_bound")
		}
	}
	if deviationPct != nil {
		switch {
		case *deviationPct <= -10:
			tags = append(tags, "below_cost_gt_10pct")
		case *deviationPct >= 10:
			tags = append(tags, "above_cost_gt_10pct")
		default:
			tags = append(tags, "near_cost_basis")
		}
	}
	return tags
}

func buildHarnessAgentBrief(holdingsCount int, allocation *Allocation, quality HarnessDataQuality, audience HarnessAudience) string {
	boundary := "Agent can read facts and run maintenance refreshes; transaction writes require confirmation; broker execution and backup producer are disabled."
	if audience != HarnessAudienceOperator {
		boundary = "Public discovery is read-only: write/maintenance/confirmation-gated tools are not advertised; broker execution and backup producer are disabled."
	}
	return fmt.Sprintf(
		"Investment Harness facts only: %d held assets, total value %.2f. Allocation: %s Data gaps: price %d, cost basis %d, change pct %d. %s",
		holdingsCount,
		allocation.TotalValue,
		allocation.AgentBrief,
		quality.StalePriceCount,
		quality.MissingCostBasisCount,
		quality.MissingChangePctCount,
		boundary,
	)
}

func fallbackString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
