/**
 * Fund Dashboard API Integration Tests — Portfolio API
 *
 * Run: npx bun test packages/server/__tests__/api-integration-portfolio.test.ts
 *
 * Tests GET endpoints under /api/portfolio: summary, xirr, timeline, allocation,
 * harness, and source-brief.
 *
 * Uses shared test-db.ts helper for in-memory SQLite + mocks.
 * No real DB, no network, no side effects.
 */

import { describe, test, expect, beforeEach } from "bun:test";
import { Hono } from "hono";
import { ApiError } from "../utils/errors";
import { initTestDb, getTestDb, clearTestDb, seedPortfolioData } from "./helpers/test-db";

// ═══════════════════════════════════════════════════════════════════════
// Init schema (mock.module is registered inside test-db.ts)
// ═══════════════════════════════════════════════════════════════════════

initTestDb();

// ═══════════════════════════════════════════════════════════════════════
// Import routes AFTER mocks are registered (by test-db.ts import above)
// ═══════════════════════════════════════════════════════════════════════

import portfolioRoutes from "../routes/portfolio";

// ═══════════════════════════════════════════════════════════════════════
// Build test Hono app (no auth middleware — endpoints are open in test)
// ═══════════════════════════════════════════════════════════════════════

const memDb = getTestDb();
const app = new Hono();
app.route("/api/portfolio", portfolioRoutes);

// Health endpoint mirroring main.ts
app.get("/api/health", (c) => {
  try {
    memDb.query("SELECT 1").get();
    return c.json({ status: "ok", uptime: process.uptime() });
  } catch (e: any) {
    return c.json({ status: "error", error: e.message }, 500);
  }
});

// Error handler — mirrors main.ts to catch ApiError from validation middleware
app.onError((err, c) => {
  if (err instanceof ApiError) {
    return c.json({ error: err.code, message: err.message }, err.status as any);
  }
  return c.json({ error: "internal", message: "Internal server error" }, 500);
});

// ═══════════════════════════════════════════════════════════════════════
// HTTP helpers
// ═══════════════════════════════════════════════════════════════════════

async function get(path: string) {
  const req = new Request(`http://localhost${path}`);
  const res = await app.fetch(req);
  const body = await res.json().catch(() => null);
  return { status: res.status, body };
}

async function post(path: string, data?: any) {
  const req = new Request(`http://localhost${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: data !== undefined ? JSON.stringify(data) : undefined,
  });
  const res = await app.fetch(req);
  const body = await res.json().catch(() => null);
  return { status: res.status, body };
}

async function patch(path: string, data?: any) {
  const req = new Request(`http://localhost${path}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: data !== undefined ? JSON.stringify(data) : undefined,
  });
  const res = await app.fetch(req);
  const body = await res.json().catch(() => null);
  return { status: res.status, body };
}

// ═══════════════════════════════════════════════════════════════════════
// Teardown — clear all tables before each test
// ═══════════════════════════════════════════════════════════════════════

beforeEach(() => {
  clearTestDb();
});

// ═══════════════════════════════════════════════════════════════════════
// PORTFOLIO — 11 tests
// ═══════════════════════════════════════════════════════════════════════

