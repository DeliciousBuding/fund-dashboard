import type { FundInfo, SecurityInfo } from "@fund-dashboard/contracts";
import { describe, expect, it } from "vitest";
import {
  CAT_ORDER,
  CATS,
  classify,
  classifySecurity,
  detectStockMarket,
  getCurrencySymbol,
  getMarketLabel,
  isUSStock,
  STOCK_CAT_ORDER,
  STOCK_CATS,
  STOCK_MARKETS,
} from "../classify";

// ── classify ────────────────────────────────────────────────────
describe("classify", () => {
  const fund = (overrides: Partial<FundInfo> = {}): FundInfo => ({
    code: "000001",
    name: "测试基金",
    type: "混合型",
    held_shares: 100,
    current_value: 100,
    unrealized_pnl: 0,
    pnl_pct: 0,
    latest_nav: 1.0,
    ...overrides,
  });

  it('classifies "纳斯达克" fund as "nasdaq"', () => {
    expect(classify(fund({ name: "广发纳斯达克100ETF" }))).toBe("nasdaq");
  });

  it('classifies "纳指" fund as "nasdaq"', () => {
    expect(classify(fund({ name: "华夏纳指ETF" }))).toBe("nasdaq");
  });

  it('classifies "红利" fund as "dividend"', () => {
    expect(classify(fund({ name: "中证红利ETF" }))).toBe("dividend");
  });

  it('classifies "红利" fund as "dividend"', () => {
    expect(classify(fund({ name: "港股通红利ETF" }))).toBe("dividend");
  });

  it('classifies type "QDII" fund as "qdii"', () => {
    expect(classify(fund({ name: "海外精选", type: "QDII" }))).toBe("qdii");
  });

  it('classifies type "债券型" fund as "bond"', () => {
    expect(classify(fund({ name: "纯债基金", type: "债券型" }))).toBe("bond");
  });

  it('classifies stock with market "sh" as "ashare"', () => {
    expect(
      classify(fund({ name: "贵州茅台", type: "股票型", security_type: "stock", market: "sh" })),
    ).toBe("ashare");
  });

  it('classifies stock with market "sz" as "ashare"', () => {
    expect(
      classify(fund({ name: "宁德时代", type: "股票型", security_type: "stock", market: "sz" })),
    ).toBe("ashare");
  });

  it('classifies stock with market "hk" as "hkstock"', () => {
    expect(
      classify(fund({ name: "腾讯控股", type: "股票型", security_type: "stock", market: "hk" })),
    ).toBe("hkstock");
  });

  it('classifies stock with market "us" as "ashare" (routed via stockGroups)', () => {
    expect(
      classify(fund({ name: "苹果", type: "股票型", security_type: "stock", market: "us" })),
    ).toBe("ashare");
  });

  it('classifies unknown fund as "other"', () => {
    expect(classify(fund({ name: "未知基金名称" }))).toBe("other");
  });

  it('classifies "债" keyword as "bond"', () => {
    expect(classify(fund({ name: "招商债券A" }))).toBe("bond");
  });

  it('classifies "货币" keyword as "money"', () => {
    expect(classify(fund({ name: "天弘货币基金" }))).toBe("money");
  });
});

// ── classifySecurity ────────────────────────────────────────────
describe("classifySecurity", () => {
  const sec = (overrides: Partial<SecurityInfo> = {}): SecurityInfo => ({
    code: "600519",
    name: "贵州茅台",
    type: "股票型",
    market: "sh",
    security_type: "stock",
    held_shares: 100,
    current_value: 100,
    unrealized_pnl: 0,
    pnl_pct: 0,
    latest_nav: 1.0,
    ...overrides,
  });

  it('classifies market "sh" as "stock-a"', () => {
    expect(classifySecurity(sec({ market: "sh" }))).toBe("stock-a");
  });

  it('classifies market "sz" as "stock-a"', () => {
    expect(classifySecurity(sec({ market: "sz" }))).toBe("stock-a");
  });

  it('classifies market "hk" as "stock-hk"', () => {
    expect(classifySecurity(sec({ market: "hk" }))).toBe("stock-hk");
  });

  it('classifies market "us" as "stock-us"', () => {
    expect(classifySecurity(sec({ market: "us" }))).toBe("stock-us");
  });

  it('falls back to "stock-a" for unknown market', () => {
    expect(classifySecurity(sec({ market: "jp" as any }))).toBe("stock-a");
  });
});

