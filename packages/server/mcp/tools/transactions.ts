/** MCP Transaction Tools — add, update, delete, import transactions */

import { z } from "zod";
import { padCode, type ToolRegistrar } from "../shared";
import { recalcSnapshot } from "../../services/index";
import { queryOne, getRwDb } from "../../db";
import { log } from "../../middleware/logger";

export const registerTransactionTools: ToolRegistrar = (server) => {
  server.tool("add_transaction", {
    description: "添加一笔新交易记录（买入/卖出/分红），支持基金和股票。添加后自动重算组合快照。",
    inputSchema: z.object({
      fund_code: z.string().describe("6位代码（基金或股票）"),
      trade_time: z.string().describe("交易时间，ISO格式如 2026-06-15T09:30:00"),
      direction: z.enum(["buy", "sell", "dividend"]).describe("交易方向：buy=买入, sell=卖出, dividend=分红"),
      trade_type: z.string().default("用户买入").describe("交易类型，如：用户买入、用户卖出、定投买入、机构分红"),
      confirm_amount: z.number().describe("确认金额（元）"),
      confirm_share: z.number().optional().describe("确认份额/股数"),
      fee: z.number().default(0).describe("手续费"),
      order_id: z.string().optional().describe("订单号，不提供则自动生成"),
    }),
    handler: async (args) => {
      const db = getRwDb();
      const orderId = args.order_id || `mcp_${Date.now()}`;
      const shares = args.confirm_share || 0;
      const amount = args.confirm_amount;
      const isDividend = args.direction === "dividend";
      const signedCash = args.direction === "buy" ? -amount : +amount;
      const signedShare = isDividend ? 0 : (args.direction === "buy" ? shares : -shares);
      try {
        db.run(`INSERT OR REPLACE INTO transactions (order_id, trade_time, trade_type, direction, fund_code, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change)
          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
          [orderId, args.trade_time, args.trade_type, args.direction, padCode(args.fund_code), amount, shares, args.fee, signedCash, signedShare]);
      } catch (e: any) {
        return { content: [{ type: "text", text: JSON.stringify({ error: "insert_failed", detail: e.message }) }] };
      }
      recalcSnapshot(padCode(args.fund_code));
      log.info(`mcp:add_transaction ${args.direction} ¥${amount} ${args.fund_code}`);
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, order_id: orderId, fund_code: args.fund_code, direction: args.direction, amount, snapshot_recalculated: true }) }] };
    },
  });

  server.tool("update_transaction", {
    description: "修改一笔交易记录（按 seq 序号）。自动重算签名现金流和组合快照。",
    inputSchema: z.object({
      seq: z.number().describe("交易序号"),
      trade_time: z.string().optional().describe("新的交易时间"),
      confirm_date: z.string().optional().describe("新的确认日期"),
      direction: z.enum(["buy", "sell", "dividend"]).optional().describe("新的交易方向"),
      trade_type: z.string().optional().describe("新的交易类型"),
      confirm_amount: z.number().optional().describe("新的确认金额"),
      confirm_share: z.number().optional().describe("新的确认份额"),
      fee: z.number().optional().describe("新的手续费"),
      fund_code: z.string().optional().describe("新的证券代码（如果改代码）"),
    }),
    handler: async (args) => {
      const tx = queryOne<any>("SELECT * FROM transactions WHERE seq = ?", args.seq);
      if (!tx) return { content: [{ type: "text", text: JSON.stringify({ error: "transaction not found", seq: args.seq }) }] };
      const updFields: Record<string, any> = {};
      if (args.trade_time !== undefined) updFields.trade_time = args.trade_time;
      if (args.confirm_date !== undefined) updFields.confirm_date = args.confirm_date;
      if (args.direction !== undefined) updFields.direction = args.direction;
      if (args.trade_type !== undefined) updFields.trade_type = args.trade_type;
      if (args.confirm_amount !== undefined) updFields.confirm_amount = args.confirm_amount;
      if (args.confirm_share !== undefined) updFields.confirm_share = args.confirm_share;
      if (args.fee !== undefined) updFields.fee = args.fee;
      if (args.fund_code !== undefined) updFields.fund_code = padCode(args.fund_code);
      if (!Object.keys(updFields).length) return { content: [{ type: "text", text: JSON.stringify({ error: "no fields to update" }) }] };
      const dir = updFields.direction || tx.direction;
      const amt = updFields.confirm_amount ?? tx.confirm_amount;
      const sh = updFields.confirm_share ?? tx.confirm_share;
      updFields.signed_cash_flow = dir === "buy" ? -(amt) : +(amt);
      updFields.signed_share_change = dir === "dividend" ? 0 : dir === "buy" ? sh : -sh;
      const sets = Object.entries(updFields).map(([k]) => `${k} = ?`).join(", ");
      const vals = Object.values(updFields);
      vals.push(args.seq);
      getRwDb().run(`UPDATE transactions SET ${sets} WHERE seq = ?`, vals);
      const fundCode = updFields.fund_code || tx.fund_code;
      recalcSnapshot(fundCode);
      if (updFields.fund_code && updFields.fund_code !== tx.fund_code) recalcSnapshot(tx.fund_code);
      log.info(`mcp:update_transaction seq=${args.seq}`);
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, seq: args.seq, updated_fields: Object.keys(updFields).filter(k => !k.startsWith("signed_")), snapshot_recalculated: true }) }] };
    },
  });

  server.tool("delete_transaction", {
    description: "删除一笔交易记录（按 seq 序号）。自动重算组合快照。",
    inputSchema: z.object({ seq: z.number().describe("交易序号") }),
    handler: async (args) => {
      const tx = queryOne<any>("SELECT * FROM transactions WHERE seq = ?", args.seq);
      if (!tx) return { content: [{ type: "text", text: JSON.stringify({ error: "not found" }) }] };
      getRwDb().run("DELETE FROM transactions WHERE seq = ?", [args.seq]);
      recalcSnapshot(tx.fund_code);
      log.info(`mcp:delete_transaction seq=${args.seq} ${tx.fund_code}`);
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, deleted: { seq: args.seq, fund_code: tx.fund_code, direction: tx.direction, amount: +tx.confirm_amount }, snapshot_recalculated: true }) }] };
    },
  });

  server.tool("import_transactions", {
    description: "批量导入交易记录。每笔需含 order_id, trade_time, direction, fund_code, confirm_amount 等字段。支持基金和股票。",
    inputSchema: z.object({
      transactions: z.array(z.object({
        order_id: z.string(), trade_time: z.string(), confirm_date: z.string().optional(),
        trade_type: z.string().default("用户买入"), direction: z.enum(["buy", "sell", "dividend"]),
        fund_code: z.string(), fund_name: z.string().optional(),
        confirm_amount: z.number(), confirm_share: z.number().optional(), fee: z.number().default(0),
        inferred_nav: z.number().optional(),
      })).describe("交易记录数组"),
    }),
    handler: async (args) => {
      const db = getRwDb();
      const insert = db.prepare(`INSERT OR IGNORE INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, inferred_nav, signed_cash_flow, signed_share_change) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`);
      let imported = 0;
      const affectedFunds = new Set<string>();
      const doImport = db.transaction((txs: any[]) => {
        for (const tx of txs) {
          const sh = tx.confirm_share || 0;
          const r = insert.run(tx.order_id, tx.trade_time, tx.confirm_date || null, tx.trade_type, tx.direction, padCode(tx.fund_code), tx.fund_name || null, tx.confirm_amount, sh, tx.fee, tx.inferred_nav || null, tx.direction === "buy" ? -tx.confirm_amount : tx.confirm_amount, tx.direction === "buy" ? sh : -sh);
          if (r.changes) { imported++; affectedFunds.add(padCode(tx.fund_code)); }
        }
      });
      doImport(args.transactions);
      for (const fc of affectedFunds) recalcSnapshot(fc);
      log.info(`mcp:import_transactions ${imported}/${args.transactions.length}`);
      return { content: [{ type: "text", text: JSON.stringify({ ok: true, imported, total: args.transactions.length, affected_funds: affectedFunds.size }) }] };
    },
  });
};
