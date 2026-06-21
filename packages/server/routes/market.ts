/** /api/market endpoints — index data and US stock lookup via Yahoo Finance API
 *
 *  Uses Yahoo Finance v8 chart API for real-time index quotes and US stock data.
 *  Caches results in SQLite (indices table, stock_realtime table, stock_kline_cache table).
 */

import { Hono } from "hono";
import { query, queryOne, getRwDb } from "../db";
import { log } from "../middleware/logger";
import {
  fetchIndexQuote,
  fetchIndexHistory,
  fetchUSStockQuote,
  fetchUSStockHistory,
} from "../crawler/yahoofinance";

interface IndexRow {
  code: string;
  name: string;
  market: string;
  price: number | null;
  change_pct: number | null;
  change_amt: number | null;
  updated_at: string;
}

const router = new Hono();

// ═══════════════════════════════════════════
// 1. GET /api/market/indices
//    Return latest cached index data for NASDAQ, S&P 500
// ═══════════════════════════════════════════

router.get("/indices", c => {
  const rows = query<IndexRow>("SELECT * FROM indices ORDER BY code");
  return c.json(rows.map((r) => ({
    code: r.code,
    name: r.name,
    market: r.market,
    price: r.price ? +r.price : null,
    change_pct: r.change_pct != null ? +r.change_pct : null,
    change_amt: r.change_amt != null ? +r.change_amt : null,
    updated_at: r.updated_at,
  })));
});

// ═══════════════════════════════════════════
// 2. GET /api/market/index/:code
//    Live-refresh from Yahoo, persist to DB, return single index
// ═══════════════════════════════════════════

router.get("/index/:code", async c => {
  const code = c.req.param("code"); // e.g. "^IXIC" for NASDAQ Composite, "^GSPC" for S&P 500

  try {
    const quote = await fetchIndexQuote(code);
    if (!quote) {
      // Fall back to cached data
      const cached = queryOne<IndexRow>("SELECT * FROM indices WHERE code = ?", code);
      if (!cached) return c.json({ error: "index not found", code }, 404);
      return c.json({
        code: cached.code,
        name: cached.name,
        price: +cached.price,
        change_pct: +cached.change_pct,
        change_amt: +cached.change_amt,
        updated_at: cached.updated_at,
        source: "cache",
      });
    }

    // Persist to indices table
    const db = getRwDb();
    const market = code.startsWith("^") ? "US" : (quote.currency === "CNY" ? "CN" : "US");
    db.run(
      `INSERT OR REPLACE INTO indices (code, name, market, price, change_pct, change_amt, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, datetime('now'))`,
      [code, quote.name, market, quote.price, quote.change_pct, quote.change],
    );

    return c.json({
      code,
      name: quote.name,
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
      source: "live",
    });
  } catch (e: any) {
    log.warn(`index fetch failed: ${code} — ${e.message}`);
    // Fall back to cache
    const cached = queryOne<any>("SELECT * FROM indices WHERE code = ?", code);
    if (!cached) return c.json({ error: "index not found and fetch failed", code }, 502);
    return c.json({
      code: cached.code,
      name: cached.name,
      price: +cached.price,
      change_pct: +cached.change_pct,
      change_amt: +cached.change_amt,
      updated_at: cached.updated_at,
      source: "cache",
    });
  }
});

// ═══════════════════════════════════════════
// 3. GET /api/market/index/:code/history
//    Fetch index history from Yahoo, persist to nav_history format
// ═══════════════════════════════════════════

const CODE_TO_INDEX: Record<string, string> = {
  "IXIC": "^IXIC",
  "NDX": "^NDX",
  "GSPC": "^GSPC",
  "DJI": "^DJI",
};

router.get("/index/:code/history", async c => {
  const codeParam = c.req.param("code");
  const range = c.req.query("range") || "1y";

  // Normalize: accept code with or without caret
  const symbol = CODE_TO_INDEX[codeParam.toUpperCase()] || (codeParam.startsWith("^") ? codeParam : `^${codeParam}`);

  try {
    const rows = await fetchIndexHistory(symbol, range);
    if (!rows.length) return c.json({ error: "no history data", symbol }, 404);

    return c.json({
      symbol: symbol.replace("^", ""),
      count: rows.length,
      range,
      data: rows.map(r => ({
        date: r.date,
        close: r.close,
        change_pct: r.change_pct,
      })),
    });
  } catch (e: any) {
    log.warn(`index history fetch failed: ${symbol} — ${e.message}`);
    return c.json({ error: "fetch failed", symbol }, 502);
  }
});

// ═══════════════════════════════════════════
// 4. GET /api/market/exchange-rate
//    Current USD/CNY exchange rate
// ═══════════════════════════════════════════

router.get("/exchange-rate", async c => {
  try {
    const { fetchExchangeRate } = await import("../crawler/yahoofinance");
    const rate = await fetchExchangeRate();
    if (!rate) return c.json({ error: "exchange rate fetch failed" }, 502);
    return c.json(rate);
  } catch (e: any) {
    return c.json({ error: e.message }, 502);
  }
});

export default router;
