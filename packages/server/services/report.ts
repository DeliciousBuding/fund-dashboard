/**
 * PDF Investment Report Service — Weekly & Monthly Reports
 *
 * Generates investment reports in HTML + PDF formats.
 * Uses Chrome headless --print-to-pdf for HTML→PDF conversion.
 *
 * Contents: portfolio summary, holdings detail, return trends,
 *           penetration analysis, risk warnings (5 sections minimum).
 *
 * Utilities extracted to services/report-utils.ts to stay < 300 lines.
 */

import { join } from "node:path";
import { readFileSync } from "node:fs";
import { query } from "../db";
import {
  getPortfolioSummary,
  getPortfolioPenetration,
  getPortfolioXirr,
  getPortfolioTimeline,
  getPortfolioAllocation,
} from "./index";
import {
  cny, pct, pctClass, esc,
  buildTrendBars, buildAllocationRows,
  buildRiskItems, buildPenetrationRows,
  htmlToPdf,
} from "./report-utils";
import { log } from "../middleware/logger";

// ══════════════════════════════════════════════════════════════════════

const TEMPLATE_DIR = join(import.meta.dirname, "report-templates");

export interface ReportResult {
  html: string;
  pdf: Uint8Array | null;
  pdfError?: string;
  generatedAt: string;
  periodStart: string;
  periodEnd: string;
}

// ══════════════════════════════════════════════════════════════════════
//  Data gathering helpers
// ══════════════════════════════════════════════════════════════════════

interface HoldingRow {
  fund_code: string; fund_name: string; security_type: string; market?: string;
  held_shares: number; total_cost: number; latest_nav: number;
  current_value: number; unrealized_pnl: number; pnl_pct: number;
}

function getHoldings(): HoldingRow[] {
  return query<HoldingRow>(
    "SELECT * FROM portfolio_snapshot WHERE held_shares > 0.001 ORDER BY COALESCE(current_value, 0) DESC",
  );
}

function getNavTimeline(
  periodStart: string,
  periodEnd: string,
): { date: string; value: number }[] {
  const rows = query<{ date: string; total_value: number }>(`
    SELECT nh.date, SUM(ps.held_shares * nh.unit_nav) as total_value
    FROM portfolio_snapshot ps
    JOIN nav_history nh ON nh.fund_code = ps.fund_code
    WHERE ps.held_shares > 0.001 AND nh.date >= ? AND nh.date <= ?
    GROUP BY nh.date ORDER BY nh.date
  `, [periodStart, periodEnd]);
  return rows.map((r) => ({
    date: typeof r.date === "string" ? r.date.substring(0, 10) : String(r.date).substring(0, 10),
    value: r.total_value || 0,
  }));
}

function getPeriodReturn(timeline: { value: number }[]): number | null {
  if (timeline.length < 2) return null;
  const start = timeline[0].value;
  const end = timeline[timeline.length - 1].value;
  return start > 0 ? ((end - start) / start) * 100 : null;
}

function getMaxDrawdownInPeriod(periodStart: string): { dd: number; peak: string; trough: string } {
  let maxDd = 0, ddPeak = "", ddTrough = "";
  const codes = query<{ fund_code: string }>(
    "SELECT fund_code FROM portfolio_snapshot WHERE held_shares > 0.001",
  );
  for (const { fund_code } of codes) {
    const navs = query<{ date: string; unit_nav: number }>(
      "SELECT date, unit_nav FROM nav_history WHERE fund_code = ? AND date >= ? ORDER BY date",
      [fund_code, periodStart],
    );
    if (navs.length < 2) continue;
    let peak = +navs[0].unit_nav, curPeakDate = navs[0].date;
    for (const r of navs) {
      const nav = +r.unit_nav;
      if (nav > peak) { peak = nav; curPeakDate = r.date; }
      const dd = (peak - nav) / peak;
      if (dd > maxDd) { maxDd = dd; ddPeak = curPeakDate; ddTrough = r.date; }
    }
  }
  return { dd: maxDd, peak: ddPeak, trough: ddTrough };
}

