// reports.ts — 组合报告生成 /api/reports (JSON v1)
// v3.0 contracts SSOT — derived from internal/service/portfolio/report.go
import { z } from "zod";

export const GenerateReportResultSchema = z.object({
  ok: z.boolean(),
  report_id: z.string(),
  title: z.string(),
  as_of: z.string(),
  generated_at: z.string(),
  format: z.string(),
  portfolio_id: z.number(),
  sections: z.record(z.string(), z.unknown()),
  decision_boundary: z.string(),
  side_effects: z.string(),
  artifact: z.string(),
});
export type GenerateReportResult = z.infer<typeof GenerateReportResultSchema>;
