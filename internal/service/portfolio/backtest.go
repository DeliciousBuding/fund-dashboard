package portfolio

import (
	"context"
	"fmt"
	"strings"
)

type BacktestOptions struct {
	FundCode          string
	Strategy          string
	StartDate         string
	BaseAmount        float64
	GridLevels        int
	MomentumMonths    int
	TargetWeight      float64
	RebalanceInterval int
}

type BacktestResult struct {
	FundCode         string             `json:"fund_code"`
	Strategy         string             `json:"strategy,omitempty"`
	StartDate        string             `json:"start_date,omitempty"`
	EndDate          string             `json:"end_date,omitempty"`
	BaseAmount       float64            `json:"base_amount,omitempty"`
	TotalInvested    float64            `json:"total_invested,omitempty"`
	FinalValue       float64            `json:"final_value,omitempty"`
	TotalReturnPct   float64            `json:"total_return_pct,omitempty"`
	AnnualReturnPct  float64            `json:"annual_return_pct,omitempty"`
	MaxDrawdownPct   float64            `json:"max_drawdown_pct,omitempty"`
	SharpeRatio      float64            `json:"sharpe_ratio,omitempty"`
	Trades           []BacktestTrade    `json:"trades,omitempty"`
	Timeline         []BacktestPoint    `json:"timeline,omitempty"`
	Comparison       BacktestComparison `json:"comparison,omitempty"`
	DecisionBoundary string             `json:"decision_boundary,omitempty"`
	SideEffects      string             `json:"side_effects,omitempty"`
	Error            string             `json:"error,omitempty"`
	Message          string             `json:"message,omitempty"`
}

type BacktestTrade struct {
	Date   string  `json:"date"`
	Action string  `json:"action"`
	Price  float64 `json:"price"`
	Shares float64 `json:"shares"`
	Amount float64 `json:"amount"`
	Reason string  `json:"reason"`
}

type BacktestPoint struct {
	Date          string  `json:"date"`
	NAV           float64 `json:"nav"`
	SharesHeld    float64 `json:"shares_held"`
	Cash          float64 `json:"cash"`
	EquityValue   float64 `json:"equity_value"`
	TotalValue    float64 `json:"total_value"`
	TotalInvested float64 `json:"total_invested"`
}

type BacktestComparison struct {
	LumpSum BacktestReturn `json:"lump_sum"`
	DCA     BacktestReturn `json:"dca"`
}

type BacktestReturn struct {
	Invested   float64 `json:"invested"`
	FinalValue float64 `json:"final_value"`
	ReturnPct  float64 `json:"return_pct"`
}

type backtestNAV struct {
	Date     string
	FundCode string
	UnitNAV  float64
}

func (s Service) RunBacktest(ctx context.Context, opts BacktestOptions) (BacktestResult, error) {
	strategy := normalizeBacktestStrategy(opts.Strategy)
	baseAmount := opts.BaseAmount
	if !isFinitePositive(baseAmount) {
		baseAmount = 1000
	}
	if baseAmount > 1_000_000 {
		baseAmount = 1_000_000
	}
	opts.StartDate = normalizeBacktestStartDate(opts.StartDate)
	opts.FundCode = strings.TrimSpace(opts.FundCode)
	navs, err := s.queryBacktestNAVs(ctx, opts.FundCode, opts.StartDate)
	if err != nil {
		return BacktestResult{}, err
	}
	if len(navs) == 0 {
		return BacktestResult{
			FundCode:         opts.FundCode,
			Strategy:         strategy,
			StartDate:        opts.StartDate,
			BaseAmount:       round2(baseAmount),
			DecisionBoundary: "facts_only",
			SideEffects:      "none",
			Error:            "no_data",
			Message:          fmt.Sprintf("no_nav_data fund=%s after=%s", opts.FundCode, opts.StartDate),
		}, nil
	}
	result := runBacktestFromNAVs(navs, BacktestOptions{
		FundCode:          opts.FundCode,
		Strategy:          strategy,
		StartDate:         opts.StartDate,
		BaseAmount:        baseAmount,
		GridLevels:        opts.GridLevels,
		MomentumMonths:    opts.MomentumMonths,
		TargetWeight:      opts.TargetWeight,
		RebalanceInterval: opts.RebalanceInterval,
	})
	result.DecisionBoundary = "facts_only"
	result.SideEffects = "none"
	return result, nil
}

