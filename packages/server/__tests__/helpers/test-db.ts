/**
 * Shared test DB helpers for API integration tests.
 *
 * Provides an in-memory SQLite database with mocked db.ts + crawler/nav.ts,
 * plus setup/teardown/seed utilities used by all split test files.
 *
 * mock.module specifiers use "../../db" and "../../crawler/nav" because
 * this file lives at __tests__/helpers/ — two levels below packages/server/.
 */

import { Database } from "bun:sqlite";
import { mock } from "bun:test";

// ═══════════════════════════════════════════════════════════════════════
// In-memory test DB
// ═══════════════════════════════════════════════════════════════════════

const memDb = new Database(":memory:");

// ═══════════════════════════════════════════════════════════════════════
// Schema DDL — extracted so both mock.factory and initTestDb() can use it
// ═══════════════════════════════════════════════════════════════════════

function runSchema(d: Database) {
  d.run(`CREATE TABLE IF NOT EXISTS fund_details (
    fund_code TEXT PRIMARY KEY, fund_name TEXT, fund_type TEXT,
    security_type TEXT DEFAULT 'fund', market TEXT DEFAULT '',
    currency TEXT DEFAULT 'CNY', exchange TEXT DEFAULT ''
  )`);

  d.run(`CREATE TABLE IF NOT EXISTS transactions (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id TEXT UNIQUE, trade_time TEXT, confirm_date TEXT,
    trade_type TEXT, direction TEXT, fund_code TEXT, fund_name TEXT,
    confirm_amount REAL, confirm_share REAL, fee REAL DEFAULT 0,
    inferred_nav REAL, nav_on_effective_date REAL, nav_verified INTEGER DEFAULT 0,
    signed_cash_flow REAL, signed_share_change REAL,
    trade_day_type TEXT, settlement_days INTEGER, effective_nav_date TEXT,
    latest_nav REAL, cost_basis REAL, unrealized_pnl REAL DEFAULT 0,
    anomaly TEXT
  )`);

  d.run(`CREATE TABLE IF NOT EXISTS nav_history (
    date TEXT, fund_code TEXT, unit_nav REAL, daily_change_pct REAL DEFAULT 0,
    security_type TEXT DEFAULT 'fund',
    PRIMARY KEY (fund_code, date)
  )`);

  d.run(`CREATE TABLE IF NOT EXISTS portfolio_snapshot (
    fund_code TEXT PRIMARY KEY, fund_name TEXT,
    held_shares REAL, total_cost REAL, latest_nav REAL,
    current_value REAL, unrealized_pnl REAL, pnl_pct REAL,
    security_type TEXT DEFAULT 'fund',
    portfolio_id INTEGER DEFAULT 1
  )`);

  d.run(`CREATE TABLE IF NOT EXISTS portfolio_definitions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
  )`);
  d.run("INSERT OR IGNORE INTO portfolio_definitions (id, name, description) VALUES (1, 'default', 'Default portfolio')");

  d.run(`CREATE TABLE IF NOT EXISTS fund_holdings (
    fund_code TEXT, stock_code TEXT, stock_name TEXT,
    weight_pct REAL, shares REAL, market_value REAL,
    report_date TEXT,
    PRIMARY KEY (fund_code, stock_code, report_date)
  )`);

  d.run(`CREATE TABLE IF NOT EXISTS indices (
    code TEXT PRIMARY KEY, name TEXT, market TEXT,
    price REAL, change_pct REAL, change_amt REAL,
    updated_at TEXT DEFAULT (datetime('now'))
  )`);

  d.run(`CREATE TABLE IF NOT EXISTS source_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL, url TEXT,
    source TEXT NOT NULL DEFAULT 'websearch',
    snippet TEXT, query TEXT,
    related_security_code TEXT, related_security_name TEXT,
    is_read INTEGER DEFAULT 0, is_useful INTEGER DEFAULT 0,
    fetched_at TEXT NOT NULL DEFAULT (datetime('now')),
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
  )`);

  d.run(`CREATE TABLE IF NOT EXISTS summary_by_fund (
    fund_code TEXT PRIMARY KEY, fund_name TEXT,
    total_shares REAL, total_cost REAL, tx_count INTEGER
  )`);

  d.run(`CREATE TABLE IF NOT EXISTS fund_status (
    fund_code TEXT PRIMARY KEY, purchase_status TEXT, redemption_status TEXT
  )`);
}

