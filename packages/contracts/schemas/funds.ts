// funds.ts — securities (fund/stock/etf/index), transactions, nav
// v3.0 contracts SSOT — migrated verbatim from web/src/api/types.ts (2026-06-21)
import { z } from "zod";

// ═══════ Fund / Security types ═══════

export const FundInfoSchema = z.object({
  code: z.string(),
  name: z.string(),
  type: z.string(),
  security_type: z.string().optional(),
  market: z.string().optional(),
  held_shares: z.number(),
  current_value: z.number().nullable(),
  unrealized_pnl: z.number().nullable(),
  pnl_pct: z.number().nullable(),
  latest_nav: z.number().nullable(),
});
export type FundInfo = z.infer<typeof FundInfoSchema>;

export const TransactionSchema = z.object({
  seq: z.number().nullable(),
  trade_time: z.string(),
  confirm_date: z.string().nullable().optional(),
  trade_type: z.string(),
  direction: z.string(),
  amount: z.number(),
  shares: z.number(),
  fee: z.number(),
  nav: z.number().nullable(),
  inferred_nav: z.number().nullable(),
  nav_verified: z.union([z.boolean(), z.number()]).nullable().optional(),
  trade_day_type: z.string().nullable().optional(),
  settlement_days: z.number().nullable().optional(),
  effective_nav_date: z.string().nullable().optional(),
  order_id: z.string().nullable().optional(),
  anomaly: z.string().nullable(),
}).passthrough();
export type Transaction = z.infer<typeof TransactionSchema>;

export const FundDetailSchema = z.object({
  code: z.string(),
  name: z.string(),
  security_type: z.string().optional(),
  market: z.string().optional(),
  held_shares: z.number(),
  total_cost: z.number(),
  latest_nav: z.number().nullable(),
  current_value: z.number().nullable(),
  unrealized_pnl: z.number().nullable(),
  pnl_pct: z.number().nullable(),
  auto_buy_count: z.number(),
  manual_buy_count: z.number(),
  auto_buy_amount: z.number(),
  manual_buy_amount: z.number(),
  auto_tx: z.number(),
  manual_tx: z.number(),
  buy_count: z.number(),
  sell_count: z.number(),
  median_settlement: z.number(),
  transactions: z.array(TransactionSchema),
}).passthrough();
export type FundDetail = z.infer<typeof FundDetailSchema>;

export const NavPointSchema = z.object({
  date: z.string(),
  unit_nav: z.number(),
  daily_change_pct: z.number().optional(),
});
export type NavPoint = z.infer<typeof NavPointSchema>;

/** Extended fund info that also covers individual stocks. */
export const SecurityInfoSchema = FundInfoSchema.extend({
  market: z.string(),
  security_type: z.enum(['fund', 'stock']),
});
export type SecurityInfo = z.infer<typeof SecurityInfoSchema>;
