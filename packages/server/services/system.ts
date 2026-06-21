/**
 * Portfolio Service — system status.
 *
 * Extracted from services/portfolio.ts (2026-06-19).
 */

import { query, queryOne } from "../db";
import type { SystemStatus } from "../utils/types";

export function getSystemStatus(): SystemStatus {
  const lastTx = queryOne<{ t: string }>("SELECT MAX(trade_time) as t FROM transactions");
  const lastNav = queryOne<{ d: string }>("SELECT MAX(date) as d FROM nav_history");
  const navStats = queryOne<{ total: number; funds: number; first: string; last: string }>("SELECT COUNT(*) as total, COUNT(DISTINCT fund_code) as funds, MIN(date) as first, MAX(date) as last FROM nav_history");
  const txCount = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM transactions")?.n ?? 0;
  const heldCount = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM portfolio_snapshot WHERE held_shares > 0.001")?.n ?? 0;
  const fundCount = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM fund_details")?.n ?? 0;
  const secByType = query<{ security_type: string; n: number }>("SELECT COALESCE(security_type,'fund') as security_type, COUNT(*) as n FROM fund_details GROUP BY security_type");
  const anomalies = query<{ seq: number; fund_code: string; direction: string; trade_time: string; anomaly: string }>("SELECT seq, fund_code, direction, trade_time, anomaly FROM transactions WHERE anomaly IS NOT NULL LIMIT 30");
  const holdingsCount = queryOne<{ funds: number; total: number }>("SELECT COUNT(DISTINCT fund_code) as funds, COUNT(*) as total FROM fund_holdings");
  const indicesLatest = query<{ code: string; name: string; price: number; change_pct: number; updated_at: string }>("SELECT code, name, price, change_pct, updated_at FROM indices LIMIT 5");
  return {
    transactions: { count: txCount, last_trade: lastTx?.t },
    nav: { count: navStats?.total, funds_covered: navStats?.funds, date_range: { first: navStats?.first, last: navStats?.last } },
    portfolio: { total_securities: fundCount, held_securities: heldCount, by_type: Object.fromEntries(secByType.map((r) => [r.security_type, r.n])) },
    holdings_data: { funds_with_holdings: holdingsCount?.funds || 0, total_stock_positions: holdingsCount?.total || 0 },
    indices: indicesLatest.map((r) => ({ code: r.code, name: r.name, price: r.price, change_pct: r.change_pct, updated: r.updated_at })),
    anomalies: { count: anomalies.length, items: anomalies },
  };
}
