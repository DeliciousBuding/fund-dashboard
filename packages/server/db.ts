/** SQLite connection + query helpers + schema initialization */

import { Database } from "bun:sqlite";
import { join } from "node:path";

const DB_PATH = process.env.DB_PATH || join(import.meta.dirname, "..", "..", "data", "fund.db");
let _db: Database | null = null;

/** Single shared read-write connection.
 *  WAL mode enables concurrent reads; a single connection avoids stale-snapshot issues
 *  where a separate read-only connection would bypass un-checkpointed writes.
 */
export function getDb(): Database {
  if (!_db) {
    _db = new Database(DB_PATH);
    _db.run("PRAGMA journal_mode=WAL");
    _db.run("PRAGMA foreign_keys = ON");
    _db.run("PRAGMA busy_timeout=5000");
    _db.run("PRAGMA cache_size=-8000");
  }
  return _db;
}

/** Alias for getDb — shared read-write WAL connection. */
export const getRwDb = getDb;

export function query<T = any>(sql: string, ...params: any[]): T[] {
  return getDb().query(sql).all(...params) as T[];
}

export function queryOne<T = any>(sql: string, ...params: any[]): T | undefined {
  return getDb().query(sql).get(...params) as T | undefined;
}

/** Ensure DB schema exists (idempotent, safe to call every startup) */
export function initSchema(db: Database) {
  /** Run ALTER TABLE, ignoring "duplicate column" / "already exists" errors */
  function safeAlter(sql: string) {
    try {
      db.run(sql);
    } catch (e: any) {
      const msg: string = e.message ?? String(e);
      if (msg.includes("duplicate column") || msg.includes("already exists")) return;
      console.error("Schema migration failed:", sql, e);
      throw e;
    }
  }

  /** Recreate a table to add NOT NULL on specified columns.
   *  Uses CREATE+COPY+DROP+RENAME — SQLite lacks ALTER COLUMN SET NOT NULL.
   *  Idempotent: skips columns that already have NOT NULL.
   *  Wrapped in try-catch so existing deployments upgrade safely. */
  function safeAddNotNulls(
    table: string,
    notNullCols: { name: string; nullDefault: string }[],
  ) {
    try {
      const cols = db.query(`PRAGMA table_info('${table}')`).all() as {
        cid: number; name: string; type: string; notnull: number; dflt_value: string | null; pk: number;
      }[];

      if (cols.length === 0) return; // table not created yet — CREATE TABLE will handle it

      const nnNames = new Set(notNullCols.map(c => c.name));
      const needed = notNullCols.filter(nc => {
        const col = cols.find(c => c.name === nc.name);
        return col && !col.notnull;
      });

      if (needed.length === 0) return; // all already NOT NULL

      // Preserve AUTOINCREMENT from original CREATE TABLE (PRAGMA table_info omits it)
      const origSql = (db.query(
        `SELECT sql FROM sqlite_master WHERE type='table' AND name='${table}'`,
      ).get() as { sql: string } | undefined)?.sql ?? "";
      const hasAutoinc = /AUTOINCREMENT/i.test(origSql);

      // Rebuild column definitions.
      // Fix: a composite PK (multiple pk>0 columns, e.g. nav_history
      // PRIMARY KEY (fund_code, date)) must be a TABLE-level constraint —
      // emitting per-column "PRIMARY KEY" produced "more than one primary key"
      // on rebuild. Only single-column PKs use the column-level form.
      const pkCols = cols.filter(c => c.pk).sort((a, b) => a.pk - b.pk);
      const compositePk = pkCols.length > 1;
      const colDefs = cols.map(c => {
        let def = `"${c.name}" ${c.type}`;
        if (c.pk && !compositePk) {
          def += " PRIMARY KEY";
          if (hasAutoinc && c.type.toUpperCase() === "INTEGER") def += " AUTOINCREMENT";
        }
        if (nnNames.has(c.name)) def += " NOT NULL";
        if (c.dflt_value !== null && c.dflt_value !== undefined) def += ` DEFAULT ${c.dflt_value}`;
        return def;
      }).join(", ");
      const pkConstraint = compositePk
        ? `, PRIMARY KEY (${pkCols.map(c => `"${c.name}"`).join(", ")})`
        : "";

      // Capture indexes to recreate after table swap
      const indexes = db.query(
        `SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name='${table}' AND sql IS NOT NULL`,
      ).all() as { sql: string }[];

      db.run("BEGIN");
      try {
        // Replace NULLs with safe defaults before applying NOT NULL
        for (const nc of notNullCols) {
          db.run(`UPDATE "${table}" SET "${nc.name}" = ${nc.nullDefault} WHERE "${nc.name}" IS NULL`);
        }

        const tmp = `${table}_nn`;
        db.run(`DROP TABLE IF EXISTS "${tmp}"`);
        db.run(`CREATE TABLE "${tmp}" (${colDefs}${pkConstraint})`);

        const colNames = cols.map(c => `"${c.name}"`).join(", ");
        db.run(`INSERT INTO "${tmp}" (${colNames}) SELECT ${colNames} FROM "${table}"`);
        db.run(`DROP TABLE "${table}"`);
        db.run(`ALTER TABLE "${tmp}" RENAME TO "${table}"`);

        // Recreate indexes on the new table
        for (const idx of indexes) {
          try { db.run(idx.sql); } catch { /* index may reference dropped columns */ }
        }

        db.run("COMMIT");
        console.log(`[schema] Added NOT NULL to ${table}(${needed.map(n => n.name).join(", ")})`);
      } catch (e) {
        try { db.run("ROLLBACK"); } catch { /* best-effort */ }
        throw e;
      }
    } catch (e: any) {
      const msg: string = e.message ?? String(e);
      if (msg.includes("NOT NULL constraint") || msg.includes("no such table")) {
        console.warn(`[schema] NOT NULL migration for ${table} skipped: ${msg}`);
        return;
      }
      console.error(`[schema] NOT NULL migration failed for ${table}:`, msg);
      throw e;
    }
  }

  /** Add a foreign key constraint to an existing table.
   *  Tries ALTER TABLE first; falls back to table recreation (SQLite lacks ALTER TABLE ADD CONSTRAINT).
   *  Idempotent: skips if FK already exists.
   *  Wrapped in try-catch so existing deployments upgrade safely. */
  function safeAddForeignKey(opts: {
    table: string;
    column: string;
    refTable: string;
    refColumn: string;
    onDelete?: string;
  }) {
    try {
      const { table, column, refTable, refColumn, onDelete } = opts;

      // Check if FK already exists
      const fkList = db.query(`PRAGMA foreign_key_list('${table}')`).all() as {
        id: number; seq: number; table: string; from: string; to: string; on_update: string; on_delete: string; match: string;
      }[];
      if (fkList.some(fk => fk.from === column && fk.table === refTable)) return;

      const cols = db.query(`PRAGMA table_info('${table}')`).all() as {
        cid: number; name: string; type: string; notnull: number; dflt_value: string | null; pk: number;
      }[];
      if (cols.length === 0) return; // table not created yet — CREATE TABLE will handle it

      // Attempt 1: ALTER TABLE (unsupported in most SQLite builds, but try anyway)
      try {
        const onDel = onDelete ? ` ON DELETE ${onDelete}` : "";
        db.run(`ALTER TABLE "${table}" ADD CONSTRAINT fk_${table}_${column} FOREIGN KEY ("${column}") REFERENCES "${refTable}"("${refColumn}")${onDel}`);
        console.log(`[schema] Added FK via ALTER: ${table}(${column}) -> ${refTable}(${refColumn})`);
        return;
      } catch {
        // ALTER TABLE ADD CONSTRAINT not supported — fall through to recreation
      }

      // Attempt 2: recreate table with FK constraint.
      // Composite PK (e.g. nav_history) must be a table-level constraint.
      const pkColsFk = cols.filter(c => c.pk).sort((a, b) => a.pk - b.pk);
      const compositePkFk = pkColsFk.length > 1;
      const colDefs = cols.map(c => {
        let def = `"${c.name}" ${c.type}`;
        if (c.pk && !compositePkFk) def += " PRIMARY KEY";
        if (c.notnull) def += " NOT NULL";
        if (c.dflt_value !== null && c.dflt_value !== undefined) def += ` DEFAULT ${c.dflt_value}`;
        return def;
      });
      if (compositePkFk) colDefs.push(`PRIMARY KEY (${pkColsFk.map(c => `"${c.name}"`).join(", ")})`);

      const onDel = onDelete ? ` ON DELETE ${onDelete}` : "";
      colDefs.push(`FOREIGN KEY ("${column}") REFERENCES "${refTable}"("${refColumn}")${onDel}`);

      const indexes = db.query(
        `SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name='${table}' AND sql IS NOT NULL`,
      ).all() as { sql: string }[];

      db.run("BEGIN");
      try {
        const tmp = `${table}_fk`;
        db.run(`DROP TABLE IF EXISTS "${tmp}"`);
        db.run(`CREATE TABLE "${tmp}" (${colDefs.join(", ")})`);

        const colNames = cols.map(c => `"${c.name}"`).join(", ");
        db.run(`INSERT INTO "${tmp}" (${colNames}) SELECT ${colNames} FROM "${table}"`);
        db.run(`DROP TABLE "${table}"`);
        db.run(`ALTER TABLE "${tmp}" RENAME TO "${table}"`);

        for (const idx of indexes) {
          try { db.run(idx.sql); } catch { /* index may reference dropped columns */ }
        }

        db.run("COMMIT");
        console.log(`[schema] Added FK via recreation: ${table}(${column}) -> ${refTable}(${refColumn})`);
      } catch (e) {
        try { db.run("ROLLBACK"); } catch { /* best-effort */ }
        throw e;
      }
    } catch (e: any) {
      const msg: string = e.message ?? String(e);
      if (msg.includes("no such table") || msg.includes("foreign key mismatch")) {
        console.warn(`[schema] FK migration for ${opts.table}.${opts.column} skipped: ${msg}`);
        return;
      }
      console.error(`[schema] FK migration failed for ${opts.table}.${opts.column}:`, msg);
      throw e;
    }
  }

  try {
    db.run(`CREATE TABLE IF NOT EXISTS fund_details (
      fund_code TEXT PRIMARY KEY, fund_name TEXT, fund_type TEXT
    )`);
    // Stock support: add security_type and market columns (idempotent ALTER TABLE IF NOT EXISTS)
    safeAlter("ALTER TABLE fund_details ADD COLUMN security_type TEXT DEFAULT 'fund'");
    safeAlter("ALTER TABLE fund_details ADD COLUMN market TEXT DEFAULT ''");
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
    db.run(`CREATE TABLE IF NOT EXISTS nav_history (
      date TEXT, fund_code TEXT, unit_nav REAL, daily_change_pct REAL DEFAULT 0,
      PRIMARY KEY (fund_code, date)
    )`);
    safeAlter("ALTER TABLE nav_history ADD COLUMN security_type TEXT DEFAULT 'fund'");
    db.run(`CREATE TABLE IF NOT EXISTS portfolio_snapshot (
      fund_code TEXT PRIMARY KEY, fund_name TEXT,
      held_shares REAL, total_cost REAL, latest_nav REAL,
      current_value REAL, unrealized_pnl REAL, pnl_pct REAL
    )`);
    safeAlter("ALTER TABLE portfolio_snapshot ADD COLUMN security_type TEXT DEFAULT 'fund'");
    safeAlter("ALTER TABLE portfolio_snapshot ADD COLUMN portfolio_id INTEGER DEFAULT 1");
    // V3: Multi-portfolio definitions
    db.run(`CREATE TABLE IF NOT EXISTS portfolio_definitions (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      name TEXT NOT NULL UNIQUE,
      description TEXT DEFAULT '',
      created_at TEXT NOT NULL DEFAULT (datetime('now'))
    )`);
    db.run("INSERT OR IGNORE INTO portfolio_definitions (id, name, description) VALUES (1, 'default', 'Default portfolio')");
    // V2 fund_details additions
    safeAlter("ALTER TABLE fund_details ADD COLUMN currency TEXT DEFAULT 'CNY'");
    safeAlter("ALTER TABLE fund_details ADD COLUMN exchange TEXT DEFAULT ''");
    // V2 tables
    db.run(`CREATE TABLE IF NOT EXISTS fund_holdings (
      fund_code TEXT, stock_code TEXT, stock_name TEXT,
      weight_pct REAL, shares REAL, market_value REAL,
      report_date TEXT,
      PRIMARY KEY (fund_code, stock_code, report_date)
    )`);
    db.run(`CREATE TABLE IF NOT EXISTS indices (
      code TEXT PRIMARY KEY, name TEXT, market TEXT,
      price REAL, change_pct REAL, change_amt REAL,
      updated_at TEXT DEFAULT (datetime('now'))
    )`);
    db.run(`CREATE TABLE IF NOT EXISTS stock_profile (
      code TEXT, name TEXT, market TEXT,
      sector TEXT, industry TEXT, market_cap REAL,
      pe REAL, description TEXT,
      PRIMARY KEY (code, market)
    )`);
    db.run(`CREATE TABLE IF NOT EXISTS fund_status (
      fund_code TEXT PRIMARY KEY, purchase_status TEXT, redemption_status TEXT
    )`);
    db.run(`CREATE TABLE IF NOT EXISTS summary_by_fund (
      fund_code TEXT PRIMARY KEY, fund_name TEXT,
      total_shares REAL, total_cost REAL, tx_count INTEGER
    )`);
    // Stock cache tables
    db.run(`CREATE TABLE IF NOT EXISTS stock_realtime (
      code TEXT, market TEXT, name TEXT, price REAL, open REAL, high REAL, low REAL,
      change_pct REAL, change_amt REAL, volume REAL, amount REAL, turnover REAL,
      pe REAL, total_mv REAL, circ_mv REAL, high52 REAL, low52 REAL,
      currency TEXT DEFAULT '', updated_at TEXT DEFAULT (datetime('now')),
      PRIMARY KEY (code, market)
    )`);
    db.run(`CREATE TABLE IF NOT EXISTS stock_kline_cache (
      code TEXT, market TEXT, date TEXT, open REAL, close REAL, high REAL, low REAL,
      volume REAL, amount REAL, amplitude REAL, change_pct REAL, turnover_rate REAL,
      PRIMARY KEY (code, market, date)
    )`);
    // V3: sector classification for penetration analysis
    db.run(`CREATE TABLE IF NOT EXISTS sector_map (
      stock_code TEXT, market TEXT, sector TEXT, industry TEXT,
      PRIMARY KEY (stock_code, market)
    )`);
    safeAlter("ALTER TABLE stock_realtime ADD COLUMN currency TEXT DEFAULT ''");
    // V4: source event queue for Hermes/Agent news & research context
    db.run(`CREATE TABLE IF NOT EXISTS source_events (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      title TEXT NOT NULL,
      url TEXT,
      source TEXT NOT NULL DEFAULT 'websearch',
      snippet TEXT,
      query TEXT,
      related_security_code TEXT,
      related_security_name TEXT,
      is_read INTEGER DEFAULT 0,
      is_useful INTEGER DEFAULT 0,
      fetched_at TEXT NOT NULL DEFAULT (datetime('now')),
      created_at TEXT NOT NULL DEFAULT (datetime('now'))
    )`);
    db.run(`CREATE INDEX IF NOT EXISTS idx_sev_code ON source_events(related_security_code)`);
    db.run(`CREATE INDEX IF NOT EXISTS idx_sev_read ON source_events(is_read)`);
    db.run(`CREATE INDEX IF NOT EXISTS idx_sev_fetched ON source_events(fetched_at)`);
    db.run(`CREATE INDEX IF NOT EXISTS idx_tx_fund ON transactions(fund_code)`);
    db.run(`CREATE INDEX IF NOT EXISTS idx_tx_time ON transactions(trade_time)`);
    db.run(`CREATE INDEX IF NOT EXISTS idx_nav_code ON nav_history(fund_code)`);
    db.run(`CREATE INDEX IF NOT EXISTS idx_nav_date ON nav_history(date)`);
    db.run(`CREATE INDEX IF NOT EXISTS idx_ps_portfolio ON portfolio_snapshot(portfolio_id)`);
    db.run(`CREATE INDEX IF NOT EXISTS idx_skline_code ON stock_kline_cache(code, market)`);
    db.run(`CREATE INDEX IF NOT EXISTS idx_skline_date ON stock_kline_cache(date)`);
    // ── NOT NULL constraints on critical columns ────────────────────
    safeAddNotNulls("transactions", [
      { name: "order_id", nullDefault: "'LEGACY_' || CAST(rowid AS TEXT)" },
      { name: "fund_code", nullDefault: "''" },
      { name: "trade_time", nullDefault: "''" },
      { name: "confirm_amount", nullDefault: "0" },
      { name: "direction", nullDefault: "''" },
    ]);
    safeAddNotNulls("nav_history", [
      { name: "unit_nav", nullDefault: "0" },
    ]);
    safeAddNotNulls("portfolio_snapshot", [
      { name: "held_shares", nullDefault: "0" },
      { name: "total_cost", nullDefault: "0" },
    ]);
    // ── Foreign key constraints ─────────────────────────────────────────
    safeAddForeignKey({ table: "transactions", column: "fund_code", refTable: "fund_details", refColumn: "fund_code", onDelete: "CASCADE" });
    safeAddForeignKey({ table: "nav_history", column: "fund_code", refTable: "fund_details", refColumn: "fund_code" });
    safeAddForeignKey({ table: "portfolio_snapshot", column: "fund_code", refTable: "fund_details", refColumn: "fund_code" });
    // ── Seed sector data ──────────────────────────────────────────────
    try { db.run("INSERT OR IGNORE INTO sector_map (stock_code, market, sector, industry) VALUES ('AAPL','US','Technology','Consumer Electronics'),('MSFT','US','Technology','Software'),('GOOGL','US','Communication','Internet'),('AMZN','US','Consumer','Retail'),('NVDA','US','Technology','Semiconductors'),('META','US','Communication','Social Media'),('TSLA','US','Consumer','Auto'),('AVGO','US','Technology','Semiconductors'),('AMD','US','Technology','Semiconductors'),('NFLX','US','Communication','Entertainment'),('ADBE','US','Technology','Software'),('CRM','US','Technology','Software'),('QCOM','US','Technology','Semiconductors'),('TSM','US','Technology','Semiconductors'),('TXN','US','Technology','Semiconductors'),('COST','US','Consumer','Retail'),('PYPL','US','Financial','Payments'),('CSCO','US','Technology','Networking'),('INTC','US','Technology','Semiconductors'),('ASML','US','Technology','Equipment')"); } catch {}
  } catch (e: any) {
    // DB file is read-only (e.g. mounted :ro) — schema already exists, safe to continue
    if (e.code === "SQLITE_READONLY") return;
    throw e;
  }
}
