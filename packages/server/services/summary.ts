/**
 * Portfolio Service — summary, XIRR, timeline, penetration, fund detail, admin ops.
 *
 * Extracted from services/portfolio.ts (2026-06-19).
 */

import { query, queryOne, getRwDb } from "../db";
import { calcXirr, getXirrForCode, buildXirrCashflows } from "./xirr";
import type {
  TransactionRow, FundDetail, FundTransaction, TimelineEntry,
  PenetrationResult, PenetrationStock,
  PortfolioSnapshotRow,
} from "../utils/types";

// ── Summary ────────────────────────────────────────────────────────

export function getPortfolioSummary(portfolioId: number = 1): {
  total_tx: number; unique_funds: number; unique_stocks: number; held_funds: number;
  total_buy: number; total_sell: number; total_fee: number; unrealized_pnl: number;
  auto_tx: number; manual_tx: number; auto_amount: number; manual_amount: number;
  first_trade: string; last_trade: string; last_nav_date: string | null;
  settlement_distribution: Record<string, number>;
  trade_type_breakdown: Record<string, number>;
  by_security_type: { security_type: string; count: number; total_value: number; total_pnl: number }[];
} | null {
  const row = queryOne<{
    total_tx: number; total_buy: number; total_sell: number; total_fee: number;
    unrealized_pnl: number; auto_tx: number; manual_tx: number;
    auto_amount: number; manual_amount: number; first_trade: string; last_trade: string;
  }>(`
    SELECT
      COUNT(*) as total_tx,
      SUM(CASE WHEN direction='buy' THEN confirm_amount ELSE 0 END) as total_buy,
      SUM(CASE WHEN direction='sell' THEN confirm_amount ELSE 0 END) as total_sell,
      SUM(COALESCE(fee,0)) as total_fee,
      SUM(COALESCE(unrealized_pnl,0)) as unrealized_pnl,
      SUM(CASE WHEN trade_type LIKE '%定投%' THEN 1 ELSE 0 END) as auto_tx,
      SUM(CASE WHEN trade_type LIKE '%用户%' THEN 1 ELSE 0 END) as manual_tx,
      SUM(CASE WHEN trade_type LIKE '%定投%' THEN confirm_amount ELSE 0 END) as auto_amount,
      SUM(CASE WHEN trade_type LIKE '%用户%' THEN confirm_amount ELSE 0 END) as manual_amount,
      MIN(trade_time) as first_trade, MAX(trade_time) as last_trade
    FROM transactions
  `);
  if (!row) return null;

  const held = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM portfolio_snapshot WHERE held_shares > 0.001 AND portfolio_id = ?", [portfolioId]);
  const fundCount = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM fund_details fd JOIN portfolio_snapshot ps ON ps.fund_code = fd.fund_code AND ps.portfolio_id = ? WHERE fd.security_type IS NULL OR fd.security_type != 'stock'", [portfolioId]);
  const stockCount = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM fund_details fd JOIN portfolio_snapshot ps ON ps.fund_code = fd.fund_code AND ps.portfolio_id = ? WHERE fd.security_type = 'stock'", [portfolioId]);
  const lastNav = queryOne<{ d: string }>("SELECT MAX(date) as d FROM nav_history");

  const sdDist: Record<string, number> = {};
  for (const r of query<{ settlement_days: number | null }>("SELECT settlement_days FROM transactions WHERE settlement_days IS NOT NULL")) {
    sdDist[String(r.settlement_days)] = (sdDist[String(r.settlement_days)] || 0) + 1;
  }

  const ttMap: Record<string, number> = {};
  for (const r of query<{ trade_type: string; n: number }>("SELECT trade_type, COUNT(*) as n FROM transactions GROUP BY trade_type")) {
    ttMap[r.trade_type] = r.n;
  }

  const typeBalance = query<{ security_type: string; count: number; total_value: number; total_pnl: number }>(`
    SELECT ps.security_type, COUNT(*) as count,
      COALESCE(SUM(ps.current_value),0) as total_value,
      COALESCE(SUM(ps.unrealized_pnl),0) as total_pnl
    FROM portfolio_snapshot ps WHERE ps.held_shares > 0.001 AND ps.portfolio_id = ?
    GROUP BY ps.security_type
  `, [portfolioId]);

  return {
    total_tx: row.total_tx,
    unique_funds: fundCount?.n ?? 0,
    unique_stocks: stockCount?.n ?? 0,
    held_funds: held?.n ?? 0,
    total_buy: +((row.total_buy || 0)).toFixed(2),
    total_sell: +((row.total_sell || 0)).toFixed(2),
    total_fee: +((row.total_fee || 0)).toFixed(2),
    unrealized_pnl: +((row.unrealized_pnl || 0)).toFixed(2),
    auto_tx: row.auto_tx, manual_tx: row.manual_tx,
    auto_amount: +((row.auto_amount || 0)).toFixed(2),
    manual_amount: +((row.manual_amount || 0)).toFixed(2),
    first_trade: String(row.first_trade || "").substring(0, 10),
    last_trade: String(row.last_trade || "").substring(0, 10),
    last_nav_date: lastNav?.d ? String(lastNav.d).substring(0, 10) : null,
    settlement_distribution: sdDist,
    trade_type_breakdown: ttMap,
    by_security_type: typeBalance.map((r) => ({
      security_type: r.security_type || "fund",
      count: r.count,
      total_value: +((r.total_value || 0)).toFixed(2),
      total_pnl: +((r.total_pnl || 0)).toFixed(2),
    })),
  };
}

