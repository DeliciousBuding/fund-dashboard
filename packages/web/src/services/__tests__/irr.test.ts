import { describe, it, expect } from "vitest";
import { calcIRR } from "../irr";

const MS_PER_YEAR = 365.25 * 24 * 3600 * 1000;
const y0 = new Date(2024, 0, 1);
const atYear = (y: number) => new Date(y0.getTime() + y * MS_PER_YEAR);

describe("calcIRR", () => {
  it("returns the rate for a single outflow + inflow one year later (10%)", () => {
    // -100 today, +110 in 1 year → IRR = 10%
    expect(calcIRR([-100, 110], [atYear(0), atYear(1)])).toBeCloseTo(0.1, 4);
  });

  it("returns a loss rate when the inflow is below cost", () => {
    expect(calcIRR([-100, 90], [atYear(0), atYear(1)])).toBeCloseTo(-0.1, 4);
  });

  it("solves a multi-period case via Newton's method", () => {
    // -1000 @ y0, -1000 @ y1, +2500 @ y2 → r ≈ 0.1583 (15.83%)
    const r = calcIRR([-1000, -1000, 2500], [atYear(0), atYear(1), atYear(2)]);
    expect(r).toBeCloseTo(0.1583, 3);
  });

  it("returns null when all cashflows are the same sign (undefined IRR)", () => {
    expect(calcIRR([100, 110], [atYear(0), atYear(1)])).toBeNull(); // all positive
    expect(calcIRR([-100, -10], [atYear(0), atYear(1)])).toBeNull(); // all negative
  });

  it("returns null for fewer than 2 cashflows", () => {
    expect(calcIRR([-100], [atYear(0)])).toBeNull();
    expect(calcIRR([], [])).toBeNull();
  });

  it("is clamped above the -100% floor", () => {
    // near-total-loss inflow stays above the -0.999 floor (returns a value, not null)
    const r = calcIRR([-100, 0.5], [atYear(0), atYear(1)]);
    expect(r).not.toBeNull();
    expect(r!).toBeGreaterThan(-0.999);
  });
});