func (s Service) queryBacktestNAVs(ctx context.Context, code string, startDate string) ([]backtestNAV, error) {
	// Cap points to bound memory/CPU for long histories (#217).
	const maxBacktestPoints = 5000
	rows, err := s.db.QueryContext(ctx, `
		SELECT date, fund_code, unit_nav
		FROM nav_history
		WHERE fund_code = ? AND date >= ? AND unit_nav > 0
		ORDER BY date
		LIMIT ?
	`, code, startDate, maxBacktestPoints)
	if err != nil {
		return nil, fmt.Errorf("query backtest navs: %w", err)
	}
	defer rows.Close()

	var navs []backtestNAV
	for rows.Next() {
		var nav backtestNAV
		if err := rows.Scan(&nav.Date, &nav.FundCode, &nav.UnitNAV); err != nil {
			return nil, fmt.Errorf("scan backtest nav: %w", err)
		}
		navs = append(navs, nav)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("backtest nav rows: %w", err)
	}
	return navs, nil
}

func runBacktestFromNAVs(navs []backtestNAV, opts BacktestOptions) BacktestResult {
	strategy := normalizeBacktestStrategy(opts.Strategy)
	trades, timeline := simulateBacktestStrategy(navs, opts)
	endDate := opts.StartDate
	invested := 0.0
	finalValue := 0.0
	if len(timeline) > 0 {
		last := timeline[len(timeline)-1]
		endDate = last.Date
		invested = last.TotalInvested
		finalValue = last.TotalValue
	}
	metrics := computeBacktestMetrics(timeline, invested, opts.StartDate, endDate)
	return BacktestResult{
		FundCode:        opts.FundCode,
		Strategy:        strategy,
		StartDate:       opts.StartDate,
		EndDate:         endDate,
		BaseAmount:      round2(opts.BaseAmount),
		TotalInvested:   round2(invested),
		FinalValue:      round2(finalValue),
		TotalReturnPct:  metrics.TotalReturnPct,
		AnnualReturnPct: metrics.AnnualReturnPct,
		MaxDrawdownPct:  metrics.MaxDrawdownPct,
		SharpeRatio:     metrics.SharpeRatio,
		Trades:          trades,
		Timeline:        timeline,
		Comparison: BacktestComparison{
			LumpSum: lumpSumBacktestReturn(navs, opts.StartDate, opts.BaseAmount*12),
			DCA:     dcaBacktestReturn(navs, opts.StartDate, opts.BaseAmount),
		},
	}
}

func simulateBacktestStrategy(navs []backtestNAV, opts BacktestOptions) ([]BacktestTrade, []BacktestPoint) {
	switch normalizeBacktestStrategy(opts.Strategy) {
	case "grid":
		return simGridBacktest(navs, opts.StartDate, opts.BaseAmount, opts.GridLevels)
	case "momentum":
		return simMomentumBacktest(navs, opts.StartDate, opts.BaseAmount, opts.MomentumMonths)
	case "rebalance":
		return simRebalanceBacktest(navs, opts.StartDate, opts.BaseAmount, opts.TargetWeight, opts.RebalanceInterval)
	default:
		return simDCABacktest(navs, opts.StartDate, opts.BaseAmount)
	}
}

func simDCABacktest(navs []backtestNAV, startDate string, baseAmount float64) ([]BacktestTrade, []BacktestPoint) {
	filtered := navsFrom(navs, startDate)
	trades := []BacktestTrade{}
	timeline := []BacktestPoint{}
	shares := 0.0
	cash := 0.0
	invested := 0.0
	lastYear, lastMonth := -1, -1
	for _, nav := range filtered {
		year, month := yearMonth(nav.Date)
		if year != lastYear || month != lastMonth {
			lastYear, lastMonth = year, month
			bought := baseAmount / nav.UnitNAV
			shares += bought
			cash -= baseAmount
			invested += baseAmount
			trades = append(trades, BacktestTrade{Date: nav.Date, Action: "buy", Price: round4(nav.UnitNAV), Shares: round4(bought), Amount: round2(baseAmount), Reason: "定期定额买入 (DCA)"})
		}
		timeline = append(timeline, backtestPoint(nav, shares, cash, invested))
	}
	return trades, timeline
}

func normalizeBacktestStartDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "2000-01-01"
	}
	if len(raw) >= 10 {
		return raw[:10]
	}
	return raw
}
