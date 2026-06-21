/** MCP Portfolio Tools — summary, XIRR, timeline, allocation, harness, source brief */

import { z } from "zod";
import type { ToolRegistrar } from "../shared";
import { query } from "../../db";
import {
  getPortfolioSummary, getPortfolioXirr, getPortfolioTimeline,
  getPortfolioAllocation, getInvestmentHarnessSnapshot, getInvestmentSourceBrief,
} from "../../services/index";

const PortfolioIdArg = z.object({
  portfolio_id: z.number().optional().describe("组合ID（可选，默认1=默认组合）"),
});

export const registerPortfolioTools: ToolRegistrar = (server) => {
  server.tool("get_portfolio_summary", {
    description: "获取投资组合全貌：总资产、盈亏、持仓分布（含基金和股票）、定投/手动统计、结算日分布",
    inputSchema: PortfolioIdArg,
    handler: async (args) => {
      const pid = args.portfolio_id ?? 1;
      const s = getPortfolioSummary(pid);
      if (!s) {
        return { content: [{ type: "text", text: JSON.stringify({
          summary: {
            total_transactions: 0, unique_funds: 0, held_funds: 0,
            total_buy: 0, total_sell: 0, total_fee: 0, unrealized_pnl: 0,
            auto_invest: { tx: 0, amount: 0 }, manual_invest: { tx: 0, amount: 0 },
            date_range: { first: "", last: "" }, settlement_distribution: {},
          },
          holdings: [],
        }, null, 2) }] };
      }
      const held = query<any>(
        "SELECT ps.*, COALESCE(fd.security_type,'fund') as security_type, COALESCE(fd.market,'') as market FROM portfolio_snapshot ps LEFT JOIN fund_details fd ON ps.fund_code = fd.fund_code WHERE ps.held_shares > 0.001 AND ps.portfolio_id = ? ORDER BY ps.current_value DESC NULLS LAST",
        [pid],
      );
      return { content: [{ type: "text", text: JSON.stringify({
        summary: {
          total_transactions: s.total_tx, unique_funds: s.unique_funds, held_funds: held.length,
          total_buy: s.total_buy, total_sell: s.total_sell,
          total_fee: s.total_fee, unrealized_pnl: s.unrealized_pnl,
          auto_invest: { tx: s.auto_tx, amount: s.auto_amount },
          manual_invest: { tx: s.manual_tx, amount: s.manual_amount },
          date_range: { first: s.first_trade, last: s.last_trade },
          settlement_distribution: s.settlement_distribution,
        },
        holdings: held.map((h: any) => ({
          code: h.fund_code, name: h.fund_name, shares: +h.held_shares,
          security_type: h.security_type, market: h.market,
          cost: h.total_cost, value: h.current_value, pnl: h.unrealized_pnl, pnl_pct: h.pnl_pct,
          nav: h.latest_nav,
        })),
      }, null, 2) }] };
    },
  });

  server.tool("get_portfolio_xirr", {
    description: "计算整个投资组合的年化收益率（XIRR），含基金和股票",
    inputSchema: PortfolioIdArg,
    handler: async (args) => {
      const pid = args.portfolio_id ?? 1;
      const xirr = getPortfolioXirr(pid);
      const ps = query<any>("SELECT held_shares, latest_nav FROM portfolio_snapshot WHERE held_shares > 0.001 AND portfolio_id = ?", [pid]);
      let pv = 0; for (const r of ps) pv += (+r.held_shares) * (+r.latest_nav);
      return { content: [{ type: "text", text: JSON.stringify({ xirr_pct: xirr, current_portfolio_value: +pv.toFixed(2) }, null, 2) }] };
    },
  });

  server.tool("get_portfolio_timeline", {
    description: "获取每日总资产时间线（价格×持仓），用于画资产走势图。覆盖基金和股票。",
    inputSchema: PortfolioIdArg,
    handler: async (args) => {
      const pid = args.portfolio_id ?? 1;
      const data = getPortfolioTimeline(pid);
      const result = data.map(({ date, total_value, total_cost, pnl }) => ({ date, total_value, total_cost, pnl }));
      return { content: [{ type: "text", text: JSON.stringify({ count: result.length, first: result[0]?.date, last: result[result.length - 1]?.date, data: result }, null, 2) }] };
    },
  });

  server.tool("get_portfolio_allocation", {
    description: "获取组合资产配置：按证券类型、市场、主题聚合，并返回适合 Agent 直接引用的配置摘要和风险提示。",
    inputSchema: PortfolioIdArg,
    handler: async (args) => {
      const pid = args.portfolio_id ?? 1;
      const data = getPortfolioAllocation(pid);
      return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
    },
  });

  server.tool("get_investment_harness_snapshot", {
    description: "获取 Hermes/Agent 使用的金融 Harness 事实快照：持仓、配置、价格/成本/涨跌幅信号、数据质量和可用工具。只提供事实，不做投资决策或扣款建议。",
    inputSchema: PortfolioIdArg,
    handler: async (args) => {
      const pid = args.portfolio_id ?? 1;
      const data = getInvestmentHarnessSnapshot(pid);
      return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
    },
  });

  server.tool("get_investment_source_brief", {
    description: "为 Hermes/WebSearch 生成消息源与爬取上下文：组合级、持仓级、穿透股票级搜索 query，外部 source target 和本地 MCP 补数入口。只提供检索上下文，不做投资判断。",
    inputSchema: z.object({
      limit: z.number().default(20).describe("最多返回多少条搜索 query，1-50"),
      portfolio_id: z.number().optional().describe("组合ID（可选，默认1=默认组合）"),
    }),
    handler: async (args) => {
      const pid = args.portfolio_id ?? 1;
      const data = getInvestmentSourceBrief({ limit: args.limit, portfolioId: pid });
      return { content: [{ type: "text", text: JSON.stringify(data, null, 2) }] };
    },
  });

  server.tool("list_portfolios", {
    description: "列出所有可用的投资组合定义（ID、名称、描述）",
    inputSchema: z.object({}),
    handler: async () => {
      const rows = query<{ id: number; name: string; description: string }>(
        "SELECT id, name, description FROM portfolio_definitions ORDER BY id",
      );
      return { content: [{ type: "text", text: JSON.stringify(rows, null, 2) }] };
    },
  });
};
