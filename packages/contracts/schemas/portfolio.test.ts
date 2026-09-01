// portfolio.test.ts — portfolio/allocation wire-shape tests (node:test).
// 覆盖 Go 端 nil slice 序列化为 null 的归一化（allocationRiskFlags）。
import assert from "node:assert/strict";
import { test } from "node:test";

import { PortfolioAllocationSchema, PortfolioSchema } from "./portfolio.ts";

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

test("PortfolioSchema parses a full summary (CI smoke shape)", () => {
  const parsed = PortfolioSchema.parse({
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
  });
  assert.equal(parsed.total_tx, 12);
  assert.equal(parsed.top_gainer, null);
});
