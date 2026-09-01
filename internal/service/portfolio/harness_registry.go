package portfolio

// availableAgentTools is the agent-facing subset advertised in harness snapshots.
// It tracks implemented MCP tools that agents commonly chain; full tools/list is 44.
var availableAgentTools = []string{
	"get_full_dashboard",
	"get_portfolio_summary",
	"get_portfolio_xirr",
	"get_portfolio_timeline",
	"get_portfolio_allocation",
	"get_portfolio_penetration",
	"list_portfolios",
	"list_dca_plans",
	"get_fund_detail",
	"get_fund_status",
	"get_fund_xirr",
	"get_fund_drawdown",
	"get_nav_history",
	"search_funds",
	"search_stocks",
	"get_us_stock",
	"get_market_indices",
	"get_data_freshness",
	"verify_data",
	"get_system_status",
	"get_source_events",
	"get_investment_source_brief",
	"get_investment_harness_snapshot",
	"compute_dca_amount",
	"run_backtest",
	"compare_funds",
	"crawl_nav",
	"crawl_fund_holdings",
	"recalculate_snapshot",
	"mark_source_event",
	"add_transaction",
	"update_transaction",
	"delete_transaction",
	"import_transactions",
	"upsert_dca_plan",
	"disable_dca_plan",
	"run_dca_auto_invest",
	"add_fund",
	"add_security",
	"update_fund",
	"delete_fund",
	"adjust_position",
	"check_alerts",
	"generate_report",
}

