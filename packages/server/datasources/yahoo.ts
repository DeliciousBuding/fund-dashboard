/**
 * Yahoo Finance DataSource — US stocks, indices, forex.
 *
 * Wraps crawler/yahoofinance.ts functions behind the DataSource interface.
 */

import type { DataSource, Quote, HistoryPoint } from "./base";
import { fetchUSStockQuote, fetchUSStockHistory } from "../crawler/yahoofinance";
import { cachedFetch, TTL } from "../utils/api-cache";

export const yahooSource: DataSource = {
  name: "yahoo",
  markets: ["US"],

  async fetchQuote(code) {
    const cacheKey = `yh:quote:${code}`;
    return cachedFetch(cacheKey, TTL.STOCK_QUOTE, async () => {
      const q = await fetchUSStockQuote(code);
      if (!q) return null;
      return {
        code: q.symbol, market: "US", name: q.name,
        price: q.price, changePct: q.changePct, changeAmt: q.change,
        currency: q.currency, updatedAt: q.marketTime,
        open: q.open, high: q.high, low: q.low,
        volume: q.volume, previousClose: q.previousClose,
      };
    });
  },

  async fetchHistory(code, _market, days?) {
    const range = days ? `${Math.max(days, 1)}d` : "5y";
    const cacheKey = `yh:history:${code}:${range}`;
    return cachedFetch(cacheKey, TTL.NAV_HISTORY, async () => {
      const history = await fetchUSStockHistory(code, range === "1d" ? "1mo" : range);
      return history.map(d => ({
        date: d.date, price: d.close, changePct: d.change_pct,
      }));
    });
  },
};
