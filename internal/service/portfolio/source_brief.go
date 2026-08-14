package portfolio

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SourceBriefOptions struct {
	PortfolioID int
	Limit       int
}

type InvestmentSourceBrief struct {
	GeneratedAt      string                   `json:"generated_at"`
	DecisionBoundary string                   `json:"decision_boundary"`
	Queries          []InvestmentSourceQuery  `json:"queries"`
	SourceTargets    []InvestmentSourceTarget `json:"source_targets"`
	Coverage         SourceBriefCoverage      `json:"coverage"`
	AgentBrief       string                   `json:"agent_brief"`
}

type InvestmentSourceQuery struct {
	ID         string  `json:"id"`
	Scope      string  `json:"scope"`
	EntityCode *string `json:"entity_code"`
	EntityName string  `json:"entity_name"`
	Query      string  `json:"query"`
	Reason     string  `json:"reason"`
	Freshness  string  `json:"freshness"`
}

type InvestmentSourceTarget struct {
	Kind        string  `json:"kind"`
	Name        string  `json:"name"`
	URLTemplate *string `json:"url_template"`
	UseFor      string  `json:"use_for"`
}

type SourceBriefCoverage struct {
	HoldingsScanned   int `json:"holdings_scanned"`
	UnderlyingScanned int `json:"underlying_scanned"`
	MaxQueries        int `json:"max_queries"`
}

type sourceBriefHolding struct {
	Code         string
	Name         string
	CurrentValue float64
	SecurityType string
	Market       string
	FundType     string
}

type sourceBriefUnderlying struct {
	Code     string
	Name     string
	Exposure float64
}

func (s Service) GetInvestmentSourceBrief(ctx context.Context, options SourceBriefOptions) (*InvestmentSourceBrief, error) {
	portfolioID := options.PortfolioID
	portfolioID = clampPortfolioID(portfolioID)
	limit := clampSourceBriefLimit(options.Limit)

	holdings, err := s.sourceBriefHoldings(ctx, portfolioID)
	if err != nil {
		return nil, err
	}
	underlying, err := s.sourceBriefUnderlying(ctx, portfolioID)
	if err != nil {
		return nil, err
	}

	queries := []InvestmentSourceQuery{portfolioMarketSourceQuery()}
	for _, holding := range holdings {
		queries = append(queries, holdingSourceQuery(holding))
	}
	for _, stock := range underlying {
		queries = append(queries, underlyingSourceQuery(stock))
	}
	finalQueries := dedupeSourceQueries(queries, limit)

	return &InvestmentSourceBrief{
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		DecisionBoundary: "source_queries_only",
		Queries:          finalQueries,
		SourceTargets:    sourceTargets(),
		Coverage: SourceBriefCoverage{
			HoldingsScanned:   len(holdings),
			UnderlyingScanned: len(underlying),
			MaxQueries:        limit,
		},
		AgentBrief: fmt.Sprintf(
			"Hermes source brief: %d source queries generated from %d holdings and %d underlying stocks. Search targets include Hermes WebSearch, DSA search providers, disclosure pages, and local MCP crawlers. This is search/crawl context only; investment decisions stay with the agent.",
			len(finalQueries),
			len(holdings),
			len(underlying),
		),
	}, nil
}

func (s Service) sourceBriefHoldings(ctx context.Context, portfolioID int) ([]sourceBriefHolding, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ps.fund_code, COALESCE(fd.fund_name, ps.fund_name, ps.fund_code) as fund_name,
			COALESCE(ps.current_value, 0),
			COALESCE(ps.security_type, fd.security_type, 'fund') as security_type,
			COALESCE(fd.market, '') as market,
			COALESCE(fd.fund_type, '') as fund_type
		FROM portfolio_snapshot ps
		LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
		WHERE ps.held_shares > 0.001 AND COALESCE(ps.portfolio_id, 1) = ?
		ORDER BY COALESCE(ps.current_value, 0) DESC, ps.fund_code
		LIMIT 20
	`, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("query source brief holdings: %w", err)
	}
	defer rows.Close()

	var holdings []sourceBriefHolding
	for rows.Next() {
		var row sourceBriefHolding
		if err := rows.Scan(&row.Code, &row.Name, &row.CurrentValue, &row.SecurityType, &row.Market, &row.FundType); err != nil {
			return nil, fmt.Errorf("scan source brief holding: %w", err)
		}
		row.Code = clampPortfolioText(row.Code, 32)
		row.Name = clampPortfolioText(row.Name, 200)
		row.SecurityType = clampPortfolioText(row.SecurityType, 32)
		row.Market = clampPortfolioText(row.Market, 32)
		row.FundType = clampPortfolioText(row.FundType, 64)
		holdings = append(holdings, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("source brief holding rows: %w", err)
	}
	return holdings, nil
}

func (s Service) sourceBriefUnderlying(ctx context.Context, portfolioID int) ([]sourceBriefUnderlying, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT fh.stock_code, fh.stock_name, SUM(COALESCE(ps.current_value, 0) * fh.weight_pct / 100.0) as exposure
		FROM fund_holdings fh
		JOIN portfolio_snapshot ps ON ps.fund_code = fh.fund_code
		WHERE ps.held_shares > 0.001 AND COALESCE(ps.portfolio_id, 1) = ?
		GROUP BY fh.stock_code, fh.stock_name
		ORDER BY exposure DESC, fh.stock_code
		LIMIT 20
	`, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("query source brief underlying: %w", err)
	}
	defer rows.Close()

	var stocks []sourceBriefUnderlying
	for rows.Next() {
		var row sourceBriefUnderlying
		if err := rows.Scan(&row.Code, &row.Name, &row.Exposure); err != nil {
			return nil, fmt.Errorf("scan source brief underlying: %w", err)
		}
		row.Code = clampPortfolioText(row.Code, 32)
		row.Name = clampPortfolioText(row.Name, 200)
		stocks = append(stocks, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("source brief underlying rows: %w", err)
	}
	return stocks, nil
}

