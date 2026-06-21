/** /api/admin/freshness — 数据新鲜度 & 告警端点 */

import { Hono } from "hono";
import { query, queryOne } from "../../db";
import { checkAndNotify, getNotifyConfig, handleFeishuEventCallback, getFeishuBotStatus } from "../../services/index";

const router = new Hono();

// ═══════════ Data Freshness (V2.6) ═══════════

router.get("/freshness", c => {
  const lastTx = queryOne<any>("SELECT MAX(trade_time) as t FROM transactions");
  const lastNav = queryOne<any>("SELECT MAX(date) as d FROM nav_history");
  const anomalies = query<any>("SELECT seq, fund_code, direction, trade_time, anomaly FROM transactions WHERE anomaly IS NOT NULL LIMIT 30");
  const fundsWithoutNav = query<any>("SELECT fd.fund_code, fd.fund_name, COALESCE(fd.security_type,'fund') as security_type FROM fund_details fd WHERE fd.fund_code NOT IN (SELECT DISTINCT fund_code FROM nav_history)");
  const staleFunds = query<any>("SELECT nh.fund_code, fd.fund_name, MAX(nh.date) as last_nav, CAST(julianday('now') - julianday(MAX(nh.date)) AS INTEGER) as stale_days FROM nav_history nh JOIN fund_details fd ON nh.fund_code = fd.fund_code GROUP BY nh.fund_code HAVING MAX(nh.date) < date('now', '-2 days') ORDER BY stale_days DESC");

  return c.json({
    last_transaction: lastTx?.t,
    last_nav_date: lastNav?.d?.substring(0, 10),
    anomaly_count: anomalies.length,
    missing_nav_securities: fundsWithoutNav.map((f: any) => ({ code: f.fund_code, name: f.fund_name, type: f.security_type })),
    stale_securities: staleFunds.map((f: any) => ({
      code: f.fund_code,
      name: f.fund_name,
      last_nav: f.last_nav,
      stale_days: f.stale_days,
    })),
    actionable: staleFunds.length > 0
      ? `建议运行 crawl_nav 刷新 ${staleFunds.length} 只过期证券的价格数据`
      : fundsWithoutNav.length > 0
        ? `建议先添加 ${fundsWithoutNav.length} 只证券的价格数据`
        : "数据新鲜度正常",
    health: staleFunds.length === 0 && fundsWithoutNav.length === 0 ? "fresh" : staleFunds.length > 3 ? "stale" : "degraded",
  });
});

router.get("/stale-report", c => {
  const staleFunds = query<any>("SELECT nh.fund_code, fd.fund_name, COALESCE(fd.security_type,'fund') as security_type, fd.market, MAX(nh.date) as last_nav, CAST(julianday('now') - julianday(MAX(nh.date)) AS INTEGER) as stale_days FROM nav_history nh JOIN fund_details fd ON nh.fund_code = fd.fund_code GROUP BY nh.fund_code HAVING MAX(nh.date) < date('now', '-2 days') ORDER BY stale_days DESC");
  const missingNav = query<any>("SELECT fd.fund_code, fd.fund_name, COALESCE(fd.security_type,'fund') as security_type FROM fund_details fd WHERE fd.fund_code NOT IN (SELECT DISTINCT fund_code FROM nav_history)");

  return c.json({
    generated_at: new Date().toISOString(),
    summary: {
      total_securities: queryOne<any>("SELECT COUNT(*) as n FROM fund_details")?.n || 0,
      stale_price_count: staleFunds.length,
      missing_price_count: missingNav.length,
      health: staleFunds.length > 3 || missingNav.length > 0 ? "action_needed" : "healthy",
    },
    stale_securities: staleFunds.map((f: any) => ({
      code: f.fund_code,
      name: f.fund_name,
      type: f.security_type,
      market: f.market,
      last_price_date: f.last_nav,
      stale_days: f.stale_days,
      recommendation: f.stale_days > 7 ? "urgent_refresh" : f.stale_days > 3 ? "refresh_recommended" : "monitor",
    })),
    missing_nav_securities: missingNav.map((f: any) => ({ code: f.fund_code, name: f.fund_name, type: f.security_type })),
    agent_brief: staleFunds.length > 0
      ? `${staleFunds.length} 只证券价格过期超过2天，建议运行 crawl_nav(all:true) 刷新。${missingNav.length > 0 ? `另有 ${missingNav.length} 只缺少价格数据。` : ''}`
      : "所有证券价格数据新鲜度正常。",
  });
});

// ═══════════ Alert / Notification ═══════════

router.post("/alerts/check", async c => {
  const body = await c.req.json().catch(() => ({}));
  const alerts = checkAndNotify({
    priceChangeThresholdPct: body.price_change_pct ?? 5,
    drawdownThresholdPct: body.drawdown_pct ?? 10,
    staleDaysThreshold: body.stale_days ?? 3,
  });
  const types = alerts.reduce((acc, a) => { acc[a.type] = (acc[a.type] || 0) + 1; return acc; }, {} as Record<string, number>);
  return c.json({
    ok: true,
    total: alerts.length,
    webhook_configured: !!process.env.FEISHU_WEBHOOK,
    by_type: types,
    alerts: alerts.map(a => ({ type: a.type, title: a.title, category: a.category, detail: a.detail, severity: a.severity, time: a.time })),
  });
});

router.get("/alerts/config", c => {
  return c.json({
    config: getNotifyConfig(),
    webhook_configured: !!process.env.FEISHU_WEBHOOK,
  });
});

// ═══════════ Feishu Bot (V2.6) ═══════════

router.post("/feishu/event", async c => {
  const body = await c.req.json();
  const headers: Record<string, string> = {};
  c.req.raw.headers.forEach((v, k) => { headers[k] = v; });
  const result = await handleFeishuEventCallback(body, headers);
  return c.json(result);
});

router.get("/feishu/status", c => {
  return c.json(getFeishuBotStatus());
});

export default router;
