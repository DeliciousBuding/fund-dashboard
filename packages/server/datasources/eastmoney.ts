/**
 * Eastmoney DataSource — Chinese funds, A-stock, HK stock.
 *
 * Wraps the existing crawler/eastmoney.ts functions behind the
 * DataSource interface defined in datasources/base.ts.
 */

import type { DataSource, Quote, HistoryPoint } from "./base";
import {
  fetchRealtimeNav, fetchNavHistory,
  fetchFundMasterList, fetchFundInfo,
  fetchStockRealtime, fetchStockKline, detectMarket,
} from "../crawler/eastmoney";
import { cachedFetch, TTL } from "../utils/api-cache";

export const eastmoneySource: DataSource = {
  name: "eastmoney",
  markets: ["CN", "SH", "SZ", "HK"],

  async fetchQuote(code, market?) {
    const mkt = market || detectMarket(code);
    const cacheKey = `em:quote:${mkt}:${code}`;

    return cachedFetch(cacheKey, TTL.STOCK_QUOTE, async () => {
      // Chinese mutual funds (6-digit codes)
      if (mkt === "CN" || code.length === 6) {
        const nav = await fetchRealtimeNav(code);
        if (nav) {
          return {
            code, market: "CN", name: code,
            price: nav.nav, changePct: nav.change_pct, changeAmt: 0,
            currency: "CNY", updatedAt: new Date().toISOString(),
          };
        }
        return null;
      }

      // A-stock / HK stock
      const q = await fetchStockRealtime(code, mkt as any);
      if (!q) return null;
      return {
        code: q.code, market: q.market, name: q.name,
        price: q.price, changePct: q.change_pct, changeAmt: q.change_amt,
        currency: "CNY", updatedAt: new Date().toISOString(),
        open: q.open, high: q.high, low: q.low,
        volume: q.volume, previousClose: q.price - q.change_amt,
      };
    });
  },

  async fetchHistory(code, market?, days?) {
    const mkt = market || detectMarket(code);
    const cacheKey = `em:history:${mkt}:${code}:${days ?? "all"}`;

    return cachedFetch(cacheKey, TTL.NAV_HISTORY, async () => {
      // Chinese mutual funds
      if (mkt === "CN" || code.length === 6) {
        const nav = await fetchNavHistory(code);
        return nav.map(d => ({
          date: d.date, price: d.unit_nav, changePct: d.change_pct,
        }));
      }

      // A-stock / HK stock
      const kl = await fetchStockKline(code, mkt as any, { lmt: days || 5000 });
      if (!kl) return [];
      return kl.klines.map(k => ({
        date: k.date, price: k.close, changePct: k.change_pct,
      }));
    });
  },
};
