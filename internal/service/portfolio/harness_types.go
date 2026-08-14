package portfolio

import "database/sql"

type HarnessSnapshot struct {
	GeneratedAt             string                   `json:"generated_at"`
	DecisionBoundary        string                   `json:"decision_boundary"`
	TotalValue              float64                  `json:"total_value"`
	HoldingsCount           int                      `json:"holdings_count"`
	Allocation              *Allocation              `json:"allocation"`
	HoldingSignals          []HarnessHoldingSignal   `json:"holding_signals"`
	DataQuality             HarnessDataQuality       `json:"data_quality"`
	AvailableAgentTools     []string                 `json:"available_agent_tools"`
	AgentPermissions        AgentPermissions         `json:"agent_permissions"`
	AgentCapabilities       []AgentCapability        `json:"agent_capabilities"`
	RecommendedAgentActions []RecommendedAgentAction `json:"recommended_agent_actions"`
	AgentBrief              string                   `json:"agent_brief"`
}

type HarnessHoldingSignal struct {
	Code         string          `json:"code"`
	Name         string          `json:"name"`
	SecurityType string          `json:"security_type"`
	Market       string          `json:"market"`
	HeldShares   float64         `json:"held_shares"`
	CurrentValue float64         `json:"current_value"`
	WeightPct    float64         `json:"weight_pct"`
	LatestNAV    float64         `json:"latest_nav"`
	CostPerShare *float64        `json:"cost_per_share"`
	ChangePct    *float64        `json:"change_pct"`
	DeviationPct *float64        `json:"deviation_pct"`
	// PNLPct is snapshot unrealized PnL % (distinct from DeviationPct = NAV vs cost/share).
	PNLPct     *float64        `json:"pnl_pct,omitempty"`
	SignalTags []string        `json:"signal_tags"`
	DataPoints SignalDataPoint `json:"data_points"`
}

type SignalDataPoint struct {
	HasPrice     bool `json:"has_price"`
	HasCostBasis bool `json:"has_cost_basis"`
	HasChangePct bool `json:"has_change_pct"`
}

type HarnessDataQuality struct {
	StalePriceCount       int     `json:"stale_price_count"`
	MissingCostBasisCount int     `json:"missing_cost_basis_count"`
	MissingChangePctCount int     `json:"missing_change_pct_count"`
	HoldingsCoveragePct   float64 `json:"holdings_coverage_pct"`
}

type AgentPermissions struct {
	DecisionBoundary     string   `json:"decision_boundary"`
	ReadScope            []string `json:"read_scope"`
	WriteScope           []string `json:"write_scope"`
	RequiresConfirmation []string `json:"requires_confirmation"`
	DisabledOperations   []string `json:"disabled_operations"`
}

type AgentCapability struct {
	Tool       string `json:"tool"`
	Scope      string `json:"scope"`
	Permission string `json:"permission"`
	RiskLevel  string `json:"risk_level"`
	UseFor     string `json:"use_for"`
}

type RecommendedAgentAction struct {
	Priority string         `json:"priority"`
	Tool     string         `json:"tool"`
	Reason   string         `json:"reason"`
	Input    map[string]any `json:"input,omitempty"`
}

type harnessHoldingRow struct {
	Code           string
	Name           string
	HeldShares     float64
	TotalCost      float64
	LatestNAV      sql.NullFloat64
	CurrentValue   sql.NullFloat64
	SecurityType   string
	Market         string
	DailyChangePct sql.NullFloat64
	PNLPct         sql.NullFloat64
}
