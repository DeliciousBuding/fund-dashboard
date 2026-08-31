// montecarlo.ts — Pure Monte Carlo simulation logic extracted from MonteCarloChart.
// Zero React/hooks dependencies; pure functions only.

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
