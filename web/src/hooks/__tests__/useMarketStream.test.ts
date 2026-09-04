import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { MARKET_INDICES_QUERY_KEY } from "../../lib/queries";
import { applyIndicesFrame } from "../useMarketStream";

// 与 contracts /api/market/indices 线型一致的合法帧样本（strict 7 键）。
const FRAME = [
  {
    code: "^GSPC",
    name: "标普500",
    market: "US",
    price: 5000.25,
    change_pct: 0.5,
    change_amt: 25.1,
    updated_at: "2026-09-03T01:00:00Z",
  },
];

describe("applyIndicesFrame (SSE 帧 → 查询缓存)", () => {
  it("writes valid frames into the market-indices cache", () => {
    const client = new QueryClient();
    const parsed = applyIndicesFrame(client, FRAME);
    expect(parsed).not.toBeNull();
    expect(client.getQueryData(MARKET_INDICES_QUERY_KEY)).toEqual(parsed);
  });

  it("skips dirty frames without touching the cache", () => {
    const client = new QueryClient();
    // 缺字段（price 非法）
    expect(applyIndicesFrame(client, [{ ...FRAME[0], price: "x" }])).toBeNull();
    // strict 契约拒绝未知键
    expect(applyIndicesFrame(client, [{ ...FRAME[0], extra: 1 }])).toBeNull();
    // 非数组帧
    expect(applyIndicesFrame(client, { code: "^GSPC" })).toBeNull();
    expect(client.getQueryData(MARKET_INDICES_QUERY_KEY)).toBeUndefined();
  });
});