// ── XIRR ────────────────────────────────────────────────────────────

export function getPortfolioXirr(portfolioId: number = 1): number | null {
  const txs = query<{ confirm_amount: number; direction: string; trade_time: string; fee: number }>(
    "SELECT t.confirm_amount, t.direction, t.trade_time, t.fee FROM transactions t JOIN portfolio_snapshot ps ON ps.fund_code = t.fund_code AND ps.portfolio_id = ? WHERE t.direction IN ('buy','sell','dividend') ORDER BY t.trade_time",
    [portfolioId],
  );
  if (txs.length < 2) return null;
  const ps = query<{ held_shares: number; latest_nav: number }>(
    "SELECT held_shares, latest_nav FROM portfolio_snapshot WHERE held_shares > 0.001 AND portfolio_id = ?",
    [portfolioId],
  );
  let pv = 0;
  for (const r of ps) pv += (+r.held_shares) * (+r.latest_nav);
  // B1: unified cashflows (incl. fee + dividend) shared with single-fund path
  const x = calcXirr(buildXirrCashflows(txs, pv));
  return x !== null ? +((x * 100).toFixed(2)) : null;
}

// ── Timeline ───────────────────────────────────────────────────────

export function getPortfolioTimeline(portfolioId: number = 1): TimelineEntry[] {
  const navRows = query<{ date: string; fund_code: string; unit_nav: number }>(`
    SELECT n.date, n.fund_code, n.unit_nav FROM nav_history n
    JOIN portfolio_snapshot ps ON ps.fund_code = n.fund_code AND ps.portfolio_id = ?
    WHERE n.date >= (SELECT MIN(date(t.trade_time)) FROM transactions t JOIN portfolio_snapshot ps2 ON ps2.fund_code = t.fund_code AND ps2.portfolio_id = ?)
    ORDER BY n.date, n.fund_code
  `, [portfolioId, portfolioId]);
  const txRows = query<{ fund_code: string; trade_date: string; signed_share_change: number; signed_cash_flow: number }>(`
    SELECT t.fund_code, date(t.trade_time) as trade_date, t.signed_share_change, t.signed_cash_flow
    FROM transactions t
    JOIN portfolio_snapshot ps ON ps.fund_code = t.fund_code AND ps.portfolio_id = ?
    ORDER BY t.fund_code, t.trade_time
  `, [portfolioId]);

  const fundTx: Record<string, { date: string; shares: number; cost: number }[]> = {};
  for (const r of txRows) {
    const prev = fundTx[r.fund_code]?.at(-1);
    (fundTx[r.fund_code] ??= []).push({
      date: r.trade_date,
      shares: (prev?.shares ?? 0) + (+(r.signed_share_change || 0)),
      cost: (prev?.cost ?? 0) + (+(r.signed_cash_flow || 0)),
    });
  }

  const daily: Record<string, { value: number; cost: number }> = {};
  const pointers: Record<string, number> = {};
  for (const r of navRows) {
    const d = String(r.date).substring(0, 10), code = r.fund_code, nav = +r.unit_nav;
    const txs = fundTx[code] || [];
    let ptr = pointers[code] || 0;
    while (ptr < txs.length && txs[ptr].date <= d) ptr++;
    pointers[code] = ptr;
    if (ptr > 0) {
      const { shares, cost } = txs[ptr - 1];
      (daily[d] ??= { value: 0, cost: 0 });
      if (shares > 0.001) {
        daily[d].value += shares * nav;
        daily[d].cost += cost;
      }
    }
  }

  return Object.entries(daily).sort(([a], [b]) => a.localeCompare(b)).map(([date, v]) => {
    const costAbs = Math.abs(v.cost);
    const pnl = +(v.value + v.cost).toFixed(2);
    return {
      date, total_value: +v.value.toFixed(2), total_cost: +costAbs.toFixed(2),
      pnl, pnl_pct: costAbs > 0.01 ? +(pnl / costAbs * 100).toFixed(2) : 0,
    };
  });
}

