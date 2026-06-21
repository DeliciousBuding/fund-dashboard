/** /api/funds + /api/securities endpoints — thin wrappers around services */

import { Hono } from "hono";
import { query, queryOne } from "../db";
import { getFundDetail, getMaxDrawdown, getSummaryByFund } from "../services/index";
import { getXirrForCode } from "../services/xirr";
import type { FundDetailRow, PortfolioSnapshotRow, TransactionRow, NavHistoryRow } from "../utils/types";
import { computeDcaPlan } from "../utils/dca";

const router = new Hono();

function normalizeCode(code: string): string {
  const normalized = code.trim();
  return /^\d+$/.test(normalized) ? normalized.padStart(6, "0") : normalized.toUpperCase();
}

// List all securities
router.get("/", c => {
  const fd = query<FundDetailRow>("SELECT * FROM fund_details ORDER BY fund_code");
  const psMap: Record<string, PortfolioSnapshotRow> = {};
  for (const r of query<PortfolioSnapshotRow>("SELECT * FROM portfolio_snapshot")) psMap[r.fund_code] = r;
  return c.json(fd.map(f => {
    const p = psMap[f.fund_code] || {};
    return {
      code: f.fund_code, name: f.fund_name, type: f.fund_type || "",
      security_type: f.security_type || "fund", market: f.market || "",
      held_shares: p.held_shares || 0, current_value: p.current_value ?? null,
      unrealized_pnl: p.unrealized_pnl ?? null, pnl_pct: p.pnl_pct ?? null, latest_nav: p.latest_nav ?? null,
    };
  }));
});

// Fund/security detail
router.get("/:code", c => {
  const code = normalizeCode(c.req.param("code"));
  const data = getFundDetail(code);
  if (!data) return c.json({ error: "not found" }, 404);
  return c.json(data);
});

// Price history
router.get("/:code/nav", c => {
  const code = normalizeCode(c.req.param("code"));
  return c.json(query<NavHistoryRow>("SELECT date, unit_nav FROM nav_history WHERE fund_code = ? ORDER BY date", code)
    .map((r) => ({ date: r.date, unit_nav: r.unit_nav })));
});

// XIRR
router.get("/:code/xirr", c => {
  const code = normalizeCode(c.req.param("code"));
  const x = getXirrForCode(code);
  return c.json({ xirr: x, code });
});

// Max drawdown
router.get("/:code/drawdown", c => {
  const code = normalizeCode(c.req.param("code"));
  const result = getMaxDrawdown(code);
  if (!result) return c.json({ error: "no nav data" }, 404);
  return c.json(result);
});

// Value Averaging DCA (uses DCA_RATE_TABLE from utils/dca.ts)
router.get("/:code/dca", c => {
  const code = normalizeCode(c.req.param("code"));
  const baseAmount = parseFloat(c.req.query("base") || "30");
  const mode = c.req.query("mode") === "change_pct" ? "change_pct" : "nav_deviation";
  const pos = queryOne<PortfolioSnapshotRow>("SELECT * FROM portfolio_snapshot WHERE fund_code = ?", code);
  if (!pos || !pos.held_shares || pos.held_shares < 0.001) {
    return c.json({ error: "no_position", message: "无持仓" }, 400);
  }
  const nav: number = pos.latest_nav;
  const costPerShare = pos.total_cost ? Math.abs(pos.total_cost) / pos.held_shares : null;
  const latestChange = queryOne<{ daily_change_pct: number }>(
    "SELECT daily_change_pct FROM nav_history WHERE fund_code = ? ORDER BY date DESC LIMIT 1",
    code,
  )?.daily_change_pct ?? null;
  if (!nav || (mode === "nav_deviation" && (!costPerShare || costPerShare <= 0))) {
    return c.json({ error: "insufficient_data" }, 400);
  }
  const plan = computeDcaPlan({ mode, baseAmount, latestNav: nav, costPerShare, changePct: latestChange });
  return c.json({
    fund_code: code,
    ...plan,
    range: plan.signal,
  });
});

// ── Summary by fund ────────────────────────────────────────────────
router.get("/summary", (c) => {
  return c.json(getSummaryByFund());
});

export default router;
