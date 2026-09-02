package portfolio

import (
	"context"
	"strings"
	"time"
)

const agentContextSchemaVersion = "agent-context-pack-v1"

type AgentContextOptions struct {
	PortfolioID  int
	SourceLimit  int
	EventLimit   int
	BaseCurrency string
}

type AgentContextPack struct {
	SchemaVersion    string                    `json:"schema_version"`
	GeneratedAt      string                    `json:"generated_at"`
	DecisionBoundary string                    `json:"decision_boundary"`
	Identity         AgentContextIdentity      `json:"identity"`
	Portfolio        AgentContextPortfolio     `json:"portfolio"`
	Holdings         []AgentContextHolding     `json:"holdings"`
	DataQuality      AgentContextDataQuality   `json:"data_quality"`
	SourceContext    AgentContextSourceContext `json:"source_context"`
	Permissions      AgentPermissions          `json:"permissions"`
	Capabilities     []AgentCapability         `json:"capabilities"`
	Maintenance      AgentContextMaintenance   `json:"maintenance"`
	AgentBrief       string                    `json:"agent_brief"`
	PublicProjection *AgentContextProjection   `json:"public_projection,omitempty"`
}

type AgentContextIdentity struct {
	PortfolioID   int     `json:"portfolio_id"`
	PortfolioName *string `json:"portfolio_name"`
	BaseCurrency  string  `json:"base_currency"`
	DataVersion   string  `json:"data_version"`
}

type AgentContextPortfolio struct {
	Summary    *Summary    `json:"summary"`
	Allocation *Allocation `json:"allocation"`
	RiskFlags  []string    `json:"risk_flags"`
}

type AgentContextHolding struct {
	Code         string          `json:"code"`
	Name         string          `json:"name"`
	SecurityType string          `json:"security_type"`
	Market       string          `json:"market"`
	HeldShares   float64         `json:"held_shares"`
	CurrentValue float64         `json:"current_value"`
	WeightPct    float64         `json:"weight_pct"`
	LatestPrice  float64         `json:"latest_price"`
	CostPerShare *float64        `json:"cost_per_share"`
	ChangePct    *float64        `json:"change_pct"`
	DeviationPct *float64        `json:"deviation_pct"`
	SignalTags   []string        `json:"signal_tags"`
	DataPoints   SignalDataPoint `json:"data_points"`
}

type AgentContextDataQuality struct {
	OverallScore          int      `json:"overall_score"`
	Level                 string   `json:"level"`
	StalePriceCount       int      `json:"stale_price_count"`
	MissingCostBasisCount int      `json:"missing_cost_basis_count"`
	MissingChangePctCount int      `json:"missing_change_pct_count"`
	HoldingsCoveragePct   float64  `json:"holdings_coverage_pct"`
	IntegrityStatus       *string  `json:"integrity_status"`
	Limitations           []string `json:"limitations"`
}

type AgentContextSourceContext struct {
	Queries             []InvestmentSourceQuery   `json:"queries"`
	Targets             []InvestmentSourceTarget  `json:"targets"`
	StoredEventsSummary SourceEventsSummary       `json:"stored_events_summary"`
	RecentEvents        []AgentContextSourceEvent `json:"recent_events"`
}

type SourceEventsSummary struct {
	Total   int `json:"total"`
	Unread  int `json:"unread"`
	Useful  int `json:"useful"`
	Ignored int `json:"ignored"`
}

type AgentContextSourceEvent struct {
	ID                  int64   `json:"id"`
	Title               string  `json:"title"`
	URL                 *string `json:"url"`
	Source              string  `json:"source"`
	Snippet             *string `json:"snippet"`
	Query               *string `json:"query"`
	RelatedSecurityCode *string `json:"related_security_code"`
	RelatedSecurityName *string `json:"related_security_name"`
	IsRead              *bool   `json:"is_read"`
	IsUseful            *bool   `json:"is_useful"`
	FetchedAt           string  `json:"fetched_at"`
}

type AgentContextMaintenance struct {
	RecommendedActions []RecommendedAgentAction `json:"recommended_actions"`
}

