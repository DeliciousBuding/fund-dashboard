// stocks.ts — US stocks profile/history/quote, sector summary
// v3.0 contracts SSOT — migrated verbatim from web/src/api/types.ts
import { z } from "zod";

export const USStockProfileSchema = z.object({
  sector: z.string(),
  industry: z.string(),
  market_cap: z.number().nullable(),
  pe: z.number().nullable(),
  description: z.string(),
});

export const USStockHistoryPointSchema = z.object({
  date: z.string(),
  close: z.number(),
  change_pct: z.number(),
});

export const USStockInfoSchema = z.object({
  code: z.string(),
  name: z.string(),
  market: z.string(),
  price: z.number(),
  previous_close: z.number(),
  change: z.number(),
  change_pct: z.number(),
  high: z.number(),
  low: z.number(),
  open: z.number(),
  volume: z.number(),
  currency: z.string(),
  market_time: z.string(),
  profile: USStockProfileSchema.nullable(),
  history: z.array(USStockHistoryPointSchema),
  source: z.string(),
});
export type USStockInfo = z.infer<typeof USStockInfoSchema>;

export const USSectorSummarySchema = z.object({
  sector: z.string(),
  avg_change_pct: z.number(),
  stock_count: z.number(),
});
export type USSectorSummary = z.infer<typeof USSectorSummarySchema>;
