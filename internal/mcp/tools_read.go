package mcp

import (
	"context"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

func (s *Server) callPortfolioSummary(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	portfolioID := intArgMax(args, "portfolio_id", 1, 1000)
	summary, err := s.portfolio.GetSummary(ctx, portfolioID)
	if err != nil {
		return nil, internalToolError(err)
	}
	harness, err := s.portfolio.GetHarnessSnapshotFor(ctx, portfolioID, s.harnessAudience())
	if err != nil {
		return nil, internalToolError(err)
	}
	payload := map[string]any{
		"summary": map[string]any{
			"total_transactions": summary.TotalTx,
			"unique_funds":       summary.UniqueFunds,
			"unique_stocks":      summary.UniqueStocks,
			"held_funds":         summary.HeldFunds,
			"total_buy":          summary.TotalBuy,
			"total_sell":         summary.TotalSell,
			"total_fee":          summary.TotalFee,
			"unrealized_pnl":     summary.UnrealizedPNL,
			"invested_cost":      summary.InvestedCost,
			"current_value":      summary.CurrentValue,
			"pnl_pct":            summary.PNLPct,
			"auto_invest": map[string]any{
				"tx":     summary.AutoTx,
				"amount": summary.AutoAmount,
			},
			"manual_invest": map[string]any{
				"tx":     summary.ManualTx,
				"amount": summary.ManualAmount,
			},
			"date_range": map[string]any{
				"first": summary.FirstTrade,
				"last":  summary.LastTrade,
			},
			"settlement_distribution": summary.SettlementDistribution,
		},
		"holdings":          mcpHoldings(harness.HoldingSignals),
		"decision_boundary": "facts_only",
	}
	return textJSONResult(payload)
}

func (s *Server) callPortfolioAllocation(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	allocation, err := s.portfolio.GetAllocation(ctx, intArgMax(args, "portfolio_id", 1, 1000))
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(allocation)
}

func (s *Server) callListPortfolios(ctx context.Context) (map[string]any, *Error) {
	portfolios, err := s.portfolio.ListPortfolioDefinitions(ctx)
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(portfolios)
}

func (s *Server) callListDCAPlans(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	plans, err := s.portfolio.ListDCAPlans(ctx, portfoliosvc.ListDCAPlansOptions{
		ActiveOnly:  boolArg(args, "active_only", false),
		PortfolioID: intArgMax(args, "portfolio_id", 0, 1000),
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"count":             len(plans),
		"decision_boundary": "facts_only",
		"side_effects":      "none",
		"plans":             plans,
	})
}

func (s *Server) callInvestmentHarnessSnapshot(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	snapshot, err := s.portfolio.GetHarnessSnapshotFor(ctx, intArgMax(args, "portfolio_id", 1, 1000), s.harnessAudience())
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(snapshot)
}

func (s *Server) callInvestmentSourceBrief(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	brief, err := s.portfolio.GetInvestmentSourceBrief(ctx, portfoliosvc.SourceBriefOptions{
		PortfolioID: intArgMax(args, "portfolio_id", 1, 1000),
		Limit:       intArgMax(args, "limit", 20, 100),
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(brief)
}

func (s *Server) callSourceEvents(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	// Prefer unread_only (registry schema); show_read remains as compatibility alias.
	showRead := boolArg(args, "show_read", false)
	if _, ok := args["unread_only"]; ok {
		showRead = !boolArg(args, "unread_only", false)
	}
	events, err := s.portfolio.GetSourceEvents(ctx, portfoliosvc.GetSourceEventsOptions{
		Limit:               intArgMax(args, "limit", 30, 200),
		Offset:              intArgMax(args, "offset", 0, 100000),
		RelatedSecurityCode: firstNonEmpty(stringArg(args, "code"), stringArg(args, "fund_code")),
		Source:              stringArg(args, "source"),
		ShowRead:            showRead,
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"count":             len(events),
		"decision_boundary": "facts_only",
		"events":            mcpSourceEvents(events),
	})
}

func (s *Server) callCrawlFundHoldings(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	if s.holdings == nil {
		return nil, jsonrpcError(-32000, "tool_error: holdings crawler is not configured")
	}
	code := adminsvc.NormalizeSecurityCode(stringArg(args, "fund_code"))
	if code == "" {
		code = adminsvc.NormalizeSecurityCode(stringArg(args, "code"))
	}
	if code != "" && len(code) > 32 {
		return nil, jsonrpcError(-32602, "invalid_params: fund_code too long")
	}
	if code != "" {
		added, reportDate, err := s.holdings.CrawlCode(ctx, code)
		if err != nil {
			return nil, internalToolError(err)
		}
		return textJSONResult(map[string]any{
			"status":            "complete",
			"mode":              "single",
			"fund_code":         code,
			"added":             added,
			"report_date":       reportDate,
			"decision_boundary": "facts_only",
			"side_effects":      "writes_fund_holdings",
		})
	}
	funds, added, err := s.holdings.CrawlAllHeld(ctx)
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"status":            "complete",
		"mode":              "held",
		"funds":             funds,
		"added":             added,
		"decision_boundary": "facts_only",
		"side_effects":      "writes_fund_holdings",
	})
}

func (s *Server) callRecalculateSnapshot(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	if s.snapshots == nil {
		return nil, jsonrpcError(-32000, "tool_error: snapshot recalculator is not configured")
	}
	code := adminsvc.NormalizeSecurityCode(stringArg(args, "fund_code"))
	if code == "" {
		code = adminsvc.NormalizeSecurityCode(stringArg(args, "code"))
	}
	if code != "" && len(code) > 32 {
		return nil, jsonrpcError(-32602, "invalid_params: fund_code too long")
	}
	if code != "" {
		if err := s.snapshots.RecalcCode(ctx, code); err != nil {
			return nil, internalToolError(err)
		}
		return textJSONResult(map[string]any{
			"status":            "complete",
			"mode":              "single",
			"fund_code":         code,
			"decision_boundary": "facts_only",
			"side_effects":      "rewrites_portfolio_snapshot",
		})
	}
	n, failed, err := s.snapshots.RecalcAll(ctx)
	if err != nil {
		return nil, internalToolError(err)
	}
	if failed == nil {
		failed = []string{}
	}
	status := jobs.RecalcAllStatus(n, failed)
	return textJSONResult(map[string]any{
		"status":            status,
		"mode":              "all",
		"codes":             n,
		"failed_codes":      failed,
		"decision_boundary": "facts_only",
		"side_effects":      "rewrites_portfolio_snapshot",
	})
}

func (s *Server) callCrawlNav(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	if s.nav == nil {
		return nil, jsonrpcError(-32000, "tool_error: nav crawler is not configured")
	}
	code := adminsvc.NormalizeSecurityCode(stringArg(args, "fund_code"))
	if code == "" {
		code = adminsvc.NormalizeSecurityCode(stringArg(args, "code"))
	}
	if code != "" && len(code) > 32 {
		return nil, jsonrpcError(-32602, "invalid_params: fund_code too long")
	}
	if code != "" {
		added, latest, err := s.nav.CrawlCode(ctx, code)
		if err != nil {
			return nil, internalToolError(err)
		}
		return textJSONResult(map[string]any{
			"status":            "complete",
			"mode":              "single",
			"fund_code":         code,
			"added":             added,
			"latest":            latest,
			"decision_boundary": "facts_only",
			"side_effects":      "writes_nav_history_and_snapshot",
		})
	}
	// Agent-friendly path: refresh only stale/missing held codes (#252).
	if boolArg(args, "stale_only", false) {
		if s.admin == nil {
			return nil, jsonrpcError(-32000, "tool_error: admin freshness service is required")
		}
		report, err := s.admin.GetFreshness(ctx)
		if err != nil {
			return nil, internalToolError(err)
		}
		codes := RecommendedRefreshCodes(report)
		if len(codes) == 0 {
			return textJSONResult(map[string]any{
				"status":            "complete",
				"mode":              "stale_only",
				"securities":        0,
				"added":             0,
				"codes":             []string{},
				"decision_boundary": "facts_only",
				"side_effects":      "none",
				"message":           "no_stale_or_missing_held_nav",
			})
		}
		totalAdded := 0
		done := make([]string, 0, len(codes))
		failed := make([]string, 0)
		for _, c := range codes {
			if err := ctx.Err(); err != nil {
				break
			}
			added, _, err := s.nav.CrawlCode(ctx, c)
			if err != nil {
				failed = append(failed, c)
				continue
			}
			totalAdded += added
			done = append(done, c)
		}
		status := "complete"
		if len(failed) > 0 && len(done) == 0 {
			status = "error"
		} else if len(failed) > 0 {
			status = "partial"
		}
		return textJSONResult(map[string]any{
			"status":            status,
			"mode":              "stale_only",
			"securities":        len(done),
			"added":             totalAdded,
			"codes":             done,
			"failed_codes":      failed,
			"decision_boundary": "facts_only",
			"side_effects":      "writes_nav_history_and_snapshot",
		})
	}
	securities, added, err := s.nav.CrawlAllHeld(ctx)
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"status":            "complete",
		"mode":              "held",
		"securities":        securities,
		"added":             added,
		"decision_boundary": "facts_only",
		"side_effects":      "writes_nav_history_and_snapshot",
	})
}

