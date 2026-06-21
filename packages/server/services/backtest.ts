/** Backtest Engine — pure-function strategy simulator. grid / momentum / rebalance / dca. */
import type { BacktestParams, BacktestResult, BacktestTrade, BacktestTimelinePoint, BacktestStrategy } from "../utils/types";

type Nav = { date: string; fund_code: string; unit_nav: number };

// ── Simulation helpers ──

function navsFrom(navs: Nav[], start: string) { return navs.filter(n => n.date >= start).sort((a, b) => a.date.localeCompare(b.date)); }

function computeMetrics(tl: BacktestTimelinePoint[], totalInv: number, start: string, end: string, rf = 0.02) {
  let peak = -Infinity, maxDd = 0;
  const rets: number[] = [];
  for (let i = 0; i < tl.length; i++) {
    if (tl[i].total_value > peak) peak = tl[i].total_value;
    const dd = peak > 0 ? (peak - tl[i].total_value) / peak : 0;
    if (dd > maxDd) maxDd = dd;
    if (i > 0 && tl[i - 1].total_value > 0) rets.push((tl[i].total_value - tl[i - 1].total_value) / tl[i - 1].total_value);
  }
  const tr = totalInv > 0 ? (tl[tl.length - 1]?.total_value - totalInv) / totalInv : 0;
  const years = Math.max((new Date(end).getTime() - new Date(start).getTime()) / 31557600000, 0.01);
  const ar = (1 + tr) ** (1 / years) - 1;
  const mean = rets.length ? rets.reduce((a, b) => a + b, 0) / rets.length : 0;
  const vari = rets.length > 1 ? rets.reduce((s, v) => s + (v - mean) ** 2, 0) / (rets.length - 1) : 0;
  const std = Math.sqrt(vari);
  const sharpe = std > 0 ? (mean * 252 - rf) / (std * Math.sqrt(252)) : 0;
  return { maxDd: +(maxDd * 100).toFixed(2), totalReturn: +(tr * 100).toFixed(2), annualReturn: +(ar * 100).toFixed(2), sharpe: +sharpe.toFixed(2) };
}

function lumpSumReturn(navs: Nav[], start: string, invested: number) {
  const f = navsFrom(navs, start); if (!f.length) return { invested, final_value: 0, return_pct: -100 };
  const shares = invested / f[0].unit_nav, fv = shares * f[f.length - 1].unit_nav;
  return { invested: +invested.toFixed(2), final_value: +fv.toFixed(2), return_pct: +((fv - invested) / invested * 100).toFixed(2) };
}

function dcaReturn(navs: Nav[], start: string, base: number) {
  const f = navsFrom(navs, start); if (!f.length) return { invested: 0, final_value: 0, return_pct: 0 };
  let shares = 0, invested = 0, lastM = -1, lastY = -1;
  for (const n of f) {
    const [y, m] = n.date.split("-").map(Number);
    if (lastY !== y || lastM !== m) { lastY = y; lastM = m; shares += base / n.unit_nav; invested += base; }
  }
  const fv = shares * f[f.length - 1].unit_nav;
  return { invested: +invested.toFixed(2), final_value: +fv.toFixed(2), return_pct: invested > 0 ? +((fv - invested) / invested * 100).toFixed(2) : 0 };
}

// ── DCA ──

function simDca(navs: Nav[], start: string, base: number) {
  const f = navsFrom(navs, start), trades: BacktestTrade[] = [], tl: BacktestTimelinePoint[] = [];
  if (!f.length) return { trades, tl };
  let shares = 0, cash = 0, invested = 0, lastY = -1, lastM = -1;
  for (const n of f) {
    const [y, m] = n.date.split("-").map(Number);
    if (lastY !== y || lastM !== m) {
      lastY = y; lastM = m;
      const s = base / n.unit_nav; shares += s; cash -= base; invested += base;
      trades.push({ date: n.date, action: "buy", price: +n.unit_nav.toFixed(4), shares: +s.toFixed(4), amount: base, reason: "定期定额买入 (DCA)" });
    }
    const eq = shares * n.unit_nav;
    tl.push({ date: n.date, nav: +n.unit_nav.toFixed(4), shares_held: +shares.toFixed(4), cash: +cash.toFixed(2), equity_value: +eq.toFixed(2), total_value: +((eq + cash).toFixed(2)), total_invested: +invested.toFixed(2) });
  }
  return { trades, tl };
}

