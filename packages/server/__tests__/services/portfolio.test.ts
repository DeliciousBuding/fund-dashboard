/**
 * Portfolio Service Unit Tests
 *
 * Uses mock.module to replace db.ts with in-memory SQLite.
 */
import { describe, test, expect, mock, beforeAll, beforeEach, afterAll } from "bun:test";
import { Database } from "bun:sqlite";

// ── In-memory test DB ─────────────────────────────────────────────────

const memDb = new Database(":memory:");

// ── Mock db.ts BEFORE importing services ─────────────────────────────
// The mock specifier must match the import path used by services/portfolio.ts
// which imports from "../db"

mock.module("../../db", () => {
  function q(sql: string, ...params: any[]) {
    return memDb.query(sql).all(...params);
  }
  function qOne(sql: string, ...params: any[]) {
    return memDb.query(sql).get(...params);
  }
  return {
    getDb: () => memDb,
    getRwDb: () => memDb,
    query: q,
    queryOne: qOne,
    initSchema: (db?: Database) => {
      const d = db || memDb;
      d.run(`CREATE TABLE IF NOT EXISTS fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT, fund_type TEXT, security_type TEXT DEFAULT 'fund', market TEXT DEFAULT '', currency TEXT DEFAULT 'CNY', exchange TEXT DEFAULT '')`);
      d.run(`CREATE TABLE IF NOT EXISTS transactions (seq INTEGER PRIMARY KEY AUTOINCREMENT, order_id TEXT, trade_time TEXT, confirm_date TEXT, trade_type TEXT, direction TEXT, fund_code TEXT, fund_name TEXT, apply_amount REAL, apply_share REAL, confirm_amount REAL, confirm_share REAL, fee REAL, inferred_nav REAL, signed_cash_flow REAL, signed_share_change REAL, nav_on_effective_date REAL, settlement_days REAL, anomaly TEXT, unrealized_pnl REAL DEFAULT 0)`);
      d.run(`CREATE TABLE IF NOT EXISTS nav_history (fund_code TEXT, date TEXT, unit_nav REAL, daily_change_pct REAL DEFAULT 0, security_type TEXT DEFAULT 'fund')`);
      d.run(`CREATE TABLE IF NOT EXISTS portfolio_snapshot (fund_code TEXT PRIMARY KEY, fund_name TEXT, held_shares REAL, total_cost REAL, latest_nav REAL, nav_date TEXT, current_value REAL, unrealized_pnl REAL, pnl_pct REAL, buy_count INTEGER DEFAULT 0, sell_count INTEGER DEFAULT 0, auto_buy_count INTEGER DEFAULT 0, manual_buy_count INTEGER DEFAULT 0, auto_buy_amount REAL DEFAULT 0, manual_buy_amount REAL DEFAULT 0, median_settlement_days REAL, security_type TEXT DEFAULT 'fund', purchase_status TEXT, redemption_status TEXT, portfolio_id INTEGER DEFAULT 1)`);
      d.run(`CREATE TABLE IF NOT EXISTS portfolio_definitions (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, description TEXT DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now')))`);
      d.run("INSERT OR IGNORE INTO portfolio_definitions (id, name, description) VALUES (1, 'default', 'Default portfolio')");
      d.run(`CREATE TABLE IF NOT EXISTS summary_by_fund (fund_code TEXT PRIMARY KEY, fund_name TEXT, total_shares REAL, total_cost REAL, tx_count INTEGER)`);
      d.run(`CREATE TABLE IF NOT EXISTS fund_holdings (fund_code TEXT, stock_code TEXT, stock_name TEXT, weight_pct REAL, shares REAL, market_value REAL, report_date TEXT, PRIMARY KEY (fund_code, stock_code, report_date))`);
      d.run(`CREATE TABLE IF NOT EXISTS indices (code TEXT PRIMARY KEY, name TEXT, market TEXT, price REAL, change_pct REAL, change_amt REAL, updated_at TEXT)`);
      d.run(`CREATE TABLE IF NOT EXISTS source_events (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, url TEXT, source TEXT NOT NULL DEFAULT 'websearch', snippet TEXT, query TEXT, related_security_code TEXT, related_security_name TEXT, is_read INTEGER DEFAULT 0, is_useful INTEGER DEFAULT 0, fetched_at TEXT NOT NULL DEFAULT (datetime('now')), created_at TEXT NOT NULL DEFAULT (datetime('now')))`);
    },
  };
});

