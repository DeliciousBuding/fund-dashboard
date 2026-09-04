package db

import (
	"context"
	"database/sql"
)

// EnsureSQLiteSchema creates the fund-dashboard business tables on SQLite
// for public self-hosted first installs: an empty database must boot into a
// usable empty state instead of a wall of internal_error 500s.
//
// Shape rules (see deploy/ci-seed.sql as the legacy SQLite reference and
// schema_pg.go as the field-complete reference; on conflict the ci-seed shape
// wins so legacy read paths stay compatible):
//   - TEXT/REAL/INTEGER/BIGINT map to TEXT/REAL/INTEGER;
//   - serial → INTEGER PRIMARY KEY AUTOINCREMENT;
//   - TIMESTAMPTZ/time text columns stay TEXT (legacy DBs store date strings);
//   - PG-only columns missing from ci-seed are added (e.g. transactions'
//     anomaly/portfolio_id — readers adapt via schema_meta probes).
//
// portfolio_snapshot uses the (fund_code, portfolio_id) composite primary key
// like PG; ci-seed's single-column PK is a legacy defect not duplicated here.
//
// EnsureSQLiteSchema brings a SQLite database to the current schema through
// the numbered migration list (migrate.go): the 0001 baseline executes the
// CREATE TABLE list below, later migrations carry the probe-and-ALTER repairs,
// and every applied step is recorded in schema_migrations.
//
// Every baseline statement is idempotent (IF NOT EXISTS) and never alters
// existing tables: column-level evolution on legacy DBs goes through numbered
// migrations, the successor of the old schema_meta probing pattern.
//
// Behavior change (accepted when versioning was introduced): the non-unique
// indexes are now enforced by migration 0001 — a failure fails startup
// instead of logging a best-effort warn, so a legacy DB missing an indexed
// column surfaces the defect on first boot. Only the (order_id, fund_code)
// unique index stays best-effort (migration 0005): legacy conversion legs can
// permanently violate it (import/DCA still use WHERE NOT EXISTS).
func EnsureSQLiteSchema(ctx context.Context, db *sql.DB) error {
	return ensureSchema(ctx, db, createSchemaMigrationsSQLite, sqliteMigrations)
}

