// Temporary mock-data seed for local visual preview (v3.0).
// Seeds 3 funds × 35 days of NAV + buy transactions + snapshot so PortfolioChart
// renders a realistic curve. NOT for production — delete after preview.
import { Database } from "bun:sqlite";
import { join } from "node:path";

const db = new Database(join(import.meta.dirname, "..", "..", "..", "data", "fund.db"));

db.run("INSERT OR IGNORE INTO portfolio_definitions (id, name, description) VALUES (1, '默认组合', 'main')");

const funds = [
  { code: "008888", name: "纳斯达克100C", type: "QDII", mkt: "CN", startNav: 1.20 },
  { code: "510300", name: "沪深300ETF", type: "指数", mkt: "sh", startNav: 4.00 },
  { code: "161725", name: "招商白酒LOF", type: "指数", mkt: "sz", startNav: 1.50 },
];

for (const f of funds) {
  db.run(
    "INSERT OR IGNORE INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES (?,?,?,?,?)",
    [f.code, f.name, f.type, "fund", f.mkt],
  );
}

const today = new Date();
function dateStr(offDays: number, withTime = true) {
  const d = new Date(today);
  d.setDate(d.getDate() - offDays);
  const s = d.toISOString().substring(0, 10);
  return withTime ? `${s} 10:00:00` : s;
}

let seq = 1;
for (const f of funds) {
  for (const [off, amt] of [[30, 5000], [15, 3000]] as [number, number][]) {
    const shares = +(amt / f.startNav).toFixed(4);
    db.run(
      `INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change)
       VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
      [`ORD${seq}`, dateStr(off), dateStr(off, false), "buy", "buy", f.code, f.name, amt, shares, 0, -amt, shares],
    );
    seq++;
  }
}

for (const f of funds) {
  for (let i = 34; i >= 0; i--) {
    const trend = 1 + (34 - i) * 0.0035; // ~+12% over the window
    const noise = 1 + Math.sin(i * 1.3) * 0.014;
    const nav = +(f.startNav * trend * noise).toFixed(4);
    db.run(
      "INSERT OR REPLACE INTO nav_history (fund_code, date, unit_nav) VALUES (?,?,?)",
      [f.code, dateStr(i, false), nav],
    );
  }
}

for (const f of funds) {
  const shares = +(8000 / f.startNav).toFixed(4);
  const latestNav = +(f.startNav * 1.12).toFixed(4);
  const value = +(shares * latestNav).toFixed(2);
  db.run(
    `INSERT OR REPLACE INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
     VALUES (?,?,?,?,?,?,?,?,?,1)`,
    [f.code, f.name, shares, 8000, latestNav, value, +(value - 8000).toFixed(2), +((value - 8000) / 8000 * 100).toFixed(2), "fund"],
  );
}

console.log(`[seed] ${funds.length} funds, ${seq - 1} transactions, ${funds.length * 35} nav points`);
db.close();
