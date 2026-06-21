/**
 * Fund Dashboard API Integration Tests — Source Events API
 *
 * Run: npx bun test packages/server/__tests__/api-integration-source-events.test.ts
 *
 * Tests CRUD endpoints under /api/portfolio/source-events:
 * create (POST), list (GET), filter by code, mark as read (PATCH).
 *
 * Uses shared test-db.ts helper for in-memory SQLite + mocks.
 * No real DB, no network, no side effects.
 */

import { describe, test, expect, beforeEach } from "bun:test";
import { Hono } from "hono";
import { ApiError } from "../utils/errors";
import { initTestDb, getTestDb, clearTestDb } from "./helpers/test-db";

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
// SOURCE EVENTS — 8 tests
// ═══════════════════════════════════════════════════════════════════════

describe("Source Events API", () => {
  // 1. POST /api/portfolio/source-events → create event returns 201 + id
  test("POST /api/portfolio/source-events creates an event and returns 201", async () => {
    const { status, body } = await post("/api/portfolio/source-events", {
      title: "纳斯达克100 ETF 资金流入创纪录",
      url: "https://example.com/nasdaq-inflow",
      source: "websearch",
      snippet: "纳斯达克100相关的QDII ETF本周资金净流入达到...",
      query: "纳斯达克 QDII 资金流向",
      related_security_code: "019173",
      related_security_name: "纳斯达克100指数(QDII)C",
    });
    expect(status).toBe(201);
    expect(body.id).toBeGreaterThan(0);
    expect(body.title).toBe("纳斯达克100 ETF 资金流入创纪录");
    expect(body.source).toBe("websearch");
    expect(body.is_read).toBe(0);
    expect(body.is_useful).toBe(0);
    expect(body.related_security_code).toBe("019173");
    expect(body.fetched_at).toBeTruthy();
    expect(body.created_at).toBeTruthy();
  });

  test("POST /api/portfolio/source-events without title returns 400", async () => {
    const { status, body } = await post("/api/portfolio/source-events", {
      source: "websearch",
      snippet: "missing title",
    });
    expect(status).toBe(400);
    expect(body.error).toBe("title is required");
  });

  // 2. GET /api/portfolio/source-events → returns unread events list
  test("GET /api/portfolio/source-events returns unread events", async () => {
    // Create two events
    await post("/api/portfolio/source-events", {
      title: "Event Alpha",
      source: "websearch",
      snippet: "alpha",
      related_security_code: "AAPL",
    });
    await post("/api/portfolio/source-events", {
      title: "Event Beta",
      source: "eastmoney",
      snippet: "beta",
      related_security_code: "00700",
    });

    const { status, body } = await get("/api/portfolio/source-events");
    expect(status).toBe(200);
    expect(Array.isArray(body)).toBe(true);
    expect(body.length).toBe(2);
    // All returned events are unread
    for (const ev of body) {
      expect(ev.is_read).toBe(0);
    }
    // Shape check
    expect(body[0]).toHaveProperty("id");
    expect(body[0]).toHaveProperty("title");
    expect(body[0]).toHaveProperty("source");
    expect(body[0]).toHaveProperty("fetched_at");
    // Events output must not contain investment advice
    const json = JSON.stringify(body);
    expect(json).not.toMatch(/买入|卖出|加仓|减仓|建议|推荐|目标价/);
  });

  // 3. GET /api/portfolio/source-events?code=AAPL → filter by code
  test("GET /api/portfolio/source-events?code=AAPL filters by security code", async () => {
    await post("/api/portfolio/source-events", {
      title: "Apple Earnings",
      source: "websearch",
      snippet: "Apple Q3 earnings...",
      related_security_code: "AAPL",
      related_security_name: "Apple Inc.",
    });
    await post("/api/portfolio/source-events", {
      title: "Tencent Gaming",
      source: "websearch",
      snippet: "Tencent gaming revenue...",
      related_security_code: "00700",
      related_security_name: "腾讯控股",
    });

    const { status, body } = await get("/api/portfolio/source-events?code=AAPL");
    expect(status).toBe(200);
    expect(Array.isArray(body)).toBe(true);
    expect(body.length).toBe(1);
    expect(body[0].title).toContain("Apple");
    expect(body[0].related_security_code).toBe("AAPL");
  });

  test("GET /api/portfolio/source-events?code=NOTFOUND returns empty", async () => {
    await post("/api/portfolio/source-events", {
      title: "Some Event",
      source: "test",
      related_security_code: "123456",
    });
    const { status, body } = await get("/api/portfolio/source-events?code=NOTFOUND");
    expect(status).toBe(200);
    expect(body).toEqual([]);
  });

  // 4. PATCH /api/portfolio/source-events/:id → mark as read
  test("PATCH /api/portfolio/source-events/:id marks event as read", async () => {
    const create = await post("/api/portfolio/source-events", {
      title: "Test Event",
      source: "test",
      snippet: "mark me read",
    });
    expect(create.status).toBe(201);
    const eventId = create.body.id;

    // Mark as read + useful
    const { status, body } = await patch(`/api/portfolio/source-events/${eventId}`, {
      is_read: true,
      is_useful: true,
    });
    expect(status).toBe(200);
    expect(body.ok).toBe(true);
    expect(body.id).toBe(eventId);

    // Verify it's no longer in unread list
    const unread = await get("/api/portfolio/source-events");
    expect(unread.body.length).toBe(0);

    // But it appears when show_read=true
    const all = await get("/api/portfolio/source-events?show_read=1");
    expect(all.body.length).toBe(1);
    expect(all.body[0].is_read).toBe(1);
    expect(all.body[0].is_useful).toBe(1);
  });

  test("PATCH /api/portfolio/source-events/:id with invalid id returns 404", async () => {
    const { status, body } = await patch("/api/portfolio/source-events/99999", {
      is_read: true,
    });
    expect(status).toBe(404);
    expect(body.error).toBe("not found or no fields to update");
  });

  test("PATCH /api/portfolio/source-events/:id with non-numeric id returns 400", async () => {
    const { status, body } = await patch("/api/portfolio/source-events/abc", {
      is_read: true,
    });
    expect(status).toBe(400);
    expect(body.error).toBe("invalid id");
  });
});
