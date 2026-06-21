// portfolio.ts — portfolio summary, allocation, DCA, penetration
// v3.0 contracts SSOT. Fixes G1 (PortfolioSchema missing unique_stocks/by_security_type).
import { z } from "zod";

// ═══════ Allocation ═══════

export const AllocationBucketSchema = z.object({
  key: z.string(),
  label: z.string(),
  value: z.number(),
  weight_pct: z.number(),
  count: z.number(),
});
export type AllocationBucket = z.infer<typeof AllocationBucketSchema>;

export const PortfolioAllocationSchema = z.object({
  total_value: z.number(),
  by_security_type: z.array(AllocationBucketSchema),
  by_market: z.array(AllocationBucketSchema),
  by_fund_type: z.array(AllocationBucketSchema),
  risk_flags: z.array(z.string()),
  agent_brief: z.string(),
});
export type PortfolioAllocation = z.infer<typeof PortfolioAllocationSchema>;

// ═══════ Portfolio summary (G1 fix: + unique_stocks, + by_security_type) ═══════
// NOTE: the portfolio *summary*'s by_security_type is
// { security_type, count, total_value, total_pnl }[] — distinct from
// AllocationBucket which is the /portfolio/allocation endpoint shape.

const BySecurityTypeSummarySchema = z.object({
  security_type: z.string(),
  count: z.number(),
  total_value: z.number(),
  total_pnl: z.number(),
});

export const PortfolioSchema = z.object({
  total_tx: z.number(),
  unique_funds: z.number(),
  unique_stocks: z.number(),
  held_funds: z.number(),
  total_buy: z.number(),
  total_sell: z.number(),
  total_fee: z.number(),
  unrealized_pnl: z.number(),
  auto_tx: z.number(),
  manual_tx: z.number(),
  auto_amount: z.number(),
  manual_amount: z.number(),
  first_trade: z.string(),
  last_trade: z.string(),
  last_nav_date: z.string().nullable(),
  settlement_distribution: z.record(z.string(), z.number()),
  trade_type_breakdown: z.record(z.string(), z.number()),
  by_security_type: z.array(BySecurityTypeSummarySchema),
});
export type Portfolio = z.infer<typeof PortfolioSchema>;

// ═══════ DCA plan ═══════

export const DcaPlanSchema = z.object({
  fund_code: z.string().optional(),
  mode: z.enum(['nav_deviation', 'change_pct']),
  base_amount: z.number(),
  latest_nav: z.number(),
  cost_per_share: z.number().nullable(),
  change_pct: z.number().nullable(),
  deviation_pct: z.number().nullable(),
  dca_rate: z.number(),
  actual_amount: z.number(),
  signal: z.string(),
  range: z.string().optional(),
  explanation: z.string(),
});
export type DcaPlan = z.infer<typeof DcaPlanSchema>;

// ═══════ Portfolio Penetration ═══════

export const PenetrationFundSchema = z.object({
  fund_code: z.string(),
  fund_name: z.string(),
  weight_pct: z.number(),
  fund_value_cny: z.number(),
});
export type PenetrationFund = z.infer<typeof PenetrationFundSchema>;

export const PenetrationStockSchema = z.object({
  stock_code: z.string(),
  stock_name: z.string(),
  total_exposure_cny: z.number(),
  weight_pct: z.number(),
  held_by_funds: z.array(PenetrationFundSchema),
});
export type PenetrationStock = z.infer<typeof PenetrationStockSchema>;

export const PenetrationResultSchema = z.object({
  penetration: z.array(PenetrationStockSchema),
  total_portfolio_value: z.number(),
  equity_fund_count: z.number(),
  unique_stocks: z.number(),
});
export type PenetrationResult = z.infer<typeof PenetrationResultSchema>;
