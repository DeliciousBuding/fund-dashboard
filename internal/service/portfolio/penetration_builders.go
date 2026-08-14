package portfolio

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

func (s Service) penetrationFunds(ctx context.Context, portfolioID int) ([]penetrationFund, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			ps.fund_code,
			ps.fund_name,
			COALESCE(ps.current_value, ps.held_shares * ps.latest_nav, 0),
			COALESCE(fd.fund_type, ''),
			COALESCE(fd.security_type, ps.security_type, 'fund')
		FROM portfolio_snapshot ps
		LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
		WHERE ps.held_shares > 0.001
			AND COALESCE(ps.portfolio_id, 1) = ?
			AND COALESCE(fd.security_type, ps.security_type, 'fund') = 'fund'
		ORDER BY COALESCE(ps.current_value, 0) DESC, ps.fund_code
		LIMIT 5000
	`, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("query penetration funds: %w", err)
	}
	defer rows.Close()

	var funds []penetrationFund
	for rows.Next() {
		var securityType string
		var fund penetrationFund
		if err := rows.Scan(&fund.Code, &fund.Name, &fund.Value, &fund.FundType, &securityType); err != nil {
			return nil, fmt.Errorf("scan penetration fund: %w", err)
		}
		fund.Value = round2(fund.Value)
		fund.NotApplicable = penetrationNotApplicableFundType(fund.FundType)
		funds = append(funds, fund)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("penetration fund rows: %w", err)
	}
	return funds, nil
}

func (s Service) penetrationLatestHoldings(ctx context.Context, funds []penetrationFund) ([]penetrationHolding, error) {
	if len(funds) == 0 {
		return nil, nil
	}
	placeholders := make([]string, 0, len(funds))
	args := make([]any, 0, len(funds))
	for _, fund := range funds {
		placeholders = append(placeholders, "?")
		args = append(args, fund.Code)
	}
	query := fmt.Sprintf(`
		SELECT fh.fund_code, fh.stock_code, fh.stock_name, fh.weight_pct, fh.report_date
		FROM fund_holdings fh
		WHERE fh.fund_code IN (%s)
			AND fh.report_date = (
				SELECT MAX(fh2.report_date)
				FROM fund_holdings fh2
				WHERE fh2.fund_code = fh.fund_code
			)
		ORDER BY fh.fund_code, fh.weight_pct DESC, fh.stock_code
		LIMIT 20000
	`, strings.Join(placeholders, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query latest penetration holdings: %w", err)
	}
	defer rows.Close()

	var holdings []penetrationHolding
	for rows.Next() {
		var holding penetrationHolding
		if err := rows.Scan(&holding.FundCode, &holding.StockCode, &holding.StockName, &holding.WeightPct, &holding.ReportDate); err != nil {
			return nil, fmt.Errorf("scan penetration holding: %w", err)
		}
		holding.FundCode = clampPortfolioText(holding.FundCode, 32)
		holding.StockCode = clampPortfolioText(holding.StockCode, 32)
		holding.StockName = clampPortfolioText(holding.StockName, 200)
		holding.ReportDate = clampPortfolioText(holding.ReportDate, 40)
		holdings = append(holdings, holding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("penetration holding rows: %w", err)
	}
	return holdings, nil
}

func (s Service) penetrationSectors(ctx context.Context) (map[string]string, error) {
	var exists int
	// Cross-dialect: try PG first, fall back to sqlite_master
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='sector_map'").Scan(&exists)
	if err != nil {
		err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sector_map'").Scan(&exists)
	}
	if err != nil {
		return nil, fmt.Errorf("check sector_map table: %w", err)
	}
	sectors := map[string]string{}
	if exists == 0 {
		return sectors, nil
	}
	rows, err := s.db.QueryContext(ctx, "SELECT stock_code, COALESCE(sector, '') FROM sector_map LIMIT 20000")
	if err != nil {
		return nil, fmt.Errorf("query sector map: %w", err)
	}
	defer rows.Close()
	sectorSets := map[string]map[string]struct{}{}
	for rows.Next() {
		var code string
		var sector string
		if err := rows.Scan(&code, &sector); err != nil {
			return nil, fmt.Errorf("scan sector map: %w", err)
		}
		sector = strings.TrimSpace(sector)
		if sector == "" {
			continue
		}
		if sectorSets[code] == nil {
			sectorSets[code] = map[string]struct{}{}
		}
		sectorSets[code][sector] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sector map rows: %w", err)
	}
	for code, values := range sectorSets {
		if len(values) != 1 {
			continue
		}
		for sector := range values {
			sectors[code] = sector
		}
	}
	return sectors, nil
}

func buildPenetrationReportDates(holdings []penetrationHolding) (*PenetrationDateRange, []PenetrationReportDate) {
	latestByFund := map[string]string{}
	for _, holding := range holdings {
		if holding.ReportDate == "" {
			continue
		}
		if holding.ReportDate > latestByFund[holding.FundCode] {
			latestByFund[holding.FundCode] = holding.ReportDate
		}
	}
	if len(latestByFund) == 0 {
		return nil, nil
	}
	fundCodes := make([]string, 0, len(latestByFund))
	for fundCode := range latestByFund {
		fundCodes = append(fundCodes, fundCode)
	}
	sort.Strings(fundCodes)
	reportDates := make([]PenetrationReportDate, 0, len(fundCodes))
	first := ""
	last := ""
	for _, fundCode := range fundCodes {
		reportDate := latestByFund[fundCode]
		reportDates = append(reportDates, PenetrationReportDate{FundCode: fundCode, ReportDate: reportDate})
		if first == "" || reportDate < first {
			first = reportDate
		}
		if reportDate > last {
			last = reportDate
		}
	}
	return &PenetrationDateRange{First: first, Last: last, Mixed: first != last}, reportDates
}

func buildPenetrationStocks(aggs map[string]*penetrationStockAgg, totalValue float64, sortBy string, limit int) []PenetrationStock {
	stocks := make([]PenetrationStock, 0, len(aggs))
	for code, agg := range aggs {
		heldViaFunds := make([]string, 0, len(agg.Funds))
		for fundCode := range agg.Funds {
			heldViaFunds = append(heldViaFunds, fundCode)
		}
		sort.Strings(heldViaFunds)
		heldByFunds := make([]PenetrationFundExposure, 0, len(heldViaFunds))
		for _, fundCode := range heldViaFunds {
			heldByFunds = append(heldByFunds, agg.Funds[fundCode])
		}
		penetrationPct := 0.0
		if totalValue > 0 {
			penetrationPct = round2(agg.Value / totalValue * 100)
		}
		stocks = append(stocks, PenetrationStock{
			StockCode:               code,
			StockName:               clampPortfolioText(agg.StockName, 200),
			Sector:                  clampPortfolioText(agg.Sector, 64),
			EstimatedMarketValueCNY: round2(agg.Value),
			PenetrationPct:          penetrationPct,
			CumulativeWeightPct:     round2(agg.WeightPct),
			FundCount:               len(agg.Funds),
			HeldViaFunds:            heldViaFunds,
			HeldByFunds:             heldByFunds,
		})
	}
	sort.SliceStable(stocks, func(i, j int) bool {
		if sortBy == "weight_pct" && stocks[i].CumulativeWeightPct != stocks[j].CumulativeWeightPct {
			return stocks[i].CumulativeWeightPct > stocks[j].CumulativeWeightPct
		}
		if stocks[i].EstimatedMarketValueCNY == stocks[j].EstimatedMarketValueCNY {
			return stocks[i].StockCode < stocks[j].StockCode
		}
		return stocks[i].EstimatedMarketValueCNY > stocks[j].EstimatedMarketValueCNY
	})
	if limit < len(stocks) {
		return stocks[:limit]
	}
	return stocks
}

func buildPenetrationSectors(aggs map[string]*penetrationStockAgg, totalValue float64) []PenetrationSector {
	sectorAgg := map[string]*PenetrationSector{}
	for _, agg := range aggs {
		sector := agg.Sector
		if sector == "" {
			sector = "other"
		}
		row := sectorAgg[sector]
		if row == nil {
			row = &PenetrationSector{Sector: sector}
			sectorAgg[sector] = row
		}
		row.TotalExposureCNY += agg.Value
		row.StockCount++
	}
	sectors := make([]PenetrationSector, 0, len(sectorAgg))
	for _, row := range sectorAgg {
		row.TotalExposureCNY = round2(row.TotalExposureCNY)
		if totalValue > 0 {
			row.PenetrationPct = round2(row.TotalExposureCNY / totalValue * 100)
		}
		sectors = append(sectors, *row)
	}
	sort.SliceStable(sectors, func(i, j int) bool {
		if sectors[i].TotalExposureCNY == sectors[j].TotalExposureCNY {
			return sectors[i].Sector < sectors[j].Sector
		}
		return sectors[i].TotalExposureCNY > sectors[j].TotalExposureCNY
	})
	return sectors
}

func buildUnavailableFunds(funds []penetrationFund) []PenetrationUnavailable {
	out := make([]PenetrationUnavailable, 0)
	for _, fund := range funds {
		if fund.NotApplicable {
			out = append(out, PenetrationUnavailable{FundCode: fund.Code, FundName: fund.Name, Reason: "bond_or_money_market"})
			continue
		}
		if !fund.HasHoldings {
			out = append(out, PenetrationUnavailable{FundCode: fund.Code, FundName: fund.Name, Reason: "no_holdings_data"})
		}
	}
	return out
}

func countFundsWithHoldings(funds []penetrationFund) int {
	count := 0
	for _, fund := range funds {
		if fund.HasHoldings {
			count++
		}
	}
	return count
}

func penetrationNotApplicableFundType(fundType string) bool {
	normalized := strings.ToLower(fundType)
	return strings.Contains(normalized, "债券") ||
		strings.Contains(normalized, "货币") ||
		strings.Contains(normalized, "bond") ||
		strings.Contains(normalized, "money")
}

func quarterLabelPtr(date string) *string {
	if len(date) < 7 {
		return nil
	}
	month := 0
	if _, err := fmt.Sscanf(date[5:7], "%d", &month); err != nil || month <= 0 {
		return nil
	}
	quarter := (month + 2) / 3
	label := fmt.Sprintf("%sQ%d", date[:4], quarter)
	return &label
}
