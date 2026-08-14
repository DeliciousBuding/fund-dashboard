package mcp

import (
	"context"

	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

func (s *Server) callPortfolioTimeline(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	timeline, err := s.portfolio.GetTimeline(ctx, intArgMax(args, "portfolio_id", 1, 1000))
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	payload := map[string]any{
		"count":             len(timeline),
		"decision_boundary": "facts_only",
		"data":              timeline,
	}
	if len(timeline) > 0 {
		payload["first"] = timeline[0].Date
		payload["last"] = timeline[len(timeline)-1].Date
	}
	return textJSONResult(payload)
}

func (s *Server) callPortfolioPenetration(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	report, err := s.portfolio.GetPenetration(ctx, portfoliosvc.PenetrationOptions{
		PortfolioID: intArgMax(args, "portfolio_id", 1, 1000),
		Limit:       intArgMax(args, "limit", 30, 200),
		SortBy:      stringArg(args, "sort_by"),
	})
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(report)
}