// ── NOW import services ──────────────────────────────────────────────

import { initSchema, getRwDb } from "../../db";
import {
  getPortfolioSummary, getPortfolioXirr, getPortfolioTimeline,
  getPortfolioPenetration, getFundDetail, getMaxDrawdown,
  getFundXirr, recalculateAllSnapshots, getSystemStatus,
  getPortfolioAllocation,
  getInvestmentHarnessSnapshot,
  getInvestmentSourceBrief,
  getSourceEvents,
  createSourceEvent,
  markSourceEventRead,
} from "../../services/index";

initSchema();

beforeEach(() => {
  const tables = ["transactions", "nav_history", "portfolio_snapshot", "summary_by_fund", "fund_details", "fund_holdings", "source_events"];
  for (const t of tables) {
    try { memDb.run(`DELETE FROM ${t}`); } catch {}
  }
});

// ═══════════════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════════════

function seedBaseData() {
  memDb.run(`INSERT OR REPLACE INTO fund_details (fund_code, fund_name, fund_type, security_type) VALUES
    ('019173', '纳斯达克100指数(QDII)C', 'QDII-股票', 'fund'),
    ('018439', '国泰纳斯达克100ETF联接C', 'QDII-ETF联接', 'fund')`);

  memDb.run(`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days, nav_on_effective_date)
    VALUES
    ('TX001', '2024-06-01T09:00:00Z', '2024-06-02', '定投买入', 'buy', '019173', '纳斯达克100指数(QDII)C', 100, 85.47, 0.15, -100, 85.47, 2, 1.1698),
    ('TX002', '2024-07-15T09:00:00Z', '2024-07-16', '用户买入', 'buy', '019173', '纳斯达克100指数(QDII)C', 200, 166.67, 0.30, -200, 166.67, 2, 1.1998),
    ('TX003', '2025-03-20T09:00:00Z', '2025-03-21', '用户卖出', 'sell', '019173', '纳斯达克100指数(QDII)C', 80, 55.17, 0.12, 80, -55.17, 3, 1.4498),
    ('TX006', '2024-06-01T09:00:00Z', '2024-06-02', '定投买入', 'buy', '018439', '国泰纳斯达克100ETF联接C', 100, 90.91, 0.15, -100, 90.91, 2, 1.0998),
    ('TX007', '2024-09-01T09:00:00Z', '2024-09-02', '定投买入', 'buy', '018439', '国泰纳斯达克100ETF联接C', 100, 78.74, 0.15, -100, 78.74, 2, 1.2698)`);

  memDb.run(`INSERT INTO nav_history (date, fund_code, unit_nav) VALUES
    ('2024-06-01', '019173', 1.1700),
    ('2024-08-01', '019173', 1.2500),
    ('2025-01-15', '019173', 1.5800),
    ('2025-03-15', '019173', 1.1000),
    ('2025-05-01', '019173', 1.3500),
    ('2024-06-01', '018439', 1.1000),
    ('2025-01-05', '018439', 1.3800)`);

  memDb.run(`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct) VALUES
    ('019173', '纳斯达克100指数(QDII)C', 196.97, 220, 1.3500, 265.91, 45.91, 20.87),
    ('018439', '国泰纳斯达克100ETF联接C', 169.65, 200, 1.3800, 234.12, 34.12, 17.06)`);
}

