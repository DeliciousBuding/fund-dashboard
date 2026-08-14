package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

const jsonrpcVersion = "2.0"

var processStartedAt = time.Now()

type Server struct {
	registry  *agenttools.Registry
	portfolio *portfoliosvc.Service
	admin     *adminsvc.Service
	agentOps  confirmationConsumer
	nav       NavCrawler
	snapshots SnapshotRecalculator
	holdings  HoldingsCrawler
	role      agenttools.Role
}

// confirmationConsumer claims (verify + atomic MarkUsed) before write side-effects.
// Claim must happen before tool execution so concurrent tools/call cannot double-write.
type confirmationConsumer interface {
	ClaimConfirmation(context.Context, agentops.ConsumeConfirmationInput) (agentops.ConsumedConfirmation, error)
}

// NavCrawler is the optional price/NAV refresh surface for crawl_nav.
// Implemented by jobs.PriceRefresher (via thin adapter) so MCP stays free of job scheduling details.
type NavCrawler interface {
	CrawlAllHeld(ctx context.Context) (securities int, added int, err error)
	CrawlCode(ctx context.Context, code string) (added int, latest string, err error)
}

// SnapshotRecalculator rebuilds portfolio_snapshot rows from transactions + nav_history.
// RecalcAll soft-fails per code: err is only for hard failures (list/scan).
// Per-code failures return failed_codes with err=nil (status partial|error at call site).
type SnapshotRecalculator interface {
	RecalcCode(ctx context.Context, code string) error
	RecalcAll(ctx context.Context) (codes int, failed []string, err error)
}

// HoldingsCrawler refreshes fund_holdings disclosures for MCP crawl_fund_holdings.
type HoldingsCrawler interface {
	CrawlCode(ctx context.Context, code string) (added int, reportDate string, err error)
	CrawlAllHeld(ctx context.Context) (funds int, added int, err error)
}

type ServerDeps struct {
	Registry  *agenttools.Registry
	Portfolio *portfoliosvc.Service
	Admin     *adminsvc.Service
	AgentOps  confirmationConsumer
	Nav       NavCrawler
	Snapshots SnapshotRecalculator
	Holdings  HoldingsCrawler
	Role      agenttools.Role
}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	Tool      string         `json:"tool"`
	Args      map[string]any `json:"args"`
}

func NewServer(deps ServerDeps) (*Server, error) {
	registry := deps.Registry
	if registry == nil {
		loaded, err := agenttools.DefaultRegistry()
		if err != nil {
			return nil, err
		}
		registry = loaded
	}
	if deps.Portfolio == nil {
		return nil, errors.New("mcp portfolio service is required")
	}
	role := deps.Role
	if role == "" {
		role = agenttools.RoleAnalyst
	}
	return &Server{registry: registry, portfolio: deps.Portfolio, admin: deps.Admin, agentOps: deps.AgentOps, nav: deps.Nav, snapshots: deps.Snapshots, holdings: deps.Holdings, role: role}, nil
}

func (s *Server) Handle(ctx context.Context, request Request) Response {
	response := Response{JSONRPC: jsonrpcVersion, ID: request.ID}
	if request.JSONRPC != "" && request.JSONRPC != jsonrpcVersion {
		response.Error = jsonrpcError(-32600, "invalid_request: jsonrpc must be 2.0")
		return response
	}

	switch request.Method {
	case "initialize":
		response.Result = map[string]any{
			"protocolVersion": "2025-06-18",
			"serverInfo": map[string]any{
				"name":    "fund-dashboard-go",
				"version": "dev",
			},
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
		}
	case "tools/list":
		response.Result = s.listTools()
	case "tools/call":
		result, err := s.callTool(ctx, request.Params)
		if err != nil {
			response.Error = err
			return response
		}
		response.Result = result
	default:
		response.Error = jsonrpcError(-32601, "method_not_found: "+request.Method)
	}
	return response
}

func (s *Server) listTools() map[string]any {
	// Only advertise tools that callTool can execute for this auth role.
	// Registry still holds the full migration matrix (44); PUBLIC/analyst must
	// not discover write/maintenance tools or confirmation-gated tools they cannot complete.
	implemented := implementedMCPTools()
	tools := make([]mcpTool, 0, len(implemented))
	for _, tool := range s.registry.Tools {
		if _, ok := implemented[tool.Name]; !ok {
			continue
		}
		if !agenttools.RoleAllowsScope(s.role, tool.Capability.Scope) {
			continue
		}
		// Non-operators cannot complete confirmation flows (AgentOps is operator-only).
		// Do not advertise confirmation-gated tools they can never successfully call.
		if s.role != agenttools.RoleOperator &&
			(tool.Capability.Permission == agenttools.PermissionRequiresConfirmation || tool.Confirmation.Required) {
			continue
		}
		tools = append(tools, mcpTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
			Annotations: mcpToolAnnotations{
				ReadOnlyHint:    tool.MCPAnnotations.ReadOnlyHint,
				DestructiveHint: tool.MCPAnnotations.DestructiveHint,
				OpenWorldHint:   tool.MCPAnnotations.OpenWorldHint,
			},
		})
	}
	return map[string]any{"tools": tools}
}

