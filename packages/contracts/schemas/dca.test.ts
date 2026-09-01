// dca.test.ts — DCA contracts wire-shape tests (node:test; run: node --test packages/contracts/schemas/)
// 覆盖 Go 端 dca_compute.go 的 omitempty 错误分支与 nil-slice 归一化。
import assert from "node:assert/strict";
import { test } from "node:test";

import {
  DcaComputeResultSchema,
  DcaExecutionItemSchema,
  DcaPlanDisableResponseSchema,
  DcaPlanSchema,
  DcaPlansResponseSchema,
  DcaPlanUpsertResponseSchema,
  DcaRunResultSchema,
} from "./dca.ts";

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

// ── /api/dca/run（RunDCAAutoInvestResult）────────────────────────────

const runWire = {
  ok: true,
  as_of: "2026-08-29",
  dry_run: true,
  executed: 0,
  skipped: 1,
  previewed: 1,
  items: [
    {
      plan_id: 1,
      fund_code: "019173",
      fund_name: "示例基金",
      amount: 100,
      order_id: "DCA-1-20260829",
      status: "preview",
      trade_type: "auto",
      portfolio_id: 1,
      weekday_mask: "1,2,3,4,5",
    },
  ],
  decision_boundary: "facts_only",
  side_effects: "none",
};

test("DcaRunResultSchema parses the real dry-run payload", () => {
  const parsed = DcaRunResultSchema.parse(runWire);
  assert.equal(parsed.dry_run, true);
  assert.equal(parsed.previewed, 1);
  assert.equal(parsed.items[0]?.order_id, "DCA-1-20260829");
});

test("DcaRunResultSchema accepts omitempty item fields (shares/nav/message absent)", () => {
  const parsed = DcaRunResultSchema.parse({
    ...runWire,
    items: [
      {
        plan_id: 2,
        fund_code: "019174",
        amount: 100,
        order_id: "DCA-2-20260829",
        status: "skipped_not_due",
        portfolio_id: 1,
      },
    ],
  });
  assert.equal(parsed.items[0]?.shares, undefined);
  assert.equal(parsed.items[0]?.message, undefined);
});

test("DcaRunResultSchema rejects a run result missing as_of", () => {
  const { as_of, ...missing } = runWire;
  assert.throws(() => DcaRunResultSchema.parse(missing));
});

test("DcaExecutionItemSchema rejects an item missing required order_id", () => {
  assert.throws(() =>
    DcaExecutionItemSchema.parse({
      plan_id: 1,
      fund_code: "019173",
      amount: 100,
      status: "preview",
      portfolio_id: 1,
    }),
  );
});

// ── /api/dca/plans upsert + disable ─────────────────────────────────

const planWire = {
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

test("DcaPlanUpsertResponseSchema parses {ok:true, plan}", () => {
  const parsed = DcaPlanUpsertResponseSchema.parse({ ok: true, plan: planWire });
  assert.equal(parsed.ok, true);
  assert.equal(parsed.plan.fund_code, "019173");
});

test("DcaPlanUpsertResponseSchema rejects a response missing the echoed plan", () => {
  assert.throws(() => DcaPlanUpsertResponseSchema.parse({ ok: true }));
});

test("DcaPlanDisableResponseSchema parses {ok,id,updated}", () => {
  const parsed = DcaPlanDisableResponseSchema.parse({ ok: true, id: 1, updated: true });
  assert.equal(parsed.id, 1);
  assert.equal(parsed.updated, true);
});

test("DcaPlanDisableResponseSchema rejects a response missing updated", () => {
  assert.throws(() => DcaPlanDisableResponseSchema.parse({ ok: true, id: 1 }));
});
