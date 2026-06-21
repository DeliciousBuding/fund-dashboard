/**
 * Feishu Notification Service — checkAndNotify(config) scans price changes,
 * drawdown, data staleness, and DCA-day; sends a Feishu interactive card via
 * FEISHU_WEBHOOK; returns the triggered alert list.
 */

import { query } from "../db";
import { log } from "../middleware/logger";

// ── Types ────────────────────────────────────────────────────────────

export interface NotifyConfig {
  priceChangeThresholdPct?: number; // default 5
  drawdownThresholdPct?: number;    // default 10
  staleDaysThreshold?: number;      // default 3
  dcaDayOfWeek?: number[];          // default [1-5]
}

export interface AlertItem {
  type: "price_change" | "drawdown" | "data_stale" | "dca_day";
  title: string; category: string; detail: string; time: string;
  severity: "info" | "warning" | "critical";
}

// ── Feishu card builder ──────────────────────────────────────────────

function nowISO() { return new Date().toISOString().replace("T"," ").substring(0,19); }

function sendCard(alerts: AlertItem[]) {
  const w = process.env.FEISHU_WEBHOOK;
  if (!w) { log.warn("notify: FEISHU_WEBHOOK not configured"); return; }
  const color = alerts.some(a=>a.severity==="critical")?"red"
    : alerts.some(a=>a.severity==="warning")?"orange":"blue";
  const md = alerts.map(a =>
    `**${a.title}**  \n分类：${a.category}  \n详情：${a.detail}  \n时间：${a.time}`
  ).join("\n---\n");
  fetch(w, {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      msg_type: "interactive",
      card: {
        header: { title: { tag: "plain_text", content: "📊 Fund Dashboard 告警" }, template: color },
        elements: [
          { tag: "div", text: { tag: "lark_md", content: md } },
          { tag: "note", elements: [{ tag: "plain_text", content: `共 ${alerts.length} 条告警 · ${nowISO()}` }] },
        ],
      },
    }),
  }).catch((e: unknown) => log.error("notify: webhook failed", { error: String(e) }));
}

// ── Check functions ──────────────────────────────────────────────────

function severity(v: number, t: number): AlertItem["severity"] {
  return Math.abs(v) >= t * 2 ? "critical" : "warning";
}

function checkPriceChanges(t: number): AlertItem[] {
  return query<{ fund_code: string; fund_name: string; date: string; daily_change_pct: number }>(
    `SELECT nh.fund_code, fd.fund_name, nh.date, nh.daily_change_pct
     FROM nav_history nh JOIN fund_details fd ON nh.fund_code = fd.fund_code
     WHERE nh.date >= date('now','-1 day') AND ABS(nh.daily_change_pct) >= ?
     ORDER BY ABS(nh.daily_change_pct) DESC LIMIT 10`, t,
  ).map(r => ({
    type: "price_change", title: `${r.fund_name}（${r.fund_code}）`, category: "价格异动",
    detail: `${r.date} 日涨跌 ${r.daily_change_pct>=0?"+":""}${r.daily_change_pct.toFixed(2)}%，超过阈值 ±${t}%`,
    time: nowISO(), severity: severity(r.daily_change_pct, t),
  }));
}

function checkDrawdown(t: number): AlertItem[] {
  return query<{ fund_code: string; fund_name: string; pnl_pct: number }>(
    `SELECT ps.fund_code, ps.fund_name, ps.pnl_pct FROM portfolio_snapshot ps
     WHERE ps.total_cost < 0 AND ps.pnl_pct <= ? ORDER BY ps.pnl_pct ASC LIMIT 10`, -t,
  ).map(r => ({
    type: "drawdown", title: `${r.fund_name}（${r.fund_code}）`, category: "回撤告警",
    detail: `浮动亏损 ${r.pnl_pct.toFixed(2)}%，超过回撤阈值 ${t}%`,
    time: nowISO(), severity: severity(r.pnl_pct, t),
  }));
}

function checkDataStaleness(d: number): AlertItem[] {
  const staleDate = new Date();
  staleDate.setDate(staleDate.getDate() - d);
  const threshold = staleDate.toISOString().substring(0, 10);
  return query<{ fund_code: string; fund_name: string; last_nav: string; stale_days: number }>(
    `SELECT nh.fund_code, fd.fund_name, MAX(nh.date) AS last_nav,
            CAST(julianday('now')-julianday(MAX(nh.date)) AS INTEGER) AS stale_days
     FROM nav_history nh JOIN fund_details fd ON nh.fund_code = fd.fund_code
     GROUP BY nh.fund_code HAVING MAX(nh.date) < ?
     ORDER BY stale_days DESC LIMIT 10`, threshold,
  ).map(r => ({
    type: "data_stale", title: `${r.fund_name}（${r.fund_code}）`, category: "数据过期",
    detail: `最近净值 ${r.last_nav}，已过期 ${r.stale_days} 天（阈值 ${d} 天）`,
    time: nowISO(), severity: r.stale_days >= d * 3 ? "critical" : "warning",
  }));
}

function checkDcaDay(days: number[]): AlertItem[] {
  const today = new Date().getDay();
  if (!days.includes(today)) return [];
  const dn = ["周日","周一","周二","周三","周四","周五","周六"];
  return [{ type: "dca_day", title: dn[today], category: "定投日提醒",
    detail: "今天是定投日，可检查持仓并按计划执行定投操作。", time: nowISO(), severity: "info" }];
}

// ── Main entry ───────────────────────────────────────────────────────

export function checkAndNotify(cfg: NotifyConfig = {}): AlertItem[] {
  const pc = cfg.priceChangeThresholdPct ?? 5;
  const dd = cfg.drawdownThresholdPct ?? 10;
  const sd = cfg.staleDaysThreshold ?? 3;
  const dw = cfg.dcaDayOfWeek ?? [1,2,3,4,5];

  const alerts: AlertItem[] = [
    ...checkPriceChanges(pc),
    ...checkDrawdown(dd),
    ...checkDataStaleness(sd),
    ...checkDcaDay(dw),
  ];

  if (alerts.length === 0) { log.info("notify: no alerts"); return []; }
  log.info(`notify: ${alerts.length} alert(s)`, { types: [...new Set(alerts.map(a=>a.type))].join(",") });
  sendCard(alerts);
  return alerts;
}

export function getNotifyConfig(): NotifyConfig {
  return { priceChangeThresholdPct: 5, drawdownThresholdPct: 10, staleDaysThreshold: 3, dcaDayOfWeek: [1,2,3,4,5] };
}
