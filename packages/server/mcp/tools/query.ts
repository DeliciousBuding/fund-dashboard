/** MCP Query Tools — search, detail, nav, XIRR, drawdown */

import { z } from "zod";
import { padCode, type ToolRegistrar } from "../shared";
import { query, queryOne } from "../../db";
import { getFundDetail, getFundXirr, getMaxDrawdown } from "../../services/index";

export const registerQueryTools: ToolRegistrar = (server) => {
  server.tool("search_funds", {
    description: "搜索基金或股票，返回匹配列表。可按名称、代码、类型搜索。基金用6位代码，股票也支持按代码前缀匹配。",
    inputSchema: z.object({ query: z.string().describe("搜索关键词（名称、代码或类型）") }),
    handler: async (args) => {
      const q = `%${args.query}%`;
      const rows = query<any>(
        `SELECT fd.*, ps.held_shares, ps.current_value, ps.unrealized_pnl, ps.pnl_pct, ps.latest_nav,
           COALESCE(fd.security_type, 'fund') as security_type, COALESCE(fd.market, '') as market
         FROM fund_details fd LEFT JOIN portfolio_snapshot ps ON fd.fund_code = ps.fund_code
         WHERE fd.fund_name LIKE ? OR fd.fund_code LIKE ? OR fd.fund_type LIKE ?
           OR (fd.fund_code LIKE (? || '%') AND LENGTH(? || '') > 0)
         ORDER BY ps.held_shares DESC NULLS LAST LIMIT 20`, q, q, q, args.query, args.query);
      return { content: [{ type: "text", text: JSON.stringify(rows.map((r: any) => ({
        code: r.fund_code, name: r.fund_name, type: r.fund_type,
        security_type: r.security_type, market: r.market,
        held_shares: r.held_shares || 0, current_value: r.current_value ?? null,
        unrealized_pnl: r.unrealized_pnl ?? null, pnl_pct: r.pnl_pct ?? null,
      })), null, 2) }] };
    },
  });

  server.tool("get_fund_detail", {
    description: "获取单只证券（基金或股票）的完整详情：所有交易记录、持仓、收益率、XIRR、交易状态",
    inputSchema: z.object({ code: z.string().describe("6位代码") }),
    handler: async (args) => {
      const code = padCode(args.code);
      const st = queryOne<any>("SELECT fund_name as name, fund_type as type, COALESCE(security_type,'fund') as security_type, COALESCE(market,'') as market FROM fund_details WHERE fund_code = ?", code);
      if (!st) return { content: [{ type: "text", text: JSON.stringify({ error: "security_not_found", code }) }] };
      const detail = getFundDetail(code);
      if (!detail) return { content: [{ type: "text", text: JSON.stringify({ error: "no_transactions", code }) }] };
      const status = queryOne<any>("SELECT * FROM fund_status WHERE fund_code = ?", code) || {};
      const nav = queryOne<any>("SELECT MAX(date) as last_date FROM nav_history WHERE fund_code = ?", code);
      const xirr = getFundXirr(code);
      const navCount = queryOne<any>("SELECT COUNT(*) as n FROM nav_history WHERE fund_code = ?", code);
      return { content: [{ type: "text", text: JSON.stringify({
        code, name: st.name, type: st.type,
        security_type: st.security_type, market: st.market,
        position: { shares: detail.held_shares, cost_basis: detail.total_cost, market_value: detail.current_value ?? 0, unrealized_pnl: detail.unrealized_pnl ?? 0, pnl_pct: detail.pnl_pct ?? 0 },
        trading_status: { purchase: status.purchase_status || "unknown", redemption: status.redemption_status || "unknown" },
        xirr_pct: xirr,
        nav: { count: navCount?.n ?? 0, last_date: nav?.last_date?.substring(0, 10) },
        transaction_count: detail.transactions.length,
        transactions: detail.transactions.map((tx: any) => ({
          seq: tx.seq, time: tx.trade_time, direction: tx.direction, type: tx.trade_type,
          amount: tx.amount, shares: tx.shares, fee: tx.fee,
          nav: tx.nav,
          settlement: tx.settlement_days != null ? `T+${tx.settlement_days}` : null,
          anomaly: tx.anomaly || null,
          order_id: tx.order_id || null,
        })),
      }, null, 2) }] };
    },
  });

  server.tool("get_nav_history", {
    description: "获取证券（基金或股票）历史价格数据",
    inputSchema: z.object({
      code: z.string().describe("6位代码"),
      limit: z.number().default(200).describe("返回条数上限"),
    }),
    handler: async (args) => {
      const code = padCode(args.code);
      const st = queryOne<any>("SELECT security_type, market FROM fund_details WHERE fund_code = ?", code);
      const rows = query<any>("SELECT date, unit_nav, daily_change_pct, COALESCE(security_type,'fund') as security_type FROM nav_history WHERE fund_code = ? ORDER BY date DESC LIMIT ?", code, args.limit);
      return { content: [{ type: "text", text: JSON.stringify({
        code,
        security_type: st?.security_type || "fund",
        market: st?.market || "",
        data: rows,
      }) }] };
    },
  });

  server.tool("get_fund_xirr", {
    description: "计算单只证券（基金或股票）的年化收益率（XIRR）",
    inputSchema: z.object({ code: z.string().describe("6位代码") }),
    handler: async (args) => {
      const code = padCode(args.code);
      const st = queryOne<any>("SELECT security_type, market FROM fund_details WHERE fund_code = ?", code);
      const xirr = getFundXirr(code);
      return { content: [{ type: "text", text: JSON.stringify({
        code,
        security_type: st?.security_type || "fund",
        market: st?.market || "",
        xirr_pct: xirr,
        message: xirr === null ? "not enough cashflows (need ≥2 buy/sell/dividend records)" : null,
      }, null, 2) }] };
    },
  });

  server.tool("get_fund_drawdown", {
    description: "计算证券（基金或股票）历史最大回撤",
    inputSchema: z.object({ code: z.string().describe("6位代码") }),
    handler: async (args) => {
      const code = padCode(args.code);
      const st = queryOne<any>("SELECT security_type, market FROM fund_details WHERE fund_code = ?", code);
      const dd = getMaxDrawdown(code);
      if (!dd) return { content: [{ type: "text", text: JSON.stringify({ error: "no nav data" }) }] };
      return { content: [{ type: "text", text: JSON.stringify({
        code,
        security_type: st?.security_type || "fund",
        market: st?.market || "",
        max_drawdown_pct: dd.max_drawdown, peak_date: dd.peak_date, trough_date: dd.trough_date,
      }, null, 2) }] };
    },
  });
};
