package portfolio

import (
	"context"
	"testing"
)

func TestServiceGetPenetrationAggregatesLatestFundHoldings(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE fund_holdings (
			fund_code TEXT,
			stock_code TEXT,
			stock_name TEXT,
			weight_pct REAL,
			shares REAL,
			market_value REAL,
			report_date TEXT
		);
		CREATE TABLE sector_map (
			stock_code TEXT,
			market TEXT,
			sector TEXT,
			PRIMARY KEY (stock_code, market)
		);
		INSERT INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date) VALUES
			('019173', 'NVDA', 'NVIDIA', 8.5, 100, 12000, '2026-03-31'),
			('019173', 'MSFT', 'Microsoft', 7.2, 100, 11000, '2026-03-31'),
			('019173', 'OLD', 'Old Holding', 50, 100, 10000, '2025-12-31'),
			('018439', 'NVDA', 'NVIDIA', 5, 100, 9000, '2026-03-31');
		INSERT INTO sector_map (stock_code, market, sector) VALUES
			('NVDA', 'US', 'Semiconductors'),
			('MSFT', 'US', 'Software');
	`); err != nil {
		t.Fatalf("seed penetration fixture: %v", err)
	}

	service := NewService(db)
	report, err := service.GetPenetration(context.Background(), PenetrationOptions{
		PortfolioID: 1,
		Limit:       10,
		SortBy:      "market_value",
	})
	if err != nil {
		t.Fatalf("GetPenetration returned error: %v", err)
	}

	if report.DecisionBoundary != "facts_only" {
		t.Fatalf("DecisionBoundary = %q, want facts_only", report.DecisionBoundary)
	}
	if report.TotalPortfolioValueCNY != 500.03 {
		t.Fatalf("TotalPortfolioValueCNY = %.2f, want 500.03", report.TotalPortfolioValueCNY)
	}
	if report.FundsWithHoldings != 2 || report.StocksFound != 2 {
		t.Fatalf("coverage counts = %d/%d, want 2/2", report.FundsWithHoldings, report.StocksFound)
	}
	if len(report.Penetration) != 2 {
		t.Fatalf("Penetration length = %d, want 2: %#v", len(report.Penetration), report.Penetration)
	}
	if report.Penetration[0].StockCode != "NVDA" ||
		report.Penetration[0].Sector != "Semiconductors" ||
		report.Penetration[0].EstimatedMarketValueCNY != 34.31 ||
		report.Penetration[0].PenetrationPct != 6.86 ||
		report.Penetration[0].FundCount != 2 {
		t.Fatalf("NVDA aggregation wrong: %#v", report.Penetration[0])
	}
	if report.Penetration[1].StockCode != "MSFT" ||
		report.Penetration[1].EstimatedMarketValueCNY != 19.15 ||
		report.Penetration[1].PenetrationPct != 3.83 {
		t.Fatalf("MSFT aggregation wrong: %#v", report.Penetration[1])
	}
	if len(report.BySector) != 2 || report.BySector[0].Sector != "Semiconductors" {
		t.Fatalf("BySector wrong: %#v", report.BySector)
	}
	if len(report.UnavailableFunds) != 0 {
		t.Fatalf("UnavailableFunds = %#v, want empty", report.UnavailableFunds)
	}
}

func TestServiceGetPenetrationReportsMixedDisclosureDates(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE fund_holdings (
			fund_code TEXT,
			stock_code TEXT,
			stock_name TEXT,
			weight_pct REAL,
			shares REAL,
			market_value REAL,
			report_date TEXT
		);
		CREATE TABLE sector_map (
			stock_code TEXT,
			market TEXT,
			sector TEXT,
			PRIMARY KEY (stock_code, market)
		);
		INSERT INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date) VALUES
			('019173', 'NVDA', 'NVIDIA', 8.5, 100, 12000, '2026-03-31'),
			('018439', 'MSFT', 'Microsoft', 7.2, 100, 11000, '2025-12-31');
	`); err != nil {
		t.Fatalf("seed mixed disclosure fixture: %v", err)
	}

	service := NewService(db)
	report, err := service.GetPenetration(context.Background(), PenetrationOptions{PortfolioID: 1})
	if err != nil {
		t.Fatalf("GetPenetration returned error: %v", err)
	}

	if report.ReportDateRange == nil {
		t.Fatalf("ReportDateRange = nil, want disclosure date coverage")
	}
	if report.ReportDateRange.First != "2025-12-31" ||
		report.ReportDateRange.Last != "2026-03-31" ||
		!report.ReportDateRange.Mixed {
		t.Fatalf("ReportDateRange = %#v, want mixed 2025-12-31..2026-03-31", report.ReportDateRange)
	}
	if len(report.FundReportDates) != 2 {
		t.Fatalf("FundReportDates length = %d, want 2: %#v", len(report.FundReportDates), report.FundReportDates)
	}
	if report.FundReportDates[0].FundCode != "018439" || report.FundReportDates[0].ReportDate != "2025-12-31" ||
		report.FundReportDates[1].FundCode != "019173" || report.FundReportDates[1].ReportDate != "2026-03-31" {
		t.Fatalf("FundReportDates = %#v, want sorted per-fund latest dates", report.FundReportDates)
	}
}

func TestServiceGetPenetrationAvoidsAmbiguousSectorMapByCode(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE fund_holdings (
			fund_code TEXT,
			stock_code TEXT,
			stock_name TEXT,
			weight_pct REAL,
			shares REAL,
			market_value REAL,
			report_date TEXT
		);
		CREATE TABLE sector_map (
			stock_code TEXT,
			market TEXT,
			sector TEXT,
			PRIMARY KEY (stock_code, market)
		);
		INSERT INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date) VALUES
			('019173', 'DUPL', 'Duplicate Code', 10, 100, 12000, '2026-03-31');
		INSERT INTO sector_map (stock_code, market, sector) VALUES
			('DUPL', 'US', 'Technology'),
			('DUPL', 'HK', 'Financial');
	`); err != nil {
		t.Fatalf("seed ambiguous sector fixture: %v", err)
	}

	service := NewService(db)
	report, err := service.GetPenetration(context.Background(), PenetrationOptions{PortfolioID: 1})
	if err != nil {
		t.Fatalf("GetPenetration returned error: %v", err)
	}
	if len(report.Penetration) == 0 {
		t.Fatalf("Penetration empty, want DUPL row")
	}
	if report.Penetration[0].StockCode != "DUPL" || report.Penetration[0].Sector != "other" {
		t.Fatalf("ambiguous sector row = %#v, want sector fallback", report.Penetration[0])
	}
}
