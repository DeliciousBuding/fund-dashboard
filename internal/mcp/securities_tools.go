package mcp

import (
	"context"

	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

func (s *Server) callAddFund(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	code := firstNonEmpty(stringArg(args, "fund_code"), stringArg(args, "code"))
	name := firstNonEmpty(stringArg(args, "fund_name"), stringArg(args, "name"))
	res, err := s.portfolio.UpsertSecurity(ctx, portfoliosvc.UpsertSecurityInput{
		Code:         code,
		Name:         name,
		FundType:     stringArg(args, "fund_type"),
		SecurityType: "fund",
		Market:       firstNonEmpty(stringArg(args, "market"), "CN"),
		Currency:     firstNonEmpty(stringArg(args, "currency"), "CNY"),
		Exchange:     stringArg(args, "exchange"),
		Source:       firstNonEmpty(stringArg(args, "source"), "mcp"),
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"ok":                res.OK,
		"created":           res.Created,
		"security":          res.Security,
		"decision_boundary": "facts_only",
		"side_effects":      "security_master_upsert",
	})
}

func (s *Server) callAddSecurity(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	code := firstNonEmpty(stringArg(args, "code"), stringArg(args, "fund_code"))
	name := firstNonEmpty(stringArg(args, "name"), stringArg(args, "fund_name"))
	secType := firstNonEmpty(stringArg(args, "security_type"), "fund")
	res, err := s.portfolio.UpsertSecurity(ctx, portfoliosvc.UpsertSecurityInput{
		Code:         code,
		Name:         name,
		FundType:     stringArg(args, "fund_type"),
		SecurityType: secType,
		Market:       stringArg(args, "market"),
		Currency:     stringArg(args, "currency"),
		Exchange:     stringArg(args, "exchange"),
		Source:       firstNonEmpty(stringArg(args, "source"), "mcp"),
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"ok":                res.OK,
		"created":           res.Created,
		"security":          res.Security,
		"decision_boundary": "facts_only",
		"side_effects":      "security_master_upsert",
	})
}

func (s *Server) callUpdateFund(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	code := firstNonEmpty(stringArg(args, "fund_code"), stringArg(args, "code"))
	// partial update via UpsertSecurity (exists path)
	res, err := s.portfolio.UpsertSecurity(ctx, portfoliosvc.UpsertSecurityInput{
		Code:         code,
		Name:         firstNonEmpty(stringArg(args, "fund_name"), stringArg(args, "name")),
		FundType:     stringArg(args, "fund_type"),
		SecurityType: stringArg(args, "security_type"),
		Market:       stringArg(args, "market"),
		Currency:     stringArg(args, "currency"),
		Exchange:     stringArg(args, "exchange"),
		Source:       stringArg(args, "source"),
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"ok":                res.OK,
		"created":           res.Created,
		"security":          res.Security,
		"decision_boundary": "facts_only",
		"side_effects":      "security_master_update",
	})
}

func (s *Server) callDeleteFund(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	code := firstNonEmpty(stringArg(args, "fund_code"), stringArg(args, "code"))
	res, err := s.portfolio.DeleteSecurity(ctx, code)
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"ok":                res.OK,
		"code":              res.Code,
		"deleted":           res.Deleted,
		"decision_boundary": "facts_only",
		"side_effects":      "security_cascade_delete",
	})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