func (s *Server) callMarkSourceEvent(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	id := int64(intArg(args, "id", 0))
	if id <= 0 {
		id = int64(intArg(args, "event_id", 0))
	}
	if id <= 0 {
		return nil, jsonrpcError(-32602, "invalid_params: id is required")
	}
	input := portfoliosvc.MarkSourceEventInput{}
	if _, ok := args["is_read"]; ok {
		v := boolArg(args, "is_read", false)
		input.IsRead = &v
	}
	if _, ok := args["is_useful"]; ok {
		v := boolArg(args, "is_useful", false)
		input.IsUseful = &v
	}
	if input.IsRead == nil && input.IsUseful == nil {
		v := true
		input.IsRead = &v
	}
	ok, err := s.portfolio.MarkSourceEventRead(ctx, id, input)
	if err != nil {
		return nil, internalToolError(err)
	}
	if !ok {
		return nil, jsonrpcError(-32000, "tool_error: not found or no fields to update")
	}
	return textJSONResult(map[string]any{
		"ok":                true,
		"id":                id,
		"decision_boundary": "facts_only",
	})
}

func (s *Server) callDataFreshness(ctx context.Context) (map[string]any, *Error) {
	if s.admin == nil {
		return nil, jsonrpcError(-32000, "tool_error: admin freshness service is required")
	}
	report, err := s.admin.GetFreshness(ctx)
	if err != nil {
		return nil, internalToolError(err)
	}
	// Only recommend crawl_nav when held NAV is stale/missing (#74).
	// health=fresh must not force maintenance (agents were always crawling).
	recommendedCodes := RecommendedRefreshCodes(report)
	needsCrawl := len(recommendedCodes) > 0
	var recommendedTool any
	var recommendedArgs any
	if needsCrawl {
		recommendedTool = "crawl_nav"
		recommendedArgs = map[string]any{"stale_only": true}
	}
	return textJSONResult(map[string]any{
		"last_transaction":                     report.LastTransaction,
		"last_nav_date":                        report.LastNAVDate,
		"anomaly_count":                        report.AnomalyCount,
		"health":                               report.Health,
		"missing_nav_securities":               report.MissingNAVSecurities,
		"watchlist_missing_nav_securities":     report.WatchlistMissingNAVSecurities,
		"stale_nav_securities":                 staleNAVSecurities(report.StaleSecurities),
		"stale_detail":                         report.StaleSecurities,
		"recommended_codes":                    recommendedCodes,
		"actionable":                           report.Actionable,
		"decision_boundary":                    report.DecisionBoundary,
		"side_effects":                         "none",
		"recommended_maintenance_tool":         recommendedTool,
		"recommended_maintenance_args":         recommendedArgs,
		"recommended_maintenance_requires_run": needsCrawl,
	})
}

