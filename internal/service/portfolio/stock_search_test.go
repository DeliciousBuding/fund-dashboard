package portfolio

import (
	"context"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

func TestServiceSearchStocksMergesLocalProfileAndSecurityFacts(t *testing.T) {
	db := testutil.OpenTempDBWithProductionSchema(t)

	for _, stmt := range []string{
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES
			('AAPL', 'Apple Inc.', '科技股', 'stock', 'US'),
			('019173', '纳斯达克100指数(QDII)C', 'QDII-股票', 'fund', 'CN')`,
		`INSERT INTO stock_profile (code, name, market, sector, industry, market_cap, pe, description) VALUES
			('MSFT', 'Microsoft Corporation', 'US', 'Technology', 'Software', 3200000000000, 35.5, 'Productivity and cloud software'),
			('00700', 'Tencent Holdings', 'HK', 'Communication Services', 'Internet Content', 3800000000000, 18.2, 'Chinese internet platform')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec fixture: %v", err)
		}
	}

	report, err := NewService(db).SearchStocks(context.Background(), StockSearchOptions{
		Query:  "t",
		Market: "all",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("SearchStocks returned error: %v", err)
	}

	if report.Query != "t" ||
		report.MarketFilter != "all" ||
		report.Count != 2 ||
		report.DecisionBoundary != "facts_only" ||
		report.SideEffects != "none" ||
		report.ExternalFetch != "not_performed" {
		t.Fatalf("report metadata = %#v", report)
	}
	if report.Results[0].Code != "00700" ||
		report.Results[0].Market != "HK" ||
		report.Results[0].Source != "local_profile" {
		t.Fatalf("first result = %#v, want HK profile sorted by code", report.Results[0])
	}
	if report.Results[1].Code != "MSFT" ||
		report.Results[1].Market != "US" ||
		report.Results[1].Sector != "Technology" ||
		report.Results[1].MarketCap == nil ||
		*report.Results[1].MarketCap != 3200000000000 {
		t.Fatalf("second result = %#v, want MSFT profile facts", report.Results[1])
	}
}

func TestServiceSearchStocksFiltersMarketAndLimit(t *testing.T) {
	db := testutil.OpenTempDBWithProductionSchema(t)

	for _, stmt := range []string{
		`INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES
			('AAPL', 'Apple Inc.', '科技股', 'stock', 'US')`,
		`INSERT INTO stock_profile (code, name, market, sector, industry) VALUES
			('MSFT', 'Microsoft Corporation', 'US', 'Technology', 'Software'),
			('00700', 'Tencent Holdings', 'HK', 'Communication Services', 'Internet Content')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec fixture: %v", err)
		}
	}

	report, err := NewService(db).SearchStocks(context.Background(), StockSearchOptions{
		Query:  "t",
		Market: "US",
		Limit:  1,
	})
	if err != nil {
		t.Fatalf("SearchStocks returned error: %v", err)
	}
	if report.Count != 1 || len(report.Results) != 1 {
		t.Fatalf("count/results = %d/%d, want 1/1", report.Count, len(report.Results))
	}
	if report.Results[0].Market != "US" {
		t.Fatalf("result market = %q, want US", report.Results[0].Market)
	}
}
