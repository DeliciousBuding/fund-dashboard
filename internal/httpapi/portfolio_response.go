package httpapi

import (
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

type portfolioSummaryJSON struct {
	TotalTx                int                       `json:"total_tx"`
	UniqueFunds            int                       `json:"unique_funds"`
	UniqueStocks           int                       `json:"unique_stocks"`
	HeldFunds              int                       `json:"held_funds"`
	TotalBuy               float64                   `json:"total_buy"`
	TotalSell              float64                   `json:"total_sell"`
	TotalFee               float64                   `json:"total_fee"`
	UnrealizedPNL          float64                   `json:"unrealized_pnl"`
	InvestedCost           float64                   `json:"invested_cost"`
	CurrentValue           float64                   `json:"current_value"`
	PNLPct                 float64                   `json:"pnl_pct"`
	TopGainer              *holdingContributorJSON   `json:"top_gainer,omitempty"`
	TopLoser               *holdingContributorJSON   `json:"top_loser,omitempty"`
	StaleNAVDays           *int                      `json:"stale_nav_days,omitempty"`
	AutoTx                 int                       `json:"auto_tx"`
	ManualTx               int                       `json:"manual_tx"`
	AutoAmount             float64                   `json:"auto_amount"`
	ManualAmount           float64                   `json:"manual_amount"`
	FirstTrade             string                    `json:"first_trade"`
	LastTrade              string                    `json:"last_trade"`
	LastNAVDate            *string                   `json:"last_nav_date"`
	SettlementDistribution map[string]int            `json:"settlement_distribution"`
	TradeTypeBreakdown     map[string]int            `json:"trade_type_breakdown"`
	BySecurityType         []securityTypeBalanceJSON `json:"by_security_type"`
}

type securityTypeBalanceJSON struct {
	SecurityType string  `json:"security_type"`
	Count        int     `json:"count"`
	TotalValue   float64 `json:"total_value"`
	TotalPNL     float64 `json:"total_pnl"`
}

type holdingContributorJSON struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	UnrealizedPNL float64 `json:"unrealized_pnl"`
	PNLPct        float64 `json:"pnl_pct"`
	CurrentValue  float64 `json:"current_value"`
}

type xirrResponse struct {
	XIRR    *float64 `json:"xirr"`
	Message *string  `json:"message,omitempty"`
	Code    string   `json:"code,omitempty"`
}

type fundDetailJSON struct {
	Code             string            `json:"code"`
	Name             string            `json:"name"`
	SecurityType     string            `json:"security_type,omitempty"`
	Market           string            `json:"market,omitempty"`
	HeldShares       float64           `json:"held_shares"`
	TotalCost        float64           `json:"total_cost"`
	LatestNAV        *float64          `json:"latest_nav"`
	CurrentValue     *float64          `json:"current_value"`
	UnrealizedPNL    *float64          `json:"unrealized_pnl"`
	PNLPct           *float64          `json:"pnl_pct"`
	AutoBuyCount     int               `json:"auto_buy_count"`
	ManualBuyCount   int               `json:"manual_buy_count"`
	AutoBuyAmount    float64           `json:"auto_buy_amount"`
	ManualBuyAmount  float64           `json:"manual_buy_amount"`
	AutoTx           int               `json:"auto_tx"`
	ManualTx         int               `json:"manual_tx"`
	BuyCount         int               `json:"buy_count"`
	SellCount        int               `json:"sell_count"`
	MedianSettlement int               `json:"median_settlement"`
	Transactions     []fundTransaction `json:"transactions"`
}

type fundTransaction struct {
	Seq            *int     `json:"seq"`
	TradeTime      string   `json:"trade_time"`
	ConfirmDate    *string  `json:"confirm_date,omitempty"`
	TradeType      string   `json:"trade_type"`
	Direction      string   `json:"direction"`
	Amount         float64  `json:"amount"`
	Shares         float64  `json:"shares"`
	Fee            float64  `json:"fee"`
	NAV            *float64 `json:"nav"`
	InferredNAV    *float64 `json:"inferred_nav"`
	SettlementDays *int     `json:"settlement_days,omitempty"`
	OrderID        *string  `json:"order_id,omitempty"`
	Anomaly        *string  `json:"anomaly"`
}

