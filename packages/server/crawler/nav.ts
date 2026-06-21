/** Security price data fetcher — incrementally updates nav_history table for funds and stocks */

import { fetchNavHistory, fetchStockKline } from "./eastmoney";
import { getRwDb } from "../db";
import { log } from "../middleware/logger";

let _crawling = false;

function ensureSchema(db: ReturnType<typeof getRwDb>) {
  try { db.run("ALTER TABLE nav_history ADD COLUMN daily_change_pct REAL DEFAULT 0"); } catch {}
  try { db.run("ALTER TABLE nav_history ADD COLUMN security_type TEXT DEFAULT 'fund'"); } catch {}
}

export type SecurityType = "fund" | "stock";

/** Fetch latest price data for a single security (fund or stock).
 *  - For funds: uses fetchNavHistory (pingzhongdata JS)
 *  - For stocks: uses fetchStockKline (eastmoney K-line API), mapping close→unit_nav, change_pct
 */
export async function refreshSecurityPrice(
  code: string,
  security_type: SecurityType = "fund",
): Promise<{ code: string; security_type: SecurityType; added: number; latest: string }> {
  let rows: { date: string; unit_nav: number; change_pct: number }[];

  if (security_type === "stock") {
    const klines = await fetchStockKline(code);
    rows = klines.map(k => ({
      date: k.date,
      unit_nav: k.close,
      change_pct: k.change_pct,
    }));
  } else {
    rows = await fetchNavHistory(code);
  }

  if (!rows.length) return { code, security_type, added: 0, latest: "none" };

  const db = getRwDb();
  ensureSchema(db);

  const insert = db.prepare(`
    INSERT OR REPLACE INTO nav_history (date, fund_code, unit_nav, daily_change_pct, security_type)
    VALUES (?, ?, ?, ?, ?)
  `);

  let added = 0;
  const doInsert = db.transaction((data: typeof rows) => {
    for (const r of data) {
      const result = insert.run(r.date, code, r.unit_nav, r.change_pct, security_type);
      if (result.changes) added++;
    }
  });
  doInsert(rows);

  const latest = rows[rows.length - 1].date;
  log.info(`price refresh: ${code} (${security_type}) +${added} rows, latest=${latest}`);
  return { code, security_type, added, latest };
}

/** @deprecated — use refreshSecurityPrice(code, 'fund') instead */
export async function refreshFundNav(code: string): Promise<{ code: string; added: number; latest: string }> {
  const r = await refreshSecurityPrice(code, "fund");
  return { code: r.code, added: r.added, latest: r.latest };
}

/** Iterate all held securities from portfolio_snapshot and refresh price data */
export async function refreshAllHeld(): Promise<{ total: number; added: number }> {
  if (_crawling) { log.info("crawl skipped: already in progress"); return { total: 0, added: 0 }; }
  _crawling = true;
  try {
    const db = getRwDb();
    const held = db.query(`
      SELECT fund_code, COALESCE(security_type, 'fund') as security_type
      FROM portfolio_snapshot WHERE held_shares > 0.001
    `).all() as { fund_code: string; security_type: SecurityType }[];

    let totalAdded = 0;
    for (const h of held) {
      try {
        const r = await refreshSecurityPrice(h.fund_code, h.security_type);
        totalAdded += r.added;
        await new Promise(r => setTimeout(r, 1500));
      } catch (e: any) { log.warn(`price refresh failed for ${h.fund_code} (${h.security_type}): ${e.message}`); }
    }
    log.info(`price refresh complete: ${held.length} securities, ${totalAdded} new rows`);
    return { total: held.length, added: totalAdded };
  } finally {
    _crawling = false;
  }
}
