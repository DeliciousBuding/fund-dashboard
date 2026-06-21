/**
 * Test fixtures: seedFundDetails
 *
 * Inserts 5 entries: 3 funds + 1 stock + 1 ETF.
 */
import type { Database } from "bun:sqlite";

export function seedFundDetails(db: Database) {
  db.run(`CREATE TABLE IF NOT EXISTS fund_details (
    fund_code TEXT PRIMARY KEY, fund_name TEXT, fund_type TEXT,
    security_type TEXT DEFAULT 'fund', market TEXT DEFAULT '', currency TEXT DEFAULT 'CNY'
  )`);

  const rows = [
    { fund_code: "019173", fund_name: "纳斯达克100指数(QDII)C", fund_type: "QDII-股票", security_type: "fund", market: "", currency: "CNY" },
    { fund_code: "018439", fund_name: "国泰纳斯达克100ETF联接C", fund_type: "QDII-ETF联接", security_type: "fund", market: "", currency: "CNY" },
    { fund_code: "000000", fund_name: "测试货币基金", fund_type: "货币型", security_type: "fund", market: "", currency: "CNY" },
    { fund_code: "600519", fund_name: "贵州茅台", fund_type: "", security_type: "stock", market: "SH", currency: "CNY" },
    { fund_code: "510050", fund_name: "上证50ETF", fund_type: "ETF", security_type: "etf", market: "SH", currency: "CNY" },
  ];

  const insert = db.prepare(
    "INSERT OR REPLACE INTO fund_details (fund_code, fund_name, fund_type, security_type, market, currency) VALUES (?, ?, ?, ?, ?, ?)",
  );

  for (const r of rows) {
    insert.run(r.fund_code, r.fund_name, r.fund_type, r.security_type, r.market, r.currency);
  }
}
