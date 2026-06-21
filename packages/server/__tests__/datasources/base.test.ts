/** DataSource base unit tests — pure functions, no mocking needed */
import { describe, test, expect, beforeEach } from "bun:test";
import { registerSource, getSource, detectMarket } from "../../datasources/base";
import type { DataSource } from "../../datasources/base";

// Create a minimal test source
function makeTestSource(name: string, markets: string[]): DataSource {
  return {
    name,
    markets,
    async fetchQuote(_code: string, _market?: string) { return null; },
    async fetchHistory(_code: string, _market?: string, _days?: number) { return []; },
  };
}

describe("registerSource / getSource", () => {
  // Note: registry is module-scoped, so tests in the same file share state.
  // We rely on unique market names to avoid cross-test interference.

  test("register and retrieve a source", () => {
    const src = makeTestSource("test-src-X", ["XX"]);
    registerSource(src);
    const found = getSource("XX");
    expect(found).toBeDefined();
    expect(found!.name).toBe("test-src-X");
  });

  test("register multiple markets on one source", () => {
    const src = makeTestSource("multi-market", ["YY", "ZZ"]);
    registerSource(src);
    const foundY = getSource("YY");
    const foundZ = getSource("ZZ");
    expect(foundY).toBeDefined();
    expect(foundZ).toBeDefined();
    expect(foundY!.name).toBe("multi-market");
    expect(foundZ!.name).toBe("multi-market");
  });

  test("getSource for unknown market returns undefined", () => {
    const result = getSource("UNKNOWN_MARKET_XYZ");
    expect(result).toBeUndefined();
  });

  test("getSource with empty string returns undefined", () => {
    const result = getSource("");
    expect(result).toBeUndefined();
  });
});

describe("detectMarket", () => {
  test("600519 -> SH (Shanghai)", () => {
    expect(detectMarket("600519")).toBe("SH");
  });

  test("000001 -> SZ (Shenzhen)", () => {
    expect(detectMarket("000001")).toBe("SZ");
  });

  test("300750 -> SZ (ChiNext)", () => {
    expect(detectMarket("300750")).toBe("SZ");
  });

  test("00700 -> HK (Hong Kong, 5-digit)", () => {
    expect(detectMarket("00700")).toBe("HK");
  });

  test("AAPL -> US (US stock)", () => {
    expect(detectMarket("AAPL")).toBe("US");
  });

  test("NVDA -> US", () => {
    expect(detectMarket("NVDA")).toBe("US");
  });

  test("6-digit fund code starting 0 (019173) matches SZ pattern", () => {
    // detectMarket checks /^[03]\d{5}$/ for SZ before checking the 6-digit CN fallback
    // 019173 starts with 0, so it matches SZ
    expect(detectMarket("019173")).toBe("SZ");
  });

  test("688111 -> SH (STAR market)", () => {
    expect(detectMarket("688111")).toBe("SH");
  });

  test("random 6-digit -> CN (defaulted to fund)", () => {
    expect(detectMarket("123456")).toBe("CN");
  });
});
