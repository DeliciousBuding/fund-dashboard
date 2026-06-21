/** /api/portfolio endpoints — thin wrappers around services/portfolio.ts */

import { Hono } from "hono";
import { getPortfolioSummary, getPortfolioXirr, getPortfolioTimeline, getPortfolioPenetration, getPortfolioAllocation, getInvestmentHarnessSnapshot, getInvestmentSourceBrief, getSourceEvents, createSourceEvent, markSourceEventRead, runBacktest } from "../services/index";
import { query, queryOne } from "../db";
import { log } from "../middleware/logger";
import { SourceEventsResponseSchema } from "@fund-dashboard/contracts";

const router = new Hono();

/** Extract portfolio_id from query params, defaulting to 1 */
function getPortfolioId(c: any): number {
  const pid = parseInt(c.req.query("portfolio_id") || "1", 10);
  return isNaN(pid) || pid < 1 ? 1 : pid;
}

/** GET /api/portfolio/portfolios — list all portfolio definitions */
router.get("/portfolios", c => {
  const rows = query<{ id: number; name: string; description: string }>(
    "SELECT id, name, description FROM portfolio_definitions ORDER BY id",
  );
  return c.json(rows);
});

router.get("/", c => {
  const t0 = Date.now();
  const pid = getPortfolioId(c);
  const data = getPortfolioSummary(pid);
  log.debug("portfolio query", { duration: Date.now() - t0, portfolio_id: pid });
  if (!data) {
    return c.json({
      total_tx: 0, unique_funds: 0, unique_stocks: 0, held_funds: 0,
      total_buy: 0, total_sell: 0, total_fee: 0, unrealized_pnl: 0,
      auto_tx: 0, manual_tx: 0, auto_amount: 0, manual_amount: 0,
      first_trade: "", last_trade: "", last_nav_date: null,
      settlement_distribution: {}, trade_type_breakdown: {}, by_security_type: [],
    });
  }
  return c.json(data);
});

router.get("/xirr", c => {
  const pid = getPortfolioId(c);
  const xirr = getPortfolioXirr(pid);
  return c.json({ xirr });
});

router.get("/timeline", c => {
  const t0 = Date.now();
  const pid = getPortfolioId(c);
  const result = getPortfolioTimeline(pid);
  log.debug("timeline built", { duration: Date.now() - t0, rows: result.length });
  return c.json(result);
});

router.get("/penetration", c => {
  const pid = getPortfolioId(c);
  const data = getPortfolioPenetration(pid);
  return c.json(data);
});

router.get("/by-type", c => {
  const pid = getPortfolioId(c);
  const summary = getPortfolioSummary(pid);
  if (!summary) return c.json([]);
  return c.json(summary.by_security_type);
});

router.get("/allocation", c => {
  const pid = getPortfolioId(c);
  return c.json(getPortfolioAllocation(pid));
});

router.get("/harness", c => {
  const pid = getPortfolioId(c);
  return c.json(getInvestmentHarnessSnapshot(pid));
});

router.get("/source-brief", c => {
  const limit = parseInt(c.req.query("limit") || "20", 10);
  const pid = getPortfolioId(c);
  return c.json(getInvestmentSourceBrief({ limit, portfolioId: pid }));
});

// ═══════════ Source Events (V4) ═══════════

router.get("/source-events", c => {
  const opts: Record<string, any> = {};
  const code = c.req.query("code");
  const source = c.req.query("source");
  const showRead = c.req.query("show_read");
  if (code) opts.related_security_code = code;
  if (source) opts.source = source;
  if (showRead) opts.show_read = showRead === "1" || showRead === "true";
  opts.limit = parseInt(c.req.query("limit") || "30", 10);
  // Normalize DB integers (0/1) → booleans to match SourceEventSchema contract,
  // and wrap in the SourceEventsResponse shape (fixes C-1/G6: was bare array).
  const events = getSourceEvents(opts).map(r => ({
    ...r,
    is_read: !!r.is_read,
    is_useful: !!r.is_useful,
  }));
  const payload = {
    count: events.length,
    decision_boundary: "facts_only" as const,
    events,
  };
  // A4: contract guard — surface field drift loudly in dev, never 500 in prod.
  const parsed = SourceEventsResponseSchema.safeParse(payload);
  if (!parsed.success) {
    log.warn("source-events response drift from contract", { issues: parsed.error.issues });
    return c.json(payload);
  }
  return c.json(parsed.data);
});

router.post("/source-events", async c => {
  const body = await c.req.json();
  if (!body.title) return c.json({ error: "title is required" }, 400);
  const event = createSourceEvent({
    title: body.title,
    url: body.url,
    source: body.source,
    snippet: body.snippet,
    query: body.query,
    related_security_code: body.related_security_code,
    related_security_name: body.related_security_name,
  });
  return c.json(event, 201);
});

router.patch("/source-events/:id", async c => {
  const id = parseInt(c.req.param("id"), 10);
  if (isNaN(id)) return c.json({ error: "invalid id" }, 400);
  const body = await c.req.json();
  const ok = markSourceEventRead(id, {
    is_read: body.is_read,
    is_useful: body.is_useful,
  });
  if (!ok) return c.json({ error: "not found or no fields to update" }, 404);
  return c.json({ ok: true, id });
});

// ═══════════ Analysis: Backtest ═══════════

router.post("/analysis/backtest", async c => {
  const body = await c.req.json();
  const fundCode = body.fund_code;
  if (!fundCode) return c.json({ error: "fund_code is required" }, 400);

  const startDate = body.start_date;
  if (!startDate) return c.json({ error: "start_date is required (YYYY-MM-DD)" }, 400);

  const baseAmount = Number(body.base_amount);
  if (!baseAmount || baseAmount <= 0) return c.json({ error: "base_amount must be > 0" }, 400);

  const strategy = (body.strategy || "dca") as string;
  if (!["grid", "momentum", "rebalance", "dca"].includes(strategy)) {
    return c.json({ error: `unknown strategy: ${strategy}. Use: grid, momentum, rebalance, dca` }, 400);
  }

  const navs = query<{ date: string; fund_code: string; unit_nav: number }>(
    "SELECT date, fund_code, unit_nav FROM nav_history WHERE fund_code = ? AND date >= ? ORDER BY date",
    [fundCode, startDate],
  );
  if (!navs.length) {
    return c.json({ error: "no nav_history data for this fund_code and date range" }, 404);
  }

  const result = runBacktest(navs, {
    fund_code: fundCode,
    strategy: strategy as any,
    start_date: startDate,
    base_amount: baseAmount,
    grid_levels: body.grid_levels,
    momentum_months: body.momentum_months,
    target_weight: body.target_weight,
    rebalance_interval: body.rebalance_interval,
  });

  return c.json(result);
});

export default router;