var sqliteSchemaTables = []string{
	`CREATE TABLE IF NOT EXISTS ant_current_positions (
		fund_code TEXT,
		fund_name TEXT,
		net_share_change REAL,
		position_flag TEXT,
		last_trade TEXT,
		raw_json TEXT,
		PRIMARY KEY (fund_code)
	)`,
	`CREATE TABLE IF NOT EXISTS ant_summary_by_fund (
		fund_code TEXT,
		fund_name TEXT,
		tx_count INTEGER,
		real_tx_count INTEGER,
		synthetic_tx_count INTEGER,
		buy_amount REAL,
		sell_amount REAL,
		dividend_amount REAL,
		conversion_in_amount REAL,
		conversion_out_amount REAL,
		fee_total REAL,
		net_share_change REAL,
		net_cash_flow REAL,
		first_trade TEXT,
		last_trade TEXT,
		notes TEXT,
		PRIMARY KEY (fund_code)
	)`,
	`CREATE TABLE IF NOT EXISTS ant_transactions_normalized (
		seq INTEGER,
		source_record_id TEXT,
		order_id TEXT,
		trade_time TEXT,
		confirm_date TEXT,
		trade_type TEXT,
		direction TEXT,
		leg_role TEXT,
		fund_code TEXT,
		fund_name TEXT,
		combo_fund_name TEXT,
		apply_amount REAL,
		apply_share REAL,
		confirm_amount REAL,
		confirm_share REAL,
		fee REAL,
		conversion_value REAL,
		inferred_nav REAL,
		nav_on_effective_date REAL,
		nav_verified INTEGER,
		signed_cash_flow REAL,
		signed_share_change REAL,
		trade_day_type TEXT,
		effective_nav_date TEXT,
		latest_nav REAL,
		cost_basis REAL,
		unrealized_pnl REAL,
		is_synthetic INTEGER,
		synthetic_reason TEXT,
		source_inference_note TEXT,
		source_page INTEGER,
		source_y0 REAL,
		export_seq INTEGER,
		settlement_days INTEGER,
		anomaly TEXT,
		security_type TEXT,
		portfolio_id INTEGER
	)`,
	`CREATE TABLE IF NOT EXISTS ant_transactions_raw (
		seq INTEGER,
		source_page INTEGER,
		source_y0 REAL,
		order_cell TEXT,
		trade_time_cell TEXT,
		trade_type_cell TEXT,
		fund_name_cell TEXT,
		combo_fund_name_cell TEXT,
		fund_code_cell TEXT,
		apply_amount_cell TEXT,
		apply_share_cell TEXT,
		confirm_amount_cell TEXT,
		confirm_share_cell TEXT,
		fee_cell TEXT,
		confirm_date_cell TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS crawl_log (
		fund_code TEXT,
		source TEXT,
		rows_added INTEGER,
		latest_date TEXT,
		status TEXT,
		crawled_at TEXT,
		PRIMARY KEY (fund_code)
	)`,
	`CREATE TABLE IF NOT EXISTS dca_plan_executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plan_id INTEGER NOT NULL,
		fund_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		amount REAL NOT NULL,
		status TEXT NOT NULL,
		order_id TEXT,
		tx_seq INTEGER,
		nav_date TEXT,
		nav REAL,
		message TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS dca_plans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fund_code TEXT NOT NULL,
		fund_name TEXT,
		amount REAL NOT NULL,
		frequency TEXT NOT NULL,
		weekday_mask TEXT NOT NULL,
		trade_type TEXT NOT NULL,
		portfolio_id INTEGER NOT NULL,
		start_date TEXT NOT NULL,
		end_date TEXT,
		active INTEGER NOT NULL,
		source TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS fund_details (
		fund_code TEXT PRIMARY KEY,
		fund_name TEXT,
		fund_type TEXT,
		security_type TEXT DEFAULT 'fund',
		market TEXT DEFAULT '',
		currency TEXT,
		exchange TEXT,
		source TEXT,
		purchase_status TEXT,
		redemption_status TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS fund_holdings (
		fund_code TEXT,
		stock_code TEXT,
		stock_name TEXT,
		weight_pct REAL,
		shares REAL,
		market_value REAL,
		report_date TEXT,
		PRIMARY KEY (fund_code, stock_code, report_date)
	)`,
	`CREATE TABLE IF NOT EXISTS fund_status (
		fund_code TEXT PRIMARY KEY,
		following INTEGER DEFAULT 1,
		last_followed_at TEXT DEFAULT (datetime('now')),
		purchase_status TEXT,
		redemption_status TEXT,
		min_purchase_amount REAL,
		daily_limit REAL,
		holdings_direction TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS indices (
		code TEXT PRIMARY KEY,
		name TEXT,
		market TEXT,
		price REAL,
		change_pct REAL,
		change_amt REAL,
		updated_at TEXT DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS nav_history (
		fund_code TEXT,
		date TEXT,
		unit_nav REAL,
		daily_change_pct REAL DEFAULT 0,
		security_type TEXT DEFAULT 'fund',
		accumulated_nav REAL,
		PRIMARY KEY (fund_code, date)
	)`,
	`CREATE TABLE IF NOT EXISTS portfolio_definitions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		description TEXT,
		created_at TEXT DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS portfolio_snapshot (
		fund_code TEXT NOT NULL,
		fund_name TEXT,
		held_shares REAL,
		total_cost REAL,
		latest_nav REAL,
		current_value REAL,
		unrealized_pnl REAL,
		pnl_pct REAL,
		security_type TEXT DEFAULT 'fund',
		portfolio_id INTEGER NOT NULL DEFAULT 1,
		position_flag TEXT,
		last_trade TEXT,
		PRIMARY KEY (fund_code, portfolio_id)
	)`,
	`CREATE TABLE IF NOT EXISTS qa_report (
		key TEXT,
		value TEXT,
		PRIMARY KEY (key)
	)`,
	`CREATE TABLE IF NOT EXISTS sector_map (
		stock_code TEXT,
		market TEXT,
		sector TEXT,
		industry TEXT,
		PRIMARY KEY (stock_code, market)
	)`,
	`CREATE TABLE IF NOT EXISTS source_events (
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
	)`,
	`CREATE TABLE IF NOT EXISTS stock_kline_cache (
		code TEXT,
		period TEXT DEFAULT 'daily',
		date TEXT,
		open REAL,
		high REAL,
		low REAL,
		close REAL,
		volume INTEGER,
		PRIMARY KEY (code, period, date)
	)`,
	`CREATE TABLE IF NOT EXISTS stock_profile (
		code TEXT PRIMARY KEY,
		name TEXT,
		sector TEXT,
		market TEXT DEFAULT 'US',
		updated_at TEXT DEFAULT (datetime('now')),
		industry TEXT,
		market_cap REAL,
		pe REAL,
		description TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS stock_realtime (
		code TEXT PRIMARY KEY,
		name TEXT,
		price REAL,
		change_pct REAL,
		volume INTEGER,
		updated_at TEXT DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE IF NOT EXISTS summary_by_fund (
		fund_code TEXT,
		fund_name TEXT,
		total_shares REAL,
		total_cost REAL,
		tx_count INTEGER,
		PRIMARY KEY (fund_code)
	)`,
	`CREATE TABLE IF NOT EXISTS transactions (
		seq INTEGER PRIMARY KEY AUTOINCREMENT,
		source_record_id TEXT,
		order_id TEXT,
		trade_time TEXT,
		confirm_date TEXT,
		trade_type TEXT,
		direction TEXT,
		leg_role TEXT,
		fund_code TEXT,
		fund_name TEXT,
		combo_fund_name TEXT,
		apply_amount REAL,
		apply_share REAL,
		confirm_amount REAL,
		confirm_share REAL,
		fee REAL,
		conversion_value REAL,
		inferred_nav REAL,
		nav_on_effective_date REAL,
		nav_verified INTEGER,
		signed_cash_flow REAL,
		signed_share_change REAL,
		trade_day_type TEXT,
		effective_nav_date TEXT,
		latest_nav REAL,
		cost_basis REAL,
		unrealized_pnl REAL,
		is_synthetic INTEGER,
		synthetic_reason TEXT,
		source_inference_note TEXT,
		source_page INTEGER,
		source_y0 REAL,
		export_seq INTEGER,
		settlement_days INTEGER,
		anomaly TEXT,
		security_type TEXT DEFAULT 'fund',
		portfolio_id INTEGER DEFAULT 1
	)`,
}

