// system.test.ts — /api/system/agent wire-shape tests (node:test).
// 对照 internal/httpapi/system.go (handleSystemAgent) 与
// internal/agenttools/registry_authz.go (RegistrySummary) 的完整线型。
import assert from "node:assert/strict";
import { test } from "node:test";

import { AgentToolSummarySchema, SystemAgentResponseSchema } from "./system.ts";

const summaryWire = {
  schema_version: "tool-registry-v1",
  generated_at: "07/06/2026 22:55:00",
  total_tools: 47,
  disabled_tools: 3,
  review_required_tools: 26,
  confirmation_required_tools: 14,
  audited_tools: 19,
  by_scope: { read: 30, write: 12, maintenance: 2, external_context: 0, disabled: 3 },
  by_permission: { allowed: 10, requires_confirmation: 14, disabled: 3 },
  by_risk_level: { low: 20, medium: 15, high: 12 },
  by_permission_source: { explicit: 30, inferred: 14, runtime: 3 },
  disabled_boundaries: ["broker_trade_execution", "cash_transfer", "backup_producer"],
  review_required_by_category: { portfolio: 5, transaction: 3 },
  confirmation_required_by_category: { maintenance: 2 },
  audit_redaction_keys: [
    "api_key",
    "token",
    "cookie",
    "authorization",
    "webhook",
    "password",
    "secret",
  ],
};

test("SystemAgentResponseSchema parses the real handler payload", () => {
  const parsed = SystemAgentResponseSchema.parse({
    endpoint: "/mcp",
    request_method: "POST",
    tools: summaryWire,
    key_env_vars: ["MCP_API_KEY", "PUBLIC_MCP_KEY"],
    keys: { mcp_api_key: "已配置", public_mcp_key: "未配置" },
  });
  assert.equal(parsed.endpoint, "/mcp");
  assert.equal(parsed.tools.total_tools, 47);
  assert.deepEqual(parsed.tools.by_scope.read, 30);
  assert.deepEqual(parsed.keys.mcp_api_key, "已配置");
});

test("AgentToolSummarySchema normalizes Go nil disabled_boundaries (null) to []", () => {
  // RegistrySummary.DisabledBoundaries 是条件 append 的 nil slice，零命中时
  // 序列化为 JSON null；契约归一化为空数组。
  const parsed = AgentToolSummarySchema.parse({ ...summaryWire, disabled_boundaries: null });
  assert.deepEqual(parsed.disabled_boundaries, []);
});

test("AgentToolSummarySchema passes disabled_boundaries arrays through", () => {
  const parsed = AgentToolSummarySchema.parse(summaryWire);
  assert.equal(parsed.disabled_boundaries.length, 3);
});

test("SystemAgentResponseSchema rejects a missing tools summary", () => {
  assert.throws(() => SystemAgentResponseSchema.parse({ endpoint: "/mcp" }));
});
