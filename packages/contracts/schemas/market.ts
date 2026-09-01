// market.ts — market indices, index live/history, exchange rate
// v3.0 contracts SSOT — derived from internal/httpapi/market.go wire shapes.
import { z } from "zod";

// /api/market/indices + /api/market/stream（event: indices）共用 7 键 map：
// 恒下发 7 键。Go 侧 MarketIndexQuote.Price/ChangePct/Change 是 float64
// （DB NULL 经 nullableFloat64Value 转 0），线型恒为 number，不会是 null。
export const MarketIndexSchema = z
  .object({
    code: z.string(),
    name: z.string(),
    market: z.string(),
    price: z.number(),
    change_pct: z.number(),
    change_amt: z.number(),
    updated_at: z.string(),
  })
  .strict();
export type MarketIndex = z.infer<typeof MarketIndexSchema>;

// /api/market/index/{code}：IndexLiveReport 直接 JSON 序列化（非 map 字面量）。
// code/name/market/price/change_pct/change_amt/updated_at 无 omitempty，恒下发；
// source/decision_boundary/side_effects/external_fetch/error/message 带 omitempty，
// 只在非空时下发。
export const IndexLiveSchema = z
  .object({
    code: z.string(),
    name: z.string(),
    market: z.string(),
    price: z.number(),
    change_pct: z.number(),
    change_amt: z.number(),
    updated_at: z.string(),
    source: z.string().optional(),
    decision_boundary: z.string().optional(),
    side_effects: z.string().optional(),
    external_fetch: z.string().optional(),
    error: z.string().optional(),
    message: z.string().optional(),
  })
  .strict();
export type IndexLive = z.infer<typeof IndexLiveSchema>;

// 历史点：IndexHistoryPointReport 三字段均无 omitempty，恒下发。
export const IndexHistoryPointSchema = z
  .object({
    date: z.string(),
    close: z.number(),
    change_pct: z.number(),
  })
  .strict();
export type IndexHistoryPoint = z.infer<typeof IndexHistoryPointSchema>;

// /api/market/index/{code}/history：handler 用 map 字面量恒下发 8 键——
// source/external_fetch/decision_boundary/side_effects 即使为空字符串也出现；
// report 上的 error/message 不进入该 map，因此不在此契约中。
export const IndexHistorySchema = z
  .object({
    symbol: z.string(),
    count: z.number(),
    range: z.string(),
    data: z.array(IndexHistoryPointSchema),
    source: z.string(),
    external_fetch: z.string(),
    decision_boundary: z.string(),
    side_effects: z.string(),
  })
  .strict();
export type IndexHistory = z.infer<typeof IndexHistorySchema>;

// Mirrors internal/service/portfolio/exchange_rate.go ExchangeRateReport:
// from/to/rate/updated_at are always emitted; source has omitempty (optional).
// strict() rejects unknown keys so contract drift surfaces instead of being
// silently stripped.
export const ExchangeRateSchema = z
  .object({
    from: z.string(),
    to: z.string(),
    rate: z.number(),
    updated_at: z.string(),
    source: z.string().optional(),
  })
  .strict();
export type ExchangeRate = z.infer<typeof ExchangeRateSchema>;
