package portfolio

import (
	"context"
	"time"
)

type PenetrationOptions struct {
	PortfolioID int
	Limit       int
	SortBy      string
}

type PenetrationReport struct {
	GeneratedAt            string                   `json:"generated_at"`
	ReportQuarter          *string                  `json:"report_quarter"`
	ReportDateRange        *PenetrationDateRange    `json:"report_date_range,omitempty"`
	FundReportDates        []PenetrationReportDate  `json:"fund_report_dates,omitempty"`
	TotalPortfolioValueCNY float64                  `json:"total_portfolio_value_cny"`
	FundsWithHoldings      int                      `json:"funds_with_holdings"`
	StocksFound            int                      `json:"stocks_found"`
	TopHoldingsNote        string                   `json:"top_holdings_note"`
	Penetration            []PenetrationStock       `json:"penetration"`
	BySector               []PenetrationSector      `json:"by_sector"`
	UnavailableFunds       []PenetrationUnavailable `json:"unavailable_funds"`
	DecisionBoundary       string                   `json:"decision_boundary"`
	SideEffects            string                   `json:"side_effects"`
}

type PenetrationDateRange struct {
	First string `json:"first"`
	Last  string `json:"last"`
	Mixed bool   `json:"mixed"`
}

type PenetrationReportDate struct {
	FundCode   string `json:"fund_code"`
	ReportDate string `json:"report_date"`
}

type PenetrationStock struct {
	StockCode               string                    `json:"stock_code"`
	StockName               string                    `json:"stock_name"`
	Sector                  string                    `json:"sector"`
	EstimatedMarketValueCNY float64                   `json:"estimated_market_value_cny"`
	PenetrationPct          float64                   `json:"penetration_pct"`
	CumulativeWeightPct     float64                   `json:"cumulative_weight_pct"`
	FundCount               int                       `json:"fund_count"`
	HeldViaFunds            []string                  `json:"held_via_funds"`
	HeldByFunds             []PenetrationFundExposure `json:"held_by_funds,omitempty"`
}

type PenetrationFundExposure struct {
	FundCode     string  `json:"fund_code"`
	FundName     string  `json:"fund_name"`
	WeightPct    float64 `json:"weight_pct"`
	FundValueCNY float64 `json:"fund_value_cny"`
}

type PenetrationSector struct {
	Sector           string  `json:"sector"`
	TotalExposureCNY float64 `json:"total_exposure_cny"`
	PenetrationPct   float64 `json:"penetration_pct"`
	StockCount       int     `json:"stock_count"`
}

type PenetrationUnavailable struct {
	FundCode string `json:"fund_code"`
	FundName string `json:"fund_name"`
	Reason   string `json:"reason"`
}

type penetrationFund struct {
	Code          string
	Name          string
	Value         float64
	FundType      string
	HasHoldings   bool
	NotApplicable bool
}

type penetrationHolding struct {
	FundCode   string
	StockCode  string
	StockName  string
	WeightPct  float64
	ReportDate string
}

type penetrationStockAgg struct {
	StockName string
	Sector    string
	Value     float64
	WeightPct float64
	Funds     map[string]PenetrationFundExposure
}

func (s Service) GetPenetration(ctx context.Context, options PenetrationOptions) (*PenetrationReport, error) {
	portfolioID := options.PortfolioID
	portfolioID = clampPortfolioID(portfolioID)
	limit := options.Limit
	if limit <= 0 {
		limit = 30
	}
	sortBy := options.SortBy
	if sortBy != "weight_pct" {
		sortBy = "market_value"
	}

	funds, err := s.penetrationFunds(ctx, portfolioID)
	if err != nil {
		return nil, err
	}
	report := &PenetrationReport{
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		TopHoldingsNote:  "Fund holding disclosures usually cover reported top positions only; uncovered exposure may exist.",
		DecisionBoundary: "facts_only",
		SideEffects:      "none",
	}
	for _, fund := range funds {
		report.TotalPortfolioValueCNY += fund.Value
	}
	report.TotalPortfolioValueCNY = round2(report.TotalPortfolioValueCNY)

	if len(funds) == 0 {
		return report, nil
	}

	holdings, err := s.penetrationLatestHoldings(ctx, funds)
	if err != nil {
		return nil, err
	}
	report.ReportDateRange, report.FundReportDates = buildPenetrationReportDates(holdings)
	if report.ReportDateRange != nil {
		report.ReportQuarter = quarterLabelPtr(report.ReportDateRange.Last)
	}
	sectors, err := s.penetrationSectors(ctx)
	if err != nil {
		return nil, err
	}

	fundsByCode := map[string]*penetrationFund{}
	for i := range funds {
		fundsByCode[funds[i].Code] = &funds[i]
	}

	stockAgg := map[string]*penetrationStockAgg{}
	for _, holding := range holdings {
		fund := fundsByCode[holding.FundCode]
		if fund == nil || fund.Value <= 0 || holding.WeightPct <= 0 {
			continue
		}
		fund.HasHoldings = true
		agg := stockAgg[holding.StockCode]
		if agg == nil {
			agg = &penetrationStockAgg{
				StockName: clampPortfolioText(holding.StockName, 200),
				Sector:    sectors[holding.StockCode],
				Funds:     map[string]PenetrationFundExposure{},
			}
			if agg.Sector == "" {
				agg.Sector = "other"
			}
			stockAgg[holding.StockCode] = agg
		}
		agg.Value += fund.Value * holding.WeightPct / 100
		agg.WeightPct += holding.WeightPct
		agg.Funds[holding.FundCode] = PenetrationFundExposure{
			FundCode:     holding.FundCode,
			FundName:     clampPortfolioText(fund.Name, 200),
			WeightPct:    round2(holding.WeightPct),
			FundValueCNY: round2(fund.Value),
		}
	}

	report.FundsWithHoldings = countFundsWithHoldings(funds)
	report.Penetration = buildPenetrationStocks(stockAgg, report.TotalPortfolioValueCNY, sortBy, limit)
	report.StocksFound = len(stockAgg)
	report.BySector = buildPenetrationSectors(stockAgg, report.TotalPortfolioValueCNY)
	report.UnavailableFunds = buildUnavailableFunds(funds)
	return report, nil
}
