package portfolio

import (
	"context"
	"fmt"
	"math"
)

func (s Service) harnessDataQuality(ctx context.Context, portfolioID int, signals []HarnessHoldingSignal) (HarnessDataQuality, error) {
	quality := HarnessDataQuality{}
	for _, signal := range signals {
		if !signal.DataPoints.HasPrice {
			quality.StalePriceCount++
		}
		if !signal.DataPoints.HasCostBasis {
			quality.MissingCostBasisCount++
		}
		if !signal.DataPoints.HasChangePct {
			quality.MissingChangePctCount++
		}
	}
	coverage, err := s.holdingsCoveragePct(ctx, portfolioID)
	if err != nil {
		return HarnessDataQuality{}, err
	}
	quality.HoldingsCoveragePct = coverage
	return quality, nil
}

func (s Service) holdingsCoveragePct(ctx context.Context, portfolioID int) (float64, error) {
	var applicable int
	var withHoldings int
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(DISTINCT CASE WHEN
				fd.security_type = 'fund'
				AND COALESCE(fd.fund_type, '') NOT LIKE '%债券%'
				AND (
					COALESCE(fd.fund_type, '') LIKE '%QDII%'
					OR COALESCE(fd.fund_type, '') LIKE '%股票%'
					OR COALESCE(fd.fund_type, '') LIKE '%指数%'
					OR COALESCE(fd.fund_type, '') LIKE '%ETF%'
					OR COALESCE(fd.fund_type, '') LIKE '%混合%'
				)
				THEN ps.fund_code END
			) as applicable,
			COUNT(DISTINCT CASE WHEN
				fd.security_type = 'fund'
				AND COALESCE(fd.fund_type, '') NOT LIKE '%债券%'
				AND (
					COALESCE(fd.fund_type, '') LIKE '%QDII%'
					OR COALESCE(fd.fund_type, '') LIKE '%股票%'
					OR COALESCE(fd.fund_type, '') LIKE '%指数%'
					OR COALESCE(fd.fund_type, '') LIKE '%ETF%'
					OR COALESCE(fd.fund_type, '') LIKE '%混合%'
				)
				AND fh.fund_code IS NOT NULL
				THEN ps.fund_code END
			) as with_holdings
		FROM portfolio_snapshot ps
		LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
		LEFT JOIN fund_holdings fh ON fh.fund_code = ps.fund_code
		WHERE ps.held_shares > 0.001 AND COALESCE(ps.portfolio_id, 1) = ?
	`, portfolioID).Scan(&applicable, &withHoldings); err != nil {
		return 0, fmt.Errorf("query holdings coverage: %w", err)
	}
	if applicable == 0 {
		return 0, nil
	}
	return round1(float64(withHoldings) / float64(applicable) * 100), nil
}

func buildRecommendedAgentActions(stalePriceCount int, missingChangePctCount int, holdingsCoveragePct float64, riskFlags []string) []RecommendedAgentAction {
	var actions []RecommendedAgentAction
	if stalePriceCount > 0 || missingChangePctCount > 0 {
		actions = append(actions, RecommendedAgentAction{
			Priority: "high",
			Tool:     "crawl_nav",
			Reason:   clampPortfolioText(fmt.Sprintf("价格/NAV 或涨跌幅字段存在缺口：stale_price=%d, missing_change_pct=%d", stalePriceCount, missingChangePctCount), 500),
			// Prefer stale-only refresh so agents do not re-fetch all held NAV (#254 / #252).
			Input: map[string]any{"stale_only": true},
		})
	}
	if holdingsCoveragePct < 100 {
		actions = append(actions, RecommendedAgentAction{
			Priority: "medium",
			Tool:     "crawl_fund_holdings",
			Reason:   clampPortfolioText(fmt.Sprintf("基金穿透覆盖率 %s%%；补全后 agent 可做更可靠的底层暴露分析", formatAllocationPct(holdingsCoveragePct)), 500),
		})
	}
	if len(riskFlags) > 0 {
		actions = append(actions, RecommendedAgentAction{
			Priority: "medium",
			Tool:     "get_portfolio_penetration",
			Reason:   clampPortfolioText(fmt.Sprintf("配置层存在风险提示：%s；建议读取穿透暴露做事实核对", joinSemicolon(riskFlags)), 500),
		})
	}
	actions = append(actions, RecommendedAgentAction{
		Priority: "low",
		Tool:     "get_investment_source_brief",
		Reason:   clampPortfolioText("生成 Hermes/DSA/WebSearch 消息源查询，用于补充外部上下文", 500),
		Input:    map[string]any{"limit": 8},
	})
	return actions
}

func joinSemicolon(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, part := range parts[1:] {
		result += "；" + part
	}
	return result
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}
