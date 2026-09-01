package mcp

import (
	"context"
	"encoding/json"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

func (s *Server) callAdjustPosition(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	code := firstNonEmpty(stringArg(args, "fund_code"), stringArg(args, "code"))
	shares, ok := floatArgPresent(args, "shares")
	if !ok {
		shares, ok = floatArgPresent(args, "held_shares")
	}
	if !ok {
		return nil, jsonrpcError(-32602, "invalid_params: shares is required")
	}
	if shares < 0 {
		return nil, jsonrpcError(-32602, "invalid_params: shares must be non-negative")
	}
	res, err := s.portfolio.AdjustPosition(ctx, portfoliosvc.AdjustPositionInput{
		Code:        code,
		Shares:      shares,
		PortfolioID: intArgMax(args, "portfolio_id", 1, 1000),
		Reason:      stringArg(args, "reason"),
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"ok":                res.OK,
		"code":              res.Code,
		"shares":            res.Shares,
		"security":          res.Security,
		"portfolio_id":      res.PortfolioID,
		"reason":            res.Reason,
		"decision_boundary": "facts_only",
		"side_effects":      "position_override",
	})
}

func (s *Server) callCheckAlerts(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	if s.admin == nil {
		return nil, jsonrpcError(-32000, "tool_error: admin service is required")
	}
	res, err := s.admin.CheckAlerts(ctx, adminsvc.CheckAlertsInput{
		PriceChangePct: floatArgMax(args, "price_change_pct", 0, 100),
		DrawdownPct:    floatArgMax(args, "drawdown_pct", 0, 100),
		StaleDays:      intArgMax(args, "stale_days", 0, 3650),
		PortfolioID:    intArgMax(args, "portfolio_id", 1, 1000),
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"ok":                res.OK,
		"count":             res.Count,
		"alerts":            res.Alerts,
		"checked_at":        res.CheckedAt,
		"price_change_pct":  res.PriceChangePct,
		"drawdown_pct":      res.DrawdownPct,
		"stale_days":        res.StaleDays,
		"portfolio_id":      res.PortfolioID,
		"decision_boundary": res.DecisionBoundary,
		"side_effects":      res.SideEffects,
		"webhook_sent":      res.WebhookSent,
	})
}

func floatArgPresent(args map[string]any, key string) (float64, bool) {
	value, ok := args[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func (s *Server) callRunDCAAutoInvest(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	// dry_run defaults true for safety unless explicitly false
	dryRun := true
	if v, ok := args["dry_run"]; ok {
		if b, ok := v.(bool); ok {
			dryRun = b
		}
	}
	res, err := s.portfolio.RunDCAAutoInvest(ctx, portfoliosvc.RunDCAAutoInvestInput{
		AsOf:        stringArg(args, "as_of"),
		PortfolioID: intArgMax(args, "portfolio_id", 1, 1000),
		PlanID:      intArg(args, "plan_id", 0),
		FundCode:    firstNonEmpty(stringArg(args, "fund_code"), stringArg(args, "code")),
		DryRun:      dryRun,
		Mode:        stringArg(args, "mode"),
		BaseAmount:  floatArgMax(args, "base_amount", 0, 1_000_000),
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"ok":                res.OK,
		"as_of":             res.AsOf,
		"dry_run":           res.DryRun,
		"executed":          res.Executed,
		"skipped":           res.Skipped,
		"previewed":         res.Previewed,
		"items":             res.Items,
		"decision_boundary": res.DecisionBoundary,
		"side_effects":      res.SideEffects,
	})
}

func (s *Server) callGenerateReport(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	res, err := s.portfolio.GenerateReport(ctx, portfoliosvc.GenerateReportInput{
		PortfolioID:  intArgMax(args, "portfolio_id", 1, 1000),
		SourceLimit:  intArgMax(args, "source_limit", 8, 50),
		EventLimit:   intArgMax(args, "event_limit", 20, 100),
		BaseCurrency: stringArg(args, "base_currency"),
		Title:        stringArg(args, "title"),
		AsOf:         stringArg(args, "as_of"),
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"ok":                res.OK,
		"report_id":         res.ReportID,
		"title":             res.Title,
		"as_of":             res.AsOf,
		"generated_at":      res.GeneratedAt,
		"format":            res.Format,
		"portfolio_id":      res.PortfolioID,
		"sections":          res.Sections,
		"decision_boundary": res.DecisionBoundary,
		"side_effects":      res.SideEffects,
		"artifact":          res.Artifact,
	})
}