// implementedMCPTools is the SSOT for tools/call switch cases below.
func implementedMCPTools() map[string]struct{} {
	return map[string]struct{}{
		"get_portfolio_summary":           {},
		"get_portfolio_xirr":              {},
		"get_portfolio_timeline":          {},
		"get_portfolio_penetration":       {},
		"get_portfolio_allocation":        {},
		"list_portfolios":                 {},
		"list_dca_plans":                  {},
		"get_investment_harness_snapshot": {},
		"get_investment_source_brief":     {},
		"get_source_events":               {},
		"mark_source_event":               {},
		"crawl_nav":                       {},
		"recalculate_snapshot":            {},
		"crawl_fund_holdings":             {},
		"get_data_freshness":              {},
		"verify_data":                     {},
		"get_fund_status":                 {},
		"get_system_status":               {},
		"get_fund_detail":                 {},
		"get_nav_history":                 {},
		"get_fund_xirr":                   {},
		"search_funds":                    {},
		"search_stocks":                   {},
		"get_us_stock":                    {},
		"get_market_indices":              {},
		"get_fund_drawdown":               {},
		"compare_funds":                   {},
		"compute_dca_amount":              {},
		"run_backtest":                    {},
		"get_full_dashboard":              {},
		"add_transaction":                 {},
		"import_transactions":             {},
		"update_transaction":              {},
		"delete_transaction":              {},
		"disable_dca_plan":                {},
		"upsert_dca_plan":                 {},
		"add_fund":                        {},
		"add_security":                    {},
		"update_fund":                     {},
		"delete_fund":                     {},
		"adjust_position":                 {},
		"check_alerts":                    {},
		"run_dca_auto_invest":             {},
		"generate_report":                 {},
	}
}

type mcpTool struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	InputSchema map[string]any     `json:"inputSchema"`
	Annotations mcpToolAnnotations `json:"annotations"`
}

type mcpToolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint"`
	OpenWorldHint   bool `json:"openWorldHint"`
}

