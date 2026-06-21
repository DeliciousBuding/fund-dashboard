/** /api/docs — OpenAPI 3.0 JSON spec + Swagger UI
 *
 *  GET /api/docs     → OpenAPI 3.0 JSON (50+ REST endpoints + 38 MCP tools)
 *  GET /api/docs/ui  → embedded Swagger UI HTML rendering the spec
 *
 *  v1.0 — 2026-06-19
 */

import { Hono } from "hono";

const router = new Hono();

// ═══════════════════════════════════════════════════════════════════════
// OpenAPI 3.0 Spec
// ═══════════════════════════════════════════════════════════════════════

function buildOpenApiSpec(): object {
  return {
    openapi: "3.0.3",
    info: {
      title: "Fund Dashboard API",
      description:
        "投资组合管理 REST API —— 基金/股票持仓追踪、价格爬取、交易记录、风险分析、PDF报告生成、MCP AI Agent 工具集。\n\n" +
        "**认证**: Admin 端点需要 `Authorization: Bearer <MCP_API_KEY>` header。\n\n" +
        "**MCP**: `/mcp` 端点提供 34 个 AI Agent 工具（需认证），使用 Streamable HTTP 传输。",
      version: "2.5.0",
      contact: { name: "VectorControl", url: "https://your-fund-domain.example.com" },
    },
    servers: [
      { url: "http://localhost:8765", description: "本地开发" },
      { url: "https://your-fund-domain.example.com", description: "生产环境" },
    ],
    tags: [
      { name: "Portfolio", description: "投资组合总览、XIRR、时间线、穿透分析、资产配置" },
      { name: "Funds", description: "基金/证券查询、详情、净值历史、回撤、DCA计算" },
      { name: "Analysis", description: "多基金对比（XIRR/波动/Sharpe/MaxDD/Calmar）" },
      { name: "Market", description: "美股指数行情、汇率、SSE实时推送" },
      { name: "Stocks", description: "美股个股详情、历史K线" },
      { name: "Report", description: "PDF投资报告（周报/月报）" },
      { name: "Export", description: "数据导出（Excel xlsx）" },
      { name: "Admin", description: "管理端点：诊断/CRUD/导入/爬虫/完整性/备份（需认证）" },
      { name: "MCP", description: "MCP AI Agent 工具集（34 tools via Streamable HTTP）" },
      { name: "System", description: "健康检查、状态、概览" },
    ],
    paths: {
      // ── System ──
      "/api/health": {
        get: {
          tags: ["System"],
          summary: "健康检查",
          description: "返回服务状态和运行时间。数据库连通性通过 SELECT 1 验证。",
          responses: {
            "200": { description: "OK", content: { "application/json": { schema: { $ref: "#/components/schemas/HealthResponse" } } } },
            "500": { description: "数据库连接失败" },
          },
        },
      },
      "/api/status": {
        get: {
          tags: ["System"],
          summary: "状态（重定向）",
          description: "重定向到 /api/admin/status。",
          responses: { "302": { description: "重定向到 /api/admin/status" } },
        },
      },
      "/api/summary": {
        get: {
          tags: ["System"],
          summary: "按基金汇总",
          description: "返回 summary_by_fund 视图的全部数据。",
          responses: { "200": { description: "汇总数据数组" } },
        },
      },

      // ── Portfolio ──
      "/api/portfolio": {
        get: {
          tags: ["Portfolio"],
          summary: "投资组合全貌",
          description: "总交易数、持仓分布、定投/手动统计、结算日分布、按证券类型分类、交易类型明细。",
          responses: { "200": { description: "组合摘要", content: { "application/json": { schema: { $ref: "#/components/schemas/PortfolioSummary" } } } } },
        },
      },
      "/api/portfolio/xirr": {
        get: {
          tags: ["Portfolio"],
          summary: "组合年化收益率",
          description: "整个投资组合的合并 XIRR 年化收益率。",
          responses: { "200": { description: "{ xirr: number|null }" } },
        },
      },
      "/api/portfolio/timeline": {
        get: {
          tags: ["Portfolio"],
          summary: "每日总资产时间线",
          description: "每日总资产=持仓份额×当日净值求和，用于画资产走势图。",
          responses: { "200": { description: "时间线数组 [{date, total_value, total_cost, pnl}]" } },
        },
      },
      "/api/portfolio/penetration": {
        get: {
          tags: ["Portfolio"],
          summary: "股权穿透分析",
          description: "通过基金持仓数据穿透计算底层股票实际持有权重和金额。",
          responses: { "200": { description: "穿透分析结果" } },
        },
      },
      "/api/portfolio/by-type": {
        get: {
          tags: ["Portfolio"],
          summary: "按证券类型分类",
          description: "从组合摘要中提取 by_security_type 分类数据（fund/stock）。",
          responses: { "200": { description: "按类型分组数组" } },
        },
      },
      "/api/portfolio/allocation": {
        get: {
          tags: ["Portfolio"],
          summary: "资产配置明细",
          description: "按证券类型、市场、主题聚合的配置摘要和风险提示。",
          responses: { "200": { description: "配置数据（含 by_type/by_market/by_theme/risk_notes）" } },
        },
      },
      "/api/portfolio/harness": {
        get: {
          tags: ["Portfolio"],
          summary: "Investment Harness 快照",
          description: "Hermes/Agent 使用的金融 Harness 事实快照：持仓、配置、价格信号、数据质量。",
          responses: { "200": { description: "Harness 快照 JSON" } },
        },
      },
      "/api/portfolio/source-brief": {
        get: {
          tags: ["Portfolio"],
          summary: "消息源与爬取上下文",
          description: "为 Hermes/WebSearch 生成搜索 query、外部 source target 和本地 MCP 补数入口。",
          parameters: [{ name: "limit", in: "query", schema: { type: "integer", default: 20 }, description: "返回条数上限" }],
          responses: { "200": { description: "搜索上下文 JSON" } },
        },
      },
      "/api/portfolio/source-events": {
        get: {
          tags: ["Portfolio"],
          summary: "获取来源事件队列",
          description: "已抓取的新闻/公告/搜索事件。支持按证券代码、来源、已读状态过滤。",
          parameters: [
            { name: "code", in: "query", schema: { type: "string" }, description: "证券代码过滤" },
            { name: "source", in: "query", schema: { type: "string" }, description: "来源过滤（websearch/eastmoney/yahoo）" },
            { name: "show_read", in: "query", schema: { type: "string" }, description: "是否包含已读事件（0/1）" },
            { name: "limit", in: "query", schema: { type: "integer", default: 30 }, description: "返回条数上限" },
          ],
          responses: { "200": { description: "事件数组" } },
        },
        post: {
          tags: ["Portfolio"],
          summary: "创建来源事件",
          description: "手动创建一个来源事件记录。",
          requestBody: { required: true, content: { "application/json": { schema: { $ref: "#/components/schemas/CreateSourceEvent" } } } },
          responses: { "201": { description: "创建成功" }, "400": { description: "缺少必要字段" } },
        },
      },
      "/api/portfolio/source-events/{id}": {
        patch: {
          tags: ["Portfolio"],
          summary: "标记来源事件",
          description: "标记来源事件为已读/有用。",
          parameters: [{ name: "id", in: "path", required: true, schema: { type: "integer" } }],
          requestBody: { content: { "application/json": { schema: { type: "object", properties: { is_read: { type: "boolean" }, is_useful: { type: "boolean" } } } } } },
          responses: { "200": { description: "更新成功" }, "404": { description: "事件不存在" } },
        },
      },
      "/api/portfolio/analysis/backtest": {
        post: {
          tags: ["Portfolio"],
          summary: "策略回测",
          description: "对指定基金的历史净值运行策略回测（grid/momentum/rebalance/dca）。",
          requestBody: { required: true, content: { "application/json": { schema: { $ref: "#/components/schemas/BacktestRequest" } } } },
          responses: { "200": { description: "回测结果" }, "400": { description: "参数错误" }, "404": { description: "无净值数据" } },
        },
      },

      // ── Funds / Securities ──
      "/api/funds": {
        get: {
          tags: ["Funds"],
          summary: "列出全部证券",
          description: "返回所有证券列表（含基金和股票），含持仓快照数据。",
          responses: { "200": { description: "证券列表数组" } },
        },
      },
      "/api/funds/{code}": {
        get: {
          tags: ["Funds"],
          summary: "证券详情",
          description: "单只证券完整详情：交易记录、持仓、收益率、XIRR、交易状态。",
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" }, description: "6位代码" }],
          responses: { "200": { description: "详情 JSON" }, "404": { description: "未找到" } },
        },
      },
      "/api/funds/{code}/nav": {
        get: {
          tags: ["Funds"],
          summary: "净值/价格历史",
          description: "返回指定证券的全部净值历史记录。",
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          responses: { "200": { description: "[{date, unit_nav}] 数组" } },
        },
      },
      "/api/funds/{code}/xirr": {
        get: {
          tags: ["Funds"],
          summary: "单只证券 XIRR",
          description: "计算单只证券的年化收益率（XIRR）。需至少2笔 buy/sell/dividend 记录。",
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          responses: { "200": { description: "{ xirr: number|null, code }" } },
        },
      },
      "/api/funds/{code}/drawdown": {
        get: {
          tags: ["Funds"],
          summary: "最大回撤",
          description: "计算历史最大回撤（峰值→谷底百分比）。",
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          responses: { "200": { description: "{ max_drawdown, peak_date, trough_date }" }, "404": { description: "无净值数据" } },
        },
      },
      "/api/funds/{code}/dca": {
        get: {
          tags: ["Funds"],
          summary: "定投金额计算",
          description: "Value Averaging DCA 定投金额计算器。支持成本偏离模式(nav_deviation)和涨跌幅模式(change_pct)。",
          parameters: [
            { name: "code", in: "path", required: true, schema: { type: "string" } },
            { name: "base", in: "query", schema: { type: "number", default: 30 }, description: "基础定投金额" },
            { name: "mode", in: "query", schema: { type: "string", enum: ["nav_deviation", "change_pct"] }, description: "定投模式" },
          ],
          responses: { "200": { description: "定投计划 {amount, signal, range, ...}" }, "400": { description: "无持仓或数据不足" } },
        },
      },
      "/api/funds/summary": {
        get: {
          tags: ["Funds"],
          summary: "按基金汇总",
          description: "getSummaryByFund() 的结果。",
          responses: { "200": { description: "汇总数组" } },
        },
      },

      // ── Securities alias (same router as funds) ──
      "/api/securities": {
        get: {
          tags: ["Funds"],
          summary: "列出全部证券（别名）",
          description: "同 /api/funds。向后兼容别名。",
          responses: { "200": { description: "证券列表" } },
        },
      },
      "/api/securities/{code}": {
        get: {
          tags: ["Funds"],
          summary: "证券详情（别名）",
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          responses: { "200": { description: "详情" }, "404": { description: "未找到" } },
        },
      },
      "/api/securities/{code}/nav": {
        get: {
          tags: ["Funds"],
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          responses: { "200": { description: "净值历史" } },
        },
      },
      "/api/securities/{code}/xirr": {
        get: {
          tags: ["Funds"],
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          responses: { "200": { description: "{ xirr }" } },
        },
      },
      "/api/securities/{code}/drawdown": {
        get: {
          tags: ["Funds"],
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          responses: { "200": { description: "回撤数据" } },
        },
      },
      "/api/securities/{code}/dca": {
        get: {
          tags: ["Funds"],
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          responses: { "200": { description: "定投计划" } },
        },
      },
      "/api/securities/summary": {
        get: { tags: ["Funds"], responses: { "200": { description: "汇总" } } },
      },

      // ── Analysis ──
      "/api/analysis/compare": {
        get: {
          tags: ["Analysis"],
          summary: "多基金对比",
          description: "对多个基金进行横向对比：XIRR、年化波动率、Sharpe、最大回撤、Calmar比率。",
          parameters: [{ name: "codes", in: "query", required: true, schema: { type: "string" }, description: "逗号分隔的基金代码列表" }],
          responses: { "200": { description: "{ funds: [{code, name, xirr, volatility, sharpe, max_drawdown, calmar}] }" }, "400": { description: "缺少 codes 参数" } },
        },
      },

      // ── Market ──
      "/api/market/indices": {
        get: {
          tags: ["Market"],
          summary: "美股指数行情",
          description: "返回本地缓存的美股指数数据（^IXIC, ^GSPC, ^DJI, ^NDX）。",
          responses: { "200": { description: "指数数组 [{code, name, price, change_pct, change_amt, updated_at}]" } },
        },
      },
      "/api/market/index/{code}": {
        get: {
          tags: ["Market"],
          summary: "实时指数行情",
          description: "从 Yahoo Finance 获取单只指数实时行情并持久化。支持 ^IXIC/^GSPC/^DJI/^NDX，可省略 ^。",
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" }, description: "指数代码（如 ^IXIC 或 IXIC）" }],
          responses: { "200": { description: "指数行情（含 source: live|cache）" }, "404": { description: "未找到" }, "502": { description: "获取失败" } },
        },
      },
      "/api/market/index/{code}/history": {
        get: {
          tags: ["Market"],
          summary: "指数历史数据",
          description: "从 Yahoo Finance 获取指数历史K线。",
          parameters: [
            { name: "code", in: "path", required: true, schema: { type: "string" } },
            { name: "range", in: "query", schema: { type: "string", default: "1y" }, description: "时间范围（1d/5d/1mo/3mo/6mo/1y/2y/5y）" },
          ],
          responses: { "200": { description: "{ symbol, count, range, data: [{date, close, change_pct}] }" }, "404": { description: "无数据" }, "502": { description: "获取失败" } },
        },
      },
      "/api/market/exchange-rate": {
        get: {
          tags: ["Market"],
          summary: "美元/人民币汇率",
          description: "当前 USD/CNY 汇率（Yahoo Finance）。",
          responses: { "200": { description: "汇率 JSON" }, "502": { description: "获取失败" } },
        },
      },
      "/api/market/stream": {
        get: {
          tags: ["Market"],
          summary: "SSE 实时价格推送",
          description: "Server-Sent Events 端点，每60秒推送一次指数价格。事件类型: indices。",
          responses: { "200": { description: "SSE 流（text/event-stream）" } },
        },
      },

      // ── Stocks ──
      "/api/stocks/{code}": {
        get: {
          tags: ["Stocks"],
          summary: "美股个股详情",
          description: "获取美股实时行情+历史K线+公司档案。数据来源：Yahoo Finance。",
          parameters: [
            { name: "code", in: "path", required: true, schema: { type: "string" }, description: "美股代码（如 AAPL）" },
            { name: "market", in: "query", schema: { type: "string", default: "US" }, description: "市场（当前仅 US）" },
            { name: "range", in: "query", schema: { type: "string", default: "1y" }, description: "历史数据范围" },
          ],
          responses: { "200": { description: "个股详情（含 profile + history）" }, "404": { description: "未找到" }, "502": { description: "获取失败" } },
        },
      },

      // ── Report ──
      "/api/report/weekly": {
        get: {
          tags: ["Report"],
          summary: "周报 PDF",
          description: "生成7天投资周报。format=html 返回 HTML 预览；默认返回 PDF 二进制。",
          parameters: [{ name: "format", in: "query", schema: { type: "string", enum: ["pdf", "html"] }, description: "输出格式" }],
          responses: { "200": { description: "PDF 二进制或 HTML" }, "500": { description: "生成失败" } },
        },
      },
      "/api/report/monthly": {
        get: {
          tags: ["Report"],
          summary: "月报 PDF",
          description: "生成30天投资月报。format=html 返回 HTML 预览；默认返回 PDF 二进制。",
          parameters: [{ name: "format", in: "query", schema: { type: "string", enum: ["pdf", "html"] } }],
          responses: { "200": { description: "PDF 二进制或 HTML" }, "500": { description: "生成失败" } },
        },
      },

      // ── Export ──
      "/api/export/transactions-xlsx": {
        post: {
          tags: ["Export"],
          summary: "导出交易记录为 Excel",
          description: "将前端传来的交易记录数组导出为 .xlsx 文件。",
          requestBody: { required: true, content: { "application/json": { schema: { $ref: "#/components/schemas/XlsxExportRequest" } } } },
          responses: { "200": { description: "xlsx 二进制文件", content: { "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {} } }, "400": { description: "无效请求" } },
        },
      },

      // ── Admin: Dashboard ──
      "/api/admin/dashboard": {
        get: {
          tags: ["Admin"],
          summary: "监控面板",
          description: "聚合系统指标：DB大小、爬虫成功率、API延迟、内存、uptime。公开端点。",
          responses: { "200": { description: "监控面板 JSON" } },
        },
      },

      // ── Admin: CRUD ──
      "/api/admin/transactions/{seq}": {
        delete: {
          tags: ["Admin"],
          summary: "删除交易记录",
          description: "按 seq 序号删除交易。自动重算快照。需认证。",
          parameters: [{ name: "seq", in: "path", required: true, schema: { type: "integer" } }],
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "删除成功" }, "400": { description: "seq 无效" }, "404": { description: "未找到" } },
        },
        put: {
          tags: ["Admin"],
          summary: "更新交易记录",
          description: "更新交易字段（trade_time/confirm_date/direction/amount/shares/fee/fund_code）。自动重算。需认证。",
          parameters: [{ name: "seq", in: "path", required: true, schema: { type: "integer" } }],
          security: [{ bearerAuth: [] }],
          requestBody: { content: { "application/json": { schema: { $ref: "#/components/schemas/UpdateTransaction" } } } },
          responses: { "200": { description: "更新成功" }, "404": { description: "未找到" } },
        },
      },
      "/api/admin/securities": {
        post: {
          tags: ["Admin"],
          summary: "创建证券",
          description: "添加新证券（基金或股票）。需认证。",
          security: [{ bearerAuth: [] }],
          requestBody: { required: true, content: { "application/json": { schema: { $ref: "#/components/schemas/CreateSecurity" } } } },
          responses: { "200": { description: "创建成功" } },
        },
      },
      "/api/admin/securities/{code}": {
        put: {
          tags: ["Admin"],
          summary: "更新证券信息",
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          security: [{ bearerAuth: [] }],
          requestBody: { content: { "application/json": { schema: { type: "object", properties: { fund_name: { type: "string" }, fund_type: { type: "string" }, security_type: { type: "string" }, market: { type: "string" } } } } } },
          responses: { "200": { description: "更新成功" }, "404": { description: "未找到" } },
        },
        delete: {
          tags: ["Admin"],
          summary: "删除证券",
          description: "删除证券及全部关联数据（交易/净值/持仓/状态）。不可逆！需认证。",
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "删除成功" }, "404": { description: "未找到" } },
        },
      },
      "/api/admin/funds": {
        post: {
          tags: ["Admin"],
          summary: "创建基金（向后兼容）",
          security: [{ bearerAuth: [] }],
          requestBody: { required: true, content: { "application/json": { schema: { type: "object", required: ["fund_code", "fund_name"], properties: { fund_code: { type: "string" }, fund_name: { type: "string" }, fund_type: { type: "string" }, security_type: { type: "string" }, market: { type: "string" } } } } } },
          responses: { "200": { description: "创建成功" } },
        },
      },
      "/api/admin/funds/{code}": {
        put: {
          tags: ["Admin"],
          summary: "更新基金（向后兼容）",
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "更新成功" }, "404": { description: "未找到" } },
        },
        delete: {
          tags: ["Admin"],
          summary: "删除基金（向后兼容）",
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "删除成功" }, "404": { description: "未找到" } },
        },
      },

      // ── Admin: Ops (diagnostics + data ops + integrity + backup) ──
      "/api/admin/status": {
        get: {
          tags: ["Admin"],
          summary: "系统诊断",
          description: "交易数/净值覆盖面/异常记录/持仓数/证券统计。需认证。",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "诊断 JSON" } },
        },
      },
      "/api/admin/status/{code}": {
        get: {
          tags: ["Admin"],
          summary: "单只证券诊断",
          parameters: [{ name: "code", in: "path", required: true, schema: { type: "string" } }],
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "证券诊断详情" } },
        },
      },
      "/api/admin/import-transactions": {
        post: {
          tags: ["Admin"],
          summary: "批量导入交易",
          description: "通过结构化 JSON 批量导入交易记录。需认证。",
          security: [{ bearerAuth: [] }],
          requestBody: { required: true, content: { "application/json": { schema: { $ref: "#/components/schemas/ImportTransactionsBody" } } } },
          responses: { "200": { description: "{ ok, imported, total, affected_funds }" }, "500": { description: "插入失败" } },
        },
      },
      "/api/admin/adjust-position": {
        post: {
          tags: ["Admin"],
          summary: "调整持仓份额",
          security: [{ bearerAuth: [] }],
          requestBody: { required: true, content: { "application/json": { schema: { type: "object", required: ["fund_code", "shares"], properties: { fund_code: { type: "string" }, shares: { type: "number" } } } } } },
          responses: { "200": { description: "{ ok: true }" } },
        },
      },
      "/api/admin/recalculate-snapshot": {
        post: {
          tags: ["Admin"],
          summary: "重建持仓快照",
          description: "完全清除并重建 portfolio_snapshot 表。需认证。",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "{ ok, funds }" } },
        },
      },
      "/api/admin/verify": {
        get: {
          tags: ["Admin"],
          summary: "数据校验",
          description: "检查缺失净值、负持仓、空结算日。需认证。",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "{ ok, issues }" } },
        },
      },
      "/api/admin/db-integrity": {
        get: {
          tags: ["Admin"],
          summary: "数据库完整性检查",
          description: "运行完整性检查（表结构/FK/索引/数据一致性）。需认证。",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "{ healthy, checks, ... }" } },
        },
      },
      "/api/admin/db-repair": {
        post: {
          tags: ["Admin"],
          summary: "自动修复数据库",
          description: "尝试自动修复检测到的完整性问题。需认证。",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "修复结果" } },
        },
      },
      "/api/admin/db-restore": {
        post: {
          tags: ["Admin"],
          summary: "从备份恢复",
          description: "从指定备份文件或最新备份恢复数据库。需认证。",
          security: [{ bearerAuth: [] }],
          requestBody: { content: { "application/json": { schema: { type: "object", properties: { backup_file: { type: "string" } } } } } },
          responses: { "200": { description: "恢复结果" }, "404": { description: "备份文件不存在" } },
        },
      },
      "/api/admin/backup-status": {
        get: {
          tags: ["Admin"],
          summary: "备份状态",
          description: "查看备份文件列表和状态。需认证。",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "{ status, latest, latest_age_hours, count }" } },
        },
      },

      // ── Admin: Freshness & Alerts ──
      "/api/admin/freshness": {
        get: {
          tags: ["Admin"],
          summary: "数据新鲜度检查",
          description: "最后交易日期、最新净值日期、过期证券清单（>2天）、健康评分。需认证。",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "{ health, stale_securities, actionable, ... }" } },
        },
      },
      "/api/admin/stale-report": {
        get: {
          tags: ["Admin"],
          summary: "过期数据报告",
          description: "详细的过期/缺失价格数据报告，含永不过期推荐。需认证。",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "过期报告 JSON" } },
        },
      },
      "/api/admin/alerts/check": {
        post: {
          tags: ["Admin"],
          summary: "手动触发告警检查",
          description: "扫描价格异动/回撤/数据过期/定投日，并通过飞书 Webhook 发送告警。需认证。",
          security: [{ bearerAuth: [] }],
          requestBody: { content: { "application/json": { schema: { type: "object", properties: { price_change_pct: { type: "number" }, drawdown_pct: { type: "number" }, stale_days: { type: "number" } } } } } },
          responses: { "200": { description: "告警列表" } },
        },
      },
      "/api/admin/alerts/config": {
        get: {
          tags: ["Admin"],
          summary: "告警配置",
          description: "查看当前告警配置和 Webhook 状态。需认证。",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "告警配置 JSON" } },
        },
      },
      "/api/admin/feishu/event": {
        post: {
          tags: ["Admin"],
          summary: "飞书事件回调",
          description: "处理飞书机器人事件回调。需认证。",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "回调处理结果" } },
        },
      },
      "/api/admin/feishu/status": {
        get: {
          tags: ["Admin"],
          summary: "飞书机器人状态",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "机器人状态" } },
        },
      },

      // ── Admin: Import (CSV + Crawler) ──
      "/api/admin/import-csv": {
        post: {
          tags: ["Admin"],
          summary: "CSV 导入交易",
          description: "从 CSV 文本批量导入交易记录。自动检测中文/英文列名，支持基金和股票代码。需认证。",
          security: [{ bearerAuth: [] }],
          requestBody: { required: true, content: { "application/json": { schema: { type: "object", required: ["csv"], properties: { csv: { type: "string", description: "CSV 文本内容（含 header）" } } } } } },
          responses: { "200": { description: "{ ok, imported, total, affected_funds }" }, "400": { description: "CSV 格式错误或缺少必要列" } },
        },
      },
      "/api/admin/crawl-nav": {
        post: {
          tags: ["Admin"],
          summary: "触发 NAV 爬取",
          description: "刷新单只证券或全部持仓的最新价格数据。需认证。",
          security: [{ bearerAuth: [] }],
          requestBody: { content: { "application/json": { schema: { type: "object", properties: { code: { type: "string" }, type: { type: "string", enum: ["fund", "stock"] } } } } } },
          responses: { "200": { description: "爬取结果或已启动" } },
        },
      },
      "/api/admin/refresh-holdings": {
        post: {
          tags: ["Admin"],
          summary: "刷新基金持仓明细",
          description: "从东方财富爬取基金季度持仓。需认证。",
          security: [{ bearerAuth: [] }],
          requestBody: { content: { "application/json": { schema: { type: "object", properties: { code: { type: "string" } } } } } },
          responses: { "200": { description: "刷新结果" }, "404": { description: "无持仓数据" } },
        },
      },
      "/api/admin/crawl-holdings": {
        post: {
          tags: ["Admin"],
          summary: "爬取基金持仓（别名）",
          security: [{ bearerAuth: [] }],
          requestBody: { content: { "application/json": { schema: { type: "object", properties: { code: { type: "string" } } } } } },
          responses: { "200": { description: "爬取已启动" } },
        },
      },
      "/api/admin/holdings-coverage": {
        get: {
          tags: ["Admin"],
          summary: "持仓覆盖率",
          description: "返回持仓数据覆盖统计（按基金类型分组）。需认证。",
          security: [{ bearerAuth: [] }],
          responses: { "200": { description: "覆盖率统计 JSON" } },
        },
      },

      // ── MCP ──
      "/mcp": {
        get: { tags: ["MCP"], summary: "MCP GET (SSE)" },
        post: { tags: ["MCP"], summary: "MCP POST (JSON-RPC)" },
        delete: { tags: ["MCP"], summary: "MCP DELETE (session)" },
      },
    },

    // ═══════════════════════════════════════════════════════════════════
    // MCP Tools (x-mcp-tools extension)
    // ═══════════════════════════════════════════════════════════════════
    "x-mcp-tools": [
      // ── Query (5 tools) ──
      { name: "search_funds", group: "Query", description: "搜索基金或股票，返回匹配列表。可按名称、代码、类型搜索。" },
      { name: "get_fund_detail", group: "Query", description: "获取单只证券的完整详情：所有交易记录、持仓、收益率、XIRR、交易状态" },
      { name: "get_nav_history", group: "Query", description: "获取证券历史价格数据" },
      { name: "get_fund_xirr", group: "Query", description: "计算单只证券的年化收益率（XIRR）" },
      { name: "get_fund_drawdown", group: "Query", description: "计算证券历史最大回撤" },

      // ── Portfolio (6 tools) ──
      { name: "get_portfolio_summary", group: "Portfolio", description: "获取投资组合全貌：总资产、盈亏、持仓分布、定投/手动统计" },
      { name: "get_portfolio_xirr", group: "Portfolio", description: "计算整个投资组合的年化收益率（XIRR）" },
      { name: "get_portfolio_timeline", group: "Portfolio", description: "获取每日总资产时间线" },
      { name: "get_portfolio_allocation", group: "Portfolio", description: "获取组合资产配置：按类型、市场、主题聚合" },
      { name: "get_investment_harness_snapshot", group: "Portfolio", description: "获取 Hermes/Agent 金融 Harness 事实快照" },
      { name: "get_investment_source_brief", group: "Portfolio", description: "为 Hermes/WebSearch 生成消息源与爬取上下文" },

      // ── Transactions (4 tools) ──
      { name: "add_transaction", group: "Transactions", description: "添加一笔新交易记录（买入/卖出/分红）" },
      { name: "update_transaction", group: "Transactions", description: "修改一笔交易记录（按 seq 序号）" },
      { name: "delete_transaction", group: "Transactions", description: "删除一笔交易记录（按 seq 序号）" },
      { name: "import_transactions", group: "Transactions", description: "批量导入交易记录" },

      // ── Admin (7 tools) ──
      { name: "get_system_status", group: "Admin", description: "获取系统完整诊断信息，含全球市场交易时段" },
      { name: "get_fund_status", group: "Admin", description: "获取单只证券的管理状态" },
      { name: "verify_data", group: "Admin", description: "数据完整性校验" },
      { name: "get_data_freshness", group: "Admin", description: "检查数据新鲜度" },
      { name: "get_source_events", group: "Admin", description: "获取来源事件队列" },
      { name: "mark_source_event", group: "Admin", description: "标记来源事件已读/有用" },
      { name: "check_alerts", group: "Admin", description: "手动触发告警检查" },

      // ── Operations (3 tools) ──
      { name: "crawl_nav", group: "Operations", description: "触发净值/价格爬取" },
      { name: "recalculate_snapshot", group: "Operations", description: "完全重建 portfolio_snapshot 表" },
      { name: "adjust_position", group: "Operations", description: "手动调整持仓份额" },

      // ── Securities (4 tools) ──
      { name: "add_fund", group: "Securities", description: "添加新基金到系统" },
      { name: "add_security", group: "Securities", description: "添加新证券（基金或股票）到系统" },
      { name: "update_fund", group: "Securities", description: "更新证券信息" },
      { name: "delete_fund", group: "Securities", description: "删除证券及其所有关联数据" },

      // ── Market (5 tools) ──
      { name: "get_market_indices", group: "Market", description: "获取美股主要指数实时行情" },
      { name: "get_portfolio_penetration", group: "Market", description: "股权穿透分析——底层股票实际持有权重和金额" },
      { name: "get_us_stock", group: "Market", description: "获取美股实时行情和历史K线" },
      { name: "search_stocks", group: "Market", description: "搜索美股/A股/港股股票" },
      { name: "crawl_fund_holdings", group: "Market", description: "爬取基金持仓明细" },

      // ── Analysis (3 tools) ──
      { name: "compute_dca_amount", group: "Analysis", description: "定投金额计算器（成本偏离/涨跌幅模式）" },
      { name: "run_backtest", group: "Analysis", description: "策略回测引擎（grid/momentum/rebalance/dca）" },
      { name: "compare_funds", group: "Analysis", description: "基金多维度对比工具（XIRR/波动/Sharpe/MaxDD/Calmar）" },

      // ── Report (1 tool) ──
      { name: "generate_report", group: "Report", description: "生成PDF投资报告（周报/月报）" },
    ],

    // ═══════════════════════════════════════════════════════════════════
    // Components / Schemas
    // ═══════════════════════════════════════════════════════════════════
    components: {
      securitySchemes: {
        bearerAuth: { type: "http", scheme: "bearer", description: "MCP_API_KEY 环境变量的值" },
      },
      schemas: {
        HealthResponse: {
          type: "object",
          properties: {
            status: { type: "string", example: "ok" },
            uptime: { type: "number", description: "秒" },
          },
        },
        PortfolioSummary: {
          type: "object",
          properties: {
            total_tx: { type: "integer" },
            unique_funds: { type: "integer" },
            unique_stocks: { type: "integer" },
            held_funds: { type: "integer" },
            total_buy: { type: "number" },
            total_sell: { type: "number" },
            total_fee: { type: "number" },
            unrealized_pnl: { type: "number" },
            auto_tx: { type: "integer" },
            manual_tx: { type: "integer" },
            auto_amount: { type: "number" },
            manual_amount: { type: "number" },
            first_trade: { type: "string" },
            last_trade: { type: "string" },
          },
        },
        CreateSourceEvent: {
          type: "object",
          required: ["title"],
          properties: {
            title: { type: "string" },
            url: { type: "string" },
            source: { type: "string", enum: ["websearch", "eastmoney", "yahoo", "manual"] },
            snippet: { type: "string" },
            query: { type: "string" },
            related_security_code: { type: "string" },
            related_security_name: { type: "string" },
          },
        },
        BacktestRequest: {
          type: "object",
          required: ["fund_code", "start_date", "base_amount"],
          properties: {
            fund_code: { type: "string" },
            strategy: { type: "string", enum: ["grid", "momentum", "rebalance", "dca"], default: "dca" },
            start_date: { type: "string", description: "YYYY-MM-DD" },
            base_amount: { type: "number" },
            grid_levels: { type: "integer" },
            momentum_months: { type: "integer" },
            target_weight: { type: "number" },
            rebalance_interval: { type: "integer" },
          },
        },
        XlsxExportRequest: {
          type: "object",
          required: ["transactions"],
          properties: {
            transactions: { type: "array", items: { $ref: "#/components/schemas/TransactionRow" } },
            fundName: { type: "string" },
          },
        },
        TransactionRow: {
          type: "object",
          properties: {
            trade_time: { type: "string" },
            confirm_date: { type: "string" },
            direction: { type: "string", enum: ["buy", "sell", "dividend", "convert_in", "convert_out", "forced_redeem"] },
            amount: { type: "number" },
            shares: { type: "number" },
            nav: { type: "number" },
            inferred_nav: { type: "number" },
            fee: { type: "number" },
            settlement_days: { type: "integer" },
            trade_day_type: { type: "string" },
          },
        },
        CreateSecurity: {
          type: "object",
          required: ["fund_code", "fund_name"],
          properties: {
            fund_code: { type: "string", description: "6位代码" },
            fund_name: { type: "string" },
            fund_type: { type: "string" },
            security_type: { type: "string", enum: ["fund", "stock"] },
            market: { type: "string", description: "SH/SZ/HK/US" },
          },
        },
        UpdateTransaction: {
          type: "object",
          properties: {
            trade_time: { type: "string" },
            confirm_date: { type: "string" },
            direction: { type: "string", enum: ["buy", "sell", "dividend"] },
            trade_type: { type: "string" },
            confirm_amount: { type: "number" },
            confirm_share: { type: "number" },
            fee: { type: "number" },
            fund_code: { type: "string" },
          },
        },
        ImportTransactionsBody: {
          type: "object",
          required: ["transactions"],
          properties: {
            transactions: {
              type: "array",
              items: {
                type: "object",
                required: ["order_id", "trade_time", "direction", "fund_code", "confirm_amount"],
                properties: {
                  order_id: { type: "string" },
                  trade_time: { type: "string" },
                  confirm_date: { type: "string" },
                  trade_type: { type: "string" },
                  direction: { type: "string", enum: ["buy", "sell", "dividend"] },
                  fund_code: { type: "string" },
                  security_code: { type: "string" },
                  fund_name: { type: "string" },
                  confirm_amount: { type: "number" },
                  confirm_share: { type: "number" },
                  fee: { type: "number" },
                  inferred_nav: { type: "number" },
                  signed_cash_flow: { type: "number" },
                  signed_share_change: { type: "number" },
                },
              },
            },
          },
        },
      },
    },

    security: [],
    "x-endpoint-count": 50,
    "x-mcp-tool-count": 38,
  };
}

