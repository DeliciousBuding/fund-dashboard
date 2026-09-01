// portfolio.ts — portfolio summary, allocation, DCA, penetration
// v3.0 contracts SSOT. Fixes G1 (PortfolioSchema missing unique_stocks/by_security_type).
import { z } from "zod";

// ═══════ Portfolio definitions / timeline ═══════

// GET /api/portfolio/portfolios → bare array of PortfolioDefinition
// (internal/service/portfolio/definitions.go). description is COALESCE'd to ""
// in SQL and has no omitempty, so it is always emitted — required, never omitted.
export const PortfolioDefinitionSchema = z.object({
  id: z.number(),
  name: z.string(),
  description: z.string(),
});
export type PortfolioDefinition = z.infer<typeof PortfolioDefinitionSchema>;

// GET /api/portfolio/timeline → bare array of TimelineEntry
// (internal/service/portfolio/timeline.go). All five fields are plain values,
// no pointers/omitempty.
export const TimelinePointSchema = z.object({
  date: z.string(),
  total_value: z.number(),
  total_cost: z.number(),
  pnl: z.number(),
  pnl_pct: z.number(),
});
export type TimelinePoint = z.infer<typeof TimelinePointSchema>;

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
  // Go 端无集中度风险提示时 nil slice 序列化为 null；归一化为空数组保持消费方类型不变。
  risk_flags: z
    .array(z.string())
    .nullable()
    .transform((v) => v ?? []),
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

const HoldingContributorSchema = z.object({
  code: z.string(),
  name: z.string(),
  unrealized_pnl: z.number(),
  pnl_pct: z.number(),
  current_value: z.number(),
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
  invested_cost: z.number().optional().default(0),
  current_value: z.number().optional().default(0),
  pnl_pct: z.number().optional().default(0),
  top_gainer: HoldingContributorSchema.nullable().optional(),
  top_loser: HoldingContributorSchema.nullable().optional(),
  stale_nav_days: z.number().nullable().optional(),
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
export type HoldingContributor = z.infer<typeof HoldingContributorSchema>;

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
