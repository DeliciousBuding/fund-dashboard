/** MCP Security CRUD Tools — add, update, delete securities (fund + stock) */

import { z } from "zod";
import { padCode, type ToolRegistrar } from "../shared";
import { queryOne, getRwDb } from "../../db";
import { log } from "../../middleware/logger";

export const registerSecurityTools: ToolRegistrar = (server) => {
  server.tool("add_fund", {
    description: "添加新基金到系统",
    inputSchema: z.object({
      fund_code: z.string().describe("6位基金代码"),
      fund_name: z.string().describe("基金名称"),
      fund_type: z.string().optional().describe("基金类型，如 股票型、混合型、QDII、货币型"),
    }),
    handler: async (args) => {
      const code = padCode(args.fund_code);
      getRwDb().run("INSERT OR REPLACE INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES (?, ?, ?, 'fund', '')", [code, args.fund_name, args.fund_type || ""]);
      log.info(`mcp:add_fund ${code} ${args.fund_name}`);
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, fund_code: code, fund_name: args.fund_name, security_type: "fund" }) }] };
    },
  });

  server.tool("add_security", {
    description: "添加新证券（基金或股票）到系统。fund 无需 market；stock 需指定 market=SH/SZ/HK/US。",
    inputSchema: z.object({
      code: z.string().describe("证券代码。基金用6位数字；股票按市场惯例：SH/SZ 6位，HK 5位，US 字母代码。"),
      name: z.string().describe("证券名称"),
      security_type: z.enum(["fund", "stock"]).describe("证券类型：fund=基金, stock=股票"),
      market: z.string().optional().describe("市场代码。股票必填：SH=上海, SZ=深圳, HK=香港, US=美国。基金无需填写。"),
      fund_type: z.string().optional().describe("基金类型（仅 fund 时有效），如 股票型、混合型、QDII"),
    }),
    handler: async (args) => {
      const code = padCode(args.code);
      const market = args.security_type === "stock" ? (args.market || "") : "";
      if (args.security_type === "stock" && !market) {
        return { content: [{ type: "text", text: JSON.stringify({ error: "market_required_for_stocks", message: "添加股票时必须指定 market: SH/SZ/HK/US" }) }] };
      }
      getRwDb().run("INSERT OR REPLACE INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES (?, ?, ?, ?, ?)",
        [code, args.name, args.fund_type || "", args.security_type, market]);
      log.info(`mcp:add_security ${code} ${args.name} type=${args.security_type} market=${market}`);
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, code, name: args.name, security_type: args.security_type, market }) }] };
    },
  });

  server.tool("update_fund", {
    description: "更新证券信息（名称、类型、市场）",
    inputSchema: z.object({
      fund_code: z.string().describe("6位代码"),
      fund_name: z.string().optional().describe("新的名称"),
      fund_type: z.string().optional().describe("新的类型"),
      market: z.string().optional().describe("新的市场代码（SH/SZ/HK/US）"),
    }),
    handler: async (args) => {
      const code = padCode(args.fund_code);
      const existing = queryOne<any>("SELECT * FROM fund_details WHERE fund_code = ?", code);
      if (!existing) return { content: [{ type: "text", text: JSON.stringify({ error: "security not found" }) }] };
      const changes: string[] = [];
      if (args.fund_name) { getRwDb().run("UPDATE fund_details SET fund_name = ? WHERE fund_code = ?", [args.fund_name, code]); changes.push("name"); }
      if (args.fund_type) { getRwDb().run("UPDATE fund_details SET fund_type = ? WHERE fund_code = ?", [args.fund_type, code]); changes.push("type"); }
      if (args.market !== undefined) { getRwDb().run("UPDATE fund_details SET market = ? WHERE fund_code = ?", [args.market, code]); changes.push("market"); }
      log.info(`mcp:update_fund ${code} fields=${changes.join(",")}`);
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, code, updated: changes }) }] };
    },
  });

  server.tool("delete_fund", {
    description: "删除证券及其所有关联数据（交易、净值、持仓）。不可逆！",
    inputSchema: z.object({ fund_code: z.string().describe("6位代码") }),
    handler: async (args) => {
      const code = padCode(args.fund_code);
      const existing = queryOne<any>("SELECT * FROM fund_details WHERE fund_code = ?", code);
      if (!existing) return { content: [{ type: "text", text: JSON.stringify({ error: "security not found" }) }] };
      const db = getRwDb();
      ["fund_status", "portfolio_snapshot", "nav_history", "transactions", "fund_details"].forEach(t => db.run(`DELETE FROM ${t} WHERE fund_code = ?`, [code]));
      log.info(`mcp:delete_fund ${code} ${existing.fund_name}`);
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, deleted: { code, name: existing.fund_name, security_type: existing.security_type || "fund" } }) }] };
    },
  });
};
