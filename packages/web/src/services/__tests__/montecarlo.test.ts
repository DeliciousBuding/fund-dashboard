import { describe, it, expect } from "vitest";
import { normalRandom, simulatePath, runMonteCarlo } from "../montecarlo";

describe("normalRandom", () => {
  it("returns a finite number", () => {
    for (let i = 0; i < 100; i++) {
      const v = normalRandom();
      expect(Number.isFinite(v)).toBe(true);
    }
  });

  it("produces a distribution with mean near 0 and std near 1 (large sample)", () => {
    const N = 50000;
    const samples: number[] = [];
    for (let i = 0; i < N; i++) samples.push(normalRandom());
    const mean = samples.reduce((s, v) => s + v, 0) / N;
    const variance = samples.reduce((s, v) => s + (v - mean) ** 2, 0) / (N - 1);
    const std = Math.sqrt(variance);

    expect(Math.abs(mean)).toBeLessThan(0.05);
    expect(std).toBeGreaterThan(0.95);
    expect(std).toBeLessThan(1.05);
  });
});

describe("simulatePath", () => {
  it("returns path of length tradingDays + 1 with first value equal to startValue", () => {
    const path = simulatePath(100, 0.001, 0.02, 252);
    expect(path.length).toBe(253);
    expect(path[0]).toBe(100);
  });

  it("with zero drift and zero volatility, value never changes", () => {
    const path = simulatePath(50, 0, 0, 10);
    expect(path.every((v) => v === 50)).toBe(true);
  });

  it("with positive drift, the final value tends above startValue (probabilistic)", () => {
    const finals: number[] = [];
    for (let i = 0; i < 2000; i++) {
      const path = simulatePath(100, 0.0005, 0.01, 252);
      finals.push(path[path.length - 1]);
    }
    finals.sort((a, b) => a - b);
    const median = finals[Math.floor(finals.length / 2)];
    expect(median).toBeGreaterThan(100);
  });

  it("with negative drift, the final value tends below startValue (probabilistic)", () => {
    const finals: number[] = [];
    for (let i = 0; i < 2000; i++) {
      const path = simulatePath(100, -0.0005, 0.01, 252);
      finals.push(path[path.length - 1]);
    }
    finals.sort((a, b) => a - b);
    const median = finals[Math.floor(finals.length / 2)];
    expect(median).toBeLessThan(100);
  });

  it("path values are all positive (no negative prices)", () => {
    const path = simulatePath(100, 0.0002, 0.01, 252);
    expect(path.every((v) => v > 0)).toBe(true);
  });
});

describe("runMonteCarlo", () => {
  // Synthetic daily returns with realistic mean/std for a mixed portfolio
  const returns = Array.from({ length: 100 }, () => (Math.random() - 0.48) * 0.02);

  it("returns correct structure with specified number of paths", () => {
    const result = runMonteCarlo(returns, 500, 100);
    expect(result.paths).toHaveLength(500);
    expect(result.paths[0]).toHaveLength(1); // terminal value only
    expect(result.histogram).toHaveLength(40); // BINS
    expect(result.percentiles).toHaveProperty("p5");
    expect(result.percentiles).toHaveProperty("p10");
    expect(result.percentiles).toHaveProperty("p25");
    expect(result.percentiles).toHaveProperty("p50");
    expect(result.percentiles).toHaveProperty("p75");
    expect(result.percentiles).toHaveProperty("p90");
    expect(result.percentiles).toHaveProperty("p95");
    expect(result.stats).toHaveProperty("mean");
    expect(result.stats).toHaveProperty("std");
    expect(result.stats).toHaveProperty("min");
    expect(result.stats).toHaveProperty("max");
  });

  it("returns sorted paths (terminal values are ascending)", () => {
    const result = runMonteCarlo(returns, 1000, 252);
    const values = result.paths.map((p) => p[0]);
    for (let i = 1; i < values.length; i++) {
      expect(values[i]).toBeGreaterThanOrEqual(values[i - 1]);
    }
  });

  it("percentiles are monotonic: p5 <= p10 <= p25 <= p50 <= p75 <= p90 <= p95", () => {
    const result = runMonteCarlo(returns, 1000, 252);
    const { p5, p10, p25, p50, p75, p90, p95 } = result.percentiles;
    expect(p5).toBeLessThanOrEqual(p10);
    expect(p10).toBeLessThanOrEqual(p25);
    expect(p25).toBeLessThanOrEqual(p50);
    expect(p50).toBeLessThanOrEqual(p75);
    expect(p75).toBeLessThanOrEqual(p90);
    expect(p90).toBeLessThanOrEqual(p95);
  });

  it("histogram counts sum to numSimulations", () => {
    const result = runMonteCarlo(returns, 500, 100);
    const totalCount = result.histogram.reduce((s, h) => s + h.count, 0);
    expect(totalCount).toBe(500);
  });

  it("stats.min <= stats.max always", () => {
    const result = runMonteCarlo(returns, 500, 100);
    expect(result.stats.min).toBeLessThanOrEqual(result.stats.max);
  });

  it("throws on empty returns array", () => {
    expect(() => runMonteCarlo([], 100, 252)).toThrow("Empty returns array");
  });

  it("handles single return value (zero variance)", () => {
    const result = runMonteCarlo([0.001], 200, 30);
    expect(result.stats.std).toBe(0);
    // All terminal values should be identical (no randomness)
    const values = result.paths.map((p) => p[0]);
    const first = values[0];
    for (const v of values) {
      expect(v).toBeCloseTo(first, 10);
    }
    // All histogram counts in one bin, rest zero
    const nonZero = result.histogram.filter((h) => h.count > 0);
    expect(nonZero.length).toBe(1);
    expect(nonZero[0].count).toBe(200);
  });

  it("works with large simulation count", () => {
    const result = runMonteCarlo(returns, 1000, 50);
    const totalCount = result.histogram.reduce((s, h) => s + h.count, 0);
    expect(totalCount).toBe(1000);
    expect(result.paths).toHaveLength(1000);
  });
});
