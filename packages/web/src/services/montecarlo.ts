// montecarlo.ts — Pure Monte Carlo simulation logic extracted from MonteCarloChart.
// Zero React/hooks dependencies; pure functions only.

export interface MonteCarloResult {
  /** Terminal fractional returns, one per simulation, sorted ascending. Each
   *  inner array has a single element (the terminal value). */
  paths: number[][];
  percentiles: {
    p5: number;
    p10: number;
    p25: number;
    p50: number;
    p75: number;
    p90: number;
    p95: number;
  };
  /** Histogram bins over the terminal-return range. `bucket` is the bin
   *  midpoint (same unit as the returns). `count` is the raw frequency. */
  histogram: { bucket: number; count: number }[];
  stats: {
    /** Daily mean of the input return series. */
    mean: number;
    /** Daily sample std of the input return series. */
    std: number;
    min: number;
    max: number;
  };
}

const BINS = 40;

/**
 * Box-Muller transform — sample from N(0,1).
 * Avoids the zero-case by re-rolling exact zeros from Math.random().
 */
export function normalRandom(): number {
  let u = 0,
    v = 0;
  while (u === 0) u = Math.random();
  while (v === 0) v = Math.random();
  return Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * v);
}

/**
 * Simulate one price/NAV path using geometric Brownian motion.
 * Returns the cumulative value at each step (length = tradingDays + 1,
 * first element = startValue).
 */
export function simulatePath(
  startValue: number,
  dailyMean: number,
  dailyStd: number,
  tradingDays: number,
): number[] {
  const path: number[] = [startValue];
  let cum = startValue;
  for (let t = 0; t < tradingDays; t++) {
    cum *= 1 + dailyMean + dailyStd * normalRandom();
    path.push(cum);
  }
  return path;
}

/**
 * Run a full Monte Carlo simulation over a series of daily returns.
 *
 * - Computes daily mean / sample std from `returns`.
 * - Simulates `numSimulations` independent price paths over `tradingDays`.
 * - Returns sorted terminal fractional returns, histogram, and key percentiles.
 *
 * Throws if `returns` is empty.
 */
export function runMonteCarlo(
  returns: number[],
  numSimulations: number,
  tradingDays: number,
): MonteCarloResult {
  if (!returns.length) {
    throw new Error("Empty returns array");
  }

  const n = returns.length;
  const mean = returns.reduce((s, v) => s + v, 0) / n;
  const variance =
    n < 2
      ? 0
      : returns.reduce((s, v) => s + (v - mean) ** 2, 0) / (n - 1);
  const std = Math.sqrt(variance);

  // Simulate paths, collect terminal fractional returns.
  const results: number[] = [];
  for (let s = 0; s < numSimulations; s++) {
    const path = simulatePath(1, mean, std, tradingDays);
    results.push(path[path.length - 1] - 1);
  }
  results.sort((a, b) => a - b);

  const percentile = (p: number) => results[Math.floor(results.length * p)];

  const minR = results[0];
  const maxR = results[results.length - 1];
  const binW = (maxR - minR) / BINS || 0.01;
  const bins = new Array<number>(BINS).fill(0);
  for (const r of results) {
    const idx = Math.min(BINS - 1, Math.floor((r - minR) / binW));
    bins[idx]++;
  }

  const histogram = bins.map((count, i) => ({
    bucket: minR + binW * (i + 0.5),
    count,
  }));

  return {
    paths: results.map((r) => [r]),
    percentiles: {
      p5: percentile(0.05),
      p10: percentile(0.1),
      p25: percentile(0.25),
      p50: percentile(0.5),
      p75: percentile(0.75),
      p90: percentile(0.9),
      p95: percentile(0.95),
    },
    histogram,
    stats: { mean, std, min: minR, max: maxR },
  };
}