// ── Grid ──

function simGrid(navs: Nav[], start: string, base: number, levels = 5) {
  const f = navsFrom(navs, start), trades: BacktestTrade[] = [], tl: BacktestTimelinePoint[] = [];
  if (!f.length) return { trades, tl };
  let shares = 0, cash = 0, invested = 0;
  const calib = f.slice(0, Math.max(Math.floor(f.length * 0.2), 5)).map(n => n.unit_nav);
  const lo = Math.min(...calib), step = (Math.max(...calib) - lo) / levels;
  let prev = -1;
  for (const n of f) {
    const g = step > 0 ? Math.min(Math.floor((n.unit_nav - lo) / step), levels - 1) : 0;
    if (prev >= 0 && g !== prev) {
      const d = prev - g, amt = base * Math.abs(d);
      if (d > 0) { const s = amt / n.unit_nav; shares += s; cash -= amt; invested += amt; trades.push({ date: n.date, action: "buy", price: +n.unit_nav.toFixed(4), shares: +s.toFixed(4), amount: +amt.toFixed(2), reason: `价格下跌至第${g + 1}格(从${prev + 1})，买入` }); }
      else { const sv = amt, ss = Math.min(sv / n.unit_nav, shares); if (ss > 0.0001) { shares -= ss; cash += ss * n.unit_nav; trades.push({ date: n.date, action: "sell", price: +n.unit_nav.toFixed(4), shares: +ss.toFixed(4), amount: +(+(ss * n.unit_nav).toFixed(2)), reason: `价格上涨至第${g + 1}格(从${prev + 1})，卖出` }); } }
    }
    prev = g;
    const eq = shares * n.unit_nav;
    tl.push({ date: n.date, nav: +n.unit_nav.toFixed(4), shares_held: +shares.toFixed(4), cash: +cash.toFixed(2), equity_value: +eq.toFixed(2), total_value: +((eq + cash).toFixed(2)), total_invested: +invested.toFixed(2) });
  }
  return { trades, tl };
}

// ── Momentum ──

function simMomentum(navs: Nav[], start: string, base: number, lookback = 3) {
  const f = navsFrom(navs, start), trades: BacktestTrade[] = [], tl: BacktestTimelinePoint[] = [];
  if (!f.length) return { trades, tl };
  const byDate = new Map<string, number>(); for (const n of navs) byDate.set(n.date, n.unit_nav);
  let shares = base / f[0].unit_nav, cash = -base, invested = base, lastY = -1, lastM = -1;
  trades.push({ date: f[0].date, action: "buy", price: +f[0].unit_nav.toFixed(4), shares: +shares.toFixed(4), amount: base, reason: "动量策略初始建仓" });
  const sortedDates = Array.from(byDate.keys()).sort();
  for (const n of f) {
    const [y, m] = n.date.split("-").map(Number);
    if (lastY !== y || lastM !== m) {
      lastY = y; lastM = m;
      const lb = new Date(y, m - 1 - lookback, 1).toISOString().substring(0, 10);
      let past: number | null = null;
      for (const d of sortedDates) { if (d >= lb && d < n.date) { past = byDate.get(d) ?? null; break; } }
      if (past === null) for (const d of sortedDates) { if (d >= start) { past = byDate.get(d) ?? null; break; } }
      if (past && past > 0) {
        const mom = (n.unit_nav - past) / past;
        if (mom > 0.02) { const s = base / n.unit_nav; shares += s; cash -= base; invested += base; trades.push({ date: n.date, action: "buy", price: +n.unit_nav.toFixed(4), shares: +s.toFixed(4), amount: base, reason: `${lookback}月动量+${(mom * 100).toFixed(1)}%，买入` }); }
        else if (mom < -0.02) { const sv = base, ss = Math.min(sv / n.unit_nav, shares); if (ss > 0.0001) { shares -= ss; cash += ss * n.unit_nav; trades.push({ date: n.date, action: "sell", price: +n.unit_nav.toFixed(4), shares: +ss.toFixed(4), amount: +(+(ss * n.unit_nav).toFixed(2)), reason: `${lookback}月动量${(mom * 100).toFixed(1)}%，卖出` }); } }
      }
    }
    const eq = shares * n.unit_nav;
    tl.push({ date: n.date, nav: +n.unit_nav.toFixed(4), shares_held: +shares.toFixed(4), cash: +cash.toFixed(2), equity_value: +eq.toFixed(2), total_value: +((eq + cash).toFixed(2)), total_invested: +invested.toFixed(2) });
  }
  return { trades, tl };
}

