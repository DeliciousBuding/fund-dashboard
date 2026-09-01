// system.ts — /api/system/* workbench read surfaces
// v3.0 contracts SSOT — derived from internal/httpapi/system.go and
// internal/agenttools/registry_authz.go (RegistrySummary).
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

// ── GET /api/system/agent ────────────────────────────────────────────

// tools 是 Go 侧 RegistrySummary 的完整线型：除 by_scope/by_permission 外，
// 后端还下发 by_risk_level、disabled_boundaries、审计键等治理字段，契约
// 按真实响应建模而非前端当前消费子集。
export const AgentToolSummarySchema = z.object({
  schema_version: z.string(),
  generated_at: z.string(),
  total_tools: z.number(),
  disabled_tools: z.number(),
  review_required_tools: z.number(),
  confirmation_required_tools: z.number(),
  audited_tools: z.number(),
  by_scope: z.record(z.string(), z.number()),
  by_permission: z.record(z.string(), z.number()),
  by_risk_level: z.record(z.string(), z.number()),
  by_permission_source: z.record(z.string(), z.number()),
  // Go nil slice → JSON null：该字段按工具是否命中禁用/边界规则条件 append，
  // 契约归一化为空数组，避免未来零禁用工具时解析崩溃。
  disabled_boundaries: z
    .array(z.string())
    .nullable()
    .transform((v) => v ?? []),
  review_required_by_category: z.record(z.string(), z.number()),
  confirmation_required_by_category: z.record(z.string(), z.number()),
  audit_redaction_keys: z.array(z.string()),
});
export type AgentToolSummary = z.infer<typeof AgentToolSummarySchema>;

export const SystemAgentResponseSchema = z.object({
  endpoint: z.string(),
  request_method: z.string(),
  tools: AgentToolSummarySchema,
  key_env_vars: z.array(z.string()),
  // 密钥只下发掩码（"已配置"/"未配置"），永不携带密钥材料。
  keys: z.object({
    mcp_api_key: z.string(),
    public_mcp_key: z.string(),
  }),
});
export type SystemAgentResponse = z.infer<typeof SystemAgentResponseSchema>;
