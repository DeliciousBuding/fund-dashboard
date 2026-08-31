import { describe, expect, it } from "vitest";
import { normalRandom, simulatePath } from "../montecarlo";

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