// ═══════════════════════════════════════════════════════════════════════
// Mock db.ts — same pattern as __tests__/services/portfolio.test.ts
// Specifier "../../db" from __tests__/helpers/ resolves to packages/server/db
// ═══════════════════════════════════════════════════════════════════════

mock.module("../../db", () => {
  function q(sql: string, ...params: any[]) {
    return memDb.query(sql).all(...params) as any[];
  }
  function qOne(sql: string, ...params: any[]) {
    return memDb.query(sql).get(...params) as any;
  }
  return {
    getDb: () => memDb,
    getRwDb: () => memDb,
    query: q,
    queryOne: qOne,
    initSchema: (db?: Database) => {
      runSchema(db || memDb);
    },
  };
});

// ═══════════════════════════════════════════════════════════════════════
// Mock crawler/nav.ts — prevent real HTTP calls when importing admin routes
// ═══════════════════════════════════════════════════════════════════════

mock.module("../../crawler/nav", () => ({
  refreshFundNav: async (code: string) => ({ code, added: 0 }),
  refreshAllHeld: async () => ({ added: 5, total: 10 }),
}));

// ═══════════════════════════════════════════════════════════════════════
// Public API
// ═══════════════════════════════════════════════════════════════════════

/** Initialize DB schema (idempotent — all tables are CREATE IF NOT EXISTS). */
export function initTestDb(): void {
  runSchema(memDb);
}

/** Return the shared in-memory Database instance. */
export function getTestDb(): Database {
  return memDb;
}

/** Delete all rows from every table (called in beforeEach). */
export function clearTestDb(): void {
  const tables = [
    "transactions", "nav_history", "portfolio_snapshot",
    "summary_by_fund", "fund_details", "fund_holdings",
    "source_events", "fund_status", "indices",
  ];
  for (const t of tables) {
    try { memDb.run(`DELETE FROM ${t}`); } catch { /* table may not exist yet */ }
  }
}

/** Seed the DB with a realistic multi-fund portfolio for tests that need data. */
export function seedPortfolioData(): void {
  memDb.run(`INSERT OR REPLACE INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES
    ('019173', '纳斯达克100指数(QDII)C', 'QDII-股票', 'fund', 'CN'),
    ('018439', '国泰纳斯达克100ETF联接C', 'QDII-ETF联接', 'fund', 'CN'),
    ('AAPL', 'Apple Inc.', '科技股', 'stock', 'US')`);

  memDb.run(`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
    VALUES
    ('TX001', '2024-06-01T09:00:00Z', '2024-06-02', '定投买入', 'buy', '019173', '纳斯达克100指数(QDII)C', 100, 85.47, 0.15, -100, 85.47, 2),
    ('TX002', '2024-07-15T09:00:00Z', '2024-07-16', '用户买入', 'buy', '019173', '纳斯达克100指数(QDII)C', 200, 166.67, 0.30, -200, 166.67, 2),
    ('TX003', '2025-03-20T09:00:00Z', '2025-03-21', '用户卖出', 'sell', '019173', '纳斯达克100指数(QDII)C', 80, 55.17, 0.12, 80, -55.17, 3),
    ('TX004', '2024-06-01T09:00:00Z', '2024-06-02', '定投买入', 'buy', '018439', '国泰纳斯达克100ETF联接C', 100, 90.91, 0.15, -100, 90.91, 2)`);

  memDb.run(`INSERT INTO nav_history (date, fund_code, unit_nav, daily_change_pct) VALUES
    ('2024-06-01', '019173', 1.1700, 0.5),
    ('2025-05-01', '019173', 1.3500, -2.1),
    ('2026-06-18', '019173', 1.5000, 3.5),
    ('2025-01-05', '018439', 1.3800, 0.8)`);

  memDb.run(`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type) VALUES
    ('019173', '纳斯达克100指数(QDII)C', 196.97, -220, 1.5000, 295.46, 75.46, 34.30, 'fund'),
    ('018439', '国泰纳斯达克100ETF联接C', 169.65, -200, 1.3800, 234.12, 34.12, 17.06, 'fund')`);
}
