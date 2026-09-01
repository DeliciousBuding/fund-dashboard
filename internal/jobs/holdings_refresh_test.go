package jobs

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
	_ "modernc.org/sqlite"
)

func TestUpsertFundHoldings_IdempotentNoRewrite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE fund_holdings (
			fund_code TEXT,
			stock_code TEXT,
			stock_name TEXT,
			weight_pct REAL,
			shares REAL,
			market_value REAL,
			report_date TEXT,
			PRIMARY KEY (fund_code, stock_code, report_date)
		)
	`); err != nil {
		t.Fatal(err)
	}

	holdings := []datasource.FundHolding{
		{StockCode: "NVDA", StockName: "NVIDIA", WeightPct: 8.5, Shares: 100, MarketValue: 12000, ReportDate: "2026-03-31"},
		{StockCode: "MSFT", StockName: "Microsoft", WeightPct: 7.2, Shares: 100, MarketValue: 11000, ReportDate: "2026-03-31"},
	}

	first, err := upsertFundHoldings(context.Background(), db, "019173", "2026-03-31", holdings)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if first != 2 {
		t.Fatalf("first added=%d want 2", first)
	}

	second, err := upsertFundHoldings(context.Background(), db, "019173", "2026-03-31", holdings)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if second != 0 {
		t.Fatalf("second added=%d want 0 for unchanged slice", second)
	}

	// Change one weight → rewrite.
	changed := []datasource.FundHolding{
		{StockCode: "NVDA", StockName: "NVIDIA", WeightPct: 9.0, Shares: 100, MarketValue: 12000, ReportDate: "2026-03-31"},
		{StockCode: "MSFT", StockName: "Microsoft", WeightPct: 7.2, Shares: 100, MarketValue: 11000, ReportDate: "2026-03-31"},
	}
	third, err := upsertFundHoldings(context.Background(), db, "019173", "2026-03-31", changed)
	if err != nil {
		t.Fatalf("third upsert: %v", err)
	}
	if third != 2 {
		t.Fatalf("third added=%d want 2 after change", third)
	}
}

// TestHoldingsCrawlTruncationObservable guards the LIMIT+1 probe and warning
// wiring for CrawlAllHeld (same pattern as price_refresh/recalc truncation).
func TestHoldingsCrawlTruncationObservable(t *testing.T) {
	raw, err := os.ReadFile("holdings_refresh.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	if !strings.Contains(src, "holdingsCrawlMaxCodes+1") && !strings.Contains(src, "holdingsCrawlMaxCodes + 1") {
		t.Fatal("expected LIMIT max+1 probe in CrawlAllHeld")
	}
	if !strings.Contains(src, `"holdings crawl code list truncated"`) {
		t.Fatal("expected truncation warning log in CrawlAllHeld")
	}
	if !strings.Contains(src, "capCodes(codes, holdingsCrawlMaxCodes)") {
		t.Fatal("expected capCodes applied to held fund codes")
	}
}
