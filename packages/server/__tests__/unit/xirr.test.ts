// Unit tests for XIRR (B1): unified cashflow semantics + calcXirr boundaries.
// Pure (no DB): buildXirrCashflows + calcXirr.
import { describe, it, expect } from "bun:test";
import { calcXirr, buildXirrCashflows, type XirrTxInput } from "../../services/xirr";

const tx = (
  direction: string,
  confirm_amount: number,
  trade_time: string,
  fee = 0,
): XirrTxInput => ({ confirm_amount, direction, trade_time, fee });

describe("buildXirrCashflows (B1 unified semantics)", () => {
  it("buy is negative incl. fee, sell is positive net fee", () => {
    const cfs = buildXirrCashflows([
      tx("buy", 1000, "2026-01-01 00:00:00", 5),
      tx("sell", 500, "2026-02-01 00:00:00", 3),
    ], 0);
    expect(cfs[0].amount).toBe(-1005); // -(1000+5)
    expect(cfs[1].amount).toBe(497); // +(500-3)
  });

  it("dividend is a positive inflow (B-XIRR-3: now shared with portfolio path)", () => {
    const cfs = buildXirrCashflows([
      tx("buy", 1000, "2026-01-01 00:00:00"),
      tx("dividend", 50, "2026-02-01 00:00:00"),
    ], 0);
    expect(cfs[1].amount).toBe(50);
  });

  it("appends currentValue at t=0", () => {
    const cfs = buildXirrCashflows([tx("buy", 1000, "2026-01-01 00:00:00")], 1200);
    const last = cfs[cfs.length - 1];
    expect(last.amount).toBe(1200);
    expect(last.years).toBe(0);
  });

  it("years measured from last tx (last = 0, earlier > 0)", () => {
    const cfs = buildXirrCashflows([
      tx("buy", 1000, "2026-01-01 00:00:00"),
      tx("buy", 1000, "2026-04-01 00:00:00"),
    ], 0);
    expect(cfs[1].years).toBe(0);
    expect(cfs[0].years).toBeGreaterThan(0);
  });

  it("omits currentValue when 0 (fully liquidated)", () => {
    const cfs = buildXirrCashflows([tx("buy", 1000, "2026-01-01 00:00:00")], 0);
    expect(cfs).toHaveLength(1);
  });

  it("empty txs → []", () => {
    expect(buildXirrCashflows([], 100)).toEqual([]);
  });

  it("unknown direction contributes 0", () => {
    const cfs = buildXirrCashflows([tx("convert_in", 100, "2026-01-01 00:00:00")], 0);
    expect(cfs[0].amount).toBe(0);
  });
});

describe("calcXirr boundaries", () => {
  it("returns null for < 2 cashflows", () => {
    expect(calcXirr([{ amount: -100, years: 0 }])).toBeNull();
  });

  it("returns null when all cashflows same sign", () => {
    expect(calcXirr([{ amount: -100, years: 1 }, { amount: -50, years: 0 }])).toBeNull();
    expect(calcXirr([{ amount: 100, years: 1 }, { amount: 50, years: 0 }])).toBeNull();
  });

  it("positive return for a profitable 1-year investment", () => {
    const r = calcXirr([{ amount: -1000, years: 1 }, { amount: 1200, years: 0 }]);
    expect(r).not.toBeNull();
    expect(r!).toBeGreaterThan(0.1);
  });

  it("negative return for a loss", () => {
    const r = calcXirr([{ amount: -1000, years: 1 }, { amount: 800, years: 0 }]);
    expect(r).not.toBeNull();
    expect(r!).toBeLessThan(0);
  });

  it("approx 0% when proceeds equal cost", () => {
    const r = calcXirr([{ amount: -1000, years: 1 }, { amount: 1000, years: 0 }]);
    expect(r).not.toBeNull();
    expect(Math.abs(r!)).toBeLessThan(0.02);
  });
});