// ══════════════════════════════════════════════════════════════════════
//  Row builders
// ══════════════════════════════════════════════════════════════════════

function buildHoldingsShort(holdings: HoldingRow[]): string {
  if (!holdings.length) return '<tr><td colspan="9" style="text-align:center;color:#94a3b8;">无持仓数据</td></tr>';
  return holdings.map((h) => {
    const secType = (h.security_type || "fund") === "stock" ? "股票" : "基金";
    return `<tr>
      <td>${esc(h.fund_code)}</td><td>${esc(h.fund_name)}</td><td>${secType}</td>
      <td class="num">${h.held_shares.toFixed(2)}</td><td class="num">¥${cny(Math.abs(h.total_cost || 0))}</td>
      <td class="num">${cny(h.latest_nav)}</td><td class="num">¥${cny(h.current_value)}</td>
      <td class="num ${pctClass(h.unrealized_pnl)}">¥${cny(h.unrealized_pnl)}</td>
      <td class="num ${pctClass(h.pnl_pct)}">${pct(h.pnl_pct)}</td>
    </tr>`;
  }).join("\n");
}

function buildHoldingsFull(holdings: HoldingRow[], totalValue: number): string {
  if (!holdings.length) return '<tr><td colspan="11" style="text-align:center;color:#94a3b8;">无持仓数据</td></tr>';
  return holdings.map((h) => {
    const secType = (h.security_type || "fund") === "stock" ? "股票" : "基金";
    const costPerShare = h.held_shares > 0.001 ? Math.abs(h.total_cost || 0) / h.held_shares : null;
    const weight = totalValue > 0 ? ((h.current_value || 0) / totalValue) * 100 : 0;
    return `<tr>
      <td>${esc(h.fund_code)}</td><td>${esc(h.fund_name)}</td><td>${secType}</td>
      <td>${esc(h.market || "")}</td>
      <td class="num">${h.held_shares.toFixed(2)}</td><td class="num">${cny(costPerShare)}</td>
      <td class="num">${cny(h.latest_nav)}</td><td class="num">¥${cny(h.current_value)}</td>
      <td class="num ${pctClass(h.unrealized_pnl)}">¥${cny(h.unrealized_pnl)}</td>
      <td class="num ${pctClass(h.pnl_pct)}">${pct(h.pnl_pct)}</td>
      <td class="num">${weight.toFixed(1)}%</td>
    </tr>`;
  }).join("\n");
}

function buildPerfRows(holdings: HoldingRow[], periodStart: string, periodEnd: string): string {
  // Batch query: fetch first & last NAV for all holdings in one query
  const codes = holdings.map(h => h.fund_code);
  const navMap: Record<string, { firstNav: number | null; lastNav: number | null }> = {};
  if (codes.length > 0) {
    const ph = codes.map(() => "?").join(",");
    const navRows = query<{ fund_code: string; first_nav: number | null; last_nav: number | null }>(`
      SELECT f.fund_code,
        (SELECT n.unit_nav FROM nav_history n WHERE n.fund_code = f.fund_code AND n.date >= ? ORDER BY n.date LIMIT 1) as first_nav,
        (SELECT n.unit_nav FROM nav_history n WHERE n.fund_code = f.fund_code AND n.date <= ? ORDER BY n.date DESC LIMIT 1) as last_nav
      FROM (SELECT DISTINCT fund_code FROM nav_history WHERE fund_code IN (${ph})) f
    `, periodStart, periodEnd, ...codes);
    for (const r of navRows) navMap[r.fund_code] = { firstNav: r.first_nav, lastNav: r.last_nav };
  }

  const rows = holdings.map((h) => {
    const nav = navMap[h.fund_code] || { firstNav: null, lastNav: null };
    const startNav = nav.firstNav || h.latest_nav;
    const endNav = nav.lastNav || h.latest_nav;
    const chg = startNav > 0 ? ((endNav - startNav) / startNav) * 100 : 0;
    const contribution = h.held_shares * (endNav - startNav);
    return { ...h, startNav, endNav, chg, contribution };
  }).sort((a, b) => b.chg - a.chg);

  return rows.map((r) => `<tr>
    <td>${esc(r.fund_code)}</td><td>${esc(r.fund_name)}</td>
    <td class="num">${cny(r.startNav)}</td><td class="num">${cny(r.endNav)}</td>
    <td class="num ${pctClass(r.chg)}">${pct(r.chg)}</td>
    <td class="num ${pctClass(r.contribution)}">¥${cny(r.contribution)}</td>
  </tr>`).join("\n") || '<tr><td colspan="6" style="text-align:center;color:#94a3b8;">无数据</td></tr>';
}