var agentCapabilities = []AgentCapability{
	{Tool: "get_full_dashboard", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "一次性读取组合、harness、数据质量、来源事件和市场缓存"},
	{Tool: "get_portfolio_summary", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取组合交易、持仓和收益概览"},
	{Tool: "get_portfolio_xirr", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取组合 XIRR 事实"},
	{Tool: "get_portfolio_timeline", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取组合价值时间线"},
	{Tool: "get_portfolio_allocation", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取资产类型、市场和主题配置"},
	{Tool: "get_portfolio_penetration", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取基金穿透底层股票暴露"},
	{Tool: "list_portfolios", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "列出组合定义"},
	{Tool: "list_dca_plans", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "列出定投计划规则（facts-only）"},
	{Tool: "get_fund_detail", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取单个证券的交易、价格和持仓事实"},
	{Tool: "get_fund_status", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取单证券管理状态"},
	{Tool: "get_fund_xirr", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取单证券 XIRR"},
	{Tool: "get_fund_drawdown", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取最大回撤事实"},
	{Tool: "get_nav_history", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取历史 NAV/价格序列"},
	{Tool: "search_funds", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "按名称/代码搜索证券"},
	{Tool: "search_stocks", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "本地股票档案搜索"},
	{Tool: "get_us_stock", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取缓存美股行情/档案"},
	{Tool: "get_market_indices", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取缓存指数"},
	{Tool: "get_investment_source_brief", Scope: "external_context", Permission: "allowed", RiskLevel: "low", UseFor: "生成 AI Agent/搜索服务 可消费的消息源查询"},
	{Tool: "get_source_events", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取已抓取新闻、公告和搜索结果队列"},
	{Tool: "get_investment_harness_snapshot", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取 agent harness 边界与可用工具"},
	{Tool: "get_data_freshness", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "检查 NAV/价格新鲜度和可行动补数入口"},
	{Tool: "verify_data", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "检查缺失净值、负持仓、空结算日等数据完整性问题"},
	{Tool: "get_system_status", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "读取系统诊断事实"},
	{Tool: "compute_dca_amount", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "运行事实型 DCA 模拟，不产生扣款指令"},
	{Tool: "run_backtest", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "运行历史策略模拟，不产生交易指令"},
	{Tool: "compare_funds", Scope: "read", Permission: "allowed", RiskLevel: "low", UseFor: "多证券历史对比事实"},
	{Tool: "crawl_nav", Scope: "maintenance", Permission: "allowed", RiskLevel: "medium", UseFor: "刷新本地价格/NAV；优先 stale_only=true（仅过期/缺失持仓）"},
	{Tool: "crawl_fund_holdings", Scope: "maintenance", Permission: "allowed", RiskLevel: "medium", UseFor: "刷新基金季报持仓穿透数据"},
	{Tool: "recalculate_snapshot", Scope: "maintenance", Permission: "allowed", RiskLevel: "medium", UseFor: "重建 portfolio_snapshot；all-mode 返回 failed_codes[] + status complete|partial|error"},
	{Tool: "mark_source_event", Scope: "write", Permission: "allowed", RiskLevel: "low", UseFor: "标记来源事件已读或有用，优化后续队列"},
	{Tool: "add_transaction", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "写入交易流水；需要确认协议（#5）"},
	{Tool: "update_transaction", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "修改交易流水；需要确认协议（#5）"},
	{Tool: "delete_transaction", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "删除交易流水；需要确认协议（#5）"},
	{Tool: "import_transactions", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "批量导入交易流水；需要确认协议（#5）"},
	{Tool: "upsert_dca_plan", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "创建/更新定投计划；需要确认协议（#5）"},
	{Tool: "disable_dca_plan", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "停用定投计划；需要确认协议（#5）"},
	{Tool: "run_dca_auto_invest", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "执行/预览到期定投（本地账本）；需要确认协议（#5）"},
	{Tool: "add_fund", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "新增基金主数据；需要确认协议（#5）"},
	{Tool: "add_security", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "新增证券主数据；需要确认协议（#5）"},
	{Tool: "update_fund", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "更新证券主数据；需要确认协议（#5）"},
	{Tool: "delete_fund", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "级联删除证券主数据；需要确认协议（#5）"},
	{Tool: "adjust_position", Scope: "write", Permission: "requires_confirmation", RiskLevel: "high", UseFor: "手动覆盖持仓份额；需要确认协议（#5）"},
	{Tool: "check_alerts", Scope: "external_context", Permission: "requires_confirmation", RiskLevel: "medium", UseFor: "扫描价格/回撤/陈旧NAV/定投日告警（facts-only，无 webhook）"},
	{Tool: "generate_report", Scope: "external_context", Permission: "requires_confirmation", RiskLevel: "medium", UseFor: "生成 JSON 组合报告（无 PDF v1）；需要确认协议（#5）"},
	{Tool: "broker_trade_execution", Scope: "write", Permission: "disabled", RiskLevel: "high", UseFor: "本系统不连接券商执行层"},
	{Tool: "backup_producer", Scope: "maintenance", Permission: "disabled", RiskLevel: "high", UseFor: "按当前运维边界，备份生产器未启用"},
}

func defaultAgentPermissions() AgentPermissions {
	return AgentPermissions{
		DecisionBoundary: "facts_only",
		ReadScope: []string{
			"portfolio", "transactions", "prices", "allocation", "fund_holdings",
			"source_events", "market_indices", "admin_health", "dca_plans", "reports_json",
		},
		WriteScope: []string{
			"source_event_feedback", "data_refresh", "transaction_records_with_confirmation",
			"dca_plans_with_confirmation", "security_master_with_confirmation", "position_override_with_confirmation",
		},
		RequiresConfirmation: []string{
			"add_transaction", "update_transaction", "delete_transaction", "import_transactions",
			"upsert_dca_plan", "disable_dca_plan", "run_dca_auto_invest",
			"add_fund", "add_security", "update_fund", "delete_fund",
			"adjust_position", "check_alerts", "generate_report",
		},
		DisabledOperations: []string{
			"broker_trade_execution", "cash_transfer", "backup_producer",
		},
	}
}

// harnessDiscovery returns tools/permissions/capabilities for the given audience.
// Public matches PUBLIC MCP least privilege (#60/#61/#64); operator restores full harness surface (#65).
func harnessDiscovery(audience HarnessAudience) ([]string, AgentPermissions, []AgentCapability) {
	if audience == HarnessAudienceOperator {
		return append([]string(nil), availableAgentTools...), defaultAgentPermissions(), append([]AgentCapability(nil), agentCapabilities...)
	}
	return publicAvailableAgentTools(), publicAgentPermissions(), publicAgentCapabilities()
}

// publicAvailableAgentTools is the unauthenticated harness discovery surface.
// Matches PUBLIC MCP tools/list least privilege (#60/#61/#64): read-only tools only.
func publicAvailableAgentTools() []string {
	out := make([]string, 0, len(availableAgentTools))
	for _, name := range availableAgentTools {
		if isPublicDiscoverableTool(name) {
			out = append(out, name)
		}
	}
	return out
}

func publicAgentCapabilities() []AgentCapability {
	out := make([]AgentCapability, 0, len(agentCapabilities))
	for _, cap := range agentCapabilities {
		if isPublicDiscoverableTool(cap.Tool) {
			out = append(out, cap)
		}
	}
	return out
}

func publicAgentPermissions() AgentPermissions {
	return AgentPermissions{
		DecisionBoundary: "facts_only",
		ReadScope: []string{
			"portfolio", "transactions", "prices", "allocation", "fund_holdings",
			"source_events", "market_indices", "admin_health", "dca_plans",
		},
		WriteScope:           []string{},
		RequiresConfirmation: []string{},
		DisabledOperations: []string{
			"broker_trade_execution", "cash_transfer", "backup_producer",
			"mcp_write_tools", "maintenance_tools", "confirmation_gated_tools",
		},
	}
}

// filterRecommendedAgentActions drops recommendations the audience cannot invoke.
func filterRecommendedAgentActions(actions []RecommendedAgentAction, allowedTools []string) []RecommendedAgentAction {
	if len(actions) == 0 {
		return actions
	}
	allowed := make(map[string]struct{}, len(allowedTools))
	for _, name := range allowedTools {
		allowed[name] = struct{}{}
	}
	out := make([]RecommendedAgentAction, 0, len(actions))
	for _, action := range actions {
		if _, ok := allowed[action.Tool]; ok {
			out = append(out, action)
		}
	}
	return out
}

func isPublicDiscoverableTool(name string) bool {
	switch name {
	case
		"get_full_dashboard",
		"get_portfolio_summary",
		"get_portfolio_xirr",
		"get_portfolio_timeline",
		"get_portfolio_allocation",
		"get_portfolio_penetration",
		"list_portfolios",
		"list_dca_plans",
		"get_fund_detail",
		"get_fund_status",
		"get_fund_xirr",
		"get_fund_drawdown",
		"get_nav_history",
		"search_funds",
		"search_stocks",
		"get_us_stock",
		"get_market_indices",
		"get_data_freshness",
		"verify_data",
		"get_system_status",
		"get_source_events",
		"get_investment_source_brief",
		"get_investment_harness_snapshot",
		"compute_dca_amount",
		"run_backtest",
		"compare_funds":
		return true
	default:
		return false
	}
}