type AgentContextProjection struct {
	PortfolioID        int      `json:"portfolio_id"`
	BaseCurrency       string   `json:"base_currency"`
	TotalValue         float64  `json:"total_value"`
	HoldingsCount      int      `json:"holdings_count"`
	QualityScore       int      `json:"quality_score"`
	QualityLevel       string   `json:"quality_level"`
	SourceEventsUnread int      `json:"source_events_unread"`
	DisabledOperations []string `json:"disabled_operations"`
}

func (s Service) GetAgentContextPack(ctx context.Context, options AgentContextOptions) (*AgentContextPack, error) {
	// Public HTTP agent-context is unauthenticated → least-privilege discovery.
	return s.GetAgentContextPackFor(ctx, options, HarnessAudiencePublic)
}

func (s Service) GetAgentContextPackFor(ctx context.Context, options AgentContextOptions, audience HarnessAudience) (*AgentContextPack, error) {
	portfolioID := options.PortfolioID
	portfolioID = clampPortfolioID(portfolioID)
	baseCurrency := strings.ToUpper(strings.TrimSpace(options.BaseCurrency))
	if baseCurrency == "" || len(baseCurrency) < 3 || len(baseCurrency) > 8 {
		baseCurrency = "CNY"
	}
	if audience == "" {
		audience = HarnessAudiencePublic
	}

	summary, err := s.GetSummary(ctx, portfolioID)
	if err != nil {
		return nil, err
	}
	harness, err := s.GetHarnessSnapshotFor(ctx, portfolioID, audience)
	if err != nil {
		return nil, err
	}
	sourceBrief, err := s.GetInvestmentSourceBrief(ctx, SourceBriefOptions{
		PortfolioID: portfolioID,
		Limit:       options.SourceLimit,
	})
	if err != nil {
		return nil, err
	}
	eventSummary, err := s.sourceEventsSummary(ctx)
	if err != nil {
		return nil, err
	}
	recentEvents, err := s.GetSourceEvents(ctx, GetSourceEventsOptions{
		Limit:    options.EventLimit,
		ShowRead: true,
	})
	if err != nil {
		return nil, err
	}

	quality := buildAgentContextDataQuality(harness.DataQuality)
	holdings := agentContextHoldings(harness.HoldingSignals)
	capabilities, permissions := agentContextDiscovery(audience, s.confirmationsAvailable)
	dataVersion := "no-nav"
	if summary.LastNAVDate != nil && *summary.LastNAVDate != "" {
		dataVersion = "nav:" + *summary.LastNAVDate
	}
	generatedAt := time.Now().UTC().Format(time.RFC3339Nano)

	pack := &AgentContextPack{
		SchemaVersion:    agentContextSchemaVersion,
		GeneratedAt:      generatedAt,
		DecisionBoundary: "facts_only",
		Identity: AgentContextIdentity{
			PortfolioID:  portfolioID,
			BaseCurrency: baseCurrency,
			DataVersion:  dataVersion,
		},
		Portfolio: AgentContextPortfolio{
			Summary:    summary,
			Allocation: harness.Allocation,
			RiskFlags:  append([]string(nil), harness.Allocation.RiskFlags...),
		},
		Holdings:    holdings,
		DataQuality: quality,
		SourceContext: AgentContextSourceContext{
			Queries:             append([]InvestmentSourceQuery(nil), sourceBrief.Queries...),
			Targets:             append([]InvestmentSourceTarget(nil), sourceBrief.SourceTargets...),
			StoredEventsSummary: eventSummary,
			RecentEvents:        agentContextSourceEvents(recentEvents),
		},
		Permissions:  permissions,
		Capabilities: capabilities,
		Maintenance: AgentContextMaintenance{
			RecommendedActions: append([]RecommendedAgentAction(nil), harness.RecommendedAgentActions...),
		},
		AgentBrief: buildAgentContextBrief(portfolioID, len(holdings), quality.OverallScore, quality.Level, eventSummary.Unread, audience, s.confirmationsAvailable),
	}
	pack.PublicProjection = &AgentContextProjection{
		PortfolioID:        portfolioID,
		BaseCurrency:       baseCurrency,
		TotalValue:         harness.TotalValue,
		HoldingsCount:      len(holdings),
		QualityScore:       quality.OverallScore,
		QualityLevel:       quality.Level,
		SourceEventsUnread: eventSummary.Unread,
		DisabledOperations: append([]string(nil), permissions.DisabledOperations...),
	}
	return pack, nil
}
