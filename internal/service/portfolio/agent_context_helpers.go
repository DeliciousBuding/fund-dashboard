package portfolio

import (
	"context"
	"fmt"
	"math"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
)

func registryAgentCapabilities() []AgentCapability {
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		return append([]AgentCapability(nil), agentCapabilities...)
	}
	capabilities := make([]AgentCapability, 0, len(registry.Tools))
	for _, tool := range registry.Tools {
		capabilities = append(capabilities, AgentCapability{
			Tool:       tool.Capability.Tool,
			Scope:      string(tool.Capability.Scope),
			Permission: string(tool.Capability.Permission),
			RiskLevel:  string(tool.Capability.RiskLevel),
			UseFor:     tool.Capability.UseFor,
		})
	}
	return capabilities
}

// agentContextDiscovery selects capability/permission surfaces by audience.
// Public HTTP must not leak write/maintenance/confirmation tool names (#65); operator
// additionally loses anything this deployment cannot execute (harness_availability.go).
func agentContextDiscovery(audience HarnessAudience, confirmationsAvailable bool) ([]AgentCapability, AgentPermissions) {
	if audience == HarnessAudienceOperator {
		return capabilitiesWithConfirmationAvailability(registryAgentCapabilities(), confirmationsAvailable),
			permissionsWithConfirmationAvailability(defaultAgentPermissions(), confirmationsAvailable)
	}
	return publicAgentCapabilities(), publicAgentPermissions()
}

func buildAgentContextBrief(portfolioID, holdingsCount, qualityScore int, qualityLevel string, unread int, audience HarnessAudience, confirmationsAvailable bool) string {
	boundary := "Permissions and disabled operations are embedded; backup producer remains disabled."
	switch {
	case audience != HarnessAudienceOperator:
		boundary = "Public discovery is read-only; write/maintenance/confirmation-gated tools are not advertised; backup producer remains disabled."
	case !confirmationsAvailable:
		boundary = "Permissions and disabled operations are embedded; confirmation-gated writes are unavailable because the AgentOps confirmation service is not wired; backup producer remains disabled."
	}
	return fmt.Sprintf(
		"AgentContextPack facts only: portfolio %d has %d holdings, quality %d/%s, %d unread source events. %s",
		portfolioID,
		holdingsCount,
		qualityScore,
		qualityLevel,
		unread,
		boundary,
	)
}

func agentContextSourceEvents(events []SourceEvent) []AgentContextSourceEvent {
	out := make([]AgentContextSourceEvent, 0, len(events))
	for _, event := range events {
		isRead := event.IsRead != 0
		isUseful := event.IsUseful != 0
		out = append(out, AgentContextSourceEvent{
			ID:                  event.ID,
			Title:               event.Title,
			URL:                 event.URL,
			Source:              event.Source,
			Snippet:             event.Snippet,
			Query:               event.Query,
			RelatedSecurityCode: event.RelatedSecurityCode,
			RelatedSecurityName: event.RelatedSecurityName,
			IsRead:              &isRead,
			IsUseful:            &isUseful,
			FetchedAt:           event.FetchedAt,
		})
	}
	return out
}

func agentContextHoldings(signals []HarnessHoldingSignal) []AgentContextHolding {
	out := make([]AgentContextHolding, 0, len(signals))
	for _, signal := range signals {
		out = append(out, AgentContextHolding{
			Code:         signal.Code,
			Name:         signal.Name,
			SecurityType: signal.SecurityType,
			Market:       signal.Market,
			HeldShares:   signal.HeldShares,
			CurrentValue: signal.CurrentValue,
			WeightPct:    signal.WeightPct,
			LatestPrice:  signal.LatestNAV,
			CostPerShare: signal.CostPerShare,
			ChangePct:    signal.ChangePct,
			DeviationPct: signal.DeviationPct,
			SignalTags:   append([]string(nil), signal.SignalTags...),
			DataPoints:   signal.DataPoints,
		})
	}
	return out
}

func buildAgentContextDataQuality(input HarnessDataQuality) AgentContextDataQuality {
	score := 100
	score -= input.StalePriceCount * 12
	score -= input.MissingCostBasisCount * 10
	score -= input.MissingChangePctCount * 6
	if input.HoldingsCoveragePct < 100 {
		score -= int(math.Round((100 - input.HoldingsCoveragePct) * 0.4))
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	limitations := []string{}
	if input.StalePriceCount > 0 {
		limitations = append(limitations, fmt.Sprintf("price: %d missing_or_stale", input.StalePriceCount))
	}
	if input.MissingCostBasisCount > 0 {
		limitations = append(limitations, fmt.Sprintf("cost_basis: %d missing", input.MissingCostBasisCount))
	}
	if input.MissingChangePctCount > 0 {
		limitations = append(limitations, fmt.Sprintf("change_pct: %d missing", input.MissingChangePctCount))
	}
	if input.HoldingsCoveragePct < 100 {
		limitations = append(limitations, fmt.Sprintf("holdings_coverage: %.1f%%", input.HoldingsCoveragePct))
	}

	return AgentContextDataQuality{
		OverallScore:          score,
		Level:                 qualityLevel(score),
		StalePriceCount:       input.StalePriceCount,
		MissingCostBasisCount: input.MissingCostBasisCount,
		MissingChangePctCount: input.MissingChangePctCount,
		HoldingsCoveragePct:   input.HoldingsCoveragePct,
		Limitations:           limitations,
	}
}

func qualityLevel(score int) string {
	switch {
	case score >= 85:
		return "good"
	case score >= 70:
		return "usable"
	case score >= 55:
		return "limited"
	default:
		return "poor"
	}
}

func (s Service) sourceEventsSummary(ctx context.Context) (SourceEventsSummary, error) {
	var summary SourceEventsSummary
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN COALESCE(is_read, 0) = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN COALESCE(is_useful, 0) = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN COALESCE(is_read, 0) = 1 AND COALESCE(is_useful, 0) = 0 THEN 1 ELSE 0 END), 0)
		FROM source_events
	`).Scan(&summary.Total, &summary.Unread, &summary.Useful, &summary.Ignored); err != nil {
		return SourceEventsSummary{}, fmt.Errorf("query source events summary: %w", err)
	}
	return summary, nil
}
