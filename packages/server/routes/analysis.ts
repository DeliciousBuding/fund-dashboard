/** /api/analysis endpoints — cross-fund comparison & metrics */

import { Hono } from "hono";
import { query, queryOne } from "../db";
import { getXirrForCode } from "../services/xirr";

const router = new Hono();

function padCode(code: string): string {
  const n = code.trim();
  return /^\d+$/.test(n) ? n.padStart(6, "0") : n.toUpperCase();
}

/** Compute annualized volatility from daily NAV returns */
function calcVolatility(navs: { date: string; unit_nav: number }[]): number | null {
  if (navs.length < 10) return null;
  const returns: number[] = [];
  for (let i = 1; i < navs.length; i++) {
    const r = Math.log(navs[i].unit_nav / navs[i - 1].unit_nav);
    returns.push(r);
  }
  const mean = returns.reduce((s, v) => s + v, 0) / returns.length;
  const variance = returns.reduce((s, v) => s + (v - mean) ** 2, 0) / (returns.length - 1);
  // Annualize: daily vol * sqrt(252)
  return Math.sqrt(variance) * Math.sqrt(252);
}

/** Compare multiple funds on risk/return metrics */
router.get("/compare", (c) => {
  const raw = c.req.query("codes");
  if (!raw) return c.json({ error: "codes parameter required (comma-separated)" }, 400);
  const codes = [...new Set(raw.split(",").map(padCode).filter(Boolean))];
  if (!codes.length) return c.json({ error: "no valid codes" }, 400);

  const results: {
    code: string; name: string; market: string;
    xirr: number | null; volatility: number | null;
    sharpe: number | null; max_drawdown: number | null;
    calmar: number | null;
  }[] = [];

  for (const code of codes) {
    const fd = queryOne<{ fund_name: string; market: string }>(
      "SELECT fund_name, market FROM fund_details WHERE fund_code = ?", code,
    );
    if (!fd) {
      results.push({ code, name: code, market: "", xirr: null, volatility: null, sharpe: null, max_drawdown: null, calmar: null });
      continue;
    }

    // XIRR
    const xirr = getXirrForCode(code);

    // Volatility
    const navs = query<{ date: string; unit_nav: number }>(
      "SELECT date, unit_nav FROM nav_history WHERE fund_code = ? ORDER BY date", code,
    );
    const volatility = navs.length >= 10 ? +(calcVolatility(navs)! * 100).toFixed(2) : null;

    // Max drawdown
    let maxDd: number | null = null;
    if (navs.length) {
      let peak = +navs[0].unit_nav, md = 0;
      for (const r of navs) {
        const nav = +r.unit_nav;
        if (nav > peak) peak = nav;
        const dd = (peak - nav) / peak;
        if (dd > md) md = dd;
      }
      maxDd = +((md * 100).toFixed(2));
    }

    // Sharpe: (xirr - rf) / vol, simplified rf=0
    const sharpe = (xirr != null && volatility != null && volatility > 0.001)
      ? +(xirr / volatility).toFixed(4) : null;

    // Calmar: xirr / max_drawdown
    const calmar = (xirr != null && maxDd != null && maxDd > 0.01)
      ? +(xirr / maxDd).toFixed(4) : null;

    results.push({
      code, name: fd.fund_name, market: fd.market || "",
      xirr, volatility,
      sharpe, max_drawdown: maxDd,
      calmar,
    });
  }

  return c.json({ funds: results });
});

export default router;
