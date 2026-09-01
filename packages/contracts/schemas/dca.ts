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

// Go 端 dca_compute.go 对 base_amount/dca_rate/actual_amount/signal/explanation 使用
// omitempty：no_position / insufficient_data 等错误分支不下发这些字段。
// 契约与线上行为对齐：全部可选；成功分支始终下发，错误分支由 error 字段短路。
export const DcaComputeResultSchema = z.object({
  fund_code: z.string(),
  security_type: z.string().optional(),
  market: z.string().optional(),
  mode: z.string().optional(),
  base_amount: z.number().optional(),
  latest_nav: z.number().optional(),
  cost_per_share: z.number().nullable().optional(),
  change_pct: z.number().nullable().optional(),
  deviation_pct: z.number().nullable().optional(),
  dca_rate: z.number().optional(),
  actual_amount: z.number().optional(),
  signal: z.string().optional(),
  range: z.string().optional(),
  explanation: z.string().optional(),
  decision_boundary: z.string().optional(),
  side_effects: z.string().optional(),
  error: z.string().optional(),
  message: z.string().optional(),
});
export type DcaComputeResult = z.infer<typeof DcaComputeResultSchema>;
// Go 端没有任何定投计划时 nil slice 序列化为 {"plans":null}；归一化为空数组。
export const DcaPlansResponseSchema = z.object({
  plans: z
    .array(DcaPlanSchema)
    .nullable()
    .transform((v) => v ?? []),
});
export type DcaPlansResponse = z.infer<typeof DcaPlansResponseSchema>;

export const DcaExecutionItemSchema = z.object({
  plan_id: z.number(),
  fund_code: z.string(),
  fund_name: z.string().optional(),
  amount: z.number(),
  shares: z.number().nullable().optional(),
  nav: z.number().nullable().optional(),
  order_id: z.string(),
  status: z.string(),
  message: z.string().optional(),
  trade_type: z.string().optional(),
  portfolio_id: z.number(),
  weekday_mask: z.string().optional(),
});
export type DcaExecutionItem = z.infer<typeof DcaExecutionItemSchema>;

export const DcaRunResultSchema = z.object({
  ok: z.boolean(),
  as_of: z.string(),
  dry_run: z.boolean(),
  executed: z.number(),
  skipped: z.number(),
  previewed: z.number(),
  items: z.array(DcaExecutionItemSchema),
  decision_boundary: z.string(),
  side_effects: z.string(),
});
export type DcaRunResult = z.infer<typeof DcaRunResultSchema>;