function buildTxRows(periodStart: string, periodEnd: string) {
  const txs = query<{
    trade_time: string; fund_code: string; trade_type: string; direction: string;
    confirm_amount: number; confirm_share: number; fee: number;
  }>(
    "SELECT trade_time, fund_code, trade_type, direction, confirm_amount, confirm_share, COALESCE(fee,0) as fee FROM transactions WHERE trade_time >= ? AND trade_time <= ? ORDER BY trade_time DESC",
    [periodStart + " 00:00:00", periodEnd + " 23:59:59"],
  );
  const rows = txs.map((tx) => `<tr>
    <td>${(tx.trade_time || "").substring(0, 10)}</td><td>${esc(tx.fund_code)}</td>
    <td>${esc(tx.trade_type || "")}</td><td>${tx.direction === "buy" ? "买入" : "卖出"}</td>
    <td class="num">¥${cny(tx.confirm_amount)}</td><td class="num">${cny(tx.confirm_share)}</td>
    <td class="num">¥${cny(tx.fee)}</td>
  </tr>`).join("\n");
  const autoTx = txs.filter((t) => (t.trade_type || "").includes("定投"));
  const manualTx = txs.filter((t) => (t.trade_type || "").includes("用户"));
  const autoAmt = autoTx.reduce((s, t) => s + (t.confirm_amount || 0), 0);
  const manualAmt = manualTx.reduce((s, t) => s + (t.confirm_amount || 0), 0);
  const fee = txs.reduce((s, t) => s + (t.fee || 0), 0);
  return { rows, autoCount: autoTx.length, manualCount: manualTx.length, autoAmt, manualAmt, fee, count: txs.length };
}

// ══════════════════════════════════════════════════════════════════════
//  Core report generators
// ══════════════════════════════════════════════════════════════════════