function seedMixedAssetData() {
  memDb.run(`INSERT OR REPLACE INTO fund_details (fund_code, fund_name, fund_type, security_type, market, currency) VALUES
    ('019173', '纳斯达克100指数(QDII)C', 'QDII-股票', 'fund', 'CN', 'CNY'),
    ('AAPL', 'Apple Inc.', '科技股', 'stock', 'US', 'USD'),
    ('00700', '腾讯控股', '港股', 'stock', 'HK', 'HKD')`);

  memDb.run(`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type) VALUES
    ('019173', '纳斯达克100指数(QDII)C', 100, -120, 1.5, 150, 30, 25, 'fund'),
    ('AAPL', 'Apple Inc.', 2, -300, 190, 380, 80, 26.67, 'stock'),
    ('00700', '腾讯控股', 10, -300, 30, 300, 0, 0, 'stock')`);

  memDb.run(`INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type) VALUES
    ('019173', '2026-06-18', 1.5, -4.2, 'fund'),
    ('AAPL', '2026-06-18', 190, 6.5, 'stock'),
    ('00700', '2026-06-18', 30, -1.2, 'stock')`);

  memDb.run(`INSERT INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date) VALUES
    ('019173', 'NVDA', 'NVIDIA', 8.5, 100, 12000, '2026-03-31'),
    ('019173', 'MSFT', 'Microsoft', 7.2, 100, 11000, '2026-03-31')`);
}

// ═══════════════════════════════════════════════════════════════════════
// Tests
// ═══════════════════════════════════════════════════════════════════════

describe("getPortfolioSummary", () => {
  test("returns shape with expected keys", () => {
    seedBaseData();
    const summary = getPortfolioSummary();
    expect(summary).toHaveProperty("total_tx");
    expect(summary).toHaveProperty("unique_funds");
    expect(summary).toHaveProperty("held_funds");
    expect(summary).toHaveProperty("total_buy");
    expect(summary).toHaveProperty("total_sell");
    expect(summary).toHaveProperty("total_fee");
    expect(summary).toHaveProperty("unrealized_pnl");
    expect(summary).toHaveProperty("settlement_distribution");
    expect(summary).toHaveProperty("by_security_type");
  });

  test("total_tx count is correct", () => {
    seedBaseData();
    expect(getPortfolioSummary().total_tx).toBe(5);
  });

  test("total_buy sum is correct", () => {
    seedBaseData();
    expect(getPortfolioSummary().total_buy).toBe(500);
  });

  test("held_funds count is correct", () => {
    seedBaseData();
    expect(getPortfolioSummary().held_funds).toBe(2);
  });
});

describe("getFundDetail", () => {
  test("returns full shape for 019173", () => {
    seedBaseData();
    const detail = getFundDetail("019173");
    expect(detail).not.toBeNull();
    expect(detail!.code).toBe("019173");
    expect(detail!.name).toBe("纳斯达克100指数(QDII)C");
    expect(detail!).toHaveProperty("held_shares");
    expect(detail!).toHaveProperty("total_cost");
    expect(detail!).toHaveProperty("current_value");
    expect(detail!).toHaveProperty("unrealized_pnl");
    expect(Array.isArray(detail!.transactions)).toBe(true);
    expect(detail!.transactions.length).toBeGreaterThan(0);
  });

  test("returns null for nonexistent fund", () => {
    seedBaseData();
    expect(getFundDetail("999999")).toBeNull();
  });
});

describe("getMaxDrawdown", () => {
  test("returns drawdown object for 019173", () => {
    seedBaseData();
    const dd = getMaxDrawdown("019173");
    expect(dd).not.toBeNull();
    expect(dd!).toHaveProperty("max_drawdown");
    expect(dd!).toHaveProperty("peak_date");
    expect(dd!).toHaveProperty("trough_date");
    expect(dd!.max_drawdown).toBeGreaterThan(0);
    expect(dd!.max_drawdown).toBeLessThan(100);
  });

  test("returns null for nonexistent fund", () => {
    seedBaseData();
    expect(getMaxDrawdown("000000")).toBeNull();
  });
});

