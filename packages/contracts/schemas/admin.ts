// admin.ts — freshness diagnostics contracts (W6 admin surface)
// v3.0 contracts SSOT — derived from internal/service/admin/freshness.go
import { z } from "zod";

export const FreshnessItemSchema = z.object({
  code: z.string(),
  name: z.string(),
  type: z.string(),
});
export type FreshnessItem = z.infer<typeof FreshnessItemSchema>;

export const StaleSecuritySchema = z.object({
  code: z.string(),
  name: z.string(),
  last_nav: z.string(),
  stale_days: z.number(),
});
export type StaleSecurity = z.infer<typeof StaleSecuritySchema>;

export const FreshnessReportSchema = z.object({
  last_transaction: z.string().nullable(),
  last_nav_date: z.string().nullable(),
  anomaly_count: z.number(),
  missing_nav_securities: z.array(FreshnessItemSchema).nullable(),
  watchlist_missing_nav_securities: z.array(FreshnessItemSchema).nullable(),
  stale_securities: z.array(StaleSecuritySchema).nullable(),
  actionable: z.string(),
  health: z.string(),
  decision_boundary: z.string(),
});
export type FreshnessReport = z.infer<typeof FreshnessReportSchema>;
