// system.ts — /api/system/* workbench read surfaces
// v3.0 contracts SSOT — derived from internal/httpapi/system.go
import { z } from "zod";

export const SystemStatusSchema = z.object({
  version: z.string(),
  db_driver: z.string(),
  go_version: z.string(),
  uptime_sec: z.number(),
  db_size_bytes: z.number().optional(),
  freshness: z.object({ health: z.string() }),
});
export type SystemStatus = z.infer<typeof SystemStatusSchema>;

export const SystemJobSchema = z.object({
  name: z.string(),
  schedule: z.string(),
  last_run: z.number().optional(),
  last_error: z.string().optional(),
  next_run: z.number(),
});
export const SystemJobsResponseSchema = z.object({ jobs: z.array(SystemJobSchema) });

export const SystemAuditEntrySchema = z.object({
  kind: z.enum(["auth", "agent"]),
  ts: z.number(),
  event: z.string(),
  summary: z.string(),
  ip: z.string().optional(),
});
export const SystemAuditResponseSchema = z.object({ events: z.array(SystemAuditEntrySchema) });