async function buildReport(type: "weekly" | "monthly"): Promise<ReportResult> {
  const isMonthly = type === "monthly";
  const generatedAt = new Date().toISOString().replace("T", " ").substring(0, 19);
  const now = new Date();
  const days = isMonthly ? 30 : 7;
  const ago = new Date(now.getTime() - days * 86400000);
  const periodEnd = now.toISOString().substring(0, 10);
  const periodStart = ago.toISOString().substring(0, 10);
  const templateFile = isMonthly ? "monthly.html" : "weekly.html";
  const template = readFileSync(join(TEMPLATE_DIR, templateFile), "utf-8");

  // Gather
  const summary = getPortfolioSummary();
  const penetration = getPortfolioPenetration();
  const timeline = getPortfolioTimeline();
  const allocation = getPortfolioAllocation();
  const holdings = getHoldings();
  const totalValue = holdings.reduce((s, h) => s + (h.current_value || 0), 0);

  // Nav timeline + return
  let navTimeline = getNavTimeline(periodStart, periodEnd);
  if (!navTimeline.length) navTimeline = timeline.slice(-days).map((t) => ({ date: t.date, value: t.total_value }));
  const periodReturn = getPeriodReturn(navTimeline);
  const trendBars = buildTrendBars(navTimeline, isMonthly ? 30 : 7);

  // Template vars
  const vars: Record<string, string> = {
    REPORT_PERIOD: `${periodStart} — ${periodEnd}`,
    GENERATED_AT: generatedAt,
    DATA_AS_OF: summary?.last_nav_date || periodEnd,
    TOTAL_VALUE: cny(totalValue),
    HELD_COUNT: String(holdings.length),
    TOTAL_COST: cny(summary?.total_buy || 0),
    TOTAL_TX: String(summary?.total_tx || 0),
    TREND_BARS: trendBars.bars,
    TREND_LABELS: trendBars.labels,
    ALLOCATION_ROWS: buildAllocationRows(allocation),
    RISK_ITEMS: buildRiskItems(summary?.total_buy || 0, summary?.total_fee || 0),
    PENETRATION_COVERAGE: penetration?.equity_fund_count ? `${penetration.equity_fund_count} 只基金有持仓数据` : "无穿透数据",
    EQUITY_FUND_COUNT: String(penetration?.equity_fund_count || 0),
    UNIQUE_STOCKS: String(penetration?.unique_stocks || 0),
  };

  if (isMonthly) {
    const xirr = getPortfolioXirr();
    const dd = getMaxDrawdownInPeriod(periodStart);
    const tx = buildTxRows(periodStart, periodEnd);
    Object.assign(vars, {
      MONTHLY_RETURN: periodReturn != null ? pct(periodReturn) : "N/A",
      MONTHLY_RETURN_CLASS: pctClass(periodReturn),
      XIRR: xirr != null ? pct(xirr) : "N/A",
      XIRR_CLASS: pctClass(xirr),
      MAX_DRAWDOWN: dd.dd > 0 ? "-" + (dd.dd * 100).toFixed(2) + "%" : "N/A",
      DRAWDOWN_PEAK: dd.peak ? dd.peak.substring(0, 10) : "-",
      DRAWDOWN_TROUGH: dd.trough ? dd.trough.substring(0, 10) : "-",
      FIRST_TRADE: summary?.first_trade || "",
      HOLDINGS_ROWS: buildHoldingsFull(holdings, totalValue),
      PERFORMANCE_ROWS: buildPerfRows(holdings, periodStart, periodEnd),
      PENETRATION_ROWS: buildPenetrationRows(penetration, 15),
      TRANSACTION_ROWS: tx.rows || '<tr><td colspan="7" style="text-align:center;color:#94a3b8;">本月无交易</td></tr>',
      MONTHLY_AUTO_AMOUNT: cny(tx.autoAmt),
      MONTHLY_MANUAL_AMOUNT: cny(tx.manualAmt),
      MONTHLY_TX_COUNT: String(tx.count),
      MONTHLY_FEE: cny(tx.fee),
      MONTHLY_AUTO_TX: String(tx.autoCount),
      MONTHLY_MANUAL_TX: String(tx.manualCount),
    });
  } else {
    Object.assign(vars, {
      WEEKLY_RETURN: periodReturn != null ? pct(periodReturn) : "N/A",
      WEEKLY_RETURN_CLASS: pctClass(periodReturn),
      TOTAL_PNL: cny(summary?.unrealized_pnl),
      TOTAL_PNL_CLASS: pctClass(summary?.unrealized_pnl),
      HOLDINGS_ROWS: buildHoldingsShort(holdings),
      PENETRATION_ROWS: buildPenetrationRows(penetration, 10),
    });
  }

  // Populate
  let html = template;
  for (const [key, val] of Object.entries(vars)) {
    html = html.replace(new RegExp(`\\{\\{${key}\\}\\}`, "g"), val);
  }

  // PDF
  let pdf: Uint8Array | null = null;
  let pdfError: string | undefined;
  try {
    pdf = await htmlToPdf(html);
  } catch (e: any) {
    pdfError = e.message;
    log.warn("PDF conversion failed, HTML-only mode", { error: e.message });
  }

  return { html, pdf, pdfError, generatedAt, periodStart, periodEnd };
}

export async function generateWeeklyReport(): Promise<ReportResult> {
  return buildReport("weekly");
}

export async function generateMonthlyReport(): Promise<ReportResult> {
  return buildReport("monthly");
}
