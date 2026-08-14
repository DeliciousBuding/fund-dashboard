package portfolio

type FundDetail struct {
	Code             string
	Name             *string
	Type             *string
	SecurityType     string
	Market           string
	Position         FundDetailPosition
	NAVCount         int
	LastNAVDate      *string
	TransactionCount int
	Transactions     []FundTransaction
}

type FundDetailPosition struct {
	Shares        float64
	CostBasis     *float64
	LatestNAV     *float64
	MarketValue   *float64
	UnrealizedPNL *float64
	PNLPct        *float64
}

type FundTransaction struct {
	Seq            int
	Time           *string
	ConfirmDate    *string
	Direction      *string
	Type           *string
	Amount         *float64
	Shares         *float64
	Fee            *float64
	SettlementDays *int
	OrderID        *string
}

type NavHistoryReport struct {
	Code             string            `json:"code"`
	SecurityType     string            `json:"security_type"`
	Market           string            `json:"market"`
	Data             []NavHistoryPoint `json:"data"`
	DecisionBoundary string            `json:"decision_boundary"`
}

type NavHistoryPoint struct {
	Date           string   `json:"date"`
	UnitNAV        float64  `json:"unit_nav"`
	DailyChangePct *float64 `json:"daily_change_pct"`
	SecurityType   string   `json:"security_type"`
}

type SearchResult struct {
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	SecurityType  string   `json:"security_type"`
	Market        string   `json:"market"`
	HeldShares    float64  `json:"held_shares"`
	CurrentValue  *float64 `json:"current_value"`
	UnrealizedPNL *float64 `json:"unrealized_pnl"`
	PNLPct        *float64 `json:"pnl_pct"`
}

type DrawdownReport struct {
	Code             string  `json:"code"`
	SecurityType     string  `json:"security_type"`
	Market           string  `json:"market"`
	MaxDrawdownPct   float64 `json:"max_drawdown_pct"`
	PeakDate         string  `json:"peak_date"`
	TroughDate       string  `json:"trough_date"`
	DecisionBoundary string  `json:"decision_boundary"`
}

type XIRRReport struct {
	Code             string   `json:"code"`
	SecurityType     string   `json:"security_type"`
	Market           string   `json:"market"`
	XIRRPct          *float64 `json:"xirr_pct"`
	Message          *string  `json:"message"`
	DecisionBoundary string   `json:"decision_boundary"`
}

type PortfolioXIRRReport struct {
	PortfolioID           int      `json:"portfolio_id"`
	XIRRPct               *float64 `json:"xirr_pct"`
	CurrentPortfolioValue float64  `json:"current_portfolio_value"`
	Message               *string  `json:"message"`
	DecisionBoundary      string   `json:"decision_boundary"`
}

type fundIdentity struct {
	Name         *string
	Type         *string
	SecurityType string
	Market       string
}
