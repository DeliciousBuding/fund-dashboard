// dca.test.ts — DCA contracts wire-shape tests (node:test; run: node --test packages/contracts/schemas/)
// 覆盖 Go 端 dca_compute.go 的 omitempty 错误分支与 nil-slice 归一化。
import assert from "node:assert/strict";
import { test } from "node:test";

import { DcaComputeResultSchema, DcaPlanSchema, DcaPlansResponseSchema } from "./dca.ts";

test("DcaComputeResultSchema accepts Go error branch (omitempty fields absent)", () => {
  // internal/service/portfolio/dca_compute.go no_position / insufficient_data:
  // base_amount/dca_rate/actual_amount/signal/explanation 均带 omitempty，错误分支不下发。
  const parsed = DcaComputeResultSchema.parse({
    fund_code: "019173",
    error: "no_position",
    message: "no held position for 019173",
    decision_boundary: "facts_only",
    side_effects: "none",
  });
  assert.equal(parsed.error, "no_position");
  assert.equal(parsed.base_amount, undefined);
  assert.equal(parsed.dca_rate, undefined);
  assert.equal(parsed.actual_amount, undefined);
  assert.equal(parsed.signal, undefined);
  assert.equal(parsed.explanation, undefined);
});

test("DcaComputeResultSchema accepts full success branch", () => {
  const parsed = DcaComputeResultSchema.parse({
    fund_code: "019173",
    security_type: "fund",
    market: "CN",
    mode: "nav_deviation",
    base_amount: 100,
    latest_nav: 1.234,
    cost_per_share: 1.3,
    change_pct: -0.8,
    deviation_pct: -5.1,
    dca_rate: 1.5,
    actual_amount: 150,
    signal: "amplify",
    range: "deviation_-5_to_-10",
    explanation: "NAV 低于成本 5.1%，放大扣款",
    decision_boundary: "facts_only",
    side_effects: "none",
  });
  assert.equal(parsed.actual_amount, 150);
  assert.equal(parsed.dca_rate, 1.5);
  assert.equal(parsed.signal, "amplify");
});

test("DcaComputeResultSchema rejects missing fund_code", () => {
  assert.throws(() => DcaComputeResultSchema.parse({ error: "no_position" }));
});

test("DcaPlansResponseSchema normalizes Go nil slice ({plans:null}) to []", () => {
  // Go ListDCAPlans returns nil slice without plans -> JSON {"plans":null}.
  const parsed = DcaPlansResponseSchema.parse({ plans: null });
  assert.deepEqual(parsed.plans, []);
});

test("DcaPlansResponseSchema passes arrays through", () => {
  const plan = {
    id: 1,
    fund_code: "019173",
    fund_name: null,
    amount: 100,
    frequency: "weekly",
    weekday_mask: "1010100",
    trade_type: "buy",
    portfolio_id: 1,
    start_date: "2026-01-01",
    end_date: null,
    active: 1,
    source: "manual",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
  assert.ok(DcaPlanSchema.parse(plan));
  const parsed = DcaPlansResponseSchema.parse({ plans: [plan] });
  assert.equal(parsed.plans.length, 1);
});
