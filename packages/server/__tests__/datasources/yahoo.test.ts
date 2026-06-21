/** Yahoo DataSource unit tests — mock globalThis.fetch for Yahoo API calls */
import { describe, test, expect, beforeAll, afterAll } from "bun:test";

let originalFetch: typeof globalThis.fetch;

beforeAll(() => { originalFetch = globalThis.fetch; });
afterAll(() => { globalThis.fetch = originalFetch; });

import { yahooSource } from "../../datasources/yahoo";

// ═══════════════════════════════════════════════════════════════════════
// Helper: build a Yahoo v8 chart API response
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
          symbol: opts.symbol || "AAPL",
          exchangeName: "NMS",
          instrumentType: "EQUITY",
          regularMarketPrice: opts.price || 180.25,
          regularMarketTime: ts[ts.length - 1],
          regularMarketDayHigh: opts.highs?.[0] || 182.0,
          regularMarketDayLow: opts.lows?.[0] || 178.0,
          regularMarketVolume: opts.volumes?.[0] || 50000000,
          chartPreviousClose: opts.previousClose || 178.0,
          longName: opts.name || "Apple Inc.",
          shortName: "AAPL",
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

describe("yahooSource", () => {
  test("source has expected properties", () => {
    expect(yahooSource.name).toBe("yahoo");
    expect(yahooSource.markets).toContain("US");
  });

  // ── fetchQuote ────────────────────────────────────────────────

  describe("fetchQuote", () => {
    test("returns Quote with currency USD, market US", async () => {
      globalThis.fetch = (async (_url: string) => ({
        json: async () => makeYahooResponse({
          symbol: "AAPL", price: 180.25, previousClose: 178.0,
          name: "Apple Inc.", currency: "USD",
        }),
        ok: true, status: 200,
        text: async () => "",
      } as any)) as typeof globalThis.fetch;

      const quote = await yahooSource.fetchQuote("AAPL");
      expect(quote).not.toBeNull();
      expect(quote!.code).toBe("AAPL");
      expect(quote!.market).toBe("US");
      expect(quote!.currency).toBe("USD");
      expect(quote!.name).toBeTruthy();
      expect(typeof quote!.price).toBe("number");
      expect(quote!).toHaveProperty("changePct");
      expect(quote!).toHaveProperty("changeAmt");
      expect(quote!).toHaveProperty("updatedAt");
      expect(quote!).toHaveProperty("open");
      expect(quote!).toHaveProperty("high");
      expect(quote!).toHaveProperty("low");
      expect(quote!).toHaveProperty("volume");
      expect(quote!).toHaveProperty("previousClose");
    });

    test("returns null when fetch fails (HTTP error)", async () => {
      globalThis.fetch = (async (_url: string) => ({
        json: async () => ({}),
        ok: false, status: 404,
        text: async () => "Not Found",
      } as any)) as typeof globalThis.fetch;

      const quote = await yahooSource.fetchQuote("NOTFOUND");
      expect(quote).toBeNull();
    });
  });

  // ── fetchHistory ──────────────────────────────────────────────

  describe("fetchHistory", () => {
    test("returns array with date/price/changePct", async () => {
      const now = Math.floor(Date.now() / 1000);
      const day = 86400;

      globalThis.fetch = (async (_url: string) => ({
        json: async () => makeYahooResponse({
          symbol: "AAPL",
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

      const history = await yahooSource.fetchHistory("AAPL");
      expect(Array.isArray(history)).toBe(true);
      expect(history.length).toBe(3);
      expect(history[0]).toHaveProperty("date");
      expect(history[0]).toHaveProperty("price");
      expect(history[0]).toHaveProperty("changePct");
      expect(history[0].price).toBeGreaterThan(0);
      expect(history[0].date).toBeTruthy();
    });
  });
});