// ── Penetration (股权穿透) ──────────────────────────────────────────

export function getPortfolioPenetration(portfolioId: number = 1): PenetrationResult {
  const held = query<{ fund_code: string; fund_name: string; current_value: number }>(
    "SELECT fund_code, fund_name, current_value FROM portfolio_snapshot WHERE held_shares > 0.001 AND portfolio_id = ?",
    [portfolioId],
  );
  const penetration: Record<string, PenetrationStock> = {};

  // Batch query: fetch all holdings for all held funds in one query
  const fundCodes = held.map(f => f.fund_code);
  if (fundCodes.length > 0) {
    const ph = fundCodes.map(() => "?").join(",");
    const allHoldings = query<{ fund_code: string; stock_code: string; stock_name: string; weight_pct: number }>(
      `SELECT fund_code, stock_code, stock_name, weight_pct FROM fund_holdings WHERE fund_code IN (${ph}) ORDER BY fund_code, weight_pct DESC`,
      ...fundCodes,
    );
    // Group by fund_code — keep top 20 per fund
    const holdingsByFund: Record<string, { stock_code: string; stock_name: string; weight_pct: number }[]> = {};
    for (const h of allHoldings) {
      const arr = holdingsByFund[h.fund_code] ??= [];
      if (arr.length < 20) arr.push(h);
    }

    for (const fund of held) {
      const holdings = holdingsByFund[fund.fund_code];
      if (!holdings || !holdings.length) continue;
      for (const h of holdings) {
        const exposure = (fund.current_value || 0) * (h.weight_pct / 100);
        if (!penetration[h.stock_code]) {
          penetration[h.stock_code] = {
            stock_code: h.stock_code, stock_name: h.stock_name,
            total_exposure_cny: 0, held_by_funds: [],
          };
        }
        penetration[h.stock_code].total_exposure_cny += exposure;
        penetration[h.stock_code].held_by_funds.push({
          fund_code: fund.fund_code, fund_name: fund.fund_name,
          weight_pct: h.weight_pct, fund_value_cny: fund.current_value || 0,
        });
      }
    }
  }

  const result = Object.values(penetration).sort((a, b) => b.total_exposure_cny - a.total_exposure_cny);
  const totalValue = held.reduce((s, f) => s + (f.current_value || 0), 0);
  return {
    penetration: result.map(r => ({
      ...r,
      total_exposure_cny: +r.total_exposure_cny.toFixed(2),
      weight_pct: totalValue > 0 ? +((r.total_exposure_cny / totalValue) * 100).toFixed(2) : 0,
    })),
    total_portfolio_value: +totalValue.toFixed(2),
    equity_fund_count: held.filter((f) => penetration[f.fund_code] != null).length,
    unique_stocks: result.length,
  };
}

// ── Fund detail ────────────────────────────────────────────────────

