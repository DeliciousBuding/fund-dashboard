// statistics.ts — pure financial/statistical helpers extracted from chart
// components (CorrelationHeatmap, MonteCarloChart). Pure functions so they can
// be unit-tested in isolation (these algorithms previously had zero coverage).

/** Daily simple returns from a NAV/price series: (p[i] - p[i-1]) / p[i-1]. */
export function dailyReturns(navs: number[]): number[] {
  const r: number[] = [];
  for (let i = 1; i < navs.length; i++) {
    r.push((navs[i] - navs[i - 1]) / navs[i - 1]);
  }
  return r;
}

/** Arithmetic mean. Returns 0 for an empty input. */
export function mean(xs: number[]): number {
  if (!xs.length) return 0;
  return xs.reduce((s, v) => s + v, 0) / xs.length;
}

/** Sample variance (denominator n-1). Returns 0 for inputs with fewer than 2 points. */
export function sampleVariance(xs: number[]): number {
  const n = xs.length;
  if (n < 2) return 0;
  const m = mean(xs);
  return xs.reduce((s, v) => s + (v - m) ** 2, 0) / (n - 1);
}

/** Sample standard deviation. */
export function sampleStd(xs: number[]): number {
  return Math.sqrt(sampleVariance(xs));
}

/** Pearson correlation coefficient. Returns 0 for fewer than 3 points or
 *  near-zero variance (avoids divide-by-zero). Range: [-1, 1]. */
export function pearson(a: number[], b: number[]): number {
  const n = a.length;
  if (n < 3) return 0;
  let sa = 0,
    sb = 0,
    saa = 0,
    sbb = 0,
    sab = 0;
  for (let i = 0; i < n; i++) {
    sa += a[i];
    sb += b[i];
    saa += a[i] * a[i];
    sbb += b[i] * b[i];
    sab += a[i] * b[i];
  }
  const num = n * sab - sa * sb;
  const da = Math.sqrt(n * saa - sa * sa);
  const db = Math.sqrt(n * sbb - sb * sb);
  return da < 1e-10 || db < 1e-10 ? 0 : num / (da * db);
}
