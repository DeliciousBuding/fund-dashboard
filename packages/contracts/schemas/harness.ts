// harness.ts — Agent harness, source brief, source events
// v3.0 contracts SSOT. Fixes G2 (data_quality.holdings_coverage_pct), G3 (SourceEvent.created_at).
import { z } from "zod";
import { PortfolioAllocationSchema } from "./portfolio";

export const InvestmentHarnessHoldingSignalSchema = z.object({
  code: z.string(),
  name: z.string(),
  security_type: z.string(),
  market: z.string(),
  held_shares: z.number(),
  current_value: z.number(),
  weight_pct: z.number(),
  latest_nav: z.number(),
  cost_per_share: z.number().nullable(),
  change_pct: z.number().nullable(),
  deviation_pct: z.number().nullable(),
  signal_tags: z.array(z.string()),
  data_points: z.object({
    has_price: z.boolean(),
    has_cost_basis: z.boolean(),
    has_change_pct: z.boolean(),
  }),
});
export type InvestmentHarnessHoldingSignal = z.infer<typeof InvestmentHarnessHoldingSignalSchema>;

export const InvestmentHarnessSnapshotSchema = z.object({
  generated_at: z.string(),
  decision_boundary: z.literal('facts_only'),
  total_value: z.number(),
  holdings_count: z.number(),
  allocation: PortfolioAllocationSchema,
  holding_signals: z.array(InvestmentHarnessHoldingSignalSchema),
  data_quality: z.object({
    stale_price_count: z.number(),
    missing_cost_basis_count: z.number(),
    missing_change_pct_count: z.number(),
    holdings_coverage_pct: z.number(), // G2 fix
  }),
  available_agent_tools: z.array(z.string()),
  agent_brief: z.string(),
});
export type InvestmentHarnessSnapshot = z.infer<typeof InvestmentHarnessSnapshotSchema>;

export const InvestmentSourceQuerySchema = z.object({
  id: z.string(),
  scope: z.enum(['portfolio', 'holding', 'underlying']),
  entity_code: z.string().nullable(),
  entity_name: z.string(),
  query: z.string(),
  reason: z.string(),
  freshness: z.enum(['intraday', 'daily', 'weekly']),
});
export type InvestmentSourceQuery = z.infer<typeof InvestmentSourceQuerySchema>;

export const InvestmentSourceTargetSchema = z.object({
  kind: z.enum(['web_search', 'market_data', 'official_disclosure', 'local_mcp']),
  name: z.string(),
  url_template: z.string().nullable(),
  use_for: z.string(),
});
export type InvestmentSourceTarget = z.infer<typeof InvestmentSourceTargetSchema>;

export const InvestmentSourceBriefSchema = z.object({
  generated_at: z.string(),
  decision_boundary: z.literal('source_queries_only'),
  queries: z.array(InvestmentSourceQuerySchema),
  source_targets: z.array(InvestmentSourceTargetSchema),
  coverage: z.object({
    holdings_scanned: z.number(),
    underlying_scanned: z.number(),
    max_queries: z.number(),
  }),
  agent_brief: z.string(),
});
export type InvestmentSourceBrief = z.infer<typeof InvestmentSourceBriefSchema>;

// ═══════ Source Events (V4) ═══════

export const SourceEventSchema = z.object({
  id: z.number(),
  title: z.string(),
  url: z.string().nullable(),
  source: z.string(),
  snippet: z.string().nullable(),
  query: z.string().nullable(),
  related_security_code: z.string().nullable(),
  related_security_name: z.string().nullable(),
  is_read: z.boolean(),
  is_useful: z.boolean(),
  fetched_at: z.string(),
  created_at: z.string(), // G3 fix (query was already present)
}).passthrough();
export type SourceEvent = z.infer<typeof SourceEventSchema>;

/**
 * /api/portfolio/source-events response wrapper (fixes C-1/G6).
 * Backend MUST return this shape, not a bare SourceEvent[].
 */
export const SourceEventsResponseSchema = z.object({
  count: z.number(),
  decision_boundary: z.literal('facts_only'),
  events: z.array(SourceEventSchema),
});
export type SourceEventsResponse = z.infer<typeof SourceEventsResponseSchema>;