describe("getPortfolioTimeline", () => {
  test("returns array sorted by date", () => {
    seedBaseData();
    const timeline = getPortfolioTimeline();
    expect(Array.isArray(timeline)).toBe(true);
    for (let i = 1; i < timeline.length; i++) {
      expect(timeline[i].date >= timeline[i - 1].date).toBe(true);
    }
  });
});

describe("getPortfolioPenetration", () => {
  test("returns penetration shape", () => {
    seedBaseData();
    const pen = getPortfolioPenetration();
    expect(pen).toHaveProperty("penetration");
    expect(pen).toHaveProperty("total_portfolio_value");
    expect(Array.isArray(pen.penetration)).toBe(true);
  });
});

describe("recalculateAllSnapshots", () => {
  test("repopulates portfolio_snapshot", () => {
    seedBaseData();
    const result = recalculateAllSnapshots();
    expect(result).toHaveProperty("securities");
    expect(result).toHaveProperty("totalValue");
  });
});

describe("getSystemStatus", () => {
  test("returns structured status object", () => {
    seedBaseData();
    const status = getSystemStatus();
    expect(status).toHaveProperty("transactions");
    expect(status).toHaveProperty("nav");
    expect(status).toHaveProperty("portfolio");
    expect(status).toHaveProperty("holdings_data");
    expect(status).toHaveProperty("indices");
    expect(status).toHaveProperty("anomalies");
    expect(status.transactions).toHaveProperty("count");
    expect(Array.isArray(status.anomalies.items)).toBe(true);
  });
});

describe("getPortfolioAllocation", () => {
  test("groups held assets by security type, market, and fund type for dashboard and agents", () => {
    seedMixedAssetData();
    const allocation = getPortfolioAllocation();

    expect(allocation.total_value).toBe(830);
    expect(allocation.by_security_type).toEqual([
      { key: "stock", label: "股票", value: 680, weight_pct: 81.93, count: 2 },
      { key: "fund", label: "基金", value: 150, weight_pct: 18.07, count: 1 },
    ]);
    expect(allocation.by_market.map((row) => row.key)).toEqual(["us_stock", "hk_stock", "cn_fund"]);
    expect(allocation.agent_brief).toContain("股票 81.93%");
    expect(allocation.risk_flags).toContain("股票资产占比高于 80%");
  });
});

describe("getInvestmentHarnessSnapshot", () => {
  test("returns facts-only signals for funds and stocks without making decisions", () => {
    seedMixedAssetData();
    const result = getInvestmentHarnessSnapshot();

    expect(result.decision_boundary).toBe("facts_only");
    expect(result.holding_signals).toHaveLength(3);
    expect(result.holding_signals.find((item) => item.code === "AAPL")?.signal_tags).toContain("price_rally_gt_5pct");
    expect(result.holding_signals.find((item) => item.code === "019173")?.signal_tags).toContain("above_cost_gt_10pct");
    expect(result.agent_brief).toContain("Agent owns all investment decisions");
    expect(JSON.stringify(result)).not.toContain("actual_amount");
  });
});

describe("getInvestmentSourceBrief", () => {
  test("builds facts-only source queries for Hermes search and crawling", () => {
    seedMixedAssetData();
    const brief = getInvestmentSourceBrief({ limit: 6 });

    expect(brief.decision_boundary).toBe("source_queries_only");
    expect(brief.queries.length).toBeGreaterThan(0);
    expect(brief.queries.some((q) => q.query.includes("Apple") || q.query.includes("AAPL"))).toBe(true);
    expect(brief.queries.some((q) => q.query.includes("NVIDIA") || q.query.includes("NVDA"))).toBe(true);
    expect(brief.source_targets.map((s) => s.kind)).toContain("web_search");
    expect(brief.agent_brief).toContain("Hermes");
    expect(JSON.stringify(brief)).not.toMatch(/买入|卖出|加仓|减仓|建议扣款/);
  });
});

