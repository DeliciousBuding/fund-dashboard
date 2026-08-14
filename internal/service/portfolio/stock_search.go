package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type StockSearchOptions struct {
	Query  string
	Market string
	Limit  int
}

type StockSearchReport struct {
	Query            string              `json:"query"`
	MarketFilter     string              `json:"market_filter"`
	Count            int                 `json:"count"`
	Results          []StockSearchResult `json:"results"`
	DecisionBoundary string              `json:"decision_boundary"`
	SideEffects      string              `json:"side_effects"`
	ExternalFetch    string              `json:"external_fetch"`
}

type StockSearchResult struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	Market       string   `json:"market"`
	Sector       string   `json:"sector,omitempty"`
	Industry     string   `json:"industry,omitempty"`
	MarketCap    *float64 `json:"market_cap,omitempty"`
	PE           *float64 `json:"pe,omitempty"`
	Description  string   `json:"description,omitempty"`
	SecurityType string   `json:"security_type"`
	Source       string   `json:"source"`
}

func (s Service) SearchStocks(ctx context.Context, opts StockSearchOptions) (StockSearchReport, error) {
	query := strings.TrimSpace(opts.Query)
	if len(query) > 100 {
		query = query[:100]
	}
	market := normalizeStockSearchMarket(opts.Market)
	limit := opts.Limit
	if limit <= 0 {
		limit = 15
	}
	if limit > 50 {
		limit = 50
	}

	merged := map[string]StockSearchResult{}
	if hasProfile, err := s.tableExists(ctx, "stock_profile"); err != nil {
		return StockSearchReport{}, err
	} else if hasProfile {
		profiles, err := s.searchStockProfiles(ctx, query, market, limit*2)
		if err != nil {
			return StockSearchReport{}, err
		}
		for _, item := range profiles {
			merged[item.Code+"_"+item.Market] = item
		}
	}

	securities, err := s.searchStockSecurities(ctx, query, market, limit*2)
	if err != nil {
		return StockSearchReport{}, err
	}
	for _, item := range securities {
		key := item.Code + "_" + item.Market
		if _, ok := merged[key]; !ok {
			merged[key] = item
		}
	}

	results := make([]StockSearchResult, 0, len(merged))
	for _, item := range merged {
		results = append(results, item)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Code == results[j].Code {
			return results[i].Market < results[j].Market
		}
		return results[i].Code < results[j].Code
	})
	if len(results) > limit {
		results = results[:limit]
	}

	return StockSearchReport{
		Query:            query,
		MarketFilter:     market,
		Count:            len(results),
		Results:          results,
		DecisionBoundary: "facts_only",
		SideEffects:      "none",
		ExternalFetch:    "not_performed",
	}, nil
}

func (s Service) searchStockProfiles(ctx context.Context, rawQuery string, market string, limit int) ([]StockSearchResult, error) {
	like := "%" + rawQuery + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			code,
			COALESCE(name, ''),
			COALESCE(market, ''),
			COALESCE(sector, ''),
			COALESCE(industry, ''),
			market_cap,
			pe,
			COALESCE(description, '')
		FROM stock_profile
		WHERE (name LIKE ? OR code LIKE ? OR code LIKE (? || '%'))
			AND (? = 'all' OR market = ?)
		ORDER BY code, market
		LIMIT ?
	`, like, like, rawQuery, market, market, limit)
	if err != nil {
		return nil, fmt.Errorf("search stock profiles: %w", err)
	}
	defer rows.Close()

	results := []StockSearchResult{}
	for rows.Next() {
		var item StockSearchResult
		var marketCap sql.NullFloat64
		var pe sql.NullFloat64
		if err := rows.Scan(&item.Code, &item.Name, &item.Market, &item.Sector, &item.Industry, &marketCap, &pe, &item.Description); err != nil {
			return nil, fmt.Errorf("scan stock profile: %w", err)
		}
		item.Code = clampPortfolioText(item.Code, 32)
		item.Name = clampPortfolioText(item.Name, 200)
		item.Market = clampPortfolioText(item.Market, 32)
		item.Sector = clampPortfolioText(item.Sector, 64)
		item.Industry = clampPortfolioText(item.Industry, 64)
		item.Description = clampPortfolioText(item.Description, 500)
		item.MarketCap = nullableFloat64Ptr(marketCap)
		item.PE = nullableFloat64Ptr(pe)
		item.SecurityType = "stock"
		item.Source = "local_profile"
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stock profile rows: %w", err)
	}
	return results, nil
}

func (s Service) searchStockSecurities(ctx context.Context, rawQuery string, market string, limit int) ([]StockSearchResult, error) {
	like := "%" + rawQuery + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			fund_code,
			COALESCE(fund_name, ''),
			COALESCE(market, '')
		FROM fund_details
		WHERE COALESCE(security_type, 'fund') = 'stock'
			AND (fund_name LIKE ? OR fund_code LIKE ? OR fund_code LIKE (? || '%'))
			AND (? = 'all' OR market = ?)
		ORDER BY fund_code, market
		LIMIT ?
	`, like, like, rawQuery, market, market, limit)
	if err != nil {
		return nil, fmt.Errorf("search stock securities: %w", err)
	}
	defer rows.Close()

	results := []StockSearchResult{}
	for rows.Next() {
		var item StockSearchResult
		if err := rows.Scan(&item.Code, &item.Name, &item.Market); err != nil {
			return nil, fmt.Errorf("scan stock security: %w", err)
		}
		item.Code = clampPortfolioText(item.Code, 32)
		item.Name = clampPortfolioText(item.Name, 200)
		item.Market = clampPortfolioText(item.Market, 32)
		item.SecurityType = "stock"
		item.Source = "fund_details"
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stock security rows: %w", err)
	}
	return results, nil
}

func normalizeStockSearchMarket(market string) string {
	switch strings.ToUpper(strings.TrimSpace(market)) {
	case "US", "SH", "SZ", "HK":
		return strings.ToUpper(strings.TrimSpace(market))
	default:
		return "all"
	}
}
