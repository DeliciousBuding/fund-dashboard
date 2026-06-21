/** Fund Dashboard API integration tests — run: bun test
 *
 *  Requires the server running on http://127.0.0.1:8765
 *  Start: bun main.ts   (from packages/server/)
 *
 *  Current data snapshot (June 2026): 448 transactions, 70 funds, 19 held positions.
 *  Tests use soft/greaterThan assertions where exact counts drift over time;
 *  critical invariants (code/name match, shape contracts, status codes) use strict assertions.
 */

import { describe, test, expect } from "bun:test";

const BASE = "http://127.0.0.1:8765";

async function get(path: string) {
  const r = await fetch(`${BASE}${path}`);
  return { status: r.status, body: await r.json().catch(() => null) };
}

async function post(path: string, data?: any) {
  const r = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: data ? JSON.stringify(data) : undefined,
  });
  return { status: r.status, body: await r.json().catch(() => null) };
}

// ═══════════════════════════════════════════════════════════════════════
//  HEALTH & META
// ═══════════════════════════════════════════════════════════════════════

describe("health & meta", () => {
  test("GET /api/health returns ok", async () => {
    const { status, body } = await get("/api/health");
    expect(status).toBe(200);
    expect(body.status).toBe("ok");
    expect(body.uptime).toBeGreaterThan(0);
  });

  test("GET /api/summary returns by-fund aggregate", async () => {
    const { status, body } = await get("/api/summary");
    expect(status).toBe(200);
    expect(Array.isArray(body)).toBe(true);
    // summary_by_fund table may be empty on a fresh DB; validate shape when populated
    if (body.length > 0) {
      expect(body[0]).toHaveProperty("fund_code");
      expect(body[0]).toHaveProperty("fund_name");
    }
  });
});

// ═══════════════════════════════════════════════════════════════════════
//  PORTFOLIO
// ═══════════════════════════════════════════════════════════════════════

describe("portfolio", () => {
  test("GET /api/portfolio returns full summary", async () => {
    const { status, body } = await get("/api/portfolio");
    expect(status).toBe(200);
    expect(body.total_tx).toBeGreaterThan(440);
    expect(body.held_funds).toBeGreaterThan(10);
    expect(body).toHaveProperty("unique_funds");
    expect(body).toHaveProperty("unique_stocks");
    expect(body.total_buy).toBeGreaterThan(0);
    expect(body.total_sell).toBeGreaterThan(0);
    expect(body.total_fee).toBeGreaterThan(0);
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
    if (body.by_security_type.length > 0) {
      expect(body.by_security_type[0]).toHaveProperty("security_type");
      expect(body.by_security_type[0]).toHaveProperty("count");
      expect(body.by_security_type[0]).toHaveProperty("total_value");
      expect(body.by_security_type[0]).toHaveProperty("total_pnl");
    }
  });

  test("GET /api/portfolio/xirr returns percentage", async () => {
    const { status, body } = await get("/api/portfolio/xirr");
    expect(status).toBe(200);
    expect(body).toHaveProperty("xirr");
    // xirr can be null if not enough cashflows, or a number
    if (body.xirr !== null) {
      expect(typeof body.xirr).toBe("number");
    }
  });

  test("GET /api/portfolio/timeline returns daily array", async () => {
    const { status, body } = await get("/api/portfolio/timeline");
    expect(status).toBe(200);
    expect(Array.isArray(body)).toBe(true);
    expect(body.length).toBeGreaterThan(100);
    expect(body[0]).toHaveProperty("date");
    expect(body[0]).toHaveProperty("total_value");
    expect(body[0]).toHaveProperty("total_cost");
    expect(body[0]).toHaveProperty("pnl");
    expect(body[0]).toHaveProperty("pnl_pct");
  });

  test("GET /api/portfolio/penetration returns penetration analysis", async () => {
    const { status, body } = await get("/api/portfolio/penetration");
    expect(status).toBe(200);
    expect(body).toHaveProperty("penetration");
    expect(body).toHaveProperty("total_portfolio_value");
    expect(body).toHaveProperty("equity_fund_count");
    expect(body).toHaveProperty("unique_stocks");
    expect(Array.isArray(body.penetration)).toBe(true);
    if (body.penetration.length > 0) {
      const entry = body.penetration[0];
      expect(entry).toHaveProperty("stock_code");
      expect(entry).toHaveProperty("stock_name");
      expect(entry).toHaveProperty("total_exposure_cny");
      expect(entry).toHaveProperty("held_by_funds");
      expect(entry).toHaveProperty("weight_pct");
      expect(Array.isArray(entry.held_by_funds)).toBe(true);
      if (entry.held_by_funds.length > 0) {
        expect(entry.held_by_funds[0]).toHaveProperty("fund_code");
        expect(entry.held_by_funds[0]).toHaveProperty("fund_name");
        expect(entry.held_by_funds[0]).toHaveProperty("weight_pct");
        expect(entry.held_by_funds[0]).toHaveProperty("fund_value_cny");
      }
    }
  });

  test("GET /api/portfolio/by-type returns security type breakdown", async () => {
    const { status, body } = await get("/api/portfolio/by-type");
    expect(status).toBe(200);
    expect(Array.isArray(body)).toBe(true);
    if (body.length > 0) {
      expect(body[0]).toHaveProperty("security_type");
      expect(body[0]).toHaveProperty("count");
      expect(body[0]).toHaveProperty("total_value");
      expect(body[0]).toHaveProperty("total_pnl");
    }
  });
});

