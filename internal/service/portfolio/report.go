package portfolio

import (
	"context"
	"fmt"
	"time"
)

type GenerateReportInput struct {
	PortfolioID  int
	SourceLimit  int
	EventLimit   int
	BaseCurrency string
	Title        string
	AsOf         string
}

type GenerateReportResult struct {
	OK               bool           `json:"ok"`
	ReportID         string         `json:"report_id"`
	Title            string         `json:"title"`
	AsOf             string         `json:"as_of"`
	GeneratedAt      string         `json:"generated_at"`
	Format           string         `json:"format"`
	PortfolioID      int            `json:"portfolio_id"`
	Sections         map[string]any `json:"sections"`
	DecisionBoundary string         `json:"decision_boundary"`
	SideEffects      string         `json:"side_effects"`
	Artifact         string         `json:"artifact"` // "json" — no PDF binary in v1
}

// GenerateReport assembles a facts-only portfolio report from existing read services.
// v1 returns structured JSON only (no PDF/binary artifact).
func (s Service) GenerateReport(ctx context.Context, in GenerateReportInput) (GenerateReportResult, error) {
	portfolioID := in.PortfolioID
	portfolioID = clampPortfolioID(portfolioID)
	sourceLimit := in.SourceLimit
	if sourceLimit <= 0 {
		sourceLimit = 8
	}
	eventLimit := in.EventLimit
	if eventLimit <= 0 {
		eventLimit = 20
	}
	asOf := in.AsOf
	if asOf == "" {
		asOf = time.Now().Format("2006-01-02")
	}
	title := in.Title
	if title == "" {
		title = fmt.Sprintf("Portfolio report %s", asOf)
	}
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	reportID := fmt.Sprintf("rpt-%d-%s", portfolioID, time.Now().UTC().Format("20060102T150405Z"))

	summary, err := s.GetSummary(ctx, portfolioID)
	if err != nil {
		return GenerateReportResult{}, fmt.Errorf("summary: %w", err)
	}
	allocation, err := s.GetAllocation(ctx, portfolioID)
	if err != nil {
		return GenerateReportResult{}, fmt.Errorf("allocation: %w", err)
	}
	// generate_report is operator-only; include full harness discovery surface.
	harness, err := s.GetHarnessSnapshotFor(ctx, portfolioID, HarnessAudienceOperator)
	if err != nil {
		return GenerateReportResult{}, fmt.Errorf("harness: %w", err)
	}
	dca, err := s.ListDCAPlans(ctx, ListDCAPlansOptions{ActiveOnly: true, PortfolioID: portfolioID})
	if err != nil {
		return GenerateReportResult{}, fmt.Errorf("dca plans: %w", err)
	}
	sourceBrief, err := s.GetInvestmentSourceBrief(ctx, SourceBriefOptions{
		PortfolioID: portfolioID,
		Limit:       sourceLimit,
	})
	if err != nil {
		return GenerateReportResult{}, fmt.Errorf("source brief: %w", err)
	}
	events, err := s.GetSourceEvents(ctx, GetSourceEventsOptions{
		Limit:    eventLimit,
		ShowRead: true,
	})
	if err != nil {
		return GenerateReportResult{}, fmt.Errorf("source events: %w", err)
	}

	// portfolio XIRR best-effort
	var xirr any
	if x, err := s.GetPortfolioXIRR(ctx, portfolioID); err == nil {
		xirr = x
	}

	sections := map[string]any{
		"summary":       summary,
		"allocation":    allocation,
		"harness":       harness,
		"dca_plans":     dca,
		"source_brief":  sourceBrief,
		"source_events": events,
	}
	if xirr != nil {
		sections["portfolio_xirr"] = xirr
	}

	return GenerateReportResult{
		OK:               true,
		ReportID:         reportID,
		Title:            title,
		AsOf:             asOf,
		GeneratedAt:      generatedAt,
		Format:           "json",
		PortfolioID:      portfolioID,
		Sections:         sections,
		DecisionBoundary: "facts_only",
		SideEffects:      "none",
		Artifact:         "json",
	}, nil
}
