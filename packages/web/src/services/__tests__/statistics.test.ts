import { describe, it, expect } from "vitest";
import { dailyReturns, mean, sampleVariance, sampleStd, pearson } from "../statistics";

describe("dailyReturns", () => {
  it("computes successive simple returns", () => {
    expect(dailyReturns([1, 2, 4, 8])).toEqual([1, 1, 1]); // +100% each step
  });
  it("returns [] for fewer than 2 points", () => {
    expect(dailyReturns([5])).toEqual([]);
    expect(dailyReturns([])).toEqual([]);
  });
  it("handles a declining series", () => {
    expect(dailyReturns([4, 2, 1])).toEqual([-0.5, -0.5]); // -50% each step
  });
});

describe("mean / sampleVariance / sampleStd", () => {
  it("mean is the arithmetic average (0 for empty)", () => {
    expect(mean([1, 2, 3, 4])).toBe(2.5);
    expect(mean([])).toBe(0);
  });
  it("sampleVariance uses n-1 denominator (0 for <2 points)", () => {
    // [1,2,3]: mean 2, sum sq dev = 2, / (n-1)=2 → 1
    expect(sampleVariance([1, 2, 3])).toBeCloseTo(1, 6);
    expect(sampleVariance([5])).toBe(0);
  });
  it("sampleStd is sqrt of sampleVariance", () => {
    expect(sampleStd([1, 1])).toBeCloseTo(0, 5);
    expect(sampleStd([0, 2])).toBeCloseTo(Math.sqrt(2), 5);
  });
});

describe("pearson", () => {
  it("returns +1 for a perfectly correlated series", () => {
    expect(pearson([1, 2, 3, 4], [1, 2, 3, 4])).toBeCloseTo(1, 6);
  });
  it("returns -1 for a perfectly anti-correlated series", () => {
    expect(pearson([1, 2, 3, 4], [4, 3, 2, 1])).toBeCloseTo(-1, 6);
  });
  it("returns 0 for fewer than 3 points", () => {
    expect(pearson([1, 2], [1, 2])).toBe(0);
  });
  it("returns 0 for a constant series (zero variance)", () => {
    expect(pearson([5, 5, 5, 5], [1, 2, 3, 4])).toBe(0);
  });
  it("matches a known positive correlation", () => {
    // a=[1..5], b=[2,4,5,4,5] → r ≈ 0.7746
    const a = [1, 2, 3, 4, 5];
    const b = [2, 4, 5, 4, 5];
    const r = pearson(a, b);
    expect(r).toBeCloseTo(0.7746, 3);
    expect(r).toBeGreaterThan(0);
    expect(r).toBeLessThan(1);
  });
});
