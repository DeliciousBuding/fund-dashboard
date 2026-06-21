/** MCP Operations Tools — crawl, recalculate, adjust position */

import { z } from "zod";
import { padCode, type ToolRegistrar } from "../shared";
import { queryOne } from "../../db";
import { log } from "../../middleware/logger";
import { recalculateAllSnapshots, adjustPosition } from "../../services/index";
import { refreshSecurityPrice, refreshAllHeld } from "../../crawler/nav";

export const registerOperationsTools: ToolRegistrar = (server) => {
  server.tool("crawl_nav", {
    description: "触发净值/价格爬取——刷新单只证券（基金或股票）或全部持仓的最新价格数据。股票从东方财富K线接口获取，基金从天天基金净值接口获取。",
    inputSchema: z.object({
      code: z.string().optional().describe("6位代码。不提供则刷新全部持仓"),
      all: z.boolean().default(false).describe("设为 true 刷新全部持仓（与不提供 code 等效）"),
      security_type: z.enum(["fund", "stock"]).default("fund").describe("证券类型：fund=基金, stock=股票"),
    }),
    handler: async (args) => {
      if (args.code) {
        const code = padCode(args.code);
        const secType = args.security_type || (queryOne<any>("SELECT security_type FROM fund_details WHERE fund_code = ?", code)?.security_type || "fund") as "fund" | "stock";
        const r = await refreshSecurityPrice(code, secType);
        return { content: [{ type: "text", text: JSON.stringify({ ok: true, code: r.code, security_type: r.security_type, added_rows: r.added, latest_date: r.latest }) }] };
      }
      refreshAllHeld().then(r => log.info(`mcp:crawl_nav all done: ${r.added} rows`));
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, status: "started", message: "Crawling all held securities in background" }) }] };
    },
  });

  server.tool("recalculate_snapshot", {
    description: "完全重建 portfolio_snapshot 表——从 transactions 和 nav_history 重新计算所有持仓",
    inputSchema: z.object({}),
    handler: async () => {
      const r = recalculateAllSnapshots();
      log.info(`mcp:recalculate_snapshot ${r.securities} securities, ¥${r.totalValue.toFixed(2)}`);
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, securities_in_snapshot: r.securities, total_portfolio_value: r.totalValue }) }] };
    },
  });

  server.tool("adjust_position", {
    description: "手动调整某只证券的持仓份额（如拆分、合并等特殊情况）。自动重算市值和盈亏。",
    inputSchema: z.object({
      fund_code: z.string().describe("6位代码"),
      shares: z.number().describe("调整后的准确持仓份额/股数"),
    }),
    handler: async (args) => {
      const code = padCode(args.fund_code);
      const pos = adjustPosition(code, args.shares);
      log.info(`mcp:adjust_position ${code} → ${args.shares} shares`);
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, fund_code: code, adjusted_shares: args.shares, position: pos ? { value: pos.current_value, pnl: pos.unrealized_pnl, pnl_pct: pos.pnl_pct } : null }) }] };
    },
  });
};