// ═══════════════════════════════════════════════════════════════════════
// Swagger UI HTML (inline, CDN-based)
// ═══════════════════════════════════════════════════════════════════════

function swaggerUiHtml(): string {
  return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Fund Dashboard API — Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    html { box-sizing: border-box; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin: 0; background: #fafafa; }
    .topbar { display: none; }
    .swagger-ui .info { margin: 20px 0; }
    .swagger-ui .info .title { font-size: 28px; }
    .swagger-ui .scheme-container { box-shadow: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js" crossorigin></script>
  <script>
    window.onload = () => {
      SwaggerUIBundle({
        url: "/api/docs",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
        layout: "BaseLayout",
        defaultModelsExpandDepth: 1,
        defaultModelExpandDepth: 1,
        docExpansion: "list",
        filter: true,
        showExtensions: true,
        showCommonExtensions: true,
        supportedSubmitMethods: ["get", "post", "put", "delete", "patch"],
      });
    };
  </script>
</body>
</html>`;
}

// ═══════════════════════════════════════════════════════════════════════
// Routes
// ═══════════════════════════════════════════════════════════════════════

router.get("/docs", (c) => {
  return c.json(buildOpenApiSpec());
});

router.get("/docs/ui", (c) => {
  return c.html(swaggerUiHtml());
});

export default router;