// ── Rebalance ──

function simRebalance(navs: Nav[], start: string, base: number, targetW = 0.6, interval = 3) {
  const f = navsFrom(navs, start), trades: BacktestTrade[] = [], tl: BacktestTimelinePoint[] = [];
  if (!f.length) return { trades, tl };
  const initEq = base * targetW, initSh = initEq / f[0].unit_nav;
  let shares = initSh, cash = base - initEq, invested = base, lastY = -1, lastM = -1;
  trades.push({ date: f[0].date, action: "buy", price: +f[0].unit_nav.toFixed(4), shares: +initSh.toFixed(4), amount: +initEq.toFixed(2), reason: "再平衡初始建仓" });
  for (const n of f) {
    const [y, m] = n.date.split("-").map(Number);
    const mos = (y - +f[0].date.split("-")[0]) * 12 + (m - +f[0].date.split("-")[1]);
    if (mos > 0 && mos % interval === 0 && (lastY !== y || lastM !== m)) {
      lastY = y; lastM = m;
      const eqV = shares * n.unit_nav, total = eqV + cash, target = total * targetW, diff = target - eqV;
      if (Math.abs(diff) > base * 0.1) {
        if (diff > 0) { const s = diff / n.unit_nav; shares += s; cash -= diff; invested += diff; trades.push({ date: n.date, action: "buy", price: +n.unit_nav.toFixed(4), shares: +s.toFixed(4), amount: +diff.toFixed(2), reason: `再平衡:权益不足${(targetW * 100).toFixed(0)}%，补仓` }); }
        else { const sa = Math.abs(diff), ss = Math.min(sa / n.unit_nav, shares); if (ss > 0.0001) { shares -= ss; cash += ss * n.unit_nav; trades.push({ date: n.date, action: "sell", price: +n.unit_nav.toFixed(4), shares: +ss.toFixed(4), amount: +(+(ss * n.unit_nav).toFixed(2)), reason: `再平衡:权益超出${(targetW * 100).toFixed(0)}%，减仓` }); } }
      }
    }
    const eq = shares * n.unit_nav;
    tl.push({ date: n.date, nav: +n.unit_nav.toFixed(4), shares_held: +shares.toFixed(4), cash: +cash.toFixed(2), equity_value: +eq.toFixed(2), total_value: +((eq + cash).toFixed(2)), total_invested: +invested.toFixed(2) });
  }
  return { trades, tl };
}

// ── Main ──

const sims: Record<BacktestStrategy, (navs: Nav[], sd: string, ba: number, o: BacktestParams) => { trades: BacktestTrade[]; tl: BacktestTimelinePoint[] }> = {
  dca: (n, s, b) => simDca(n, s, b),
  grid: (n, s, b, o) => simGrid(n, s, b, o.grid_levels ?? 5),
  momentum: (n, s, b, o) => simMomentum(n, s, b, o.momentum_months ?? 3),
  rebalance: (n, s, b, o) => simRebalance(n, s, b, o.target_weight ?? 0.6, o.rebalance_interval ?? 3),
};

/** Run strategy backtest against historical NAV data. Pure function, no side effects. */
export function runBacktest(navs: Nav[], params: BacktestParams): BacktestResult {
  const { trades, tl } = sims[params.strategy || "dca"](navs, params.start_date, params.base_amount, params);
  const last = tl[tl.length - 1], inv = last?.total_invested ?? 0, fv = last?.total_value ?? 0;
  const m = computeMetrics(tl, inv, params.start_date, last?.date ?? params.start_date);
  return {
    fund_code: params.fund_code, strategy: params.strategy || "dca",
    start_date: params.start_date, end_date: last?.date ?? params.start_date,
    base_amount: params.base_amount, total_invested: +(inv.toFixed(2)), final_value: +(fv.toFixed(2)),
    total_return_pct: m.totalReturn, annual_return_pct: m.annualReturn,
    max_drawdown_pct: m.maxDd, sharpe_ratio: m.sharpe, trades, timeline: tl,
    comparison: { lump_sum: lumpSumReturn(navs, params.start_date, params.base_amount * 12), dca: dcaReturn(navs, params.start_date, params.base_amount) },
  };
}
