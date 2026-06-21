/**
 * Test fixtures: seedTransactions
 *
 * Inserts ~10 transaction rows covering:
 * - Buy (direction "buy") with different fund codes (019173, 018439)
 * - Sell (direction "sell")
 * - Auto-buy (trade_type "定投买入") and manual-buy (trade_type "用户买入")
 * - Dividend (direction "dividend")
 * - Different time periods to enable XIRR testing
 */
import type { Database } from "bun:sqlite";

export function seedTransactions(db: Database) {
  db.run(`CREATE TABLE IF NOT EXISTS transactions (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id TEXT UNIQUE, trade_time TEXT, confirm_date TEXT,
    trade_type TEXT, direction TEXT, fund_code TEXT, fund_name TEXT,
    confirm_amount REAL, confirm_share REAL, fee REAL DEFAULT 0,
    inferred_nav REAL, nav_on_effective_date REAL, nav_verified INTEGER DEFAULT 0,
    signed_cash_flow REAL, signed_share_change REAL,
    trade_day_type TEXT, settlement_days INTEGER, effective_nav_date TEXT,
    latest_nav REAL, cost_basis REAL, unrealized_pnl REAL,
    anomaly TEXT
  )`);

  const rows = [
    // Fund 019173: auto-buy at 2024-06-01
    { order_id: "TX001", trade_time: "2024-06-01T09:00:00Z", confirm_date: "2024-06-02", trade_type: "定投买入", direction: "buy", fund_code: "019173", fund_name: "纳斯达克100指数(QDII)C", confirm_amount: 100, confirm_share: 85.47, fee: 0.15, inferred_nav: 1.1700, nav_on_effective_date: 1.1698, nav_verified: 1, signed_cash_flow: -100, signed_share_change: 85.47, settlement_days: 2, effective_nav_date: "2024-06-01", latest_nav: 1.3500, cost_basis: 100, unrealized_pnl: 15.38, anomaly: null },
    // Fund 019173: manual-buy at 2024-07-15
    { order_id: "TX002", trade_time: "2024-07-15T09:00:00Z", confirm_date: "2024-07-16", trade_type: "用户买入", direction: "buy", fund_code: "019173", fund_name: "纳斯达克100指数(QDII)C", confirm_amount: 200, confirm_share: 166.67, fee: 0.30, inferred_nav: 1.2000, nav_on_effective_date: 1.1998, nav_verified: 1, signed_cash_flow: -200, signed_share_change: 166.67, settlement_days: 2, effective_nav_date: "2024-07-15", latest_nav: 1.3500, cost_basis: 200, unrealized_pnl: 25.00, anomaly: null },
    // Fund 019173: manual-buy at 2024-10-01
    { order_id: "TX003", trade_time: "2024-10-01T09:00:00Z", confirm_date: "2024-10-02", trade_type: "用户买入", direction: "buy", fund_code: "019173", fund_name: "纳斯达克100指数(QDII)C", confirm_amount: 150, confirm_share: 113.64, fee: 0.22, inferred_nav: 1.3200, nav_on_effective_date: 1.3198, nav_verified: 1, signed_cash_flow: -150, signed_share_change: 113.64, settlement_days: 2, effective_nav_date: "2024-10-01", latest_nav: 1.3500, cost_basis: 150, unrealized_pnl: 3.41, anomaly: null },
    // Fund 019173: dividend at 2025-01-10
    { order_id: "TX004", trade_time: "2025-01-10T09:00:00Z", confirm_date: "2025-01-10", trade_type: "分红", direction: "dividend", fund_code: "019173", fund_name: "纳斯达克100指数(QDII)C", confirm_amount: 8.50, confirm_share: 0, fee: 0, inferred_nav: null, nav_on_effective_date: null, nav_verified: 0, signed_cash_flow: 8.50, signed_share_change: 0, settlement_days: 0, effective_nav_date: "", latest_nav: null, cost_basis: null, unrealized_pnl: null, anomaly: null },
    // Fund 019173: sell partial at 2025-03-20
    { order_id: "TX005", trade_time: "2025-03-20T09:00:00Z", confirm_date: "2025-03-21", trade_type: "用户卖出", direction: "sell", fund_code: "019173", fund_name: "纳斯达克100指数(QDII)C", confirm_amount: 80, confirm_share: 55.17, fee: 0.12, inferred_nav: 1.4500, nav_on_effective_date: 1.4498, nav_verified: 1, signed_cash_flow: 80, signed_share_change: -55.17, settlement_days: 3, effective_nav_date: "2025-03-20", latest_nav: 1.4500, cost_basis: -80, unrealized_pnl: null, anomaly: null },

    // Fund 018439: auto-buy at 2024-06-01
    { order_id: "TX006", trade_time: "2024-06-01T09:00:00Z", confirm_date: "2024-06-02", trade_type: "定投买入", direction: "buy", fund_code: "018439", fund_name: "国泰纳斯达克100ETF联接C", confirm_amount: 100, confirm_share: 90.91, fee: 0.15, inferred_nav: 1.1000, nav_on_effective_date: 1.0998, nav_verified: 1, signed_cash_flow: -100, signed_share_change: 90.91, settlement_days: 2, effective_nav_date: "2024-06-01", latest_nav: 1.3800, cost_basis: 100, unrealized_pnl: 25.46, anomaly: null },
    // Fund 018439: auto-buy at 2024-09-01
    { order_id: "TX007", trade_time: "2024-09-01T09:00:00Z", confirm_date: "2024-09-02", trade_type: "定投买入", direction: "buy", fund_code: "018439", fund_name: "国泰纳斯达克100ETF联接C", confirm_amount: 100, confirm_share: 78.74, fee: 0.15, inferred_nav: 1.2700, nav_on_effective_date: 1.2698, nav_verified: 1, signed_cash_flow: -100, signed_share_change: 78.74, settlement_days: 2, effective_nav_date: "2024-09-01", latest_nav: 1.3800, cost_basis: 100, unrealized_pnl: 8.66, anomaly: null },
    // Fund 018439: manual-buy at 2025-01-05 with anomaly
    { order_id: "TX008", trade_time: "2025-01-05T09:00:00Z", confirm_date: "2025-01-06", trade_type: "用户买入", direction: "buy", fund_code: "018439", fund_name: "国泰纳斯达克100ETF联接C", confirm_amount: 500, confirm_share: 362.32, fee: 0.75, inferred_nav: 1.3800, nav_on_effective_date: 1.3798, nav_verified: 1, signed_cash_flow: -500, signed_share_change: 362.32, settlement_days: null, effective_nav_date: "2025-01-05", latest_nav: 1.3800, cost_basis: 500, unrealized_pnl: 0, anomaly: "settlement_days missing" },
  ];

  const insert = db.prepare(`
    INSERT OR IGNORE INTO transactions
    (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name,
     confirm_amount, confirm_share, fee, inferred_nav, nav_on_effective_date, nav_verified,
     signed_cash_flow, signed_share_change, settlement_days, effective_nav_date,
     latest_nav, cost_basis, unrealized_pnl, anomaly)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
  `);

  for (const r of rows) {
    insert.run(r.order_id, r.trade_time, r.confirm_date, r.trade_type, r.direction,
      r.fund_code, r.fund_name, r.confirm_amount, r.confirm_share, r.fee,
      r.inferred_nav, r.nav_on_effective_date, r.nav_verified,
      r.signed_cash_flow, r.signed_share_change, r.settlement_days, r.effective_nav_date,
      r.latest_nav, r.cost_basis, r.unrealized_pnl, r.anomaly);
  }
}