// ═══════════════════════════════════════════════════════════════════════
//  FUNDS / SECURITIES
// ═══════════════════════════════════════════════════════════════════════

describe("funds & securities", () => {
  test("GET /api/funds lists all securities (funds + stocks)", async () => {
    const { status, body } = await get("/api/funds");
    expect(status).toBe(200);
    expect(Array.isArray(body)).toBe(true);
    expect(body.length).toBeGreaterThan(60);
    expect(body[0]).toHaveProperty("code");
    expect(body[0]).toHaveProperty("name");
    expect(body[0]).toHaveProperty("security_type");
    expect(body[0]).toHaveProperty("market");
    expect(body[0]).toHaveProperty("held_shares");
  });

  test("GET /api/funds/:code returns fund detail with transactions", async () => {
    const { status, body } = await get("/api/funds/019173");
    expect(status).toBe(200);
    expect(body.code).toBe("019173");
    expect(body.name).toContain("纳斯达克");
    expect(body.transactions.length).toBeGreaterThan(100);
    expect(body).toHaveProperty("held_shares");
    expect(body).toHaveProperty("total_cost");
    expect(body).toHaveProperty("latest_nav");
    expect(body).toHaveProperty("current_value");
    expect(body).toHaveProperty("unrealized_pnl");
    expect(body).toHaveProperty("pnl_pct");
    expect(body).toHaveProperty("auto_buy_count");
    expect(body).toHaveProperty("manual_buy_count");
    expect(body).toHaveProperty("buy_count");
    expect(body).toHaveProperty("sell_count");
  });

  test("GET /api/funds/:code/nav returns price history", async () => {
    const { status, body } = await get("/api/funds/019173/nav");
    expect(status).toBe(200);
    expect(Array.isArray(body)).toBe(true);
    expect(body.length).toBeGreaterThan(500);
    expect(body[0]).toHaveProperty("date");
    expect(body[0]).toHaveProperty("unit_nav");
  });

  test("GET /api/funds/:code/xirr returns fund-level XIRR", async () => {
    const { status, body } = await get("/api/funds/019173/xirr");
    expect(status).toBe(200);
    expect(body).toHaveProperty("xirr");
    expect(body).toHaveProperty("code");
    expect(body.code).toBe("019173");
  });

  test("GET /api/funds/:code/drawdown returns max drawdown", async () => {
    const { status, body } = await get("/api/funds/019173/drawdown");
    expect(status).toBe(200);
    expect(body).toHaveProperty("max_drawdown");
    expect(body).toHaveProperty("peak_date");
    expect(body).toHaveProperty("trough_date");
    expect(body).toHaveProperty("code");
    expect(body.code).toBe("019173");
  });

  test("GET /api/funds/:code/dca returns DCA recommendation for held fund", async () => {
    const { status, body } = await get("/api/funds/019173/dca");
    expect(status).toBe(200);
    expect(body.fund_code).toBe("019173");
    expect(body).toHaveProperty("base_amount");
    expect(body).toHaveProperty("latest_nav");
    expect(body).toHaveProperty("cost_per_share");
    expect(body).toHaveProperty("deviation_pct");
    expect(body).toHaveProperty("dca_rate");
    expect(body).toHaveProperty("actual_amount");
    expect(body).toHaveProperty("range");
    // range is Chinese label: 加仓 / 减仓 / 正常
    expect(["加仓", "减仓", "正常"]).toContain(body.range);
  });

  test("GET /api/funds/:code/dca with base param", async () => {
    const { status, body } = await get("/api/funds/019173/dca?base=50");
    expect(status).toBe(200);
    expect(body.fund_code).toBe("019173");
    expect(body.base_amount).toBe(50);
  });

  test("GET /api/funds/:code/dca returns 400 for unheld fund", async () => {
    const { status, body } = await get("/api/funds/000218/dca");
    expect(status).toBe(400);
    expect(body.error).toBe("no_position");
  });

  test("GET /api/funds/999999 returns 404", async () => {
    const { status, body } = await get("/api/funds/999999");
    expect(status).toBe(404);
    expect(body.error).toBe("not found");
  });

  test("GET /api/funds/NONEXIST returns 404", async () => {
    const { status, body } = await get("/api/funds/NONEXIST");
    expect(status).toBe(404);
    expect(body.error).toBe("not found");
  });

  test("GET /api/funds/019173 includes settlement_days and anomaly in transaction shape", async () => {
    const { status, body } = await get("/api/funds/019173");
    expect(status).toBe(200);
    expect(body.transactions.length).toBeGreaterThan(0);
    const tx = body.transactions[0];
    expect(tx).toHaveProperty("settlement_days");
    expect(tx).toHaveProperty("anomaly");
    expect(tx).toHaveProperty("nav_verified");
    expect(tx).toHaveProperty("trade_day_type");
    expect(tx).toHaveProperty("effective_nav_date");
    expect(tx).toHaveProperty("inferred_nav");
    expect(tx).toHaveProperty("nav");
    expect(tx).toHaveProperty("order_id");
  });
});

