import { Database } from "bun:sqlite";

const src = new Database("data/fund.db");
const dst = new Database("data/fund_clean.db");
dst.run("PRAGMA journal_mode=WAL");
dst.run("PRAGMA busy_timeout=5000");

const createStmts = [
  `CREATE TABLE IF NOT EXISTS fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT, fund_type TEXT)`,
  `CREATE TABLE IF NOT EXISTS transactions (seq INTEGER PRIMARY KEY AUTOINCREMENT, order_id TEXT UNIQUE, trade_time TEXT, confirm_date TEXT, trade_type TEXT, direction TEXT, fund_code TEXT, fund_name TEXT, confirm_amount REAL, confirm_share REAL, fee REAL DEFAULT 0, inferred_nav REAL, nav_on_effective_date REAL, nav_verified INTEGER DEFAULT 0, signed_cash_flow REAL, signed_share_change REAL, trade_day_type TEXT, settlement_days INTEGER, effective_nav_date TEXT, latest_nav REAL, cost_basis REAL, unrealized_pnl REAL, anomaly TEXT)`,
  `CREATE TABLE IF NOT EXISTS nav_history (date TEXT, fund_code TEXT, unit_nav REAL, daily_change_pct REAL DEFAULT 0, PRIMARY KEY (fund_code, date))`,
  `CREATE TABLE IF NOT EXISTS portfolio_snapshot (fund_code TEXT PRIMARY KEY, fund_name TEXT, held_shares REAL, total_cost REAL, latest_nav REAL, current_value REAL, unrealized_pnl REAL, pnl_pct REAL)`,
  `CREATE TABLE IF NOT EXISTS fund_holdings (fund_code TEXT, stock_code TEXT, stock_name TEXT, weight_pct REAL, shares REAL, market_value REAL, report_date TEXT, PRIMARY KEY (fund_code, stock_code, report_date))`,
  `CREATE TABLE IF NOT EXISTS indices (code TEXT PRIMARY KEY, name TEXT, market TEXT, price REAL, change_pct REAL, change_amt REAL, updated_at TEXT DEFAULT (datetime('now')))`,
  `CREATE TABLE IF NOT EXISTS stock_profile (code TEXT, name TEXT, market TEXT, sector TEXT, industry TEXT, market_cap REAL, pe REAL, description TEXT, PRIMARY KEY (code, market))`,
  `CREATE TABLE IF NOT EXISTS fund_status (fund_code TEXT PRIMARY KEY, purchase_status TEXT, redemption_status TEXT)`,
  `CREATE TABLE IF NOT EXISTS summary_by_fund (fund_code TEXT PRIMARY KEY, fund_name TEXT, total_shares REAL, total_cost REAL, tx_count INTEGER)`,
  `CREATE TABLE IF NOT EXISTS stock_realtime (code TEXT, market TEXT, name TEXT, price REAL, open REAL, high REAL, low REAL, change_pct REAL, change_amt REAL, volume REAL, amount REAL, turnover REAL, pe REAL, total_mv REAL, circ_mv REAL, high52 REAL, low52 REAL, currency TEXT DEFAULT '', updated_at TEXT DEFAULT (datetime('now')), PRIMARY KEY (code, market))`,
  `CREATE TABLE IF NOT EXISTS stock_kline_cache (code TEXT, market TEXT, date TEXT, open REAL, close REAL, high REAL, low REAL, volume REAL, amount REAL, amplitude REAL, change_pct REAL, turnover_rate REAL, PRIMARY KEY (code, market, date))`,
  `CREATE TABLE IF NOT EXISTS sector_map (stock_code TEXT, market TEXT, sector TEXT, industry TEXT, PRIMARY KEY (stock_code, market))`,
  `CREATE TABLE IF NOT EXISTS qa_report (key TEXT PRIMARY KEY, value TEXT)`,
];

// Also add ALTER TABLE for extension columns
const alterStmts = [
  `ALTER TABLE fund_details ADD COLUMN security_type TEXT DEFAULT 'fund'`,
  `ALTER TABLE fund_details ADD COLUMN market TEXT DEFAULT ''`,
  `ALTER TABLE fund_details ADD COLUMN currency TEXT DEFAULT 'CNY'`,
  `ALTER TABLE fund_details ADD COLUMN exchange TEXT DEFAULT ''`,
  `ALTER TABLE nav_history ADD COLUMN security_type TEXT DEFAULT 'fund'`,
  `ALTER TABLE portfolio_snapshot ADD COLUMN security_type TEXT DEFAULT 'fund'`,
];

for (const stmt of createStmts) {
  try { dst.run(stmt); } catch(e) { console.log("create skip:", e.message.substring(0,80)); }
}
for (const stmt of alterStmts) {
  try { dst.run(stmt); } catch(e) {}
}