describe("source events", () => {
  test("createSourceEvent stores event and returns it with id", () => {
    const event = createSourceEvent({
      title: "纳指100 ETF 资金流入创纪录",
      url: "https://example.com/nasdaq-inflow",
      source: "websearch",
      snippet: "纳斯达克100相关的QDII ETF本周资金净流入达到...",
      query: "纳斯达克 QDII 资金流向",
      related_security_code: "019173",
      related_security_name: "纳斯达克100指数(QDII)C",
    });

    expect(event.id).toBeGreaterThan(0);
    expect(event.title).toBe("纳指100 ETF 资金流入创纪录");
    expect(event.is_read).toBe(0);
    expect(event.is_useful).toBe(0);
    expect(event.source).toBe("websearch");
    expect(event.related_security_code).toBe("019173");
    expect(event.fetched_at).toBeTruthy();
  });

  test("getSourceEvents returns events filtered by security code", () => {
    seedMixedAssetData();
    createSourceEvent({
      title: "Apple 发布新财报",
      url: "https://example.com/apple-earnings",
      source: "websearch",
      snippet: "Apple Inc. Q3 earnings beat expectations...",
      query: "AAPL earnings Q3 2026",
      related_security_code: "AAPL",
      related_security_name: "Apple Inc.",
    });
    createSourceEvent({
      title: "腾讯游戏业务增长",
      url: "https://example.com/tencent-gaming",
      source: "websearch",
      snippet: "腾讯控股游戏收入同比增长...",
      query: "00700 腾讯 游戏 收入",
      related_security_code: "00700",
      related_security_name: "腾讯控股",
    });
    createSourceEvent({
      title: "港股通资金流向变化",
      source: "eastmoney",
      snippet: "南向资金连续3日净流入...",
      related_security_code: "00700",
    });

    // Filter by security code
    const appleEvents = getSourceEvents({ related_security_code: "AAPL" });
    expect(appleEvents.length).toBe(1);
    expect(appleEvents[0].title).toContain("Apple");

    // Filter by source
    const emEvents = getSourceEvents({ source: "eastmoney" });
    expect(emEvents.length).toBe(1);
    expect(emEvents[0].source).toBe("eastmoney");

    // All events (unread only by default)
    const all = getSourceEvents({});
    expect(all.length).toBe(3);
    // All returned (order depends on fetched_at timestamp precision)
    const titles = all.map(e => e.title);
    expect(titles).toContain("Apple 发布新财报");
    expect(titles).toContain("腾讯游戏业务增长");
    expect(titles).toContain("港股通资金流向变化");
  });

  test("markSourceEventRead toggles is_read and is_useful", () => {
    const event = createSourceEvent({
      title: "测试事件",
      source: "test",
      snippet: "test snippet",
    });

    const marked = markSourceEventRead(event.id, { is_read: true, is_useful: true });
    expect(marked).toBe(true);

    const events = getSourceEvents({ is_read: 0 });
    expect(events.length).toBe(0);

    const allEvents = getSourceEvents({ show_read: true });
    expect(allEvents.length).toBe(1);
    expect(allEvents[0].is_read).toBe(1);
    expect(allEvents[0].is_useful).toBe(1);
  });

  test("source events output never contains investment advice", () => {
    seedMixedAssetData();
    createSourceEvent({
      title: "Market update",
      source: "websearch",
      snippet: "Markets moved...",
      related_security_code: "AAPL",
    });
    const events = getSourceEvents({});
    const json = JSON.stringify(events);
    expect(json).not.toMatch(/买入|卖出|加仓|减仓|建议|推荐|目标价/);
  });
});
