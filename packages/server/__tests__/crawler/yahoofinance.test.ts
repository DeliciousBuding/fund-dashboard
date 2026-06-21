/** Yahoo Finance crawler unit tests — mock globalThis.fetch */
import { describe, test, expect, beforeAll, afterAll } from "bun:test";

let originalFetch: typeof globalThis.fetch;

beforeAll(() => { originalFetch = globalThis.fetch; });
afterAll(() => { globalThis.fetch = originalFetch; });

import { fetchUSStockQuote, fetchUSStockHistory } from "../../crawler/yahoofinance";

// ═══════════════════════════════════════════════════════════════════════

function makeYahooResponse(opts: {
  symbol?: string;
  price?: number;
  previousClose?: number;
  name?: string;
  currency?: string;
  timestamps?: number[];
  closes?: number[];
  highs?: number[];
  lows?: number[];
  opens?: number[];
  volumes?: number[];
}) {
  const ts = opts.timestamps || [Date.now() / 1000];
  const closes = opts.closes || [opts.price || 180.0];
  return {
    chart: {
      result: [{
        meta: {
          currency: opts.currency || "USD",
          symbol: opts.symbol || "NVDA",
          exchangeName: "NMS",
          instrumentType: "EQUITY",
          regularMarketPrice: opts.price || 180.25,
          regularMarketTime: ts[ts.length - 1],
          regularMarketDayHigh: opts.highs?.[0] ?? 182.0,
          regularMarketDayLow: opts.lows?.[0] ?? 178.0,
          regularMarketVolume: opts.volumes?.[0] ?? 50000000,
          chartPreviousClose: opts.previousClose ?? 178.0,
          longName: opts.name || "NVIDIA Corporation",
          shortName: "NVIDIA",
          dataGranularity: "1d",
          range: ts.length > 1 ? "5d" : "1d",
        },
        timestamp: ts,
        indicators: {
          quote: [{
            open: opts.opens || ts.map(() => 179.0),
            high: opts.highs || ts.map(() => 182.0),
            low: opts.lows || ts.map(() => 178.0),
            close: closes,
            volume: opts.volumes || ts.map(() => 50000000),
          }],
        },
      }],
      error: null,
    },
  };
}

// ═══════════════════════════════════════════════════════════════════════

describe("fetchUSStockQuote", () => {
  test("returns StockQuote with price/change/change_pct for NVDA", async () => {
    globalThis.fetch = (async (_url: string) => ({
      json: async () => makeYahooResponse({
        symbol: "NVDA", price: 180.25, previousClose: 178.0,
        name: "NVIDIA Corporation", currency: "USD",
      }),
      ok: true, status: 200,
      text: async () => "",
    } as any)) as typeof globalThis.fetch;

    const quote = await fetchUSStockQuote("NVDA");
    expect(quote).not.toBeNull();
    expect(quote!.symbol).toBe("NVDA");
    expect(quote!.name).toContain("NVIDIA");
    expect(quote!.price).toBe(180.25);
    expect(quote!.currency).toBe("USD");
    // StockQuote uses snake_case: change_pct
    expect(quote!).toHaveProperty("change_pct");
    expect(quote!).toHaveProperty("change");
    expect(quote!).toHaveProperty("high");
    expect(quote!).toHaveProperty("low");
    expect(quote!).toHaveProperty("open");
    expect(quote!).toHaveProperty("volume");
    expect(quote!).toHaveProperty("previousClose");
    expect(quote!).toHaveProperty("marketTime");
    // change = 180.25 - 178.00 = 2.25
    expect(Math.abs(quote!.change - 2.25)).toBeLessThan(0.1);
  });

  test("returns null on network error", async () => {
    globalThis.fetch = (async (_url: string) => {
      throw new Error("Network error");
    }) as typeof globalThis.fetch;

    const quote = await fetchUSStockQuote("INVALID");
    expect(quote).toBeNull();
  });

  test("returns null when API returns error", async () => {
    globalThis.fetch = (async (_url: string) => ({
      json: async () => ({
        chart: { result: null, error: { code: "Not Found", description: "No data found" } },
      }),
      ok: true, status: 200,
      text: async () => "",
    } as any)) as typeof globalThis.fetch;

    const quote = await fetchUSStockQuote("NONEXISTENT");
    expect(quote).toBeNull();
  });
});

describe("fetchUSStockHistory", () => {
  test("returns array of NavHistoryRow with date/close/unit_nav", async () => {
    const now = Math.floor(Date.now() / 1000);
    const day = 86400;

    globalThis.fetch = (async (_url: string) => ({
      json: async () => makeYahooResponse({
        symbol: "NVDA",
        timestamps: [now - 2 * day, now - day, now],
        closes: [178.0, 179.5, 180.25],
        opens: [177.5, 179.0, 179.8],
        highs: [178.5, 180.0, 182.0],
        lows: [176.0, 178.5, 178.0],
        volumes: [40000000, 45000000, 50000000],
      }),
      ok: true, status: 200,
      text: async () => "",
    } as any)) as typeof globalThis.fetch;

    const history = await fetchUSStockHistory("NVDA", "5d");
    expect(Array.isArray(history)).toBe(true);
    expect(history.length).toBe(3);
    expect(history[0]).toHaveProperty("date");
    expect(history[0]).toHaveProperty("close");
    expect(history[0]).toHaveProperty("unit_nav");
    expect(history[0]).toHaveProperty("change_pct");
    expect(history[0]).toHaveProperty("open");
    expect(history[0]).toHaveProperty("high");
    expect(history[0]).toHaveProperty("low");
    expect(history[0]).toHaveProperty("volume");
    expect(history[0].close).toBe(178.0);
    expect(history[2].close).toBe(180.25);
  });

  test("returns empty array on error", async () => {
    globalThis.fetch = (async (_url: string) => {
      throw new Error("Network error");
    }) as typeof globalThis.fetch;

    const history = await fetchUSStockHistory("INVALID", "1y");
    expect(Array.isArray(history)).toBe(true);
    expect(history.length).toBe(0);
  });
});