func portfolioMarketSourceQuery() InvestmentSourceQuery {
	return InvestmentSourceQuery{
		ID:         "portfolio-global-market",
		Scope:      "portfolio",
		EntityCode: nil,
		EntityName: "portfolio",
		Query:      "今日 全球市场 纳斯达克 港股 A股 汇率 影响 QDII 基金",
		Reason:     "组合跨 CN/HK/US 市场，需要宏观和市场层消息作为背景。",
		Freshness:  "intraday",
	}
}

func holdingSourceQuery(holding sourceBriefHolding) InvestmentSourceQuery {
	market := holding.Market
	if market == "" {
		if holding.SecurityType == "stock" {
			market = "stock"
		} else {
			market = "fund"
		}
	}
	code := holding.Code
	freshness := "daily"
	if holding.SecurityType == "stock" {
		freshness = "intraday"
	}
	return InvestmentSourceQuery{
		ID:         "holding-" + holding.Code,
		Scope:      "holding",
		EntityCode: &code,
		EntityName: clampPortfolioText(holding.Name, 200),
		Query:      clampPortfolioText(fmt.Sprintf("%s %s %s 最新消息 公告 持仓 估值", holding.Name, holding.Code, market), 500),
		Reason:     clampPortfolioText(fmt.Sprintf("持仓级消息源，用于核对 %s 的公告、净值/股价和主题变化。", holding.Name), 500),
		Freshness:  freshness,
	}
}

func underlyingSourceQuery(stock sourceBriefUnderlying) InvestmentSourceQuery {
	code := stock.Code
	return InvestmentSourceQuery{
		ID:         "underlying-" + stock.Code,
		Scope:      "underlying",
		EntityCode: &code,
		EntityName: clampPortfolioText(stock.Name, 200),
		Query:      clampPortfolioText(fmt.Sprintf("%s %s earnings news guidance regulation", stock.Name, stock.Code), 500),
		Reason:     clampPortfolioText("穿透持仓底层股票消息源，估算组合间接暴露的新闻风险。", 500),
		Freshness:  "intraday",
	}
}

func dedupeSourceQueries(queries []InvestmentSourceQuery, limit int) []InvestmentSourceQuery {
	seen := map[string]struct{}{}
	out := make([]InvestmentSourceQuery, 0, limit)
	for _, query := range queries {
		key := strings.ToLower(query.Query)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, query)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func sourceTargets() []InvestmentSourceTarget {
	return []InvestmentSourceTarget{
		{Kind: "web_search", Name: "Hermes WebSearch", UseFor: "新闻、公告、监管和宏观消息检索"},
		{Kind: "web_search", Name: "DSA search providers", URLTemplate: stringPtr("dsa:search({query})"), UseFor: "复用 daily_stock_analysis 的 SerpAPI/Tavily/Brave/SearXNG 等搜索源做新闻兜底"},
		{Kind: "market_data", Name: "DSA market context", URLTemplate: stringPtr("dsa:market-review({market})"), UseFor: "复用 daily_stock_analysis 的多市场复盘、交易日历和数据质量上下文"},
		{Kind: "market_data", Name: "fund-dashboard MCP", URLTemplate: stringPtr("mcp:get_fund_detail({code})"), UseFor: "本地持仓、价格、成本、交易流水事实"},
		{Kind: "official_disclosure", Name: "Eastmoney / fundf10", URLTemplate: stringPtr("https://fundf10.eastmoney.com/ccmx_{code}.html"), UseFor: "基金季报持仓和披露核对"},
		{Kind: "official_disclosure", Name: "Yahoo Finance", URLTemplate: stringPtr("https://finance.yahoo.com/quote/{code}"), UseFor: "美股行情、财报和公司事件入口"},
		{Kind: "local_mcp", Name: "crawl_fund_holdings", URLTemplate: stringPtr("mcp:crawl_fund_holdings({fund_code})"), UseFor: "补全本地 fund_holdings 穿透数据"},
	}
}

func clampSourceBriefLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit < 1 {
		return 1
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func stringPtr(value string) *string {
	return &value
}
