// funds.ts — securities (fund/stock/etf/index), transactions, nav
// v3.0 contracts SSOT — derived from internal/service/portfolio (detail_types.go,
// securities.go, transactions_list.go) and internal/httpapi/fund_response.go.
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

export const TransactionSchema = z
  .object({
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
    settlement_days: z.number().nullable().optional(),
    order_id: z.string().nullable().optional(),
    anomaly: z.string().nullable(),
  })
  .passthrough();

export const FundDetailSchema = z
  .object({
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
  })
  .passthrough();
export type FundDetail = z.infer<typeof FundDetailSchema>;

export const NavPointSchema = z.object({
  date: z.string(),
  unit_nav: z.number(),
  // Go NavHistoryPoint.DailyChangePct has no omitempty: the field is always
  // emitted and is null when the row has no change (never absent).
  daily_change_pct: z.number().nullable(),
  // Go COALESCE(security_type,'fund') in GetNavHistory: always a string.
  security_type: z.string(),
});
export type NavPoint = z.infer<typeof NavPointSchema>;

/** Extended fund info that also covers individual stocks. */
export const SecurityInfoSchema = FundInfoSchema.extend({
  market: z.string(),
  security_type: z.string(),
});
export type SecurityInfo = z.infer<typeof SecurityInfoSchema>;
export const TransactionsListItemSchema = z.object({
  seq: z.number(),
  trade_time: z.string().nullable(),
  confirm_date: z.string().nullable(),
  direction: z.string().nullable(),
  trade_type: z.string().nullable(),
  fund_code: z.string(),
  fund_name: z.string().nullable(),
  amount: z.number().nullable(),
  shares: z.number().nullable(),
  fee: z.number().nullable(),
  order_id: z.string().nullable(),
  anomaly: z.string().nullable(),
  settlement_days: z.number().nullable(),
  portfolio_id: z.number().nullable(),
});
export type TransactionsListItem = z.infer<typeof TransactionsListItemSchema>;

export const TransactionsResponseSchema = z.object({
  transactions: z.array(TransactionsListItemSchema),
  total: z.number(),
});
export type TransactionsResponse = z.infer<typeof TransactionsResponseSchema>;