// ── detectStockMarket ───────────────────────────────────────────
describe("detectStockMarket", () => {
  it('detects "sh" for 6xxxxx codes', () => {
    expect(detectStockMarket("600519")).toBe("sh");
    expect(detectStockMarket("600000")).toBe("sh");
  });

  it('detects "sz" for 0xxxxx codes', () => {
    expect(detectStockMarket("000001")).toBe("sz");
  });

  it('detects "sz" for 3xxxxx codes', () => {
    expect(detectStockMarket("300750")).toBe("sz");
  });

  it('detects "hk" for 5-digit codes', () => {
    expect(detectStockMarket("00700")).toBe("hk");
    expect(detectStockMarket("09988")).toBe("hk");
  });

  it('detects "us" for alphabetic tickers', () => {
    expect(detectStockMarket("AAPL")).toBe("us");
    expect(detectStockMarket("NVDA")).toBe("us");
    expect(detectStockMarket("meta")).toBe("us");
  });

  it("returns undefined for empty string", () => {
    expect(detectStockMarket("")).toBeUndefined();
  });

  it("returns undefined for unrecognized patterns", () => {
    expect(detectStockMarket("1234")).toBeUndefined();
    expect(detectStockMarket("1234567")).toBeUndefined();
  });
});

// ── getMarketLabel ──────────────────────────────────────────────
describe("getMarketLabel", () => {
  it("returns market label for sh codes", () => {
    expect(getMarketLabel("600519")).toBe("沪A");
  });

  it("returns market label for sz codes", () => {
    expect(getMarketLabel("000001")).toBe("深A");
  });

  it("returns market label for hk codes", () => {
    expect(getMarketLabel("00700")).toBe("港股");
  });

  it("returns market label for us codes", () => {
    expect(getMarketLabel("AAPL")).toBe("美股");
  });

  it("returns empty string for unknown code", () => {
    expect(getMarketLabel("")).toBe("");
  });
});

// ── isUSStock ───────────────────────────────────────────────────
describe("isUSStock", () => {
  it("returns true for US tickers", () => {
    expect(isUSStock("AAPL")).toBe(true);
  });

  it("returns false for non-US codes", () => {
    expect(isUSStock("600519")).toBe(false);
    expect(isUSStock("00700")).toBe(false);
  });
});

// ── STOCK_MARKETS ───────────────────────────────────────────────
describe("STOCK_MARKETS", () => {
  it("has sh, sz, hk, us entries with labelKey", () => {
    expect(STOCK_MARKETS).toHaveProperty("sh");
    expect(STOCK_MARKETS).toHaveProperty("sz");
    expect(STOCK_MARKETS).toHaveProperty("hk");
    expect(STOCK_MARKETS).toHaveProperty("us");
    expect(STOCK_MARKETS.sh.label).toBe("沪A");
    expect(STOCK_MARKETS.sz.label).toBe("深A");
    expect(STOCK_MARKETS.hk.label).toBe("港股");
    expect(STOCK_MARKETS.us.label).toBe("美股");
  });
});

// ── CATS and CAT_ORDER ──────────────────────────────────────────
describe("CATS and CAT_ORDER", () => {
  it("CATS has expected categories with zh label and Chinese keyword funds", () => {
    expect(CATS).toHaveProperty("nasdaq");
    expect(CATS).toHaveProperty("dividend");
    expect(CATS).toHaveProperty("tech");
    expect(CATS).toHaveProperty("gold");
    expect(CATS).toHaveProperty("bond");
    expect(CATS).toHaveProperty("qdii");
    expect(CATS).toHaveProperty("money");
    expect(CATS).toHaveProperty("ashare");
    expect(CATS).toHaveProperty("hkstock");
    expect(CATS).toHaveProperty("other");
    expect(CATS.nasdaq.label).toBe("纳斯达克");
    expect(CATS.nasdaq.funds).toContain("纳斯达克");
    expect(CATS.tech.funds).toContain("科创");
    expect(CATS.dividend.funds).toContain("红利");
  });

  it("CAT_ORDER has expected length", () => {
    expect(CAT_ORDER.length).toBeGreaterThan(0);
    expect(CAT_ORDER).toContain("nasdaq");
  });
});

// ── STOCK_CATS ──────────────────────────────────────────────────
describe("STOCK_CATS", () => {
  it("has stock-a, stock-hk, stock-us with zh label", () => {
    expect(STOCK_CATS).toHaveProperty("stock-a");
    expect(STOCK_CATS).toHaveProperty("stock-hk");
    expect(STOCK_CATS).toHaveProperty("stock-us");
    expect(STOCK_CATS["stock-a"].label).toBe("A股");
    expect(STOCK_CATS["stock-hk"].label).toBe("港股通");
    expect(STOCK_CATS["stock-us"].label).toBe("美股");
  });
});

// ── STOCK_CAT_ORDER ─────────────────────────────────────────────
describe("STOCK_CAT_ORDER", () => {
  it("has expected order", () => {
    expect(STOCK_CAT_ORDER).toEqual(["stock-a", "stock-hk", "stock-us"]);
  });
});

// ── getCurrencySymbol ───────────────────────────────────────────
describe("getCurrencySymbol", () => {
  it('returns "$" for us market', () => {
    expect(getCurrencySymbol("us")).toBe("$");
  });

  it('returns "¥" for non-us market', () => {
    expect(getCurrencySymbol("sh")).toBe("¥");
    expect(getCurrencySymbol("hk")).toBe("¥");
  });

  it('returns "¥" for undefined market', () => {
    expect(getCurrencySymbol()).toBe("¥");
  });
});
