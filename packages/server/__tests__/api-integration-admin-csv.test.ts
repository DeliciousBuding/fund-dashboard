/**
 * Fund Dashboard API Integration Tests — Admin CSV Import
 *
 * Run: npx bun test packages/server/__tests__/api-integration-admin-csv.test.ts
 *
 * Tests CSV import endpoint: English / Chinese / mixed headers,
 * and error handling (missing field, no data, bad columns).
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
// ADMIN CSV IMPORT — 6 tests
// ═══════════════════════════════════════════════════════════════════════

describe("Admin API — CSV Import", () => {
  // 1. POST /api/admin/import-csv — English column names
  test("POST /api/admin/import-csv imports CSV with English column headers", async () => {
    const csv = [
      "date,code,name,direction,amount,share,fee,type",
      "2024-01-15,019173,纳斯达克100指数,buy,500,425.17,0.75,定投买入",
      "2024-02-20,018439,国泰纳斯达克ETF联接,buy,300,272.73,0.45,用户买入",
      "2024-03-10,019173,纳斯达克100指数,sell,200,138.89,0.30,用户卖出",
    ].join("\n");

    const { status, body } = await post("/api/admin/import-csv", { csv });
    expect(status).toBe(200);
    expect(body.ok).toBe(true);
    expect(body.imported).toBe(3);
    expect(body.total).toBe(3);

    // Verify transactions were inserted
    const count = memDb.query("SELECT COUNT(*) as n FROM transactions").get() as any;
    expect(count.n).toBe(3);
  });

  // 2. POST /api/admin/import-csv — Chinese column names (中英文列名自动检测)
  test("POST /api/admin/import-csv auto-detects Chinese column headers", async () => {
    const csv = [
      "交易日期,代码,名称,方向,金额,份额,手续费,类型",
      "2024-01-15,019173,纳斯达克100指数,买入,500,425.17,0.75,定投买入",
      "2024-02-20,018439,国泰纳斯达克ETF联接,买入,300,272.73,0.45,用户买入",
      "2024-03-10,019173,纳斯达克100指数,卖出,200,138.89,0.30,用户卖出",
    ].join("\n");

    const { status, body } = await post("/api/admin/import-csv", { csv });
    expect(status).toBe(200);
    expect(body.ok).toBe(true);
    expect(body.imported).toBe(3);
    expect(body.total).toBe(3);

    // Verify correct directions were parsed
    const rows = memDb.query("SELECT direction, confirm_amount FROM transactions ORDER BY trade_time").all() as any[];
    expect(rows).toHaveLength(3);
    expect(rows[0].direction).toBe("buy");
    expect(rows[0].confirm_amount).toBe(500);
    expect(rows[2].direction).toBe("sell");
  });

  test("POST /api/admin/import-csv with mixed Chinese/English headers works", async () => {
    const csv = [
      "date,代码,name,方向,amount,份额,fee,type",
      "2024-04-01,019173,Nasdaq100,buy,150,100,0.2,定投买入",
    ].join("\n");

    const { status, body } = await post("/api/admin/import-csv", { csv });
    expect(status).toBe(200);
    expect(body.ok).toBe(true);
    expect(body.imported).toBe(1);
  });

  test("POST /api/admin/import-csv returns 400 when csv field is missing", async () => {
    const { status, body } = await post("/api/admin/import-csv", {});
    expect(status).toBe(400);
    expect(body.error).toBe("csv field required");
  });

  test("POST /api/admin/import-csv returns 400 when csv has no data rows", async () => {
    const { status, body } = await post("/api/admin/import-csv", { csv: "date,code,direction,amount" });
    expect(status).toBe(400);
    expect(body.error).toContain("header + at least 1 row");
  });

  test("POST /api/admin/import-csv returns 400 when required columns missing", async () => {
    const { status, body } = await post("/api/admin/import-csv", {
      csv: "col1,col2\nval1,val2",
    });
    expect(status).toBe(400);
    expect(body.error).toContain("CSV needs columns");
  });
});
