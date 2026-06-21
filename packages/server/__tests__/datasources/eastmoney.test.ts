/** Eastmoney DataSource unit tests — mock globalThis.fetch for crawler calls */
import { describe, test, expect, beforeAll, afterAll, beforeEach } from "bun:test";

let originalFetch: typeof globalThis.fetch;

beforeAll(() => { originalFetch = globalThis.fetch; });
afterAll(() => { globalThis.fetch = originalFetch; });

import { eastmoneySource } from "../../datasources/eastmoney";
import { clearCache } from "../../utils/api-cache";

beforeEach(() => {
  clearCache();
});

// ═══════════════════════════════════════════════════════════════════════
// Smart mock: dispatches based on URL to simulate different eastmoney APIs
// ═══════════════════════════════════════════════════════════════════════

function mockEastmoneyFetch(url: string) {
  const urlStr = typeof url === "string" ? url : String(url);

  // Real-time NAV endpoint (fundgz)
  if (urlStr.includes("fundgz.1234567.com.cn")) {
    if (urlStr.includes("019173")) {
      return {
        text: async () => `jsonpgz({"fundcode":"019173","name":"test","jzrq":"2025-06-01","dwjz":"1.3500","gsz":"1.3550","gszzl":"0.75","gztime":"2025-06-01 15:00"})`,
        json: async () => ({}), ok: true, status: 200,
      };
    }
    // For stock codes (6 digits), return invalid so fetchRealtimeNav returns null
    // and the datasource falls through to the stock path
    return {
      text: async () => `not a fund`,
      json: async () => ({}), ok: true, status: 200,
    };
  }

  // NAV history (pingzhongdata)
  if (urlStr.includes("pingzhongdata")) {
    const testData = [
      { x: 1717200000000, y: 1.35, equityReturn: 0.75 },
      { x: 1717113600000, y: 1.34, equityReturn: 0.5 },
      { x: 1717027200000, y: 1.33, equityReturn: -0.3 },
    ];
    return {
      text: async () => `var Data_netWorthTrend = ${JSON.stringify(testData)};`,
      json: async () => ({}), ok: true, status: 200,
    };
  }

  // Stock realtime (push2)
  if (urlStr.includes("push2.eastmoney.com") && !urlStr.includes("kline")) {
    return {
      json: async () => ({
        data: {
          f43: 5000, f44: 5050, f45: 4900, f46: 4950,
          f57: "600519", f58: "贵州茅台",
          f169: 60, f170: 1.2, f47: 100000, f48: 5000000,
          f168: 2.5, f115: 20, f20: 100000000, f21: 80000000,
        },
      }),
      ok: true, status: 200,
      text: async () => "",
    };
  }

  // Stock K-line (push2his)
  if (urlStr.includes("push2his")) {
    return {
      json: async () => ({
        data: {
          code: "600519", market: "SH", name: "贵州茅台",
          klines: ["2025-06-01,49.5,50.0,50.5,49.0,100000,5000000,3,1.2,0.6,2.5"],
        },
      }),
      ok: true, status: 200,
      text: async () => "",
    };
  }

  // Default
  return {
    text: async () => "", json: async () => ({}), ok: true, status: 200,
  };
}

// ═══════════════════════════════════════════════════════════════════════

describe("eastmoneySource", () => {
  test("source has expected properties", () => {
    expect(eastmoneySource.name).toBe("eastmoney");
    expect(eastmoneySource.markets).toContain("CN");
    expect(eastmoneySource.markets).toContain("SH");
    expect(eastmoneySource.markets).toContain("SZ");
    expect(eastmoneySource.markets).toContain("HK");
  });

  // ── fetchQuote (fund) ─────────────────────────────────────────

  describe("fetchQuote for fund", () => {
    test("returns Quote with price, name, changePct", async () => {
      globalThis.fetch = (async (url: string) => mockEastmoneyFetch(url)) as typeof globalThis.fetch;

      const quote = await eastmoneySource.fetchQuote("019173", "CN");
      expect(quote).not.toBeNull();
      expect(quote!.code).toBe("019173");
      expect(quote!.market).toBe("CN");
      expect(quote!.price).toBe(1.35);
      expect(quote!.changePct).toBe(0.75);
      expect(quote!.currency).toBe("CNY");
    });

    test("returns null when crawler returns null", async () => {
      globalThis.fetch = (async (url: string) => ({
        text: async () => `invalid`,
        json: async () => ({}),
        ok: true, status: 200,
      } as any)) as typeof globalThis.fetch;

      const quote = await eastmoneySource.fetchQuote("000000", "CN");
      expect(quote).toBeNull();
    });
  });

  // ── fetchQuote (stock) ────────────────────────────────────────

  describe("fetchQuote for stock", () => {
    test("6-digit codes go through fund path (code.length===6 check)", async () => {
      // 600519 has 6 digits → datasource routes all 6-digit codes to fund path
      // This is a known limitation: A-stock codes (6 digits) can't be distinguished
      // from fund codes (6 digits) by length alone.
      globalThis.fetch = (async (url: string) => mockEastmoneyFetch(url)) as typeof globalThis.fetch;

      // fund path: fetchRealtimeNav returns null for our mock → returns null
      const quote = await eastmoneySource.fetchQuote("600519", "SH");
      expect(quote).toBeNull();
    });

    test("returns null when stock not found", async () => {
      globalThis.fetch = (async (_url: string) => ({
        json: async () => ({ data: null }),
        ok: true, status: 200,
        text: async () => "",
      } as any)) as typeof globalThis.fetch;

      const quote = await eastmoneySource.fetchQuote("999999");
      expect(quote).toBeNull();
    });
  });

  // ── fetchHistory ──────────────────────────────────────────────

  describe("fetchHistory", () => {
    test("returns array of HistoryPoint for fund", async () => {
      globalThis.fetch = (async (url: string) => mockEastmoneyFetch(url)) as typeof globalThis.fetch;

      const history = await eastmoneySource.fetchHistory("019173", "CN");
      expect(Array.isArray(history)).toBe(true);
      expect(history.length).toBe(3);
      expect(history[0]).toHaveProperty("date");
      expect(history[0]).toHaveProperty("price");
      expect(history[0]).toHaveProperty("changePct");
      expect(history[0].price).toBe(1.35);
    });

    test("returns empty array for no-data fund", async () => {
      globalThis.fetch = (async (_url: string) => ({
        text: async () => `var something = 1;`,
        json: async () => ({}),
        ok: true, status: 200,
      } as any)) as typeof globalThis.fetch;

      const history = await eastmoneySource.fetchHistory("000000", "CN");
      expect(Array.isArray(history)).toBe(true);
      expect(history.length).toBe(0);
    });

    test("6-digit stock codes hit fund history path (length===6)", async () => {
      // 600519 has 6 digits → fund history path returns Data_netWorthTrend data
      globalThis.fetch = (async (url: string) => mockEastmoneyFetch(url)) as typeof globalThis.fetch;

      const history = await eastmoneySource.fetchHistory("600519", "SH");
      // The datasource checks code.length===6 first → fund path
      // Our mock returns 3 fund history rows
      expect(Array.isArray(history)).toBe(true);
      expect(history.length).toBe(3);
      expect(history[0].price).toBe(1.35);
    });
  });
});
