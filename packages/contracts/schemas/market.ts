// market.ts — market indices, index history, exchange rate
// v3.0 contracts SSOT — migrated verbatim from web/src/api/types.ts
import { z } from "zod";

export const MarketIndexSchema = z.object({
  code: z.string(),
  name: z.string(),
  market: z.string(),
  price: z.number().nullable(),
  change_pct: z.number().nullable(),
  change_amt: z.number().nullable(),
  updated_at: z.string(),
});
export type MarketIndex = z.infer<typeof MarketIndexSchema>;

export const IndexHistoryPointSchema = z.object({
  date: z.string(),
  close: z.number(),
  change_pct: z.number(),
});
export type IndexHistoryPoint = z.infer<typeof IndexHistoryPointSchema>;

export const IndexHistorySchema = z.object({
  symbol: z.string(),
  count: z.number(),
  range: z.string(),
  data: z.array(IndexHistoryPointSchema),
});
export type IndexHistory = z.infer<typeof IndexHistorySchema>;

export const ExchangeRateSchema = z.object({
  from: z.string(),
  to: z.string(),
  rate: z.number(),
  updated_at: z.string(),
});
export type ExchangeRate = z.infer<typeof ExchangeRateSchema>;