export function getFundDetail(code: string): FundDetail | null {
  const txs = query<TransactionRow>("SELECT * FROM transactions WHERE fund_code = ? ORDER BY trade_time DESC", code);
  if (!txs.length) return null;

  const agg = queryOne<{ total_shares: number; total_cost: number; buy_count: number; sell_count: number }>(`
    SELECT SUM(signed_share_change) as total_shares, SUM(signed_cash_flow) as total_cost,
           SUM(CASE WHEN direction='buy' THEN 1 ELSE 0 END) as buy_count,
           SUM(CASE WHEN direction='sell' THEN 1 ELSE 0 END) as sell_count
    FROM transactions WHERE fund_code = ?
  `, code);

  const navVals = txs.filter((r) => r.latest_nav).map((r) => +r.latest_nav!);
  const latestNav: number | null = navVals.length ? navVals[navVals.length - 1] : null;
  const shares = +(agg?.total_shares || 0);
  const cost = +(agg?.total_cost || 0);
  const cv = latestNav && shares > 0.001 ? shares * latestNav : null;
  const upnl = cv !== null ? cv + cost : null;
  const pct = upnl && cost ? (upnl / Math.abs(cost) * 100) : null;

  const sdVals = txs.filter((r) => r.settlement_days != null).map((r) => r.settlement_days!).sort((a, b) => a - b);
  const medianSd = sdVals.length ? sdVals[Math.floor(sdVals.length / 2)] : 0;

  const autoBuy = txs.filter((r) => r.trade_type === "定投买入");
  const manualBuy = txs.filter((r) => r.trade_type === "用户买入");
  const auto = txs.filter((r) => (r.trade_type || "").includes("定投"));
  const manual = txs.filter((r) => (r.trade_type || "").includes("用户"));

  // Get name from fund_details first, fall back to transactions
  const info = queryOne<{ fund_name: string }>("SELECT fund_name FROM fund_details WHERE fund_code = ?", code);

  return {
    code, name: info?.fund_name || txs[0].fund_name,
    held_shares: +shares.toFixed(2), total_cost: +cost.toFixed(2),
    latest_nav: latestNav ? +latestNav.toFixed(4) : null,
    current_value: cv ? +cv.toFixed(2) : null,
    unrealized_pnl: upnl !== null ? +upnl.toFixed(2) : null,
    pnl_pct: pct !== null ? +pct.toFixed(2) : null,
    auto_buy_count: autoBuy.length, manual_buy_count: manualBuy.length,
    auto_buy_amount: +autoBuy.reduce((s, r) => s + (+r.confirm_amount || 0), 0).toFixed(2),
    manual_buy_amount: +manualBuy.reduce((s, r) => s + (+r.confirm_amount || 0), 0).toFixed(2),
    auto_tx: auto.length, manual_tx: manual.length,
    buy_count: agg?.buy_count ?? 0, sell_count: agg?.sell_count ?? 0,
    median_settlement: medianSd,
    transactions: txs.map((tx): FundTransaction => ({
      seq: tx.seq, trade_time: tx.trade_time,
      confirm_date: (tx.confirm_date || "").substring(0, 10),
      trade_type: tx.trade_type, direction: tx.direction,
      amount: +((tx.confirm_amount || 0)).toFixed(2),
      shares: +((tx.confirm_share || 0)).toFixed(2),
      fee: +((tx.fee || 0)).toFixed(2),
      nav: tx.nav_on_effective_date ? +((+tx.nav_on_effective_date).toFixed(4)) : null,
      inferred_nav: tx.inferred_nav && +tx.inferred_nav > 0 ? +((+tx.inferred_nav).toFixed(6)) : null,
      nav_verified: tx.nav_verified === "True" || tx.nav_verified === true || tx.nav_verified === 1,
      trade_day_type: tx.trade_day_type || "",
      settlement_days: tx.settlement_days,
      effective_nav_date: tx.effective_nav_date || "",
      anomaly: tx.anomaly || null,
      order_id: tx.order_id || "",
    })),
  };
}

// ── Max drawdown ───────────────────────────────────────────────────

export function getMaxDrawdown(code: string): { max_drawdown: number; peak_date: string; trough_date: string; code: string } | null {
  const navs = query<{ date: string; unit_nav: number }>("SELECT date, unit_nav FROM nav_history WHERE fund_code = ? ORDER BY date", code);
  if (!navs.length) return null;
  let peak = +navs[0].unit_nav, maxDd = 0, peakDate = navs[0].date, troughDate = navs[0].date, curPeakDate = navs[0].date;
  for (const r of navs) {
    const nav = +r.unit_nav;
    if (nav > peak) { peak = nav; curPeakDate = r.date; }
    const dd = (peak - nav) / peak;
    if (dd > maxDd) { maxDd = dd; peakDate = curPeakDate; troughDate = r.date; }
  }
  return { max_drawdown: +((maxDd * 100).toFixed(2)), peak_date: peakDate, trough_date: troughDate, code };
}

// ── Fund XIRR ─────────────────────────────────────────────────────

export function getFundXirr(code: string): number | null {
  return getXirrForCode(code);
}

// ── Admin operations ──────────────────────────────────────────────

