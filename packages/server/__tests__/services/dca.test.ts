import { describe, test, expect } from "bun:test";
import { computeDcaPlan } from "../../utils/dca";

describe("computeDcaPlan", () => {
  test("supports change_pct mode that invests more after drops and less after rallies", () => {
    const drop = computeDcaPlan({
      mode: "change_pct",
      baseAmount: 100,
      latestNav: 1.2,
      changePct: -6.5,
    });
    const rally = computeDcaPlan({
      mode: "change_pct",
      baseAmount: 100,
      latestNav: 1.2,
      changePct: 8.2,
    });

    expect(drop.actual_amount).toBeGreaterThan(100);
    expect(drop.signal).toBe("跌幅加仓");
    expect(rally.actual_amount).toBeLessThan(100);
    expect(rally.signal).toBe("涨幅控仓");
  });

  test("keeps value averaging compatible with cost-basis deviation mode", () => {
    const plan = computeDcaPlan({
      mode: "nav_deviation",
      baseAmount: 200,
      latestNav: 0.8,
      costPerShare: 1.0,
    });

    expect(plan.deviation_pct).toBe(-20);
    expect(plan.dca_rate).toBeGreaterThan(1);
    expect(plan.actual_amount).toBeGreaterThan(200);
    expect(plan.signal).toBe("加仓");
  });
});
