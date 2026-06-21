/** /api/export — 数据导出端点: CSV / Excel (xlsx)
 *
 *  POST /api/export/transactions-xlsx — 返回 xlsx 二进制文件
 *  请求体: { transactions: Transaction[], fundName: string }
 *
 *  v2.4 — 2026-06-19
 */

import { Hono } from "hono";
import { log } from "../middleware/logger";
import * as XLSX from "xlsx";

const router = new Hono();

interface TxRow {
  trade_time: string;
  confirm_date: string;
  direction: string;
  amount: number;
  shares: number;
  nav?: number | null;
  inferred_nav?: number | null;
  fee: number;
  settlement_days?: number | null;
  trade_day_type?: string;
}

const DIR_MAP: Record<string, string> = {
  buy: "买入", sell: "卖出", dividend: "分红",
  convert_in: "转入", convert_out: "转出", forced_redeem: "强赎",
};

/** POST /api/export/transactions-xlsx */
router.post("/transactions-xlsx", async (c) => {
  const t0 = Date.now();
  let body: { transactions?: TxRow[]; fundName?: string };
  try {
    body = await c.req.json();
  } catch {
    return c.json({ error: "invalid JSON body" }, 400);
  }

  const { transactions, fundName } = body;
  if (!transactions || !Array.isArray(transactions)) {
    return c.json({ error: "transactions array required" }, 400);
  }

  const headers = [
    "交易时间", "确认日期", "类型", "金额", "份额",
    "成交净值", "推算净值", "手续费", "结算", "交易日",
  ];

  const rows: string[][] = [headers];
  for (const tx of transactions) {
    rows.push([
      (tx.trade_time ?? "").substring(0, 16),
      tx.confirm_date ?? "",
      DIR_MAP[tx.direction] || tx.direction || "",
      tx.amount?.toFixed(2) ?? "0.00",
      tx.shares?.toFixed(2) ?? "0.00",
      tx.nav != null ? tx.nav.toFixed(4) : "",
      tx.inferred_nav != null ? tx.inferred_nav.toFixed(6) : "",
      tx.fee > 0 ? tx.fee.toFixed(2) : "",
      tx.settlement_days != null ? `T+${tx.settlement_days}` : "",
      tx.trade_day_type || "",
    ]);
  }

  // Build workbook
  const ws = XLSX.utils.aoa_to_sheet(rows);

  // Set column widths
  ws["!cols"] = [
    { wch: 18 }, { wch: 12 }, { wch: 8 }, { wch: 12 },
    { wch: 10 }, { wch: 10 }, { wch: 10 }, { wch: 10 },
    { wch: 8 }, { wch: 12 },
  ];

  const wb = XLSX.utils.book_new();
  const sheetName = (fundName || "transactions").substring(0, 31);
  XLSX.utils.book_append_sheet(wb, ws, sheetName);

  const buf = XLSX.write(wb, { type: "buffer", bookType: "xlsx" });

  const elapsed = Date.now() - t0;
  log.info("xlsx export", { rows: transactions.length, fund: fundName, ms: elapsed });

  return new Response(buf, {
    status: 200,
    headers: {
      "Content-Type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
      "Content-Disposition": `attachment; filename="${encodeURIComponent(sheetName)}.xlsx"`,
      "Content-Length": String(buf.byteLength),
    },
  });
});

export default router;
