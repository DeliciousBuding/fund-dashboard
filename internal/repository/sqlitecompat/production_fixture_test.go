package sqlitecompat

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func createProductionShapedFundDashboardFixture(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "fund.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("enable WAL: %v", err)
	}

	for _, stmt := range productionShapedFixtureStatements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec production-shaped fixture statement %q: %v", stmt, err)
		}
	}

	return dbPath
}

var productionShapedFixtureStatements = []string{
	`CREATE TABLE fund_details (
		fund_code TEXT PRIMARY KEY,
		fund_name TEXT,
		fund_type TEXT,
		security_type TEXT DEFAULT 'fund',
		market TEXT DEFAULT '',
		currency TEXT DEFAULT 'CNY',
		exchange TEXT DEFAULT ''
	)`,
	`CREATE TABLE transactions (
		seq INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id TEXT UNIQUE,
		trade_time TEXT,
		confirm_date TEXT,
		trade_type TEXT,
		direction TEXT,
		fund_code TEXT,
		fund_name TEXT,
		confirm_amount REAL,
		confirm_share REAL,
		fee REAL DEFAULT 0,
		inferred_nav REAL,
		nav_on_effective_date REAL,
		nav_verified INTEGER DEFAULT 0,
		signed_cash_flow REAL,
		signed_share_change REAL,
		trade_day_type TEXT,
		settlement_days INTEGER,
		effective_nav_date TEXT,
		latest_nav REAL,
		cost_basis REAL,
		unrealized_pnl REAL DEFAULT 0,
		anomaly TEXT
	)`,
	`CREATE TABLE nav_history (
		date TEXT,
		fund_code TEXT,
		unit_nav REAL,
		daily_change_pct REAL DEFAULT 0,
		security_type TEXT DEFAULT 'fund',
		PRIMARY KEY (fund_code, date)
	)`,
	`CREATE TABLE portfolio_snapshot (
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
			PRIMARY KEY (fund_code, portfolio_id)
		)`,
	`CREATE TABLE portfolio_definitions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE fund_holdings (
		fund_code TEXT,
		stock_code TEXT,
		stock_name TEXT,
		weight_pct REAL,
		shares REAL,
		market_value REAL,
		report_date TEXT,
		PRIMARY KEY (fund_code, stock_code, report_date)
	)`,
	`CREATE TABLE indices (
		code TEXT PRIMARY KEY,
		name TEXT,
		market TEXT,
		price REAL,
		change_pct REAL,
		change_amt REAL,
		updated_at TEXT DEFAULT (datetime('now'))
	)`,
	`CREATE TABLE stock_profile (
		code TEXT,
		name TEXT,
		market TEXT,
		sector TEXT,
		industry TEXT,
		market_cap REAL,
		pe REAL,
		description TEXT,
		PRIMARY KEY (code, market)
	)`,
	`CREATE TABLE fund_status (
		fund_code TEXT PRIMARY KEY,
		purchase_status TEXT,
		redemption_status TEXT
	)`,
	`CREATE TABLE summary_by_fund (
		fund_code TEXT PRIMARY KEY,
		fund_name TEXT,
		total_shares REAL,
		total_cost REAL,
		tx_count INTEGER
	)`,
	`CREATE TABLE stock_realtime (
		code TEXT,
		market TEXT,
		name TEXT,
		price REAL,
		open REAL,
		high REAL,
		low REAL,
		change_pct REAL,
		change_amt REAL,
		volume REAL,
		amount REAL,
		turnover REAL,
		pe REAL,
		total_mv REAL,
		circ_mv REAL,
		high52 REAL,
		low52 REAL,
		currency TEXT DEFAULT '',
		updated_at TEXT DEFAULT (datetime('now')),
		PRIMARY KEY (code, market)
	)`,
	`CREATE TABLE stock_kline_cache (
		code TEXT,
		market TEXT,
		date TEXT,
		open REAL,
		close REAL,
		high REAL,
		low REAL,
		volume REAL,
		amount REAL,
		amplitude REAL,
		change_pct REAL,
		turnover_rate REAL,
		PRIMARY KEY (code, market, date)
	)`,
	`CREATE TABLE sector_map (
		stock_code TEXT,
		market TEXT,
		sector TEXT,
		industry TEXT,
		PRIMARY KEY (stock_code, market)
	)`,
	`CREATE TABLE source_events (
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
	`CREATE TABLE dca_plans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fund_code TEXT NOT NULL,
		fund_name TEXT,
		amount REAL NOT NULL,
		frequency TEXT NOT NULL DEFAULT 'weekday',
		weekday_mask TEXT NOT NULL DEFAULT '1,2,3,4,5',
		trade_type TEXT NOT NULL DEFAULT '定投买入',
		portfolio_id INTEGER NOT NULL DEFAULT 1,
		start_date TEXT NOT NULL,
		end_date TEXT,
		active INTEGER NOT NULL DEFAULT 1,
		source TEXT NOT NULL DEFAULT 'manual',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		UNIQUE (fund_code, portfolio_id, source)
	)`,
	`CREATE TABLE dca_plan_executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plan_id INTEGER NOT NULL,
		fund_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		amount REAL NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending_nav',
		order_id TEXT,
		tx_seq INTEGER,
		nav_date TEXT,
		nav REAL,
		message TEXT,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		UNIQUE (plan_id, trade_date)
	)`,
	`CREATE INDEX idx_sev_code ON source_events(related_security_code)`,
	`CREATE INDEX idx_sev_read ON source_events(is_read)`,
	`CREATE INDEX idx_sev_fetched ON source_events(fetched_at)`,
	`CREATE INDEX idx_dca_plans_active ON dca_plans(active, fund_code)`,
	`CREATE INDEX idx_dca_exec_status ON dca_plan_executions(status, trade_date)`,
	`CREATE INDEX idx_tx_fund ON transactions(fund_code)`,
	`CREATE INDEX idx_tx_time ON transactions(trade_time)`,
	`CREATE INDEX idx_nav_code ON nav_history(fund_code)`,
	`CREATE INDEX idx_nav_date ON nav_history(date)`,
	`CREATE INDEX idx_nav_fund_date ON nav_history(fund_code, date DESC)`,
	`CREATE INDEX idx_tx_fund_time ON transactions(fund_code, trade_time)`,
	`CREATE INDEX idx_ps_portfolio ON portfolio_snapshot(portfolio_id)`,
	`CREATE INDEX idx_skline_code ON stock_kline_cache(code, market)`,
	`CREATE INDEX idx_skline_date ON stock_kline_cache(date)`,
	`INSERT OR IGNORE INTO portfolio_definitions (id, name, description) VALUES (1, 'default', 'Default portfolio')`,
	`INSERT OR REPLACE INTO fund_details (fund_code, fund_name, fund_type, security_type, market, currency, exchange) VALUES
		('019173', 'Nasdaq 100 QDII C', 'QDII-stock', 'fund', 'CN', 'CNY', ''),
		('AAPL', 'Apple Inc.', 'Technology', 'stock', 'US', 'USD', 'NASDAQ')`,
	`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
		VALUES ('FIX-TX-001', '2026-06-01T09:00:00Z', '2026-06-02', 'scheduled_buy', 'buy', '019173', 'Nasdaq 100 QDII C', 100, 85.47, 0.15, -100, 85.47, 2)`,
	`INSERT OR REPLACE INTO nav_history (date, fund_code, unit_nav, daily_change_pct, security_type) VALUES
		('2026-06-01', '019173', 1.1700, 0.5, 'fund'),
		('2026-06-25', '019173', 1.5000, 3.5, 'fund')`,
	`INSERT OR REPLACE INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
		VALUES ('019173', 'Nasdaq 100 QDII C', 85.47, -100, 1.5000, 128.21, 28.21, 28.21, 'fund', 1)`,
	`INSERT OR REPLACE INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date)
		VALUES ('019173', 'AAPL', 'Apple Inc.', 5.10, 60, 10800, '2026-03-31')`,
	`INSERT INTO source_events (title, url, source, snippet, query, related_security_code, related_security_name, is_read, is_useful, fetched_at, created_at)
		VALUES ('Nasdaq research note', 'https://example.test/nasdaq', 'websearch', 'Research summary.', 'Nasdaq holdings', '019173', 'Nasdaq 100 QDII C', 0, 1, '2026-06-25T12:00:00Z', '2026-06-25T12:00:00Z')`,
	`INSERT INTO dca_plans (fund_code, fund_name, amount, frequency, weekday_mask, trade_type, portfolio_id, start_date, active, source)
		VALUES ('019173', 'Nasdaq 100 QDII C', 100, 'weekday', '1,3,5', 'scheduled_buy', 1, '2026-06-01', 1, 'fixture')`,
}
