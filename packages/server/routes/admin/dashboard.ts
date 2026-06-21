/** /api/admin DASHBOARD — 聚合监控面板: DB大小 · 爬虫成功率 · API延迟 · 内存 · uptime
 *
 *  GET /api/admin/dashboard — 所有指标聚合为单一 JSON 响应
 *
 *  v2.4 — 2026-06-19
 */

import { Hono } from "hono";
import { query, queryOne, getDb } from "../../db";
import { log } from "../../middleware/logger";
import { join } from "node:path";

const router = new Hono();

/** DB file size in bytes (reads the WAL-mode DB file) */
function getDbSizeBytes(): number {
  try {
    const dbPath = process.env.DB_PATH || join(import.meta.dir, "..", "..", "..", "data", "fund.db");
    const f = Bun.file(dbPath);
    return f.size;
  } catch { return 0; }
}

/** Crude crawler success stats — counts NAV records updated recently */
function getCrawlerStats(): { nav_total: number; nav_fresh_24h: number; success_rate: number } {
  try {
    const db = getDb();
    const total = (db.query("SELECT COUNT(DISTINCT fund_code) as n FROM nav_history").get() as { n: number })?.n ?? 0;
    const fresh24 = (db.query("SELECT COUNT(DISTINCT fund_code) as n FROM nav_history WHERE date >= date('now','-1 day')").get() as { n: number })?.n ?? 0;
    const fundsHeld = (db.query("SELECT COUNT(*) as n FROM portfolio_snapshot WHERE held_shares > 0.001").get() as { n: number })?.n ?? 1;
    return {
      nav_total: total,
      nav_fresh_24h: fresh24,
      success_rate: fundsHeld > 0 ? +((fresh24 / fundsHeld) * 100).toFixed(1) : 0,
    };
  } catch { return { nav_total: 0, nav_fresh_24h: 0, success_rate: 0 }; }
}

/** GET /api/admin/dashboard — all metrics in a single response */
router.get("/dashboard", (c) => {
  const t0 = Date.now();

  // System metrics
  const mem = process.memoryUsage?.() ?? { rss: 0, heapTotal: 0, heapUsed: 0, external: 0 };
  const uptime = process.uptime();

  // DB metrics
  const dbSize = getDbSizeBytes();
  const crawler = getCrawlerStats();

  // DB state
  const txCount = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM transactions")?.n ?? 0;
  const lastTx = queryOne<{ t: string }>("SELECT MAX(trade_time) as t FROM transactions");
  const lastNav = queryOne<{ d: string }>("SELECT MAX(date) as d FROM nav_history");
  const heldFunds = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM portfolio_snapshot WHERE held_shares > 0.001")?.n ?? 0;
  const navStats = queryOne<{ total: number; funds: number }>(
    "SELECT COUNT(*) as total, COUNT(DISTINCT fund_code) as funds FROM nav_history"
  );
  const secTotal = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM fund_details")?.n ?? 0;
  const anomalies = query<{ seq: number; fund_code: string; anomaly: string }>(
    "SELECT seq, fund_code, anomaly FROM transactions WHERE anomaly IS NOT NULL LIMIT 10"
  );

  const responseMs = Date.now() - t0;

  return c.json({
    ok: true,
    timestamp: new Date().toISOString(),
    response_ms: responseMs,
    system: {
      uptime_sec: +uptime.toFixed(1),
      uptime_human: formatUptime(uptime),
      memory: {
        rss_mb: +(mem.rss / 1024 / 1024).toFixed(1),
        heap_used_mb: +(mem.heapUsed / 1024 / 1024).toFixed(1),
        heap_total_mb: +(mem.heapTotal / 1024 / 1024).toFixed(1),
      },
      node_version: process.version,
      platform: process.platform,
    },
    database: {
      size_bytes: dbSize,
      size_mb: +(dbSize / 1024 / 1024).toFixed(2),
    },
    crawler: {
      nav_total: crawler.nav_total,
      nav_fresh_24h: crawler.nav_fresh_24h,
      success_rate_pct: crawler.success_rate,
    },
    state: {
      transaction_count: txCount,
      last_transaction: lastTx?.t ?? null,
      last_nav_date: lastNav?.d ?? null,
      held_funds: heldFunds,
      nav_records: navStats?.total ?? 0,
      nav_funds: navStats?.funds ?? 0,
      securities_total: secTotal,
      anomaly_count: anomalies.length,
      recent_anomalies: anomalies,
    },
  });
});

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  const parts: string[] = [];
  if (d > 0) parts.push(`${d}d`);
  if (h > 0) parts.push(`${h}h`);
  if (m > 0) parts.push(`${m}m`);
  parts.push(`${s}s`);
  return parts.join(" ");
}

export default router;
