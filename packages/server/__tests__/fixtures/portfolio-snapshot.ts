/**
 * Test fixtures: seedPortfolioSnapshot
 *
 * Inserts 2 held positions (019173 and 018439).
 */
import type { Database } from "bun:sqlite";

export function seedPortfolioSnapshot(db: Database) {
  db.run(`CREATE TABLE IF NOT EXISTS portfolio_snapshot (
    fund_code TEXT PRIMARY KEY, fund_name TEXT,
    held_shares REAL, total_cost REAL, latest_nav REAL,
    current_value REAL, unrealized_pnl REAL, pnl_pct REAL,
    portfolio_id INTEGER DEFAULT 1
  )`);

  const rows = [
    { fund_code: "019173", fund_name: "纳斯达克100指数(QDII)C", held_shares: 310.61, total_cost: -450, latest_nav: 1.3500, current_value: 419.32, unrealized_pnl: -30.68, pnl_pct: 6.82 },
    { fund_code: "018439", fund_name: "国泰纳斯达克100ETF联接C", held_shares: 531.97, total_cost: -700, latest_nav: 1.3800, current_value: 734.12, unrealized_pnl: 34.12, pnl_pct: 4.87 },
  ];

  const insert = db.prepare(
    "INSERT OR REPLACE INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
  );

  for (const r of rows) {
    insert.run(r.fund_code, r.fund_name, r.held_shares, r.total_cost, r.latest_nav, r.current_value, r.unrealized_pnl, r.pnl_pct);
  }
}
