// alerts.ts — W6 告警诊断面 /api/alerts
// v3.0 contracts SSOT — derived from internal/service/admin/alerts.go
import { z } from "zod";

export const AlertItemSchema = z.object({
  kind: z.string(),
  code: z.string(),
  name: z.string().optional(),
  severity: z.string(),
  message: z.string(),
  value: z.number().nullable().optional(),
  threshold: z.number().nullable().optional(),
  as_of: z.string().optional(),
  security_type: z.string().optional(),
  market: z.string().optional(),
});
export type AlertItem = z.infer<typeof AlertItemSchema>;

export const CheckAlertsResponseSchema = z.object({
  ok: z.boolean(),
  count: z.number(),
  alerts: z.array(AlertItemSchema),
  checked_at: z.string(),
  price_change_pct: z.number(),
  drawdown_pct: z.number(),
  stale_days: z.number(),
  portfolio_id: z.number(),
  decision_boundary: z.string(),
  side_effects: z.string(),
  webhook_sent: z.boolean(),
});
export type CheckAlertsResponse = z.infer<typeof CheckAlertsResponseSchema>;
