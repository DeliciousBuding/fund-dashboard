package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"math"
)

type ComputeDCAAmountOptions struct {
	Code        string
	BaseAmount  float64
	Mode        string
	PortfolioID int
}

type DCAAmountPlan struct {
	FundCode         string   `json:"fund_code"`
	SecurityType     string   `json:"security_type,omitempty"`
	Market           string   `json:"market,omitempty"`
	Mode             string   `json:"mode,omitempty"`
	BaseAmount       float64  `json:"base_amount,omitempty"`
	LatestNAV        float64  `json:"latest_nav,omitempty"`
	CostPerShare     *float64 `json:"cost_per_share,omitempty"`
	ChangePct        *float64 `json:"change_pct,omitempty"`
	DeviationPct     *float64 `json:"deviation_pct,omitempty"`
	DCARate          float64  `json:"dca_rate,omitempty"`
	ActualAmount     float64  `json:"actual_amount,omitempty"`
	Signal           string   `json:"signal,omitempty"`
	Range            string   `json:"range,omitempty"`
	Explanation      string   `json:"explanation,omitempty"`
	DecisionBoundary string   `json:"decision_boundary,omitempty"`
	SideEffects      string   `json:"side_effects,omitempty"`
	Error            string   `json:"error,omitempty"`
	Message          string   `json:"message,omitempty"`
}

type dcaPositionFacts struct {
	HeldShares   float64
	TotalCost    sql.NullFloat64
	LatestNAV    sql.NullFloat64
	SecurityType string
	Market       string
}

type dcaRateRow struct {
	Lower float64
	Upper float64
	Rate  float64
}

var dcaRateTable = []dcaRateRow{
	{Lower: 0.25, Upper: math.Inf(1), Rate: 0.50},
	{Lower: 0.20, Upper: 0.25, Rate: 0.525},
	{Lower: 0.15, Upper: 0.20, Rate: 0.55},
	{Lower: 0.10, Upper: 0.15, Rate: 0.60},
	{Lower: 0.075, Upper: 0.10, Rate: 0.70},
	{Lower: 0.05, Upper: 0.075, Rate: 0.80},
	{Lower: 0.025, Upper: 0.05, Rate: 0.90},
	{Lower: -0.025, Upper: 0.025, Rate: 1.00},
	{Lower: -0.05, Upper: -0.025, Rate: 1.20},
	{Lower: -0.075, Upper: -0.05, Rate: 1.40},
	{Lower: -0.10, Upper: -0.075, Rate: 1.60},
	{Lower: -0.15, Upper: -0.10, Rate: 1.80},
	{Lower: -0.20, Upper: -0.15, Rate: 1.90},
	{Lower: -0.25, Upper: -0.20, Rate: 1.95},
	{Lower: math.Inf(-1), Upper: -0.25, Rate: 2.00},
}

func (s Service) ComputeDCAAmount(ctx context.Context, opts ComputeDCAAmountOptions) (DCAAmountPlan, error) {
	mode := opts.Mode
	if mode != "change_pct" {
		mode = "nav_deviation"
	}
	baseAmount := opts.BaseAmount
	if !isFinitePositive(baseAmount) {
		baseAmount = 30
	}

	portfolioID := opts.PortfolioID
	portfolioID = clampPortfolioID(portfolioID)
	position, err := s.queryDCAPositionFacts(ctx, opts.Code, portfolioID)
	if err != nil {
		return DCAAmountPlan{}, err
	}
	if position == nil || position.HeldShares < 0.001 {
		return DCAAmountPlan{
			FundCode:         opts.Code,
			DecisionBoundary: "facts_only",
			SideEffects:      "none",
			Error:            "no_position",
			Message:          "该证券无持仓，无法计算偏离率；仅返回基础金额作为模拟参考。",
		}, nil
	}

	latestNAV := 0.0
	if position.LatestNAV.Valid {
		latestNAV = position.LatestNAV.Float64
	}
	costPerShare := (*float64)(nil)
	if position.TotalCost.Valid && position.HeldShares > 0 {
		value := round4(math.Abs(position.TotalCost.Float64) / position.HeldShares)
		if value > 0 {
			costPerShare = &value
		}
	}
	changePct, err := s.queryLatestChangePct(ctx, opts.Code)
	if err != nil {
		return DCAAmountPlan{}, err
	}
	if latestNAV <= 0 || (mode == "nav_deviation" && (costPerShare == nil || *costPerShare <= 0)) {
		return DCAAmountPlan{
			FundCode:         opts.Code,
			DecisionBoundary: "facts_only",
			SideEffects:      "none",
			Error:            "insufficient_data",
			LatestNAV:        round4(latestNAV),
			CostPerShare:     costPerShare,
		}, nil
	}

	plan := computeDCAPlanFacts(mode, baseAmount, latestNAV, costPerShare, changePct)
	plan.FundCode = opts.Code
	plan.SecurityType = position.SecurityType
	plan.Market = position.Market
	plan.DecisionBoundary = "facts_only"
	plan.SideEffects = "none"
	return plan, nil
}

