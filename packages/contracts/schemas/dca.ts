// dca.ts — DCA plan entity + smart amount compute output
// v3.0 contracts SSOT — derived from internal/service/portfolio/dca.go + dca_compute.go
import { z } from "zod";

export const DcaPlanSchema = z.object({
  id: z.number(),
  fund_code: z.string(),
  fund_name: z.string().nullable(),
  amount: z.number(),
  frequency: z.string(),
  weekday_mask: z.string(),
  trade_type: z.string(),
  portfolio_id: z.number(),
  start_date: z.string(),
  end_date: z.string().nullable(),
  active: z.number(),
  source: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
});
export type DcaPlan = z.infer<typeof DcaPlanSchema>;

// 消费方 holdings.$code 直接访问 base_amount/dca_rate/actual_amount/signal/explanation，
// 故保留为必填（错误分支由 error 字段 + UI guard 短路）。
export const DcaComputeResultSchema = z.object({
  fund_code: z.string(),
  security_type: z.string().optional(),
  market: z.string().optional(),
  mode: z.string().optional(),
  base_amount: z.number(),
  latest_nav: z.number().optional(),
  cost_per_share: z.number().nullable().optional(),
  change_pct: z.number().nullable().optional(),
  deviation_pct: z.number().nullable().optional(),
  dca_rate: z.number(),
  actual_amount: z.number(),
  signal: z.string(),
  range: z.string().optional(),
  explanation: z.string(),
  decision_boundary: z.string().optional(),
  side_effects: z.string().optional(),
  error: z.string().optional(),
  message: z.string().optional(),
});
export type DcaComputeResult = z.infer<typeof DcaComputeResultSchema>;
