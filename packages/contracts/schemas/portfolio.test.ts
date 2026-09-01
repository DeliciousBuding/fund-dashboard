// portfolio.test.ts — portfolio/allocation wire-shape tests (node:test).
// 覆盖 Go 端 nil slice 序列化为 null 的归一化（allocationRiskFlags）。
import assert from "node:assert/strict";
import { test } from "node:test";

import {
  PortfolioAllocationSchema,
  PortfolioDefinitionSchema,
  PortfolioSchema,
  TimelinePointSchema,
} from "./portfolio.ts";

const allocationBase = {
  total_value: 12345.67,
  by_security_type: [{ key: "fund", label: "Fund", value: 12345.67, weight_pct: 100, count: 3 }],
  by_market: [],
  by_fund_type: [],
  agent_brief: "Allocation: Fund 100%. Risk: no concentration alerts.",
};

test("PortfolioAllocationSchema normalizes Go nil risk_flags (null) to []", () => {
  // internal/service/portfolio/allocation.go allocationRiskFlags returns a nil
  // slice when no concentration rule fires -> JSON "risk_flags": null.
  const parsed = PortfolioAllocationSchema.parse({ ...allocationBase, risk_flags: null });
  assert.deepEqual(parsed.risk_flags, []);
});

test("PortfolioAllocationSchema passes risk_flags arrays through", () => {
  const parsed = PortfolioAllocationSchema.parse({
    ...allocationBase,
    risk_flags: ["Stock weight above 80%"],
  });
  assert.deepEqual(parsed.risk_flags, ["Stock weight above 80%"]);
});

const summaryWire = {
  total_tx: 12,
  unique_funds: 3,
  unique_stocks: 0,
  held_funds: 3,
  total_buy: 1000,
  total_sell: 100,
  total_fee: 1.5,
  unrealized_pnl: 20.5,
  invested_cost: 900,
  current_value: 920.5,
  pnl_pct: 2.28,
  top_gainer: null,
  top_loser: null,
  stale_nav_days: null,
  auto_tx: 8,
  manual_tx: 4,
  auto_amount: 800,
  manual_amount: 200,
  first_trade: "2024-01-02",
  last_trade: "2026-08-29",
  last_nav_date: "2026-08-29",
  settlement_distribution: { "2": 10 },
  trade_type_breakdown: { buy: 11, sell: 1 },
  by_security_type: [{ security_type: "fund", count: 3, total_value: 920.5, total_pnl: 20.5 }],
};

test("PortfolioSchema parses a full summary (CI smoke shape)", () => {
  const parsed = PortfolioSchema.parse(summaryWire);
  assert.equal(parsed.total_tx, 12);
  assert.equal(parsed.top_gainer, null);
});

test("PortfolioSchema requires invested_cost (Go always emits it)", () => {
  const { invested_cost, ...rest } = summaryWire;
  assert.throws(() => PortfolioSchema.parse(rest));
});

test("PortfolioSchema requires current_value (Go always emits it)", () => {
  const { current_value, ...rest } = summaryWire;
  assert.throws(() => PortfolioSchema.parse(rest));
});

test("PortfolioSchema requires pnl_pct (Go always emits it)", () => {
  const { pnl_pct, ...rest } = summaryWire;
  assert.throws(() => PortfolioSchema.parse(rest));
});

// ── /api/portfolio/portfolios + /api/portfolio/timeline ─────────────

const definitionWire = {
  id: 1,
  name: "默认组合",
  description: "主账户持仓",
};

test("PortfolioDefinitionSchema parses the real definition shape", () => {
  const parsed = PortfolioDefinitionSchema.parse(definitionWire);
  assert.equal(parsed.id, 1);
  assert.equal(parsed.description, "主账户持仓");
});

test("PortfolioDefinitionSchema requires the COALESCE'd description field", () => {
  assert.throws(() => PortfolioDefinitionSchema.parse({ id: 1, name: "默认组合" }));
});

const timelineWire = {
  date: "2026-08-29",
  total_value: 12345.67,
  total_cost: 10000,
  pnl: 2345.67,
  pnl_pct: 23.46,
};

test("TimelinePointSchema parses the real timeline point shape", () => {
  const parsed = TimelinePointSchema.parse(timelineWire);
  assert.equal(parsed.total_value, 12345.67);
  assert.equal(parsed.pnl_pct, 23.46);
});

test("TimelinePointSchema rejects a point missing pnl_pct", () => {
  assert.throws(() =>
    TimelinePointSchema.parse({
      date: "2026-08-29",
      total_value: 12345.67,
      total_cost: 10000,
      pnl: 2345.67,
    }),
  );
});