func (s *Server) callVerifyData(ctx context.Context) (map[string]any, *Error) {
	if s.admin == nil {
		return nil, jsonrpcError(-32000, "tool_error: admin service is required")
	}
	report, err := s.admin.VerifyData(ctx)
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"healthy":           report.OK,
		"issues":            report.Issues,
		"details":           report.Details,
		"decision_boundary": report.DecisionBoundary,
	})
}

func (s *Server) callFundStatus(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	if s.admin == nil {
		return nil, jsonrpcError(-32000, "tool_error: admin service is required")
	}
	code := adminsvc.NormalizeSecurityCode(firstNonEmpty(stringArg(args, "code"), stringArg(args, "fund_code")))
	if code == "" {
		return nil, jsonrpcError(-32602, "invalid_params: code or fund_code required")
	}
	status, err := s.admin.GetStatusByCode(ctx, code)
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(map[string]any{
		"code":          status.Code,
		"name":          status.Name,
		"type":          status.Type,
		"security_type": status.SecurityType,
		"market":        status.Market,
		"transactions": map[string]any{
			"count": status.Transactions.N,
			"first": dateOnlyStringPtr(status.Transactions.First),
			"last":  dateOnlyStringPtr(status.Transactions.Last),
		},
		"nav": map[string]any{
			"count": status.NAV.N,
			"first": dateOnlyStringPtr(status.NAV.First),
			"last":  dateOnlyStringPtr(status.NAV.Last),
		},
		"position": map[string]any{
			"shares":  status.Position.HeldShares,
			"cost":    status.Position.TotalCost,
			"value":   status.Position.CurrentValue,
			"pnl":     status.Position.UnrealizedPNL,
			"pnl_pct": status.Position.PNLPct,
		},
		"trading":           status.Trading,
		"decision_boundary": "facts_only",
	})
}

