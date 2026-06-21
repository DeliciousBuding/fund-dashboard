/** Fund holdings data fetcher — populates fund_holdings table for QDII funds
 *
 *  Uses eastmoney FundArchivesDatas.aspx?type=jjcc API to scrape
 *  the latest quarterly holdings and persist them to SQLite.
 *
 *  The fund_holdings table schema:
 *    fund_code TEXT, stock_code TEXT, stock_name TEXT,
 *    weight_pct REAL, shares REAL, market_value REAL,
 *    report_date TEXT,
 *    PRIMARY KEY (fund_code, stock_code, report_date)
 */

import { fetchFundHoldings } from "./eastmoney";
import { getRwDb } from "../db";
import { log } from "../middleware/logger";

let _crawlingHoldings = false;

/**
 * Fetch and persist holdings for a single fund.
 * Uses INSERT OR REPLACE so re-running for the same report_date is idempotent.
 *
 * @param fundCode - 6-digit fund code
 * @returns result with code, name, report_date, and count of holdings persisted
 */
export async function refreshFundHoldings(
  fundCode: string,
): Promise<{ fund_code: string; fund_name: string; report_date: string; count: number } | null> {
  const result = await fetchFundHoldings(fundCode);
  if (!result || !result.holdings.length) {
    log.info(`holdings: ${fundCode} — no data`);
    return null;
  }

  const db = getRwDb();
  const insert = db.prepare(`
    INSERT OR REPLACE INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date)
    VALUES (?, ?, ?, ?, ?, ?, ?)
  `);

  let count = 0;
  const doInsert = db.transaction((data: typeof result.holdings) => {
    for (const h of data) {
      const r = insert.run(fundCode, h.stock_code, h.stock_name, h.weight_pct, h.shares, h.market_value, result.report_date);
      if (r.changes) count++;
    }
  });
  doInsert(result.holdings);

  log.info(`holdings: ${fundCode} ${result.fund_name} ${result.report_date} — ${count} stocks`);
  return {
    fund_code: fundCode,
    fund_name: result.fund_name,
    report_date: result.report_date,
    count,
  };
}

/**
 * Refresh holdings for all held funds that have a QDII or stock-heavy type.
 *
 * Iterates portfolio_snapshot for held funds, filters to likely equity funds
 * (QDII, 股票型, 混合型, 指数型, ETF), fetches holdings from eastmoney.
 */
export async function refreshAllHoldings(): Promise<{
  total: number;
  updated: number;
  details: { fund_code: string; fund_name: string; report_date: string; count: number }[];
}> {
  if (_crawlingHoldings) {
    log.info("holdings crawl skipped: already in progress");
    return { total: 0, updated: 0, details: [] };
  }
  _crawlingHoldings = true;
  try {
    const db = getRwDb();
    // Only crawl funds that are likely to have stock holdings: QDII, 股票型, 混合型, 指数型, ETF
    const held = db.query(`
      SELECT fd.fund_code, fd.fund_name, fd.fund_type
      FROM portfolio_snapshot ps
      JOIN fund_details fd ON ps.fund_code = fd.fund_code
      WHERE ps.held_shares > 0.001
        AND fd.security_type = 'fund'
        AND (fd.fund_type LIKE '%QDII%' OR fd.fund_type LIKE '%股票%' OR fd.fund_type LIKE '%混合%' OR fd.fund_type LIKE '%指数%' OR fd.fund_type LIKE '%ETF%')
    `).all() as { fund_code: string; fund_name: string; fund_type: string }[];

    const details: { fund_code: string; fund_name: string; report_date: string; count: number }[] = [];
    let updated = 0;
    const maxRetries = 2;

    for (let i = 0; i < held.length; i++) {
      const f = held[i];
      let lastError: Error | null = null;
      let success = false;

      for (let attempt = 0; attempt <= maxRetries; attempt++) {
        try {
          if (attempt > 0) {
            log.info(`holdings retry ${attempt}/${maxRetries} for ${f.fund_code} (${f.fund_name})`);
            await new Promise(resolve => setTimeout(resolve, 1000));
          }
          const r = await refreshFundHoldings(f.fund_code);
          if (r) {
            updated++;
            details.push(r);
          }
          success = true;
          break;
        } catch (e: any) {
          lastError = e;
        }
      }

      if (!success) {
        log.warn(`holdings refresh failed for ${f.fund_code} (${f.fund_name}) after ${maxRetries + 1} attempts: ${lastError?.message}`);
      }

      // Progress tracking: log every 5 funds
      if ((i + 1) % 5 === 0 || i === held.length - 1) {
        log.info(`holdings progress: ${i + 1}/${held.length} (${Math.round((i + 1) / held.length * 100)}%)`);
      }

      // Rate limit: at most one request per 500ms
      if (i < held.length - 1) {
        await new Promise(resolve => setTimeout(resolve, 500));
      }
    }

    log.info(`holdings refresh complete: ${held.length} funds checked, ${updated} updated`);
    return { total: held.length, updated, details };
  } finally {
    _crawlingHoldings = false;
  }
}