func (s *Server) callTool(ctx context.Context, rawParams json.RawMessage) (map[string]any, *Error) {
	var params ToolCallParams
	if len(rawParams) > 0 {
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, jsonrpcError(-32602, "invalid_params")
		}
	}
	name := params.Name
	args := params.Arguments
	if name == "" {
		name = params.Tool
		args = params.Args
	}
	if args == nil {
		args = map[string]any{}
	}

	authorizeRequest := agenttools.AuthorizeRequest{
		Tool:            name,
		Role:            s.role,
		EnforceReviewed: boolArg(args, "enforce_reviewed", false),
	}
	decision := s.registry.Authorize(authorizeRequest)
	if !decision.Allowed && decision.Reason != agenttools.DenyConfirmationRequired {
		return nil, jsonrpcError(-32001, fmt.Sprintf("tool_denied: %s", decision.Reason))
	}
	// Claim confirmation BEFORE any write side-effect (atomic CAS on used_at IS NULL).
	// Concurrent tools/call with the same confirmation_id+token: exactly one claimer wins.
	if decision.RequiresConfirmation {
		if confirmationErr := s.claimWriteConfirmation(ctx, name, args); confirmationErr != nil {
			return nil, confirmationErr
		}
		authorizeRequest.Confirmed = true
		decision = s.registry.Authorize(authorizeRequest)
	}
	if !decision.Allowed {
		return nil, jsonrpcError(-32001, fmt.Sprintf("tool_denied: %s", decision.Reason))
	}

	var result map[string]any
	var callErr *Error
	switch name {
	case "get_portfolio_summary":
		result, callErr = s.callPortfolioSummary(ctx, args)
	case "get_portfolio_xirr":
		result, callErr = s.callPortfolioXIRR(ctx, args)
	case "get_portfolio_timeline":
		result, callErr = s.callPortfolioTimeline(ctx, args)
	case "get_portfolio_penetration":
		result, callErr = s.callPortfolioPenetration(ctx, args)
	case "get_portfolio_allocation":
		result, callErr = s.callPortfolioAllocation(ctx, args)
	case "list_portfolios":
		result, callErr = s.callListPortfolios(ctx)
	case "list_dca_plans":
		result, callErr = s.callListDCAPlans(ctx, args)
	case "get_investment_harness_snapshot":
		result, callErr = s.callInvestmentHarnessSnapshot(ctx, args)
	case "get_investment_source_brief":
		result, callErr = s.callInvestmentSourceBrief(ctx, args)
	case "get_source_events":
		result, callErr = s.callSourceEvents(ctx, args)
	case "mark_source_event":
		result, callErr = s.callMarkSourceEvent(ctx, args)
	case "crawl_nav":
		result, callErr = s.callCrawlNav(ctx, args)
	case "recalculate_snapshot":
		result, callErr = s.callRecalculateSnapshot(ctx, args)
	case "crawl_fund_holdings":
		result, callErr = s.callCrawlFundHoldings(ctx, args)
	case "get_data_freshness":
		result, callErr = s.callDataFreshness(ctx)
	case "verify_data":
		result, callErr = s.callVerifyData(ctx)
	case "get_fund_status":
		result, callErr = s.callFundStatus(ctx, args)
	case "get_system_status":
		result, callErr = s.callSystemStatus(ctx)
	case "get_fund_detail":
		result, callErr = s.callFundDetail(ctx, args)
	case "get_nav_history":
		result, callErr = s.callNavHistory(ctx, args)
	case "get_fund_xirr":
		result, callErr = s.callFundXIRR(ctx, args)
	case "search_funds":
		result, callErr = s.callSearchFunds(ctx, args)
	case "search_stocks":
		result, callErr = s.callSearchStocks(ctx, args)
	case "get_us_stock":
		result, callErr = s.callUSStock(ctx, args)
	case "get_market_indices":
		result, callErr = s.callMarketIndices(ctx)
	case "get_fund_drawdown":
		result, callErr = s.callFundDrawdown(ctx, args)
	case "compare_funds":
		result, callErr = s.callCompareFunds(ctx, args)
	case "compute_dca_amount":
		result, callErr = s.callComputeDCAAmount(ctx, args)
	case "run_backtest":
		result, callErr = s.callRunBacktest(ctx, args)
	case "get_full_dashboard":
		result, callErr = s.callFullDashboard(ctx, args)
	case "add_transaction":
		result, callErr = s.callAddTransaction(ctx, args)
	case "import_transactions":
		result, callErr = s.callImportTransactions(ctx, args)
	case "update_transaction":
		result, callErr = s.callUpdateTransaction(ctx, args)
	case "delete_transaction":
		result, callErr = s.callDeleteTransaction(ctx, args)
	case "upsert_dca_plan":
		result, callErr = s.callUpsertDCAPlan(ctx, args)
	case "disable_dca_plan":
		result, callErr = s.callDisableDCAPlan(ctx, args)
	case "add_fund":
		result, callErr = s.callAddFund(ctx, args)
	case "add_security":
		result, callErr = s.callAddSecurity(ctx, args)
	case "update_fund":
		result, callErr = s.callUpdateFund(ctx, args)
	case "delete_fund":
		result, callErr = s.callDeleteFund(ctx, args)
	case "adjust_position":
		result, callErr = s.callAdjustPosition(ctx, args)
	case "check_alerts":
		result, callErr = s.callCheckAlerts(ctx, args)
	case "run_dca_auto_invest":
		result, callErr = s.callRunDCAAutoInvest(ctx, args)
	case "generate_report":
		result, callErr = s.callGenerateReport(ctx, args)
	default:
		result, callErr = nil, jsonrpcError(-32601, "tool_not_implemented: "+name)
	}

	if callErr != nil {
		// Confirmation already claimed/burned before side-effect (safe under-commit).
		// Prefer re-prepare over risking double-write under concurrent tools/call.
		return nil, callErr
	}
	return result, nil
}

// claimWriteConfirmation atomically verifies and marks the confirmation used before the write.
// On claim failure (invalid/expired/already-used) the tool is not executed.
func (s *Server) claimWriteConfirmation(ctx context.Context, name string, args map[string]any) *Error {
	tool, ok := s.registry.Lookup(name)
	if !ok || tool.Capability.Permission != agenttools.PermissionRequiresConfirmation {
		return nil
	}
	if s.agentOps == nil {
		return jsonrpcError(-32001, "tool_denied: confirmation_service_unavailable")
	}
	confirmationID := int64(intArg(args, "confirmation_id", 0))
	token := stringArg(args, "confirmation_token")
	if confirmationID <= 0 || token == "" {
		// Point agents at the prepare HTTP endpoint and field map (token -> confirmation_token).
		// Bare confirmed=true is not accepted (fail closed).
		return jsonrpcError(-32602, "invalid_params: confirmation_id and confirmation_token required; prepare via POST /api/agent/confirmations/prepare then pass confirmation_id + confirmation_token (prepare returns token)")
	}
	payload := confirmationPayload(args)
	in := agentops.ConsumeConfirmationInput{
		Tool:            name,
		Role:            s.role,
		Caller:          valueOrDefault(stringArg(args, "caller"), "mcp"),
		RequestID:       stringArg(args, "request_id"),
		ConfirmationID:  confirmationID,
		Token:           token,
		Payload:         payload,
		EnforceReviewed: boolArg(args, "enforce_reviewed", false),
		ResultSummary:   map[string]any{"authorization": "claimed"},
	}
	if _, err := s.agentOps.ClaimConfirmation(ctx, in); err != nil {
		return jsonrpcError(-32001, "tool_denied: invalid_confirmation")
	}
	return nil
}

func confirmationPayload(args map[string]any) map[string]any {
	payload := make(map[string]any, len(args))
	for key, value := range args {
		switch key {
		case "confirmed", "confirmation_id", "confirmation_token", "caller", "request_id", "enforce_reviewed":
			continue
		default:
			payload[key] = value
		}
	}
	return payload
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
