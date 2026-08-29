import type { FundInfo } from "@fund-dashboard/contracts";
import { describe, expect, it } from "vitest";
import { classify, isNasdaqFundName, nameMatchesKeyword } from "../classify";
import { sanitizeUserError } from "../userError";

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

describe("classify EN + ETF (#188)", () => {
  it("classifies English NASDAQ name as nasdaq", () => {
    expect(classify(fund({ name: "XYZ NASDAQ-100 Index Fund" }))).toBe("nasdaq");
  });

  it("classifies fund_type containing ETF", () => {
    expect(classify(fund({ name: "某指数增强", type: "ETF-股票" }))).toBe("other");
  });

  it("isNasdaqFundName matches CN and EN tokens", () => {
    expect(isNasdaqFundName("华夏纳指ETF")).toBe(true);
    expect(isNasdaqFundName("Foo Nasdaq Bar")).toBe(true);
    expect(isNasdaqFundName("普通混合基金")).toBe(false);
  });

  it("nameMatchesKeyword is case-insensitive for Latin", () => {
    expect(nameMatchesKeyword("nasdaq 100 etf", "NASDAQ")).toBe(true);
    expect(nameMatchesKeyword("纳斯达克100", "纳斯达克")).toBe(true);
  });
});

describe("sanitizeUserError (#188)", () => {
  it("passes short user-safe messages through", () => {
    expect(sanitizeUserError(new Error("持仓不存在"), "fallback")).toBe("持仓不存在");
  });

  it("replaces technical dumps with fallback", () => {
    expect(
      sanitizeUserError(new Error("Expected string, received number at path.foo"), "加载失败"),
    ).toBe("加载失败");
    expect(sanitizeUserError(new Error("Failed to fetch"), "加载失败")).toBe("加载失败");
    expect(sanitizeUserError(new Error("HTTP 500: Internal Server Error"), "加载失败")).toBe(
      "加载失败",
    );
  });
});
