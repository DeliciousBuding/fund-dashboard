/** Value Averaging DCA rate table and helpers.
 *
 *  The table maps deviation (current price vs cost basis) to a multiplier
 *  applied to the base investment amount:
 *    - price is high  (deviation >= 0)  → invest less  (rate < 1)
 *    - price is low   (deviation <  0)  → invest more  (rate > 1)
 *    - price is flat  (deviation near 0) → normal amount (rate = 1)
 *
 *  Deviation formula: r = (nav - costPerShare) / costPerShare
 */

/** DCA rate lookup table — 15 rows covering [-∞, +∞).
 *  Each entry is [lowerBound, upperBound, multiplier]. */
export const DCA_RATE_TABLE: [number, number, number][] = [
  [0.25, Infinity, 0.50],
  [0.20, 0.25, 0.525],
  [0.15, 0.20, 0.55],
  [0.10, 0.15, 0.60],
  [0.075, 0.10, 0.70],
  [0.05, 0.075, 0.80],
  [0.025, 0.05, 0.90],
  [-0.025, 0.025, 1.00],
  [-0.05, -0.025, 1.20],
  [-0.075, -0.05, 1.40],
  [-0.10, -0.075, 1.60],
  [-0.15, -0.10, 1.80],
  [-0.20, -0.15, 1.90],
  [-0.25, -0.20, 1.95],
  [-Infinity, -0.25, 2.00],
];

/** Look up the DCA multiplier for a given deviation. */
export function computeDcaRate(deviation: number): number {
  for (const [lo, hi, rate] of DCA_RATE_TABLE) {
    if (deviation >= lo && deviation < hi) return rate;
  }
  return 1.0; // fallback (should never be reached — table covers [-∞, +∞))
}

export type DcaMode = "nav_deviation" | "change_pct";

export interface DcaPlanInput {
  mode?: DcaMode;
  baseAmount: number;
  latestNav: number;
  costPerShare?: number | null;
  changePct?: number | null;
}

export interface DcaPlan {
  mode: DcaMode;
  base_amount: number;
  latest_nav: number;
  cost_per_share: number | null;
  change_pct: number | null;
  deviation_pct: number | null;
  dca_rate: number;
  actual_amount: number;
  signal: string;
  explanation: string;
}

function computeChangePctRate(changePct: number): number {
  if (changePct <= -8) return 2.0;
  if (changePct <= -5) return 1.6;
  if (changePct <= -3) return 1.35;
  if (changePct <= -1) return 1.15;
  if (changePct < 1) return 1.0;
  if (changePct < 3) return 0.85;
  if (changePct < 5) return 0.65;
  return 0.5;
}

function rateSignal(mode: DcaMode, rate: number): string {
  if (mode === "change_pct") {
    if (rate > 1) return "跌幅加仓";
    if (rate < 1) return "涨幅控仓";
    return "震荡定投";
  }
  if (rate > 1) return "加仓";
  if (rate < 1) return "减仓";
  return "正常";
}

/** Build a reusable DCA plan for REST, MCP, and UI.
 *
 * nav_deviation: value averaging against cost basis.
 * change_pct: rise/fall mode based on latest price change percentage.
 */
export function computeDcaPlan(input: DcaPlanInput): DcaPlan {
  const mode = input.mode || "nav_deviation";
  const baseAmount = Number.isFinite(input.baseAmount) && input.baseAmount > 0 ? input.baseAmount : 30;
  const latestNav = Number.isFinite(input.latestNav) ? input.latestNav : 0;
  const costPerShare = input.costPerShare != null && input.costPerShare > 0 ? input.costPerShare : null;
  const changePct = input.changePct != null && Number.isFinite(input.changePct) ? input.changePct : null;

  let deviation: number | null = null;
  let rate = 1.0;

  if (mode === "change_pct") {
    rate = computeChangePctRate(changePct ?? 0);
  } else if (costPerShare && latestNav > 0) {
    deviation = (latestNav - costPerShare) / costPerShare;
    rate = computeDcaRate(deviation);
  }

  const actualAmount = +(baseAmount * rate).toFixed(2);
  const signal = rateSignal(mode, rate);
  const explanation = mode === "change_pct"
    ? `最近涨跌幅 ${((changePct ?? 0)).toFixed(2)}%，${signal}，投入 ${actualAmount.toFixed(2)}。`
    : `当前价格相对成本偏离 ${((deviation ?? 0) * 100).toFixed(2)}%，${signal}，投入 ${actualAmount.toFixed(2)}。`;

  return {
    mode,
    base_amount: +baseAmount.toFixed(2),
    latest_nav: +latestNav.toFixed(4),
    cost_per_share: costPerShare != null ? +costPerShare.toFixed(4) : null,
    change_pct: changePct != null ? +changePct.toFixed(2) : null,
    deviation_pct: deviation != null ? +(deviation * 100).toFixed(2) : null,
    dca_rate: rate,
    actual_amount: actualAmount,
    signal,
    explanation,
  };
}
