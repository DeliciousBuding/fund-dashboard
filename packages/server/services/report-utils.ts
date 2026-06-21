/**
 * Report Utilities — formatting helpers, HTML→PDF conversion, chart builders
 *
 * Extracted from services/report.ts to keep per-file size < 300 lines.
 */

import { join } from "node:path";
import { existsSync, mkdirSync, unlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import { query, queryOne } from "../db";
import { getPortfolioAllocation } from "./index";
import { log } from "../middleware/logger";

// ══════════════════════════════════════════════════════════════════════

/** Format a number as ¥ currency with 2 decimal places */
export function cny(n: number | null | undefined): string {
  if (n == null || isNaN(n)) return "0.00";
  return n.toFixed(2);
}

/** Format percentage with sign */
export function pct(n: number | null | undefined): string {
  if (n == null || isNaN(n)) return "0.00%";
  return (n >= 0 ? "+" : "") + n.toFixed(2) + "%";
}

/** CSS sign class for coloring */
export function pctClass(n: number | null | undefined): string {
  if (n == null || isNaN(n)) return "";
  return n >= 0 ? "positive" : "negative";
}

/** Escape HTML entities */
export function esc(s: string | null | undefined): string {
  if (s == null) return "";
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

// ══════════════════════════════════════════════════════════════════════

/** Build a simple bar chart HTML from daily values */
export function buildTrendBars(
  dailyValues: { date: string; value: number }[],
  maxBars = 30,
): { bars: string; labels: string } {
  const values = dailyValues.slice(-maxBars);
  if (values.length === 0) return { bars: "", labels: "" };

  const maxVal = Math.max(...values.map((v) => v.value), 1);
  const bars = values
    .map((v) => {
      const h = Math.max(4, (v.value / maxVal) * 100);
      return `<div class="bar" style="height:${h.toFixed(0)}%"><span class="bar-val">${(v.value / 1000).toFixed(1)}k</span></div>`;
    })
    .join("\n");

  const labels = values
    .map((v) => `<span>${v.date.substring(5)}</span>`)
    .join("\n");

  return { bars, labels };
}

/** Build allocation bar rows HTML */
export function buildAllocationRows(allocation: ReturnType<typeof getPortfolioAllocation>): string {
  if (!allocation) return "";

  const colors = ["#2563eb", "#7c3aed", "#059669", "#d97706", "#dc2626", "#0891b2", "#4f46e5", "#db2777"];
  const rows: string[] = [];
  let ci = 0;
  for (const bucket of allocation.by_security_type) {
    const color = colors[ci % colors.length];
    rows.push(`<div class="alloc-row">
      <span class="alloc-label">${esc(bucket.label)}</span>
      <div class="alloc-bar-bg"><div class="alloc-bar-fill" style="width:${bucket.weight_pct.toFixed(1)}%;background:${color}"></div></div>
      <span class="alloc-pct">${bucket.weight_pct.toFixed(1)}%</span>
    </div>`);
    ci++;
  }
  return rows.join("\n");
}

/** Build risk warning items from portfolio analysis */
export function buildRiskItems(totalBuy: number, totalFee: number): string {
  const items: string[] = [];

  const holdings = query<{ fund_code: string; fund_name: string; current_value: number }>(
    "SELECT fund_code, fund_name, current_value FROM portfolio_snapshot WHERE held_shares > 0.001",
  );
  const totalVal = holdings.reduce((s, h) => s + (h.current_value || 0), 0);

  // Concentration risk
  for (const h of holdings) {
    const w = totalVal > 0 ? ((h.current_value || 0) / totalVal) * 100 : 0;
    if (w > 30) {
      items.push(`<div class="risk-item">集中度风险：${esc(h.fund_name)} (${esc(h.fund_code)}) 占组合 ${w.toFixed(1)}%，超过30%警戒线。建议分散配置。</div>`);
    }
  }

  // Large drawdown per holding
  const codes = holdings.map((h) => h.fund_code);
  if (codes.length > 0) {
    const drawdowns = query<{ fund_code: string; fund_name: string; max_dd: number }>(
      `SELECT ps.fund_code, ps.fund_name,
        CASE WHEN ps.total_cost != 0 THEN ((ps.current_value + ps.total_cost) / ABS(ps.total_cost)) * 100 ELSE 0 END as max_dd
       FROM portfolio_snapshot ps WHERE ps.fund_code IN (${codes.map(() => "?").join(",")}) AND ps.total_cost != 0`,
      codes,
    );
    for (const d of drawdowns) {
      if (d.max_dd < -15) {
        items.push(`<div class="risk-item">大幅亏损：${esc(d.fund_name)} (${esc(d.fund_code)}) 累计亏损 ${d.max_dd.toFixed(1)}%，超过-15%警戒线。考虑止损或补仓。</div>`);
      }
    }
  }

  // Stale data
  const lastNav = queryOne<{ d: string }>("SELECT MAX(date) as d FROM nav_history");
  if (lastNav?.d) {
    const daysAgo = Math.floor((Date.now() - new Date(lastNav.d).getTime()) / 86400000);
    if (daysAgo > 3) {
      items.push(`<div class="risk-item">数据延迟：最近净值日期为 ${lastNav.d.substring(0, 10)}，已延迟 ${daysAgo} 天。报告数据可能滞后。</div>`);
    }
  }

  // Too few holdings
  if (holdings.length < 3 && holdings.length > 0) {
    items.push(`<div class="risk-item">持仓数量少：当前仅 ${holdings.length} 个持仓，分散度不足。建议增加至5只以上。</div>`);
  }

  // Fee ratio
  if (totalBuy > 0 && totalFee / totalBuy > 0.015) {
    items.push(`<div class="risk-item">费率偏高：累计手续费 ¥${cny(totalFee)} 占买入金额 ${((totalFee / totalBuy) * 100).toFixed(2)}%，超过1.5%。</div>`);
  }

  if (items.length === 0) {
    items.push('<div class="risk-item">当前未检测到显著风险指标。组合整体健康。</div>');
  }
  return items.join("\n");
}

/** Build penetration analysis table rows */
export function buildPenetrationRows(
  penetration: { penetration: { stock_code: string; stock_name: string; total_exposure_cny: number; weight_pct: number; held_by_funds: { fund_name: string; weight_pct: number }[] }[] } | null,
  limit = 10,
): string {
  if (!penetration || !penetration.penetration.length) {
    return '<tr><td colspan="5" style="text-align:center;color:#94a3b8;">无穿透数据</td></tr>';
  }
  return penetration.penetration.slice(0, limit).map((p) => {
    const funds = p.held_by_funds.map((f) => `${esc(f.fund_name)}(${f.weight_pct.toFixed(1)}%)`).join("、");
    return `<tr>
      <td>${esc(p.stock_code)}</td>
      <td>${esc(p.stock_name)}</td>
      <td class="num">¥${cny(p.total_exposure_cny)}</td>
      <td class="num">${p.weight_pct.toFixed(2)}%</td>
      <td style="font-size:10px">${funds}</td>
    </tr>`;
  }).join("\n");
}

// ══════════════════════════════════════════════════════════════════════
//  HTML → PDF via Chrome headless
// ══════════════════════════════════════════════════════════════════════

function findChrome(): string | null {
  const candidates = [
    "C:/Program Files/Google/Chrome/Application/chrome.exe",
    "C:/Program Files (x86)/Google/Chrome/Application/chrome.exe",
    "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe",
    "C:/Program Files/Microsoft/Edge/Application/msedge.exe",
    "/usr/bin/google-chrome",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
  ];
  for (const c of candidates) {
    if (existsSync(c)) return c;
  }
  return null;
}

/** Convert HTML string to PDF binary using Chrome headless --print-to-pdf */
export async function htmlToPdf(html: string): Promise<Uint8Array> {
  const chrome = findChrome();
  if (!chrome) {
    throw new Error("No Chromium-based browser found for PDF conversion. Install Chrome or Edge.");
  }

  const tmpDir = join(tmpdir(), "fund-report");
  if (!existsSync(tmpDir)) mkdirSync(tmpDir, { recursive: true });

  const htmlPath = join(tmpDir, `report-${Date.now()}.html`);
  const pdfPath = join(tmpDir, `report-${Date.now()}.pdf`);

  try {
    Bun.write(htmlPath, html);
    const proc = Bun.spawn([chrome, "--headless", "--disable-gpu", "--no-sandbox", `--print-to-pdf=${pdfPath}`, htmlPath]);
    const exitCode = await proc.exited;
    if (exitCode !== 0) {
      const stderr = await new Response(proc.stderr).text();
      throw new Error(`Chrome exited with code ${exitCode}: ${stderr}`);
    }
    return await Bun.file(pdfPath).bytes();
  } finally {
    try { if (existsSync(htmlPath)) unlinkSync(htmlPath); } catch { /* best-effort */ }
    try { if (existsSync(pdfPath)) unlinkSync(pdfPath); } catch { /* best-effort */ }
  }
}
