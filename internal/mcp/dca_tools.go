package mcp

import (
	"context"

	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

func (s *Server) callUpsertDCAPlan(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	in := portfoliosvc.UpsertDCAPlanInput{
		ID:          intArg(args, "id", 0),
		FundCode:    stringArg(args, "fund_code"),
		FundName:    stringArg(args, "fund_name"),
		Amount:      floatArgMax(args, "amount", 0, 1_000_000),
		Frequency:   stringArg(args, "frequency"),
		WeekdayMask: stringArg(args, "weekday_mask"),
		TradeType:   stringArg(args, "trade_type"),
		PortfolioID: intArgMax(args, "portfolio_id", 1, 1000),
		StartDate:   stringArg(args, "start_date"),
		EndDate:     stringArg(args, "end_date"),
		Source:      stringArg(args, "source"),
	}
	if raw, ok := args["active"]; ok {
		active, valid := integerFlag(raw)
		if !valid || (active != 0 && active != 1) {
			return nil, jsonrpcError(-32602, "invalid_params: active must be 0 or 1")
		}
		in.Active = &active
	}
	res, err := s.portfolio.UpsertDCAPlan(ctx, in)
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"ok":                res.OK,
		"plan":              res.Plan,
		"decision_boundary": "facts_only",
		"side_effects":      "dca_plan_upsert",
	})
}

func (s *Server) callDisableDCAPlan(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	id := intArg(args, "id", 0)
	if id <= 0 {
		id = intArg(args, "plan_id", 0)
	}
	res, err := s.portfolio.DisableDCAPlan(ctx, id)
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"ok":                res.OK,
		"id":                res.ID,
		"updated":           res.Updated,
		"decision_boundary": "facts_only",
		"side_effects":      "dca_plan_disable",
	})
}