describe("Portfolio API", () => {
  // 1. GET /api/portfolio — returns summary (empty DB → defaults)
  test("GET /api/portfolio returns summary with expected shape on empty DB", async () => {
    const { status, body } = await get("/api/portfolio");
    expect(status).toBe(200);
    expect(body).toHaveProperty("total_tx");
    expect(body).toHaveProperty("unique_funds");
    expect(body).toHaveProperty("held_funds");
    expect(body).toHaveProperty("total_buy");
    expect(body).toHaveProperty("total_sell");
    expect(body).toHaveProperty("total_fee");
    expect(body).toHaveProperty("unrealized_pnl");
    expect(body).toHaveProperty("auto_tx");
    expect(body).toHaveProperty("manual_tx");
    expect(body).toHaveProperty("first_trade");
    expect(body).toHaveProperty("last_trade");
    expect(body).toHaveProperty("last_nav_date");
    expect(body).toHaveProperty("settlement_distribution");
    expect(body).toHaveProperty("trade_type_breakdown");
    expect(body).toHaveProperty("by_security_type");
    expect(Array.isArray(body.by_security_type)).toBe(true);
    // Empty DB: counts should be zero
    expect(typeof body.total_tx).toBe("number");
    expect(typeof body.held_funds).toBe("number");
  });

  test("GET /api/portfolio with seeded data returns correct totals", async () => {
    seedPortfolioData();
    const { status, body } = await get("/api/portfolio");
    expect(status).toBe(200);
    expect(body.total_tx).toBe(4);
    expect(body.held_funds).toBe(2);
    expect(body.unique_funds).toBeGreaterThanOrEqual(2);
    expect(body.total_buy).toBeGreaterThan(0);
    expect(body.total_fee).toBeGreaterThan(0);
    expect(body.first_trade).toBeTruthy();
    expect(body.last_trade).toBeTruthy();
    expect(body.by_security_type.length).toBeGreaterThan(0);
  });

  // 2. GET /api/portfolio/xirr
  test("GET /api/portfolio/xirr returns xirr or null on empty DB", async () => {
    const { status, body } = await get("/api/portfolio/xirr");
    expect(status).toBe(200);
    expect(body).toHaveProperty("xirr");
    // xirr can be null (insufficient cashflows) or a number
    if (body.xirr !== null) {
      expect(typeof body.xirr).toBe("number");
    }
  });

  test("GET /api/portfolio/xirr with seeded data returns a number", async () => {
    seedPortfolioData();
    const { status, body } = await get("/api/portfolio/xirr");
    expect(status).toBe(200);
    expect(body).toHaveProperty("xirr");
    // With seeded data we should get a computed XIRR
    expect(body.xirr).not.toBeNull();
    expect(typeof body.xirr).toBe("number");
  });

  // 3. GET /api/portfolio/timeline
  test("GET /api/portfolio/timeline returns array", async () => {
    seedPortfolioData();
    const { status, body } = await get("/api/portfolio/timeline");
    expect(status).toBe(200);
    expect(Array.isArray(body)).toBe(true);
    // Timeline is built from nav_history + transactions; may be empty if no matching data
    if (body.length > 0) {
      expect(body[0]).toHaveProperty("date");
      expect(body[0]).toHaveProperty("total_value");
      expect(body[0]).toHaveProperty("total_cost");
      expect(body[0]).toHaveProperty("pnl");
      expect(body[0]).toHaveProperty("pnl_pct");
    }
  });

  // 4. GET /api/portfolio/allocation
  test("GET /api/portfolio/allocation returns allocation shape", async () => {
    seedPortfolioData();
    const { status, body } = await get("/api/portfolio/allocation");
    expect(status).toBe(200);
    expect(body).toHaveProperty("total_value");
    expect(body).toHaveProperty("by_security_type");
    expect(body).toHaveProperty("by_market");
    expect(body).toHaveProperty("by_fund_type");
    expect(body).toHaveProperty("risk_flags");
    expect(body).toHaveProperty("agent_brief");
    expect(Array.isArray(body.by_security_type)).toBe(true);
    expect(Array.isArray(body.by_market)).toBe(true);
    expect(Array.isArray(body.risk_flags)).toBe(true);
    // With 2 funds seeded, total_value should be positive
    expect(body.total_value).toBeGreaterThan(0);
    // by_security_type should group by fund/stock
    const types = body.by_security_type.map((b: any) => b.key);
    expect(types).toContain("fund");
  });

  test("GET /api/portfolio/allocation on empty DB returns zero values", async () => {
    const { status, body } = await get("/api/portfolio/allocation");
    expect(status).toBe(200);
    expect(body.total_value).toBe(0);
    expect(body.by_security_type).toEqual([]);
    expect(body.agent_brief).toContain("暂无持仓");
  });

  // 5. GET /api/portfolio/harness
  test("GET /api/portfolio/harness returns facts-only snapshot", async () => {
    seedPortfolioData();
    const { status, body } = await get("/api/portfolio/harness");
    expect(status).toBe(200);
    expect(body.decision_boundary).toBe("facts_only");
    expect(body).toHaveProperty("generated_at");
    expect(body).toHaveProperty("total_value");
    expect(body).toHaveProperty("holdings_count");
    expect(body).toHaveProperty("allocation");
    expect(body).toHaveProperty("holding_signals");
    expect(body).toHaveProperty("data_quality");
    expect(body).toHaveProperty("available_agent_tools");
    expect(body).toHaveProperty("agent_brief");
    expect(Array.isArray(body.holding_signals)).toBe(true);
    expect(Array.isArray(body.available_agent_tools)).toBe(true);
    expect(body.holdings_count).toBeGreaterThan(0);
    // Each holding signal has required shape
    if (body.holding_signals.length > 0) {
      const sig = body.holding_signals[0];
      expect(sig).toHaveProperty("code");
      expect(sig).toHaveProperty("name");
      expect(sig).toHaveProperty("security_type");
      expect(sig).toHaveProperty("market");
      expect(sig).toHaveProperty("held_shares");
      expect(sig).toHaveProperty("current_value");
      expect(sig).toHaveProperty("weight_pct");
      expect(sig).toHaveProperty("latest_nav");
      expect(sig).toHaveProperty("cost_per_share");
      expect(sig).toHaveProperty("change_pct");
      expect(sig).toHaveProperty("deviation_pct");
      expect(sig).toHaveProperty("signal_tags");
      expect(sig).toHaveProperty("data_points");
      expect(Array.isArray(sig.signal_tags)).toBe(true);
      expect(sig.data_points).toHaveProperty("has_price");
      expect(sig.data_points).toHaveProperty("has_cost_basis");
      expect(sig.data_points).toHaveProperty("has_change_pct");
    }
    // Harness output must not contain investment advice
    const json = JSON.stringify(body);
    expect(json).not.toMatch(/买入|卖出|加仓|减仓|建议扣款|目标价/);
  });

  test("GET /api/portfolio/harness on empty DB returns zero holdings", async () => {
    const { status, body } = await get("/api/portfolio/harness");
    expect(status).toBe(200);
    expect(body.decision_boundary).toBe("facts_only");
    expect(body.holdings_count).toBe(0);
    expect(body.holding_signals).toEqual([]);
    expect(body.total_value).toBe(0);
  });

  // 6. GET /api/portfolio/source-brief?limit=5
  test("GET /api/portfolio/source-brief?limit=5 returns source queries", async () => {
    seedPortfolioData();
    const { status, body } = await get("/api/portfolio/source-brief?limit=5");
    expect(status).toBe(200);
    expect(body.decision_boundary).toBe("source_queries_only");
    expect(body).toHaveProperty("generated_at");
    expect(body).toHaveProperty("queries");
    expect(body).toHaveProperty("source_targets");
    expect(body).toHaveProperty("coverage");
    expect(body).toHaveProperty("agent_brief");
    expect(Array.isArray(body.queries)).toBe(true);
    expect(Array.isArray(body.source_targets)).toBe(true);
    // At minimum the global-market query is always present
    expect(body.queries.length).toBeGreaterThan(0);
    expect(body.queries.some((q: any) => q.scope === "portfolio")).toBe(true);
    // Output must not contain investment advice
    const json = JSON.stringify(body);
    expect(json).not.toMatch(/买入|卖出|加仓|减仓|建议|推荐|目标价/);
  });

  test("GET /api/portfolio/source-brief on empty DB still returns global-market query", async () => {
    const { status, body } = await get("/api/portfolio/source-brief?limit=3");
    expect(status).toBe(200);
    expect(body.decision_boundary).toBe("source_queries_only");
    expect(body.queries.length).toBeGreaterThanOrEqual(1);
    expect(body.queries[0].scope).toBe("portfolio");
  });
});
