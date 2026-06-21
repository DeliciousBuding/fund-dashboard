/** XIRR: Newton + bisection, normalized cashflows.
 *
 * v3.0 (B1): unified cashflow semantics — buy includes fee, sell nets fee,
 * dividend treated as cash inflow. Shared by single-fund and portfolio paths,
 * eliminating the prior divergence (single-fund counted dividend, portfolio
 * filtered it out; neither counted fee → XIRR biased high).
 */

import { query, queryOne } from "../db";

export interface XirrTxInput {
  confirm_amount: number;
  direction: string; // 'buy' | 'sell' | 'dividend' | ...
  trade_time: string;
  fee: number;
}

/**
 * Build XIRR cashflows from transactions + current market value.
 * Unified semantics (fixes B-XIRR-1/2/3):
 *   buy      → -(amount + fee)   // real cost incl. commission
 *   sell     → +(amount - fee)   // real proceeds net of commission
 *   dividend → +amount           // cash dividend inflow
 *                              // (DB cannot distinguish reinvested dividends;
 *                              //  treated as inflow — same as legacy single-fund path)
 *   + currentValue at t=0        // mark-to-market of held position
 * `txs` MUST be ordered by trade_time ascending (caller's responsibility).
 */
export function buildXirrCashflows(
  txs: XirrTxInput[],
  currentValue: number,
): { amount: number; years: number }[] {
  if (txs.length === 0) return [];
  const lastMs = new Date(txs[txs.length - 1].trade_time).getTime();
  const cfs = txs.map(tx => {
    const amt = +tx.confirm_amount;
    const fee = +tx.fee || 0;
    let signed: number;
    switch (tx.direction) {
      case "buy": signed = -(amt + fee); break;
      case "sell": signed = +(amt - fee); break;
      case "dividend": signed = +amt; break;
      default: signed = 0; // convert_in/out, forced_redeem, etc. — neutral for now
    }
    return {
      amount: signed,
      years: (lastMs - new Date(tx.trade_time).getTime()) / 31536000000,
    };
  });
  if (currentValue > 0) cfs.push({ amount: currentValue, years: 0 });
  return cfs;
}

/** Compute XIRR for a single fund code. Returns annualized return as percentage (e.g. 12.5 = 12.5%), or null. */
export function getXirrForCode(code: string): number | null {
  const cfRows = query<XirrTxInput>(
    "SELECT confirm_amount, direction, trade_time, fee FROM transactions WHERE fund_code = ? AND direction IN ('buy','sell','dividend') ORDER BY trade_time",
    code,
  );
  if (cfRows.length < 2) return null;
  const latestNav = queryOne<{ unit_nav: number }>(
    "SELECT unit_nav FROM nav_history WHERE fund_code = ? ORDER BY date DESC LIMIT 1", code,
  );
  const shares = queryOne<{ s: number }>(
    "SELECT SUM(signed_share_change) as s FROM transactions WHERE fund_code = ?", code,
  );
  const currentValue = shares?.s && shares.s > 0.001 && latestNav ? shares.s * latestNav.unit_nav : 0;
  const x = calcXirr(buildXirrCashflows(cfRows, currentValue));
  return x !== null ? +((x * 100).toFixed(2)) : null;
}

export function calcXirr(cashflows: { amount: number; years: number }[]): number | null {
  if (cashflows.length < 2) return null;
  if (cashflows.every(cf => cf.amount <= 0)) return null;
  if (cashflows.every(cf => cf.amount >= 0)) return null;

  // Normalize: last cashflow at t=0, others at positive t
  const lastT = cashflows[cashflows.length - 1].years;
  const norm = cashflows.map(cf => ({ amount: cf.amount, years: cf.years - lastT }));

  const npv = (r: number) => {
    let s = 0;
    for (const cf of norm) {
      const y = Math.max(cf.years, 1e-10);
      // `years` is measured FROM the last cashflow (past = positive). Compound
      // past flows forward to the evaluation point (multiply), NOT discount
      // (divide). The prior `/` solved a rate with inverted sign (B-XIRR-DIR):
      // a +20% gain returned ~-16.7%. Fixed in v3.0.
      s += cf.amount * Math.pow(1 + r, y);
    }
    return s;
  };

  // Newton from multiple starts
  for (const guess of [0.1, 0.3, 0.5, 0.7, 0.9, -0.3, -0.5]) {
    let rate = guess, prev = Infinity;
    for (let iter = 0; iter < 80; iter++) {
      const fv = npv(rate);
      if (Math.abs(fv) < 0.001) return rate;
      const df = (npv(rate + 1e-6) - fv) / 1e-6;
      if (Math.abs(df) < 1e-14) break;
      const nr = rate - fv / df;
      if (Math.abs(nr - prev) < 1e-9) return nr;
      prev = rate;
      rate = Math.max(-0.999, Math.min(nr, 1e6));
    }
  }

  // Bisection sweep — try standard bound then higher bound
  const tryBisect = (hi: number): number | null => {
    let lo = -0.999;
    for (let i = 0; i < 200; i++) {
      const mid = (lo + hi) / 2;
      const v = npv(mid);
      if (Math.abs(v) < 0.001) return mid;
      if (npv(lo) * v < 0) hi = mid; else lo = mid;
    }
    return null;
  };

  const r1 = tryBisect(10);
  if (r1 !== null) return r1;

  // For very profitable portfolios the IRR can be high; try a wider bracket
  const r2 = tryBisect(1000);
  if (r2 !== null) return r2;

  // Truly monotonic positive NPV across a wide range → no IRR exists
  if (npv(0) > 0.01 && npv(1000) > 0) return null;

  return null;
}