// sqliteSchemaIndexes mirrors schema_pg.go idx_* (minus auth/agent tables,
// which have their own EnsureSchema). Executed by migration 0001 with fatal
// errors: a legacy DB missing a referenced column now fails startup instead
// of booting without the index (accepted behavior change, see
// EnsureSQLiteSchema).
var sqliteSchemaIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_transactions_fund_code ON transactions(fund_code)`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_trade_time ON transactions(trade_time)`,
	`CREATE INDEX IF NOT EXISTS idx_nav_history_fund ON nav_history(fund_code)`,
	`CREATE INDEX IF NOT EXISTS idx_nav_history_date ON nav_history(date)`,
	`CREATE INDEX IF NOT EXISTS idx_nav_history_fund_date ON nav_history(fund_code, date DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_dca_plans_active_portfolio ON dca_plans(active, portfolio_id)`,
	`CREATE INDEX IF NOT EXISTS idx_dca_exec_plan_date ON dca_plan_executions(plan_id, trade_date)`,
	`CREATE INDEX IF NOT EXISTS idx_fund_holdings_fund ON fund_holdings(fund_code)`,
	`CREATE INDEX IF NOT EXISTS idx_portfolio_snapshot_portfolio ON portfolio_snapshot(portfolio_id)`,
	`CREATE INDEX IF NOT EXISTS idx_dca_plans_fund ON dca_plans(fund_code)`,
}