// ═══════════════════════════════════════════════════════════════════════
//  MARKET INDICES
// ═══════════════════════════════════════════════════════════════════════

describe("market indices", () => {
  test("GET /api/market/indices returns cached index data", async () => {
    const { status, body } = await get("/api/market/indices");
    expect(status).toBe(200);
    expect(Array.isArray(body)).toBe(true);
    if (body.length > 0) {
      const idx = body[0];
      expect(idx).toHaveProperty("code");
      expect(idx).toHaveProperty("name");
      expect(idx).toHaveProperty("market");
      expect(idx).toHaveProperty("price");
      expect(idx).toHaveProperty("change_pct");
      expect(idx).toHaveProperty("change_amt");
      expect(idx).toHaveProperty("updated_at");
    }
  });

  test("GET /api/market/index/:code returns live/cached single index", async () => {
    // ^GSPC = S&P 500; live fetch may fail in CI, falls back to cache
    const { status, body } = await get("/api/market/index/%5EGSPC");
    // Accept 200 (live/cache), 404 (no data), 502 (fetch fail + no cache)
    expect([200, 404, 502]).toContain(status);
    if (status === 200) {
      expect(body).toHaveProperty("code");
      expect(body).toHaveProperty("price");
    }
  });

  test("GET /api/market/index/^NDX returns Nasdaq data", async () => {
    const { status, body } = await get("/api/market/index/%5ENDX");
    expect([200, 404, 502]).toContain(status);
    if (status === 200) {
      expect(body).toHaveProperty("code");
      expect(body).toHaveProperty("name");
      expect(body).toHaveProperty("price");
      expect(body).toHaveProperty("change_pct");
    }
  });

  test("GET /api/market/index/^GSPC/history returns index history", async () => {
    const { status, body } = await get("/api/market/index/%5EGSPC/history");
    // May return 404 if no history data, 502 on fetch failure, or 200
    expect([200, 404, 502]).toContain(status);
    if (status === 200) {
      expect(body).toHaveProperty("symbol");
      expect(body).toHaveProperty("count");
      expect(body).toHaveProperty("range");
      expect(body).toHaveProperty("data");
      expect(Array.isArray(body.data)).toBe(true);
      if (body.data.length > 0) {
        expect(body.data[0]).toHaveProperty("date");
        expect(body.data[0]).toHaveProperty("close");
        expect(body.data[0]).toHaveProperty("change_pct");
      }
    }
  });
});

// ═══════════════════════════════════════════════════════════════════════
//  STOCKS
// ═══════════════════════════════════════════════════════════════════════

describe("stocks", () => {
  test("GET /api/stocks/AAPL returns US stock detail", async () => {
    const { status, body } = await get("/api/stocks/AAPL");
    // Live fetch may fail in CI; accept 200/404/502
    expect([200, 404, 502]).toContain(status);
    if (status === 200) {
      expect(body.code).toBe("AAPL");
      expect(body.market).toBe("US");
      expect(body).toHaveProperty("price");
      expect(body).toHaveProperty("name");
      expect(body).toHaveProperty("source"); // "live" or "cache"
    }
  });
});

// ═══════════════════════════════════════════════════════════════════════
//  ADMIN (requires auth if MCP_API_KEY is set)
// ═══════════════════════════════════════════════════════════════════════