export function recalculateAllSnapshots(): { securities: number; totalValue: number } {
  // NOTE: multi-portfolio is incomplete — snapshots always land in portfolio_id=1.
  const db = getRwDb();
  db.run("DELETE FROM portfolio_snapshot");
  db.run(`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, security_type, portfolio_id)
    SELECT fund_code, MAX(fund_name), SUM(signed_share_change), SUM(signed_cash_flow),
      (SELECT unit_nav FROM nav_history WHERE fund_code = transactions.fund_code ORDER BY date DESC LIMIT 1),
      COALESCE((SELECT security_type FROM fund_details WHERE fund_code = transactions.fund_code), 'fund'),
      1
    FROM transactions GROUP BY fund_code`);
  db.run(`UPDATE portfolio_snapshot SET current_value = held_shares * latest_nav,
    unrealized_pnl = (held_shares * latest_nav) + total_cost,
    pnl_pct = CASE WHEN total_cost != 0 THEN ((held_shares * latest_nav) + total_cost) / ABS(total_cost) * 100 END
    WHERE held_shares > 0.001 AND latest_nav IS NOT NULL`);
  const n = queryOne<{ n: number }>("SELECT COUNT(*) as n FROM portfolio_snapshot")?.n ?? 0;
  const total = queryOne<{ v: number }>("SELECT SUM(current_value) as v FROM portfolio_snapshot")?.v ?? 0;
  populateSummaryByFund();
  return { securities: n, totalValue: +total.toFixed(2) };
}

export function populateSummaryByFund() {
  const db = getRwDb();
  db.run("DELETE FROM summary_by_fund");
  db.run(`
    INSERT INTO summary_by_fund (fund_code, fund_name, total_shares, total_cost, tx_count)
    SELECT t.fund_code, COALESCE(f.fund_name, t.fund_name),
      SUM(COALESCE(t.signed_share_change, 0)),
      SUM(COALESCE(t.signed_cash_flow, 0)),
      COUNT(*)
    FROM transactions t
    LEFT JOIN fund_details f ON f.fund_code = t.fund_code
    GROUP BY t.fund_code
  `);
}

export function getSummaryByFund(): { fund_code: string; fund_name: string; total_shares: number; total_cost: number; tx_count: number }[] {
  return query("SELECT fund_code, fund_name, total_shares, total_cost, tx_count FROM summary_by_fund ORDER BY fund_code");
}

export function adjustPosition(code: string, shares: number): PortfolioSnapshotRow | undefined {
  getRwDb().run(`UPDATE portfolio_snapshot SET held_shares = ?, current_value = held_shares * latest_nav,
    unrealized_pnl = (held_shares * latest_nav) + total_cost,
    pnl_pct = CASE WHEN total_cost != 0 THEN ((held_shares * latest_nav) + total_cost) / ABS(total_cost) * 100 END
    WHERE fund_code = ?`, [shares, code]);
  return queryOne<PortfolioSnapshotRow>("SELECT * FROM portfolio_snapshot WHERE fund_code = ?", code);
}

/** Recalculate snapshot for a single security after transaction changes */
export function recalcSnapshot(code: string) {
  const db = getRwDb();
  const agg = queryOne<any>(
    "SELECT SUM(signed_share_change) as shares, SUM(signed_cash_flow) as cost FROM transactions WHERE fund_code = ?",
    code,
  );
  const st = queryOne<any>(
    "SELECT security_type FROM fund_details WHERE fund_code = ?",
    code,
  );
  const secType = st?.security_type || "fund";
  const nav = queryOne<any>(
    "SELECT unit_nav FROM nav_history WHERE fund_code = ? ORDER BY date DESC LIMIT 1",
    code,
  );
  if (agg && nav?.unit_nav && agg.shares > 0.001) {
    const oldRow = queryOne<any>("SELECT portfolio_id FROM portfolio_snapshot WHERE fund_code = ?", [code]);
    const portfolioId = oldRow?.portfolio_id ?? 1;
    db.run("DELETE FROM portfolio_snapshot WHERE fund_code = ?", [code]);
    db.run(
      `INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
       VALUES (?, (SELECT fund_name FROM fund_details WHERE fund_code = ?), ?, ?, ?, ? * ?, (? * ?) + ?, CASE WHEN ? != 0 THEN ((? * ?) + ?) / ABS(?) * 100 END, ?, ?)`,
      [
        code, code, agg.shares, agg.cost, nav.unit_nav,
        agg.shares, nav.unit_nav, agg.shares, nav.unit_nav, agg.cost,
        agg.cost, agg.shares, nav.unit_nav, agg.cost, agg.cost, secType, portfolioId,
      ],
    );
  }
}
