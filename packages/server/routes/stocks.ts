/** /api/stocks/:code endpoint — US stock detail
 *
 *  Returns price snapshot, history, and profile for a single US stock.
 *  Queries market=US by default.
 */

import { Hono } from "hono";
import { query, queryOne, getRwDb } from "../db";
import { log } from "../middleware/logger";
import {
  fetchUSStockQuote,
  fetchUSStockHistory,
} from "../crawler/yahoofinance";

interface StockRealtimeRow {
  code: string;
  name: string;
  market: string;
  price: number;
  open: number;
  high: number;
  low: number;
  change_pct: number;
  change_amt: number;
  volume: number;
  amount: number;
  pe: number | null;
  total_mv: number | null;
  high52: number | null;
  low52: number | null;
  updated_at: string;
}

interface StockKlineRow {
  date: string;
  open: number;
  close: number;
  high: number;
  low: number;
  volume: number;
  change_pct: number;
}

interface StockProfileRow {
  sector: string;
  industry: string;
  market_cap: number | null;
  pe: number | null;
  description: string;
}

const router = new Hono();

// ═══════════════════════════════════════════
// GET /api/stocks/:code?market=US
//   US stock detail: price snapshot + history + DB profile
// ═══════════════════════════════════════════

router.get("/:code", async c => {
  const code = c.req.param("code").toUpperCase();
  const market = (c.req.query("market") || "US").toUpperCase();
  const range = c.req.query("range") || "1y";

  if (market !== "US") {
    return c.json({ error: "currently only US market is supported via Yahoo Finance", market }, 400);
  }

  try {
    // Fetch live quote and history in parallel
    const [quote, history] = await Promise.all([
      fetchUSStockQuote(code),
      fetchUSStockHistory(code, range),
    ]);

    if (!quote) {
      // Fall back to cached realtime data
      const cachedRt = queryOne<StockRealtimeRow>(
        "SELECT * FROM stock_realtime WHERE code = ? AND market = ?",
        code, market
      );

      if (!cachedRt) {
        return c.json({ error: "stock not found and fetch failed", code, market }, 404);
      }

      const cachedKlines = query<StockKlineRow>(
        "SELECT * FROM stock_kline_cache WHERE code = ? AND market = ? ORDER BY date DESC LIMIT 250",
        code, market
      );

      const profile = queryOne<StockProfileRow>(
        "SELECT * FROM stock_profile WHERE code = ? AND market = ?",
        code, market
      );

      return c.json({
        code: cachedRt.code,
        name: cachedRt.name,
        market: cachedRt.market,
        price: +cachedRt.price,
        open: +cachedRt.open,
        high: +cachedRt.high,
        low: +cachedRt.low,
        change_pct: +cachedRt.change_pct,
        change_amt: +cachedRt.change_amt,
        volume: +cachedRt.volume,
        amount: +cachedRt.amount,
        pe: cachedRt.pe != null ? +cachedRt.pe : null,
        total_mv: cachedRt.total_mv != null ? +cachedRt.total_mv : null,
        high52: cachedRt.high52 != null ? +cachedRt.high52 : null,
        low52: cachedRt.low52 != null ? +cachedRt.low52 : null,
        updated_at: cachedRt.updated_at,
        profile: profile ? {
          sector: profile.sector,
          industry: profile.industry,
          market_cap: profile.market_cap != null ? +profile.market_cap : null,
          pe: profile.pe != null ? +profile.pe : null,
          description: profile.description,
        } : null,
        history: cachedKlines.map((r) => ({
          date: r.date,
          open: +r.open,
          close: +r.close,
          high: +r.high,
          low: +r.low,
          volume: +r.volume,
          change_pct: +r.change_pct,
        })),
        source: "cache",
      });
    }

    // Persist realtime quote to DB
    const db = getRwDb();
    db.run(
      `INSERT OR REPLACE INTO stock_realtime
       (code, market, name, price, open, high, low, change_pct, change_amt, volume, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
      [code, market, quote.name, quote.price, quote.open, quote.high, quote.low, quote.change_pct, quote.change, quote.volume],
    );

    // Persist daily kline data
    if (history.length > 0) {
      const insertKline = db.prepare(`
        INSERT OR REPLACE INTO stock_kline_cache
        (code, market, date, open, close, high, low, volume, change_pct)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
      `);
      const doInsert = db.transaction((rows: { date: string; open: number; close: number; high: number; low: number; volume: number; change_pct: number }[]) => {
        for (const r of rows) {
          insertKline.run(code, market, r.date, r.open, r.close, r.high, r.low, r.volume, r.change_pct);
        }
      });
      doInsert(history);
    }

    // Look up profile from DB
    const profile = queryOne<StockProfileRow>(
      "SELECT * FROM stock_profile WHERE code = ? AND market = ?",
      code, market
    );

    return c.json({
      code,
      name: quote.name,
      market,
      price: quote.price,
      previous_close: quote.previousClose,
      change: quote.change,
      change_pct: quote.change_pct,
      high: quote.high,
      low: quote.low,
      open: quote.open,
      volume: quote.volume,
      currency: quote.currency,
      market_time: quote.marketTime,
      profile: profile ? {
        sector: profile.sector,
        industry: profile.industry,
        market_cap: profile.market_cap != null ? +profile.market_cap : null,
        pe: profile.pe != null ? +profile.pe : null,
        description: profile.description,
      } : null,
      history: history.map(r => ({
        date: r.date,
        close: r.close,
        change_pct: r.change_pct,
      })),
      source: "live",
    });
  } catch (e: any) {
    log.warn(`stock fetch failed: ${code} — ${e.message}`);
    return c.json({ error: "stock fetch failed", code, message: e.message }, 502);
  }
});

export default router;
