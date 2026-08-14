package portfolio

import (
	"context"

	"database/sql"

	"strings"

	"testing"
)

func TestServiceGetAllocationMatchesCurrentPortfolioSemantics(t *testing.T) {

	db := openSummaryFixture(t)

	defer db.Close()

	seedMixedAllocationData(t, db)

	service := NewService(db)

	allocation, err := service.GetAllocation(context.Background(), 1)

	if err != nil {

		t.Fatalf("GetAllocation returned error: %v", err)

	}

	if allocation.TotalValue != 830 {

		t.Fatalf("TotalValue = %.2f, want 830", allocation.TotalValue)

	}

	if len(allocation.BySecurityType) != 2 {

		t.Fatalf("BySecurityType length = %d, want 2: %#v", len(allocation.BySecurityType), allocation.BySecurityType)

	}

	assertBucket(t, allocation.BySecurityType[0], AllocationBucket{Key: "stock", Label: "Stock", Value: 680, WeightPct: 81.93, Count: 2})

	assertBucket(t, allocation.BySecurityType[1], AllocationBucket{Key: "fund", Label: "Fund", Value: 150, WeightPct: 18.07, Count: 1})

	if got := bucketKeys(allocation.ByMarket); strings.Join(got, ",") != "us_stock,hk_stock,cn_fund" {

		t.Fatalf("ByMarket keys = %v, want us_stock,hk_stock,cn_fund", got)

	}

	assertBucket(t, allocation.ByMarket[0], AllocationBucket{Key: "us_stock", Label: "US Stocks", Value: 380, WeightPct: 45.78, Count: 1})

	assertBucket(t, allocation.ByMarket[1], AllocationBucket{Key: "hk_stock", Label: "HK Stocks", Value: 300, WeightPct: 36.14, Count: 1})

	assertBucket(t, allocation.ByMarket[2], AllocationBucket{Key: "cn_fund", Label: "CN Funds", Value: 150, WeightPct: 18.07, Count: 1})

	if got := bucketKeys(allocation.ByFundType); strings.Join(got, ",") != "科技股,港股,QDII-股票" {

		t.Fatalf("ByFundType keys = %v, want sorted by value", got)

	}

	if len(allocation.RiskFlags) != 1 || allocation.RiskFlags[0] != "Stock weight above 80%" {

		t.Fatalf("RiskFlags = %#v, want stock concentration only", allocation.RiskFlags)

	}

	if !strings.Contains(allocation.AgentBrief, "Stock 81.93%") {

		t.Fatalf("AgentBrief = %q, want stock allocation fact", allocation.AgentBrief)

	}

	if strings.Contains(allocation.AgentBrief, "买入") || strings.Contains(allocation.AgentBrief, "卖出") {

		t.Fatalf("AgentBrief = %q, want facts-only brief without trading guidance", allocation.AgentBrief)

	}

}

func TestServiceGetAllocationReturnsEmptyFactsForEmptyPortfolio(t *testing.T) {

	db := openSummaryFixture(t)

	defer db.Close()

	service := NewService(db)

	allocation, err := service.GetAllocation(context.Background(), 2)

	if err != nil {

		t.Fatalf("GetAllocation returned error: %v", err)

	}

	if allocation.TotalValue != 0 {

		t.Fatalf("TotalValue = %.2f, want 0", allocation.TotalValue)

	}

	if len(allocation.BySecurityType) != 0 || len(allocation.ByMarket) != 0 || len(allocation.ByFundType) != 0 {

		t.Fatalf("buckets = %#v/%#v/%#v, want empty slices", allocation.BySecurityType, allocation.ByMarket, allocation.ByFundType)

	}

	if len(allocation.RiskFlags) != 0 {

		t.Fatalf("RiskFlags = %#v, want none", allocation.RiskFlags)

	}

	if !strings.Contains(allocation.AgentBrief, "no holdings") {

		t.Fatalf("AgentBrief = %q, want empty holding fact", allocation.AgentBrief)

	}

	if !strings.Contains(allocation.AgentBrief, "no concentration alerts") {

		t.Fatalf("AgentBrief = %q, want no concentration warning", allocation.AgentBrief)

	}

}

func seedMixedAllocationData(t *testing.T, db execer) {

	t.Helper()

	if _, err := db.ExecContext(context.Background(), `

		DELETE FROM portfolio_snapshot;

		DELETE FROM fund_details;

		INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES

			('019173', '纳斯达克100指数(QDII)C', 'QDII-股票', 'fund', 'CN'),

			('AAPL', 'Apple Inc.', '科技股', 'stock', 'US'),

			('00700', '腾讯控股', '港股', 'stock', 'HK');

		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES

			('019173', '纳斯达克100指数(QDII)C', 100, -120, 1.5, 150, 30, 25, 'fund', 1),

			('AAPL', 'Apple Inc.', 2, -300, 190, 380, 80, 26.67, 'stock', 1),

			('00700', '腾讯控股', 10, -300, 30, 300, 0, 0, 'stock', 1);

	`); err != nil {

		t.Fatalf("seed mixed allocation data: %v", err)

	}

}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func assertBucket(t *testing.T, got AllocationBucket, want AllocationBucket) {

	t.Helper()

	if got != want {

		t.Fatalf("bucket = %#v, want %#v", got, want)

	}

}

func bucketKeys(rows []AllocationBucket) []string {

	keys := make([]string, 0, len(rows))

	for _, row := range rows {

		keys = append(keys, row.Key)

	}

	return keys

}