describe("admin", () => {
  test("GET /api/admin/status returns system diagnostics", async () => {
    const { status, body } = await get("/api/admin/status");
    expect(status).toBe(200);
    expect(body.ok).toBe(true);
    expect(body.transactions.count).toBeGreaterThan(440);
    expect(body).toHaveProperty("uptime_sec");
    expect(body).toHaveProperty("response_ms");
    expect(body).toHaveProperty("nav");
    expect(body.nav).toHaveProperty("count");
    expect(body.nav).toHaveProperty("funds");
    expect(body).toHaveProperty("portfolio");
    expect(body.portfolio).toHaveProperty("held_funds");
    expect(body).toHaveProperty("securities");
    expect(body.securities).toHaveProperty("total");
    expect(body.securities).toHaveProperty("funds");
    expect(body.securities).toHaveProperty("stocks");
    expect(body).toHaveProperty("anomalies");
    expect(body.anomalies).toHaveProperty("count");
    expect(Array.isArray(body.anomalies.items)).toBe(true);
  });

  test("GET /api/admin/status/:code returns per-fund diagnostics", async () => {
    const { status, body } = await get("/api/admin/status/019173");
    expect(status).toBe(200);
    expect(body.code).toBe("019173");
    expect(body).toHaveProperty("name");
    expect(body).toHaveProperty("security_type");
    expect(body).toHaveProperty("transactions");
    expect(body.transactions).toHaveProperty("n");
    expect(body.transactions).toHaveProperty("first");
    expect(body.transactions).toHaveProperty("last");
    expect(body).toHaveProperty("nav");
    expect(body.nav).toHaveProperty("n");
    expect(body).toHaveProperty("position");
    expect(body.position).toHaveProperty("held_shares");
    expect(body).toHaveProperty("trading");
  });

  test("GET /api/admin/verify returns diagnostic issues", async () => {
    const { status, body } = await get("/api/admin/verify");
    expect(status).toBe(200);
    expect(body).toHaveProperty("ok");
    expect(Array.isArray(body.issues)).toBe(true);
  });

  test("GET /api/admin/db-integrity returns integrity report", async () => {
    const { status, body } = await get("/api/admin/db-integrity");
    expect(status).toBe(200);
    expect(body).toHaveProperty("checks");
  });

  test("GET /api/admin/backup-status returns backup health", async () => {
    const { status, body } = await get("/api/admin/backup-status");
    expect(status).toBe(200);
    expect(body).toHaveProperty("status");
    expect(body).toHaveProperty("path");
  });

  test("POST /api/admin/crawl-nav triggers NAV crawl for a fund", async () => {
    const { status, body } = await post("/api/admin/crawl-nav", { code: "019173" });
    expect(status).toBe(200);
    expect(body).toHaveProperty("code");
    expect(body.code).toBe("019173");
    expect(body).toHaveProperty("added");
  });

  test("POST /api/admin/crawl-nav without code starts background crawl", async () => {
    const { status, body } = await post("/api/admin/crawl-nav", {});
    expect(status).toBe(200);
    if (body.status) {
      expect(body.status).toBe("started");
    } else {
      expect(body).toHaveProperty("total");
    }
  });

  test("POST /api/admin/recalculate-snapshot recalculates", async () => {
    const { status, body } = await post("/api/admin/recalculate-snapshot");
    expect(status).toBe(200);
    expect(body.ok).toBe(true);
    expect(body.funds).toBeGreaterThan(0);
  });
});

// ═══════════════════════════════════════════════════════════════════════
//  ROUTING CONSISTENCY
// ═══════════════════════════════════════════════════════════════════════

describe("routing consistency", () => {
  test("/api/securities is an alias of /api/funds", async () => {
    const funds = await get("/api/funds");
    const secs = await get("/api/securities");
    expect(funds.status).toBe(200);
    expect(secs.status).toBe(200);
    expect(Array.isArray(funds.body)).toBe(true);
    expect(Array.isArray(secs.body)).toBe(true);
    // Both return same length (same underlying query)
    expect(secs.body.length).toBe(funds.body.length);
    expect(secs.body[0]).toHaveProperty("security_type");
  });

  test("/api/status redirects to /api/admin/status", async () => {
    const r = await fetch(`${BASE}/api/status`, { redirect: "manual" });
    expect(r.status).toBe(302);
    expect(r.headers.get("location")).toEndWith("/api/admin/status");
  });

  test("POST /api/mcp without auth returns 401", async () => {
    const r = await fetch(`${BASE}/api/mcp`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ method: "tools/list" }),
    });
    expect(r.status).toBe(401);
  });
});