func (s Service) queryDCAPositionFacts(ctx context.Context, code string, portfolioID int) (*dcaPositionFacts, error) {
	var facts dcaPositionFacts
	var securityType sql.NullString
	var market sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT
			ps.held_shares,
			ps.total_cost,
			ps.latest_nav,
			fd.security_type,
			fd.market
		FROM portfolio_snapshot ps
		LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
		WHERE ps.fund_code = ?
		  AND COALESCE(ps.portfolio_id, 1) = ?
		ORDER BY ps.held_shares DESC
		LIMIT 1
	`, code, portfolioID).Scan(&facts.HeldShares, &facts.TotalCost, &facts.LatestNAV, &securityType, &market)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query dca position: %w", err)
	}
	facts.SecurityType = clampPortfolioText(nullableStringValue(securityType, "fund"), 32)
	facts.Market = clampPortfolioText(nullableStringValue(market, ""), 32)
	return &facts, nil
}

func (s Service) queryLatestChangePct(ctx context.Context, code string) (*float64, error) {
	var change sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
		SELECT daily_change_pct
		FROM nav_history
		WHERE fund_code = ?
		ORDER BY date DESC
		LIMIT 1
	`, code).Scan(&change)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query latest change pct: %w", err)
	}
	if !change.Valid || !isFinite(change.Float64) {
		return nil, nil
	}
	value := round2(change.Float64)
	return &value, nil
}

func computeDCAPlanFacts(mode string, baseAmount float64, latestNAV float64, costPerShare *float64, changePct *float64) DCAAmountPlan {
	rate := 1.0
	deviation := (*float64)(nil)
	if mode == "change_pct" {
		change := 0.0
		if changePct != nil {
			change = *changePct
		}
		rate = computeChangePctDCARate(change)
	} else if costPerShare != nil && *costPerShare > 0 && latestNAV > 0 {
		value := round2((latestNAV - *costPerShare) / *costPerShare * 100)
		deviation = &value
		rate = computeDeviationDCARate(value / 100)
	}
	actualAmount := round2(baseAmount * rate)
	signal := dcaRateSignal(mode, rate)
	// Locale-neutral explanation templates for MCP/API; SPA maps signal codes to i18n (#208).
	explanation := fmt.Sprintf("deviation=%.2f%% signal=%s amount=%.2f", 0.0, signal, actualAmount)
	if mode == "change_pct" {
		change := 0.0
		if changePct != nil {
			change = *changePct
		}
		explanation = fmt.Sprintf("change_pct=%.2f%% signal=%s amount=%.2f", change, signal, actualAmount)
	} else if deviation != nil {
		explanation = fmt.Sprintf("deviation=%.2f%% signal=%s amount=%.2f", *deviation, signal, actualAmount)
	}
	return DCAAmountPlan{
		Mode:         mode,
		BaseAmount:   round2(baseAmount),
		LatestNAV:    round4(latestNAV),
		CostPerShare: costPerShare,
		ChangePct:    changePct,
		DeviationPct: deviation,
		DCARate:      rate,
		ActualAmount: actualAmount,
		Signal:       signal,
		Range:        signal,
		Explanation:  explanation,
	}
}

func computeDeviationDCARate(deviation float64) float64 {
	for _, row := range dcaRateTable {
		if deviation >= row.Lower && deviation < row.Upper {
			return row.Rate
		}
	}
	return 1
}

func computeChangePctDCARate(changePct float64) float64 {
	if changePct <= -8 {
		return 2.0
	}
	if changePct <= -5 {
		return 1.6
	}
	if changePct <= -3 {
		return 1.35
	}
	if changePct <= -1 {
		return 1.15
	}
	if changePct < 1 {
		return 1.0
	}
	if changePct < 3 {
		return 0.85
	}
	if changePct < 5 {
		return 0.65
	}
	return 0.5
}

// dcaRateSignal returns locale-neutral signal codes for SPA i18n (#201).
// Codes: increase | decrease | normal | dip_buy | rally_control | range_dca
func dcaRateSignal(mode string, rate float64) string {
	if mode == "change_pct" {
		if rate > 1 {
			return "dip_buy"
		}
		if rate < 1 {
			return "rally_control"
		}
		return "range_dca"
	}
	if rate > 1 {
		return "increase"
	}
	if rate < 1 {
		return "decrease"
	}
	return "normal"
}

func isFinitePositive(value float64) bool {
	return value > 0 && isFinite(value)
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
