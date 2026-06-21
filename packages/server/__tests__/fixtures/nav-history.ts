/**
 * Test fixtures: seedNavHistory
 *
 * Inserts ~10 price points for funds 019173 and 018439
 * with a clear peak-to-trough pattern for drawdown testing.
 */
import type { Database } from "bun:sqlite";

export function seedNavHistory(db: Database) {
  db.run(`CREATE TABLE IF NOT EXISTS nav_history (
    date TEXT, fund_code TEXT, unit_nav REAL, daily_change_pct REAL DEFAULT 0,
    PRIMARY KEY (fund_code, date)
  )`);

  // Fund 019173: clear peak-to-trough pattern
  // Peak at 2025-01-15 (1.5800), trough at 2025-03-15 (1.1000)
  const rows019173 = [
    { date: "2024-06-01", fund_code: "019173", unit_nav: 1.1700, daily_change_pct: 0.5 },
    { date: "2024-08-01", fund_code: "019173", unit_nav: 1.2500, daily_change_pct: 1.2 },
    { date: "2024-12-01", fund_code: "019173", unit_nav: 1.4200, daily_change_pct: 0.8 },
    { date: "2025-01-15", fund_code: "019173", unit_nav: 1.5800, daily_change_pct: 1.5 }, // peak
    { date: "2025-02-01", fund_code: "019173", unit_nav: 1.4000, daily_change_pct: -1.1 },
    { date: "2025-02-15", fund_code: "019173", unit_nav: 1.2500, daily_change_pct: -0.9 },
    { date: "2025-03-01", fund_code: "019173", unit_nav: 1.1500, daily_change_pct: -0.7 },
    { date: "2025-03-15", fund_code: "019173", unit_nav: 1.1000, daily_change_pct: -0.5 }, // trough
    { date: "2025-04-01", fund_code: "019173", unit_nav: 1.2500, daily_change_pct: 1.3 },
    { date: "2025-05-01", fund_code: "019173", unit_nav: 1.3500, daily_change_pct: 0.8 },
  ];

  // Fund 018439
  const rows018439 = [
    { date: "2024-06-01", fund_code: "018439", unit_nav: 1.1000, daily_change_pct: 0.3 },
    { date: "2024-09-01", fund_code: "018439", unit_nav: 1.2700, daily_change_pct: 0.6 },
    { date: "2025-01-05", fund_code: "018439", unit_nav: 1.3800, daily_change_pct: 0.5 }, // peak
    { date: "2025-05-01", fund_code: "018439", unit_nav: 1.3200, daily_change_pct: -0.2 },
  ];

  const insert = db.prepare(
    "INSERT OR REPLACE INTO nav_history (date, fund_code, unit_nav, daily_change_pct) VALUES (?, ?, ?, ?)",
  );

  for (const r of [...rows019173, ...rows018439]) {
    insert.run(r.date, r.fund_code, r.unit_nav, r.daily_change_pct);
  }
}
