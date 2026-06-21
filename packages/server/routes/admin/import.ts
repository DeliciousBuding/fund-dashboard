/** /api/admin IMPORT — CSV 导入 · 爬虫触发端点 */

import { Hono } from "hono";
import { getRwDb } from "../../db";
import { log } from "../../middleware/logger";
import { refreshFundNav, refreshAllHeld } from "../../crawler/nav";
import { refreshFundHoldings, refreshAllHoldings } from "../../crawler/holdings";
import { recalcFundSnapshot } from "./crud";

const router = new Hono();

/** Pad fund code: numeric codes get 6-digit padding, alpha codes get uppercased */
function padCodeFlex(code: string): string {
  const trimmed = code.trim();
  return /^\d+$/.test(trimmed) ? trimmed.padStart(6, "0") : trimmed.toUpperCase();
}

function parseDirection(val: string): "buy" | "sell" | "dividend" {
  const v = val.trim().toLowerCase();
  if (v === "buy" || v === "买入" || v === "买") return "buy";
  if (v === "sell" || v === "卖出" || v === "卖") return "sell";
  if (v === "dividend" || v === "分红" || v === "股息") return "dividend";
  return "buy";
}

// ═══════════ CSV IMPORT ═══════════

/** CSV import — accepts CSV text body, auto-detects numeric vs alpha codes */
router.post("/import-csv", async c => {
  const body = await c.req.json();
  const csv: string = body.csv;
  if (!csv || typeof csv !== "string") return c.json({ error: "csv field required" }, 400);

  const lines = csv.trim().split(/\r?\n/);
  if (lines.length < 2) return c.json({ error: "csv must have header + at least 1 row" }, 400);

  const header = lines[0].toLowerCase().replace(/^﻿/, ""); // strip BOM
  const cols = header.split(",").map(h => h.trim().replace(/"/g, ""));
  const idx = (name: string) => cols.findIndex(c => c.includes(name));

  const dateIdx = idx("date") !== -1 ? idx("date") : idx("交易");
  const codeIdx = idx("code") !== -1 ? idx("code") : idx("代码");
  const nameIdx = idx("name") !== -1 ? idx("name") : idx("名称");
  const dirIdx = idx("direction") !== -1 ? idx("direction") : idx("方向");
  const amountIdx = idx("amount") !== -1 ? idx("amount") : idx("金额");
  const shareIdx = idx("share") !== -1 ? idx("share") : idx("份额");
  const feeIdx = idx("fee") !== -1 ? idx("fee") : idx("手续费");
  const typeIdx = idx("type") !== -1 ? idx("type") : idx("类型");

  if (dateIdx === -1 || codeIdx === -1 || amountIdx === -1 || dirIdx === -1) {
    return c.json({ error: "CSV needs columns: date, code, direction, amount (or 交易日期,代码,方向,金额)" }, 400);
  }

  const transactions: any[] = [];
  const errors: string[] = [];
  for (let i = 1; i < lines.length; i++) {
    const cells = lines[i].split(",").map(c => c.trim().replace(/^"|"$/g, ""));
    if (cells.length < 3 || !cells[dateIdx]) continue;
    const code = padCodeFlex(cells[codeIdx]);
    const direction = parseDirection(cells[dirIdx]);
    const amount = parseFloat(cells[amountIdx]);
    if (isNaN(amount)) { errors.push(`row ${i}: invalid amount`); continue; }
    transactions.push({
      order_id: `csv_${Date.now()}_${crypto.randomUUID().slice(0, 8)}_${i}`,
      trade_time: cells[dateIdx],
      confirm_date: cells[dateIdx]?.substring(0, 10),
      trade_type: cells[typeIdx] !== undefined ? cells[typeIdx] : (direction === "buy" ? "用户买入" : direction === "sell" ? "用户卖出" : "机构分红"),
      direction,
      fund_code: code,
      fund_name: nameIdx !== -1 ? cells[nameIdx] : null,
      confirm_amount: amount,
      confirm_share: shareIdx !== -1 ? parseFloat(cells[shareIdx]) || 0 : 0,
      fee: feeIdx !== -1 ? parseFloat(cells[feeIdx]) || 0 : 0,
      inferred_nav: null,
      signed_cash_flow: direction === "buy" ? -amount : amount,
      signed_share_change: direction === "buy" ? (shareIdx !== -1 ? parseFloat(cells[shareIdx]) || 0 : 0) : (shareIdx !== -1 ? -(parseFloat(cells[shareIdx]) || 0) : 0),
    });
  }

  const db = getRwDb();
  const insert = db.prepare(`INSERT OR IGNORE INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, inferred_nav, signed_cash_flow, signed_share_change) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`);
  let imported = 0;
  const affectedFunds = new Set<string>();
  const doImport = db.transaction((txs: any[]) => {
    for (const tx of txs) {
      const r = insert.run(tx.order_id, tx.trade_time, tx.confirm_date, tx.trade_type, tx.direction, tx.fund_code, tx.fund_name, tx.confirm_amount, tx.confirm_share, tx.fee, tx.inferred_nav, tx.signed_cash_flow, tx.signed_share_change);
      if (r.changes) { imported++; affectedFunds.add(tx.fund_code); }
    }
  });
  try {
    doImport(transactions);
    for (const fc of affectedFunds) recalcFundSnapshot(fc);
    log.info(`csv import: ${imported}/${transactions.length} tx, recalc ${affectedFunds.size}`);
    return c.json({ ok: true, imported, total: transactions.length, affected_funds: affectedFunds.size, errors: errors.length ? errors : undefined });
  } catch (e: any) {
    return c.json({ error: e.message }, 500);
  }
});

// ═══════════ CRAWLER TRIGGERS ═══════════

/** 触发 NAV 爬取 (单只或全部持仓).  Accepts optional `type` param (fund|stock). */
router.post("/crawl-nav", async c => {
  const body = await c.req.json().catch(() => ({}));
  const code: string | undefined = body.code;
  const type: string | undefined = body.type; // "fund" | "stock"
  if (code) {
    const r = await refreshFundNav(code.padStart(6, "0"));
    return c.json(r);
  }
  refreshAllHeld().then(r => log.info(`background crawl done: ${r.added} rows`));
  return c.json({ status: "started", message: "crawling all held funds", type: type || "fund" });
});

/** POST /api/admin/refresh-holdings — trigger fund holdings refresh.
 *  Same as crawl-holdings but with explicit "refresh" naming for V2 compat.
 *  Accepts optional `code` to refresh a single fund.
 */
router.post("/refresh-holdings", async c => {
  const body = await c.req.json().catch(() => ({}));
  const code: string | undefined = body.code;
  if (code) {
    const r = await refreshFundHoldings(code.padStart(6, "0"));
    if (!r) return c.json({ error: "no holdings data found", fund_code: code }, 404);
    return c.json(r);
  }
  refreshAllHoldings().then(r => log.info(`background holdings crawl done: ${r.updated}/${r.total} funds`));
  return c.json({ status: "started", message: "crawling holdings for all held equity funds" });
});

router.post("/crawl-holdings", async c => {
  const body = await c.req.json().catch(() => ({}));
  const code: string | undefined = body.code;
  if (code) {
    const r = await refreshFundHoldings(code.padStart(6, "0"));
    if (!r) return c.json({ error: "no holdings data found", fund_code: code }, 404);
    return c.json(r);
  }
  refreshAllHoldings().then(r => log.info(`background holdings crawl done: ${r.updated}/${r.total} funds`));
  return c.json({ status: "started", message: "crawling holdings for all held equity funds" });
});

// ═══════════ HOLDINGS COVERAGE ═══════════

/** GET /api/admin/holdings-coverage — returns coverage stats grouped by fund_type */
router.get("/holdings-coverage", c => {
  const db = getRwDb();
  const totalRow = db.query(`SELECT COUNT(DISTINCT fund_code) as cnt FROM portfolio_snapshot WHERE held_shares > 0.001`).get() as { cnt: number };
  const totalFunds = totalRow?.cnt ?? 0;
  const withRow = db.query(`
    SELECT COUNT(DISTINCT ps.fund_code) as cnt
    FROM portfolio_snapshot ps
    JOIN fund_holdings fh ON fh.fund_code = ps.fund_code
    WHERE ps.held_shares > 0.001
  `).get() as { cnt: number };
  const fundsWithHoldings = withRow?.cnt ?? 0;
  const coveragePct = totalFunds > 0 ? +((fundsWithHoldings / totalFunds) * 100).toFixed(1) : 0;

  const byType = (db.query(`
    SELECT COALESCE(fd.fund_type, '未分类') as fund_type,
      COUNT(DISTINCT ps.fund_code) as total,
      COUNT(DISTINCT CASE WHEN fh.fund_code IS NOT NULL THEN ps.fund_code END) as with_holdings
    FROM portfolio_snapshot ps
    LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
    LEFT JOIN fund_holdings fh ON fh.fund_code = ps.fund_code
    WHERE ps.held_shares > 0.001
    GROUP BY fd.fund_type
    ORDER BY total DESC
  `).all() as { fund_type: string; total: number; with_holdings: number }[])
    .map(row => ({
      fund_type: row.fund_type || "未分类",
      total: row.total,
      with_holdings: row.with_holdings,
      coverage_pct: row.total > 0 ? +((row.with_holdings / row.total) * 100).toFixed(1) : 0,
    }));

  return c.json({
    total_funds: totalFunds,
    funds_with_holdings: fundsWithHoldings,
    coverage_pct: coveragePct,
    by_fund_type: byType,
  });
});

export default router;
