// stocks.ts — US stocks profile/history/quote, sector summary
// v3.0 contracts SSOT — derived from internal/httpapi/market.go
// (usStockSPAResponse flat SPA wire shape, #98).
import { z } from "zod";

// Mirrors the profile map built in usStockSPAResponse: all five keys are
// always emitted; market_cap/pe are Go *float64 pointers -> null when absent.
export const USStockProfileSchema = z
  .object({
    sector: z.string(),
    industry: z.string(),
    market_cap: z.number().nullable(),
    pe: z.number().nullable(),
    description: z.string(),
  })
  .strict();

// Mirrors the history point map built in usStockSPAResponse: date/close/
// change_pct are always emitted.
export const USStockHistoryPointSchema = z
  .object({
    date: z.string(),
    close: z.number(),
    change_pct: z.number(),
  })
  .strict();

// Mirrors usStockSPAResponse: the 19 always-emitted keys use zero/""/null/[]
// defaults before conditional overwrites; error/message appear only when
// report.Error != "". strict() rejects unknown keys.
export const USStockInfoSchema = z
  .object({
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
    decision_boundary: z.string(),
    side_effects: z.string(),
    external_fetch: z.string(),
    error: z.string().optional(),
    message: z.string().optional(),
  })
  .strict();
export type USStockInfo = z.infer<typeof USStockInfoSchema>;
