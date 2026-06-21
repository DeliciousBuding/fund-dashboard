// analysis.ts — XIRR, drawdown, fund comparison
// v3.0 contracts SSOT — migrated verbatim from web/src/api/types.ts
import { z } from "zod";

export const XirrResultSchema = z.object({
  xirr: z.number().nullable(),
  message: z.string().optional(),
  code: z.string().optional(),
});
export type XirrResult = z.infer<typeof XirrResultSchema>;

export const DrawdownResultSchema = z.object({
  max_drawdown: z.number(),
  peak_date: z.string(),
  trough_date: z.string(),
  code: z.string().optional(),
});
export type DrawdownResult = z.infer<typeof DrawdownResultSchema>;

export const CompareFundSchema = z.object({
  code: z.string(),
  name: z.string(),
  market: z.string(),
  xirr: z.number().nullable(),
  volatility: z.number().nullable(),
  sharpe: z.number().nullable(),
  max_drawdown: z.number().nullable(),
  calmar: z.number().nullable(),
});
export type CompareFund = z.infer<typeof CompareFundSchema>;

export const CompareResultSchema = z.object({
  funds: z.array(CompareFundSchema),
});
export type CompareResult = z.infer<typeof CompareResultSchema>;