// Copy small tables with simple SELECT
console.log("Copying small tables...");
const smallTables: Record<string, string[]> = {
  fund_details: ["fund_code", "fund_name", "fund_type"],
  fund_holdings: ["fund_code", "stock_code", "stock_name", "weight_pct", "shares", "market_value", "report_date"],
  fund_status: ["fund_code", "purchase_status", "redemption_status"],
  summary_by_fund: ["fund_code", "fund_name", "total_shares", "total_cost", "tx_count"],
  stock_kline_cache: ["code", "market", "date", "open", "close", "high", "low", "volume", "amount", "amplitude", "change_pct", "turnover_rate"],
  stock_realtime: ["code", "market", "name", "price", "open", "high", "low", "change_pct", "change_amt", "volume", "amount", "turnover", "pe", "total_mv", "circ_mv", "high52", "low52", "currency", "updated_at"],
  sector_map: ["stock_code", "market", "sector", "industry"],
  qa_report: ["key", "value"],
};

for (const [tbl, cols] of Object.entries(smallTables)) {
  try {
    const rows = src.query("SELECT " + cols.join(",") + " FROM " + tbl).all();
    if (!rows.length) { console.log(tbl + ": 0 rows"); continue; }
    const ph = cols.map(() => "?").join(",");
    const insert = dst.prepare("INSERT OR IGNORE INTO " + tbl + " (" + cols.join(",") + ") VALUES (" + ph + ")");
    const doTx = dst.transaction((rws: any[]) => {
      for (const r of rws) insert.run(...cols.map(c => r[c]));
    });
    doTx(rows);
    console.log(tbl + ": " + rows.length + " rows");
  } catch(e: any) {
    console.log(tbl + ": ERROR - " + e.message.substring(0,100));
  }
}

// Copy transactions via rowid scan
console.log("Copying transactions via rowid scan...");
const txCols = ["order_id", "trade_time", "confirm_date", "trade_type", "direction", "fund_code", "fund_name", "confirm_amount", "confirm_share", "fee", "inferred_nav", "signed_cash_flow", "signed_share_change", "anomaly"];
const txPh = txCols.map(() => "?").join(",");
const txInsert = dst.prepare("INSERT OR IGNORE INTO transactions (" + txCols.join(",") + ") VALUES (" + txPh + ")");
let txCopied = 0;
for (let rowid = 1; rowid <= 500; rowid++) {
  try {
    const r = src.query("SELECT " + txCols.join(",") + " FROM transactions WHERE rowid = ?").get(rowid);
    if (r) {
      txInsert.run(...txCols.map(c => r[c]));
      txCopied++;
    }
  } catch(e) {}
}
console.log("transactions: " + txCopied + " rows");

// Copy nav_history via rowid scan (large table, batched)
console.log("Copying nav_history via rowid scan...");
const navInsert = dst.prepare("INSERT OR IGNORE INTO nav_history (fund_code, date, unit_nav, daily_change_pct) VALUES (?,?,?,?)");
let navCopied = 0;
const batch: any[] = [];
for (let rowid = 1; rowid <= 80000; rowid++) {
  try {
    const r = src.query("SELECT fund_code, date, unit_nav, daily_change_pct FROM nav_history WHERE rowid = ?").get(rowid);
    if (r) {
      batch.push(r);
      if (batch.length >= 5000) {
        const doTx = dst.transaction((b: any[]) => { for (const n of b) navInsert.run(n.fund_code, n.date, n.unit_nav, n.daily_change_pct || 0); });
        doTx(batch);
        navCopied += batch.length;
        batch.length = 0;
        console.log("nav: " + navCopied + " rows so far");
      }
    }
  } catch(e) {}
}
if (batch.length) {
  const doTx = dst.transaction((b: any[]) => { for (const n of b) navInsert.run(n.fund_code, n.date, n.unit_nav, n.daily_change_pct || 0); });
  doTx(batch);
  navCopied += batch.length;
}
console.log("nav_history: " + navCopied + " rows");

// Create indices
for (const idx of [
  "CREATE INDEX IF NOT EXISTS idx_tx_fund ON transactions(fund_code)",
  "CREATE INDEX IF NOT EXISTS idx_tx_time ON transactions(trade_time)",
  "CREATE INDEX IF NOT EXISTS idx_nav_code ON nav_history(fund_code)",
  "CREATE INDEX IF NOT EXISTS idx_nav_date ON nav_history(date)",
  "CREATE INDEX IF NOT EXISTS idx_skline_code ON stock_kline_cache(code, market)",
  "CREATE INDEX IF NOT EXISTS idx_skline_date ON stock_kline_cache(date)",
]) {
  try { dst.run(idx); } catch(e) {}
}

// Verify integrity
const integrity = dst.query("PRAGMA integrity_check").all();
console.log("New DB integrity:", JSON.stringify(integrity).substring(0, 500));

// Count rows in new DB
for (const tbl of ["fund_details", "transactions", "nav_history", "fund_holdings", "fund_status", "summary_by_fund", "stock_kline_cache", "stock_realtime", "sector_map", "qa_report"]) {
  try {
    const cnt = dst.query("SELECT COUNT(*) as n FROM " + tbl).get();
    console.log(tbl + ": " + (cnt?.n ?? 0) + " rows");
  } catch(e) {}
}

src.close();
dst.close();
console.log("Recovery complete!");
