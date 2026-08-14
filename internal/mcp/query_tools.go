package mcp

import (
	"context"
	"strings"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

func (s *Server) callFundDetail(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	code := adminsvc.NormalizeSecurityCode(firstNonEmpty(stringArg(args, "code"), stringArg(args, "fund_code")))
	portfolioID := intArgMax(args, "portfolio_id", 1, 1000)
	detail, err := s.portfolio.GetFundDetail(ctx, code, portfolioID)
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	if detail == nil {
		return textJSONResult(map[string]any{"error": "security_not_found", "code": code})
	}
	payload := map[string]any{
		"code":          detail.Code,
		"name":          detail.Name,
		"type":          detail.Type,
		"security_type": detail.SecurityType,
		"market":        detail.Market,
		"position": map[string]any{
			"shares":         detail.Position.Shares,
			"cost_basis":     detail.Position.CostBasis,
			"market_value":   detail.Position.MarketValue,
			"unrealized_pnl": detail.Position.UnrealizedPNL,
			"pnl_pct":        detail.Position.PNLPct,
		},
		"nav": map[string]any{
			"count":     detail.NAVCount,
			"last_date": detail.LastNAVDate,
		},
		"xirr_pct":          nil,
		"transaction_count": detail.TransactionCount,
		"transactions":      mcpFundTransactions(detail.Transactions),
		"decision_boundary": "facts_only",
	}
	// Populate XIRR when computable (#93); keep null on no_data / error without failing detail.
	if xirr, err := s.portfolio.GetFundXIRR(ctx, code, portfolioID); err == nil && xirr.XIRRPct != nil {
		payload["xirr_pct"] = *xirr.XIRRPct
	}
	if detail.TransactionCount == 0 {
		payload["warning"] = "no_transactions"
	}
	return textJSONResult(payload)
}

func (s *Server) callNavHistory(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	report, err := s.portfolio.GetNavHistory(ctx, adminsvc.NormalizeSecurityCode(firstNonEmpty(stringArg(args, "code"), stringArg(args, "fund_code"))), intArgMax(args, "limit", 200, 2000))
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(report)
}

func (s *Server) callFundXIRR(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	code := adminsvc.NormalizeSecurityCode(firstNonEmpty(stringArg(args, "code"), stringArg(args, "fund_code")))
	portfolioID := intArgMax(args, "portfolio_id", 1, 1000)
	report, err := s.portfolio.GetFundXIRR(ctx, code, portfolioID)
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(report)
}

func (s *Server) callPortfolioXIRR(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	report, err := s.portfolio.GetPortfolioXIRR(ctx, intArgMax(args, "portfolio_id", 1, 1000))
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(report)
}

func (s *Server) callSearchFunds(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	results, err := s.portfolio.SearchFunds(ctx, stringArg(args, "query"))
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(results)
}

func (s *Server) callSearchStocks(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	report, err := s.portfolio.SearchStocks(ctx, portfoliosvc.StockSearchOptions{
		Query:  stringArg(args, "query"),
		Market: stringArg(args, "market"),
		Limit:  intArgMax(args, "limit", 15, 50),
	})
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(report)
}

func (s *Server) callUSStock(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	report, err := s.portfolio.GetUSStock(ctx, portfoliosvc.USStockOptions{
		Symbol:         stringArg(args, "symbol"),
		Range:          stringArg(args, "range"),
		IncludeHistory: boolArg(args, "include_history", true),
	})
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(report)
}

func (s *Server) callMarketIndices(ctx context.Context) (map[string]any, *Error) {
	report, err := s.portfolio.GetMarketIndices(ctx)
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(report)
}

func (s *Server) callFundDrawdown(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	report, err := s.portfolio.GetFundDrawdown(ctx, adminsvc.NormalizeSecurityCode(firstNonEmpty(stringArg(args, "code"), stringArg(args, "fund_code"))))
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	if report == nil {
		return textJSONResult(map[string]any{"error": "no nav data"})
	}
	return textJSONResult(report)
}

func (s *Server) callCompareFunds(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	rawCodes := stringSliceArg(args, "codes")
	if len(rawCodes) == 0 {
		return textJSONResult(map[string]any{"error": "codes required"})
	}
	seen := map[string]struct{}{}
	codes := make([]string, 0, len(rawCodes))
	for _, rawCode := range rawCodes {
		code := adminsvc.NormalizeSecurityCode(rawCode)
		if strings.TrimSpace(code) == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return textJSONResult(map[string]any{"error": "codes required"})
	}
	const maxCompareCodes = 8
	if len(codes) > maxCompareCodes {
		return textJSONResult(map[string]any{"error": "codes max 8"})
	}
	portfolioID := intArgMax(args, "portfolio_id", 1, 1000)
	results, err := s.portfolio.CompareFunds(ctx, codes, portfolioID)
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(map[string]any{
		"funds":             results,
		"decision_boundary": "facts_only",
		"side_effects":      "none",
	})
}

func (s *Server) callComputeDCAAmount(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	code := firstNonEmpty(stringArg(args, "fund_code"), stringArg(args, "code"))
	report, err := s.portfolio.ComputeDCAAmount(ctx, portfoliosvc.ComputeDCAAmountOptions{
		Code:       adminsvc.NormalizeSecurityCode(code),
		BaseAmount: floatArgMax(args, "base_amount", 30, 1_000_000),
		Mode:       stringArg(args, "mode"),
	})
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(report)
}

func (s *Server) callRunBacktest(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	// Accept code or fund_code — agents often pass code like other query tools (#91).
	code := firstNonEmpty(stringArg(args, "fund_code"), stringArg(args, "code"))
	report, err := s.portfolio.RunBacktest(ctx, portfoliosvc.BacktestOptions{
		FundCode:          adminsvc.NormalizeSecurityCode(code),
		Strategy:          stringArg(args, "strategy"),
		StartDate:         stringArg(args, "start_date"),
		BaseAmount:        floatArgMax(args, "base_amount", 1000, 1_000_000),
		GridLevels:        intArgMax(args, "grid_levels", 0, 50),
		MomentumMonths:    intArgMax(args, "momentum_months", 0, 36),
		TargetWeight:      floatArgMax(args, "target_weight", 0, 1),
		RebalanceInterval: intArgMax(args, "rebalance_interval", 0, 365),
	})
	if err != nil {
		return nil, jsonrpcError(-32000, "tool_error: internal_error")
	}
	return textJSONResult(report)
}

func mcpFundTransactions(transactions []portfoliosvc.FundTransaction) []map[string]any {
	out := make([]map[string]any, 0, len(transactions))
	for _, tx := range transactions {
		out = append(out, map[string]any{
			"seq":             tx.Seq,
			"time":            tx.Time,
			"confirm_date":    tx.ConfirmDate,
			"direction":       tx.Direction,
			"type":            tx.Type,
			"amount":          tx.Amount,
			"shares":          tx.Shares,
			"fee":             tx.Fee,
			"settlement_days": tx.SettlementDays,
			"order_id":        tx.OrderID,
		})
	}
	return out
}