type drawdownJSON struct {
	Code        string  `json:"code,omitempty"`
	MaxDrawdown float64 `json:"max_drawdown"`
	PeakDate    string  `json:"peak_date"`
	TroughDate  string  `json:"trough_date"`
}

type penetrationJSON struct {
	Penetration         []penetrationStockJSON `json:"penetration"`
	TotalPortfolioValue float64                `json:"total_portfolio_value"`
	EquityFundCount     int                    `json:"equity_fund_count"`
	UniqueStocks        int                    `json:"unique_stocks"`
}

type penetrationStockJSON struct {
	StockCode        string                `json:"stock_code"`
	StockName        string                `json:"stock_name"`
	TotalExposureCNY float64               `json:"total_exposure_cny"`
	WeightPct        float64               `json:"weight_pct"`
	HeldByFunds      []penetrationFundJSON `json:"held_by_funds"`
}

type penetrationFundJSON struct {
	FundCode     string  `json:"fund_code"`
	FundName     string  `json:"fund_name"`
	WeightPct    float64 `json:"weight_pct"`
	FundValueCNY float64 `json:"fund_value_cny"`
}

func portfolioSummaryResponse(summary *portfoliosvc.Summary) portfolioSummaryJSON {
	if summary == nil {
		return portfolioSummaryJSON{
			SettlementDistribution: map[string]int{},
			TradeTypeBreakdown:     map[string]int{},
			BySecurityType:         []securityTypeBalanceJSON{},
		}
	}
	return portfolioSummaryJSON{
		TotalTx:                summary.TotalTx,
		UniqueFunds:            summary.UniqueFunds,
		UniqueStocks:           summary.UniqueStocks,
		HeldFunds:              summary.HeldFunds,
		TotalBuy:               summary.TotalBuy,
		TotalSell:              summary.TotalSell,
		TotalFee:               summary.TotalFee,
		UnrealizedPNL:          summary.UnrealizedPNL,
		InvestedCost:           summary.InvestedCost,
		CurrentValue:           summary.CurrentValue,
		PNLPct:                 summary.PNLPct,
		TopGainer:              holdingContributorResponse(summary.TopGainer),
		TopLoser:               holdingContributorResponse(summary.TopLoser),
		StaleNAVDays:           summary.StaleNAVDays,
		AutoTx:                 summary.AutoTx,
		ManualTx:               summary.ManualTx,
		AutoAmount:             summary.AutoAmount,
		ManualAmount:           summary.ManualAmount,
		FirstTrade:             summary.FirstTrade,
		LastTrade:              summary.LastTrade,
		LastNAVDate:            summary.LastNAVDate,
		SettlementDistribution: summary.SettlementDistribution,
		TradeTypeBreakdown:     summary.TradeTypeBreakdown,
		BySecurityType:         securityTypeBalanceResponses(summary.BySecurityType),
	}
}

func securityTypeBalanceResponses(rows []portfoliosvc.SecurityTypeBalance) []securityTypeBalanceJSON {
	out := make([]securityTypeBalanceJSON, 0, len(rows))
	for _, row := range rows {
		out = append(out, securityTypeBalanceJSON{
			SecurityType: row.SecurityType,
			Count:        row.Count,
			TotalValue:   row.TotalValue,
			TotalPNL:     row.TotalPNL,
		})
	}
	return out
}

func holdingContributorResponse(c *portfoliosvc.HoldingContributor) *holdingContributorJSON {
	if c == nil {
		return nil
	}
	return &holdingContributorJSON{
		Code:          c.Code,
		Name:          c.Name,
		UnrealizedPNL: c.UnrealizedPNL,
		PNLPct:        c.PNLPct,
		CurrentValue:  c.CurrentValue,
	}
}
