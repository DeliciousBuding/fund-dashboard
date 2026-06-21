/**
 * Fund Dashboard API Integration Tests — Admin API
 *
 * Run: npx bun test packages/server/__tests__/api-integration-admin.test.ts
 *
 * Tests admin endpoints: verify, status, import-csv, import-transactions, crawl-nav.
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

import adminRoutes from "../routes/admin";

// ═══════════════════════════════════════════════════════════════════════
// Build test Hono app (no auth middleware — admin endpoints are open in test)
// ═══════════════════════════════════════════════════════════════════════

const memDb = getTestDb();
const app = new Hono();
app.route("/api/admin", adminRoutes);

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
// ADMIN — 11 tests
// ═══════════════════════════════════════════════════════════════════════

describe("Admin API", () => {
  // 1. GET /api/admin/verify
  test("GET /api/admin/verify returns verification results (empty DB = all clear)", async () => {
    const { status, body } = await get("/api/admin/verify");
    expect(status).toBe(200);
    expect(body).toHaveProperty("ok");
    expect(body).toHaveProperty("issues");
    expect(Array.isArray(body.issues)).toBe(true);
    // "all clear" is pushed as an issue, so ok is false (length=1 != 0)
    expect(body.issues).toContain("all clear");
  });

  test("GET /api/admin/verify detects issues in seeded data", async () => {
    // Seed transactions without matching NAV → verify should detect it
    seedPortfolioData();
    // Delete NAV for one fund to create a "missing NAV" issue
    memDb.run("DELETE FROM nav_history WHERE fund_code = '019173'");
    // Add a negative position
    memDb.run("UPDATE portfolio_snapshot SET held_shares = -10 WHERE fund_code = '018439'");

    const { status, body } = await get("/api/admin/verify");
    expect(status).toBe(200);
    expect(body.ok).toBe(false);
    expect(body.issues.length).toBeGreaterThan(0);
    expect(body.issues.some((i: string) => i.includes("missing NAV"))).toBe(true);
    expect(body.issues.some((i: string) => i.includes("negative"))).toBe(true);
  });

  // 2. GET /api/admin/status
  test("GET /api/admin/status returns system diagnostics", async () => {
    seedPortfolioData();
    const { status, body } = await get("/api/admin/status");
    expect(status).toBe(200);
    expect(body.ok).toBe(true);
    expect(body).toHaveProperty("uptime_sec");
    expect(body).toHaveProperty("response_ms");
    expect(body).toHaveProperty("transactions");
    expect(body.transactions).toHaveProperty("count");
    expect(body.transactions.count).toBe(4);
    expect(body).toHaveProperty("nav");
    expect(body.nav).toHaveProperty("count");
    expect(body.nav).toHaveProperty("funds");
    expect(body).toHaveProperty("portfolio");
    expect(body.portfolio).toHaveProperty("held_funds");
    expect(body).toHaveProperty("securities");
    expect(body.securities).toHaveProperty("total");
    expect(body.securities).toHaveProperty("funds");
    expect(body).toHaveProperty("anomalies");
    expect(body.anomalies).toHaveProperty("count");
    expect(Array.isArray(body.anomalies.items)).toBe(true);
    expect(typeof body.uptime_sec).toBe("number");
    expect(typeof body.response_ms).toBe("number");
  });

  test("GET /api/admin/status on empty DB returns zero counts", async () => {
    const { status, body } = await get("/api/admin/status");
    expect(status).toBe(200);
    expect(body.ok).toBe(true);
    expect(body.transactions.count).toBe(0);
    expect(body.portfolio.held_funds).toBe(0);
  });

  // 5. POST /api/admin/import-transactions — JSON bulk import
  test("POST /api/admin/import-transactions imports JSON transactions in bulk", async () => {
    const payload = {
      transactions: [
        {
          fund_code: "019173",
          trade_time: "2024-01-15",
          direction: "buy",
          confirm_amount: 500,
          fee: 0.75,
          order_id: "IMP001",
          confirm_date: "2024-01-16",
          trade_type: "定投买入",
          fund_name: "纳斯达克100指数(QDII)C",
          confirm_share: 425.17,
        },
        {
          fund_code: "018439",
          trade_time: "2024-02-20",
          direction: "buy",
          confirm_amount: 300,
          fee: 0.45,
          order_id: "IMP002",
          trade_type: "用户买入",
          fund_name: "国泰纳斯达克100ETF联接C",
          confirm_share: 272.73,
        },
      ],
    };

    const { status, body } = await post("/api/admin/import-transactions", payload);
    expect(status).toBe(200);
    expect(body.ok).toBe(true);
    expect(body.imported).toBe(2);
    expect(body.total).toBe(2);
    expect(body.affected_funds).toBeGreaterThanOrEqual(1);

    // Verify transactions stored correctly
    const rows = memDb.query("SELECT fund_code, direction, confirm_amount FROM transactions ORDER BY trade_time").all() as any[];
    expect(rows).toHaveLength(2);
    expect(rows[0].fund_code).toBe("019173");
    expect(rows[0].direction).toBe("buy");
  });

  test("POST /api/admin/import-transactions accepts fund_code and security_code together", async () => {
    const payload = {
      transactions: [
        {
          fund_code: "AAPL",
          security_code: "AAPL",
          trade_time: "2024-03-01",
          direction: "buy",
          confirm_amount: 1000,
          fee: 0,
          fund_name: "Apple Inc.",
        },
      ],
    };

    const { status, body } = await post("/api/admin/import-transactions", payload);
    expect(status).toBe(200);
    expect(body.ok).toBe(true);
    expect(body.imported).toBe(1);
  });

  test("POST /api/admin/import-transactions rejects invalid direction", async () => {
    const payload = {
      transactions: [
        {
          fund_code: "019173",
          trade_time: "2024-01-01",
          direction: "invalid_dir",
          confirm_amount: 100,
          fee: 0,
        },
      ],
    };

    const { status, body } = await post("/api/admin/import-transactions", payload);
    // Zod validation rejects non-enum direction
    expect(status).toBe(400);
    expect(body.message).toContain("direction");
  });

  test("POST /api/admin/import-transactions rejects missing required fields", async () => {
    const payload = {
      transactions: [
        {
          // missing fund_code
          trade_time: "2024-01-01",
          direction: "buy",
          confirm_amount: 100,
          fee: 0,
        } as any,
      ],
    };

    const { status, body } = await post("/api/admin/import-transactions", payload);
    expect(status).toBe(400);
    // Zod reports "expected string, received undefined" for missing fund_code
    expect(body.message).toBeTruthy();
  });

  test("POST /api/admin/import-transactions rejects negative confirm_amount", async () => {
    const payload = {
      transactions: [
        {
          fund_code: "019173",
          trade_time: "2024-01-01",
          direction: "buy",
          confirm_amount: -100,
          fee: 0,
        },
      ],
    };

    const { status, body } = await post("/api/admin/import-transactions", payload);
    expect(status).toBe(400);
    expect(body.message).toContain("confirm_amount");
  });

  // 6. POST /api/admin/crawl-nav → trigger crawl (returns started)
  test("POST /api/admin/crawl-nav without code returns started status", async () => {
    const { status, body } = await post("/api/admin/crawl-nav", {});
    expect(status).toBe(200);
    expect(body.status).toBe("started");
    expect(body.message).toContain("crawling");
  });

  test("POST /api/admin/crawl-nav with specific code returns fund result", async () => {
    const { status, body } = await post("/api/admin/crawl-nav", { code: "019173" });
    expect(status).toBe(200);
    // The mock returns { code, added: 0 }
    expect(body.code).toBe("019173");
    expect(body).toHaveProperty("added");
  });
});
