// irr.ts — Internal Rate of Return via Newton's method, extracted from
// DcaBacktestChart. Pure function; unit-tested (the algorithm previously had
// zero coverage). Returns null when IRR is undefined (all-positive/all-negative
// cashflows, < 2 cashflows, or non-convergence past the -0.999 floor).

/** Annualized IRR. `dates` align 1:1 with `cashflows` (negative = outflow).
 *  Returns the rate as a fraction (0.1 = 10%), or null if undefined. */
export function calcIRR(cashflows: number[], dates: Date[]): number | null {
  if (cashflows.length < 2) return null;
  const allPos = cashflows.every((c) => c >= 0);
  const allNeg = cashflows.every((c) => c <= 0);
  if (allPos || allNeg) return null;

  let rate = 0.1;
  const msPerYear = 365.25 * 24 * 3600 * 1000;
  const t0 = dates[0].getTime();

  for (let iter = 0; iter < 200; iter++) {
    let npv = 0, dnpv = 0;
    for (let i = 0; i < cashflows.length; i++) {
      const yrs = (dates[i].getTime() - t0) / msPerYear;
      const denom = Math.pow(1 + rate, yrs);
      npv += cashflows[i] / denom;
      if (yrs > 0) dnpv += (-yrs * cashflows[i]) / (denom * (1 + rate));
    }
    if (Math.abs(dnpv) < 1e-15 || Math.abs(npv) < 1e-12) break;
    const nr = rate - npv / dnpv;
    if (Math.abs(nr - rate) < 1e-12) { rate = nr; break; }
    rate = nr;
    if (rate <= -1) rate = -0.999;
    if (rate > 100) rate = 100;
  }

  return isNaN(rate) || !isFinite(rate) || rate <= -0.999 ? null : rate;
}