func (s *Server) callSystemStatus(ctx context.Context) (map[string]any, *Error) {
	if s.admin == nil {
		return nil, jsonrpcError(-32000, "tool_error: admin service is required")
	}
	status, err := s.admin.GetSystemStatus(ctx, processStartedAt, nowUTC())
	if err != nil {
		return nil, internalToolError(err)
	}
	return textJSONResult(status)
}

func (s *Server) callFullDashboard(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	portfolioID := intArgMax(args, "portfolio_id", 1, 1000)
	sourceLimit := intArgMax(args, "source_limit", 8, 50)
	eventLimit := intArgMax(args, "event_limit", 20, 100)

	summary, err := s.portfolio.GetSummary(ctx, portfolioID)
	if err != nil {
		return nil, internalToolError(err)
	}
	harness, err := s.portfolio.GetHarnessSnapshotFor(ctx, portfolioID, s.harnessAudience())
	if err != nil {
		return nil, internalToolError(err)
	}
	allocation, err := s.portfolio.GetAllocation(ctx, portfolioID)
	if err != nil {
		return nil, internalToolError(err)
	}
	sourceBrief, err := s.portfolio.GetInvestmentSourceBrief(ctx, portfoliosvc.SourceBriefOptions{
		PortfolioID: portfolioID,
		Limit:       sourceLimit,
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	sourceEvents, err := s.portfolio.GetSourceEvents(ctx, portfoliosvc.GetSourceEventsOptions{
		Limit:    eventLimit,
		ShowRead: true,
	})
	if err != nil {
		return nil, internalToolError(err)
	}
	agentContext, err := s.portfolio.GetAgentContextPackFor(ctx, portfoliosvc.AgentContextOptions{
		PortfolioID:  portfolioID,
		SourceLimit:  sourceLimit,
		EventLimit:   eventLimit,
		BaseCurrency: stringArg(args, "base_currency"),
	}, s.harnessAudience())
	if err != nil {
		return nil, internalToolError(err)
	}

	return textJSONResult(map[string]any{
		"decision_boundary": "facts_only",
		"summary":           summary,
		"harness_snapshot":  harness,
		"allocation":        allocation,
		"source_brief":      sourceBrief,
		"source_events": map[string]any{
			"count":  len(sourceEvents),
			"events": mcpSourceEvents(sourceEvents),
		},
		"agent_context": agentContext,
	})
}

func (s *Server) harnessAudience() portfoliosvc.HarnessAudience {
	if s.role == agenttools.RoleOperator {
		return portfoliosvc.HarnessAudienceOperator
	}
	return portfoliosvc.HarnessAudiencePublic
}
