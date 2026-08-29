package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
)

// EnsurePGSchema creates the fund-dashboard schema on PostgreSQL.
// Auto-generated from production schema (2026-07-17).
func EnsurePGSchema(ctx context.Context, db *sql.DB) error {
	for i, stmt := range pgSchemaStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("pg schema stmt %d: %w", i, err)
		}
	}
	// Conversion legs intentionally share order_id across two fund_codes.
	// Uniqueness is (order_id, fund_code) — verified 0 true dups on production (#203).
	if _, err := db.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_order_fund_unique
		ON transactions(order_id, fund_code)
	`); err != nil {
		// do not fail boot — import/DCA still use WHERE NOT EXISTS
		slog.Warn("pg unique index transactions(order_id,fund_code) skipped", "error", err)
	}
	migratePortfolioSnapshotPK(ctx, db)
	return nil
}

// migratePortfolioSnapshotPK upgrades legacy PRIMARY KEY (fund_code) to
// PRIMARY KEY (fund_code, portfolio_id) when safe (#262).
func migratePortfolioSnapshotPK(ctx context.Context, db *sql.DB) {
	var condef string
	err := db.QueryRowContext(ctx, `
		SELECT pg_get_constraintdef(c.oid)
		FROM pg_constraint c
		JOIN pg_class t ON c.conrelid = t.oid
		WHERE t.relname = 'portfolio_snapshot' AND c.contype = 'p'
		LIMIT 1
	`).Scan(&condef)
	if err == nil && strings.Contains(condef, "fund_code") && strings.Contains(condef, "portfolio_id") {
		return
	}
	if err != nil && err != sql.ErrNoRows {
		slog.Warn("portfolio_snapshot pk probe skipped", "error", err)
		return
	}
	if _, err := db.ExecContext(ctx, `UPDATE portfolio_snapshot SET portfolio_id = 1 WHERE portfolio_id IS NULL`); err != nil {
		slog.Warn("portfolio_snapshot null portfolio_id fill skipped", "error", err)
		return
	}
	var dups int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT fund_code, COALESCE(portfolio_id,1) AS pid
			FROM portfolio_snapshot
			GROUP BY fund_code, COALESCE(portfolio_id,1)
			HAVING COUNT(*) > 1
		) t
	`).Scan(&dups); err != nil {
		slog.Warn("portfolio_snapshot dup check skipped", "error", err)
		return
	}
	if dups > 0 {
		slog.Warn("portfolio_snapshot composite PK migration skipped: duplicate groups", "groups", dups)
		return
	}
	if _, err := db.ExecContext(ctx, `
		DO $$
		DECLARE
			pk_name text;
		BEGIN
			SELECT c.conname INTO pk_name
			FROM pg_constraint c
			JOIN pg_class t ON c.conrelid = t.oid
			WHERE t.relname = 'portfolio_snapshot' AND c.contype = 'p'
			LIMIT 1;
			IF pk_name IS NOT NULL THEN
				EXECUTE format('ALTER TABLE portfolio_snapshot DROP CONSTRAINT %I', pk_name);
			END IF;
			EXECUTE 'ALTER TABLE portfolio_snapshot ALTER COLUMN portfolio_id SET DEFAULT 1';
			EXECUTE 'ALTER TABLE portfolio_snapshot ALTER COLUMN portfolio_id SET NOT NULL';
			EXECUTE 'ALTER TABLE portfolio_snapshot ADD PRIMARY KEY (fund_code, portfolio_id)';
		END $$;
	`); err != nil {
		slog.Warn("portfolio_snapshot composite PK migration failed", "error", err)
		return
	}
	slog.Info("portfolio_snapshot primary key is (fund_code, portfolio_id)")
}

var pgSchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS agent_audit_events (
		id SERIAL,
		request_id TEXT NOT NULL,
		caller TEXT NOT NULL,
		tool TEXT NOT NULL,
		event_type TEXT NOT NULL,
		status TEXT NOT NULL,
		scope TEXT NOT NULL,
		permission TEXT NOT NULL,
		risk_level TEXT NOT NULL,
		redacted_args_json TEXT NOT NULL,
		result_summary_json TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY (id)
	)`,
	`CREATE TABLE IF NOT EXISTS agent_confirmations (
		id SERIAL,
		tool TEXT NOT NULL,
		token_hash TEXT NOT NULL,
		payload_hash TEXT NOT NULL,
		expires_at TIMESTAMPTZ NOT NULL,
		used_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY (id)
	)`,
	`CREATE TABLE IF NOT EXISTS ant_current_positions (
		fund_code TEXT,
		fund_name TEXT,
		net_share_change DOUBLE PRECISION,
		position_flag TEXT,
		last_trade TEXT,
		raw_json TEXT,
		PRIMARY KEY (fund_code)
	)`,
	`CREATE TABLE IF NOT EXISTS ant_summary_by_fund (
		fund_code TEXT,
		fund_name TEXT,
		tx_count BIGINT,
		real_tx_count BIGINT,
		synthetic_tx_count BIGINT,
		buy_amount DOUBLE PRECISION,
		sell_amount DOUBLE PRECISION,
		dividend_amount DOUBLE PRECISION,
		conversion_in_amount DOUBLE PRECISION,
		conversion_out_amount DOUBLE PRECISION,
		fee_total DOUBLE PRECISION,
		net_share_change DOUBLE PRECISION,
		net_cash_flow DOUBLE PRECISION,
		first_trade TEXT,
		last_trade TEXT,
		notes TEXT,
		PRIMARY KEY (fund_code)
	)`,
	`CREATE TABLE IF NOT EXISTS ant_transactions_normalized (
		seq BIGINT,
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
		apply_amount DOUBLE PRECISION,
		apply_share DOUBLE PRECISION,
		confirm_amount DOUBLE PRECISION,
		confirm_share DOUBLE PRECISION,
		fee DOUBLE PRECISION,
		conversion_value DOUBLE PRECISION,
		inferred_nav DOUBLE PRECISION,
		nav_on_effective_date DOUBLE PRECISION,
		nav_verified BIGINT,
		signed_cash_flow DOUBLE PRECISION,
		signed_share_change DOUBLE PRECISION,
		trade_day_type TEXT,
		effective_nav_date TEXT,
		latest_nav DOUBLE PRECISION,
		cost_basis DOUBLE PRECISION,
		unrealized_pnl DOUBLE PRECISION,
		is_synthetic BIGINT,
		synthetic_reason TEXT,
		source_inference_note TEXT,
		source_page BIGINT,
		source_y0 DOUBLE PRECISION,
		export_seq BIGINT,
		settlement_days BIGINT,
		anomaly TEXT,
		security_type TEXT,
		portfolio_id BIGINT
	)`,
	`CREATE TABLE IF NOT EXISTS ant_transactions_raw (
		seq BIGINT,
		source_page BIGINT,
		source_y0 DOUBLE PRECISION,
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
		rows_added BIGINT,
		latest_date TEXT,
		status TEXT,
		crawled_at TEXT,
		PRIMARY KEY (fund_code)
	)`,
	`CREATE TABLE IF NOT EXISTS dca_plan_executions (
		id SERIAL,
		plan_id BIGINT NOT NULL,
		fund_code TEXT NOT NULL,
		trade_date TEXT NOT NULL,
		amount DOUBLE PRECISION NOT NULL,
		status TEXT NOT NULL,
		order_id TEXT,
		tx_seq BIGINT,
		nav_date TEXT,
		nav DOUBLE PRECISION,
		message TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (id)
	)`,
	`CREATE TABLE IF NOT EXISTS dca_plans (
		id SERIAL,
		fund_code TEXT NOT NULL,
		fund_name TEXT,
		amount DOUBLE PRECISION NOT NULL,
		frequency TEXT NOT NULL,
		weekday_mask TEXT NOT NULL,
		trade_type TEXT NOT NULL,
		portfolio_id BIGINT NOT NULL,
		start_date TEXT NOT NULL,
		end_date TEXT,
		active BIGINT NOT NULL,
		source TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (id)
	)`,
	`CREATE TABLE IF NOT EXISTS fund_details (
		fund_code TEXT,
		fund_name TEXT,
		fund_type TEXT,
		security_type TEXT,
		market TEXT,
		currency TEXT,
		exchange TEXT,
		source TEXT,
		purchase_status TEXT,
		redemption_status TEXT,
		PRIMARY KEY (fund_code)
	)`,
	`CREATE TABLE IF NOT EXISTS fund_holdings (
		fund_code TEXT,
		stock_code TEXT,
		stock_name TEXT,
		weight_pct DOUBLE PRECISION,
		shares DOUBLE PRECISION,
		market_value DOUBLE PRECISION,
		report_date TEXT,
		PRIMARY KEY (fund_code, stock_code, report_date)
	)`,
	`CREATE TABLE IF NOT EXISTS fund_status (
		fund_code TEXT,
		following BOOLEAN DEFAULT true,
		last_followed_at TIMESTAMPTZ DEFAULT now(),
		purchase_status TEXT,
		redemption_status TEXT,
		min_purchase_amount DOUBLE PRECISION,
		daily_limit DOUBLE PRECISION,
		holdings_direction TEXT,
		PRIMARY KEY (fund_code)
	)`,
	`CREATE TABLE IF NOT EXISTS indices (
		code TEXT,
		name TEXT,
		market TEXT,
		price DOUBLE PRECISION,
		change_pct DOUBLE PRECISION,
		change_amt DOUBLE PRECISION,
		updated_at TEXT,
		PRIMARY KEY (code)
	)`,
	`CREATE TABLE IF NOT EXISTS nav_history (
		date TEXT,
		fund_code TEXT,
		unit_nav DOUBLE PRECISION NOT NULL,
		accumulated_nav DOUBLE PRECISION,
		daily_change_pct DOUBLE PRECISION,
		security_type TEXT,
		PRIMARY KEY (date, fund_code)
	)`,
	`CREATE TABLE IF NOT EXISTS portfolio_definitions (
		id SERIAL,
		name TEXT NOT NULL,
		description TEXT,
		created_at TEXT NOT NULL,
		PRIMARY KEY (id)
	)`,
	`CREATE TABLE IF NOT EXISTS portfolio_snapshot (
		fund_code TEXT NOT NULL,
		fund_name TEXT,
		held_shares DOUBLE PRECISION NOT NULL,
		total_cost DOUBLE PRECISION NOT NULL,
		latest_nav DOUBLE PRECISION,
		current_value DOUBLE PRECISION,
		unrealized_pnl DOUBLE PRECISION,
		pnl_pct DOUBLE PRECISION,
		security_type TEXT,
		portfolio_id BIGINT NOT NULL DEFAULT 1,
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
		id SERIAL,
		title TEXT NOT NULL,
		url TEXT,
		source TEXT NOT NULL DEFAULT 'websearch'::text,
		snippet TEXT,
		query TEXT,
		related_security_code TEXT,
		related_security_name TEXT,
		is_read INTEGER DEFAULT 0,
		is_useful INTEGER DEFAULT 0,
		fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY (id)
	)`,
	`CREATE TABLE IF NOT EXISTS stock_kline_cache (
		code TEXT,
		period TEXT DEFAULT 'daily'::text,
		date TEXT,
		open DOUBLE PRECISION,
		high DOUBLE PRECISION,
		low DOUBLE PRECISION,
		close DOUBLE PRECISION,
		volume BIGINT,
		PRIMARY KEY (code, period, date)
	)`,
	`CREATE TABLE IF NOT EXISTS stock_profile (
		code TEXT,
		name TEXT,
		sector TEXT,
		market TEXT DEFAULT 'US'::text,
		updated_at TIMESTAMPTZ DEFAULT now(),
		industry TEXT,
		market_cap DOUBLE PRECISION,
		pe DOUBLE PRECISION,
		description TEXT,
		PRIMARY KEY (code)
	)`,
	`CREATE TABLE IF NOT EXISTS stock_realtime (
		code TEXT,
		name TEXT,
		price DOUBLE PRECISION,
		change_pct DOUBLE PRECISION,
		volume BIGINT,
		updated_at TIMESTAMPTZ DEFAULT now(),
		PRIMARY KEY (code)
	)`,
	`CREATE TABLE IF NOT EXISTS summary_by_fund (
		fund_code TEXT,
		fund_name TEXT,
		total_shares DOUBLE PRECISION,
		total_cost DOUBLE PRECISION,
		tx_count BIGINT,
		PRIMARY KEY (fund_code)
	)`,
	`CREATE TABLE IF NOT EXISTS transactions (
		seq SERIAL,
		source_record_id TEXT,
		order_id TEXT NOT NULL,
		trade_time TEXT NOT NULL,
		confirm_date TEXT,
		trade_type TEXT,
		direction TEXT NOT NULL,
		leg_role TEXT,
		fund_code TEXT NOT NULL,
		fund_name TEXT,
		combo_fund_name TEXT,
		apply_amount DOUBLE PRECISION,
		apply_share DOUBLE PRECISION,
		confirm_amount DOUBLE PRECISION NOT NULL,
		confirm_share DOUBLE PRECISION,
		fee DOUBLE PRECISION,
		conversion_value DOUBLE PRECISION,
		inferred_nav DOUBLE PRECISION,
		nav_on_effective_date DOUBLE PRECISION,
		nav_verified BIGINT,
		signed_cash_flow DOUBLE PRECISION,
		signed_share_change DOUBLE PRECISION,
		trade_day_type TEXT,
		effective_nav_date TEXT,
		latest_nav DOUBLE PRECISION,
		cost_basis DOUBLE PRECISION,
		unrealized_pnl DOUBLE PRECISION,
		is_synthetic BIGINT,
		synthetic_reason TEXT,
		source_inference_note TEXT,
		source_page BIGINT,
		source_y0 DOUBLE PRECISION,
		export_seq BIGINT,
		settlement_days BIGINT,
		anomaly TEXT,
		security_type TEXT,
		portfolio_id BIGINT,
		PRIMARY KEY (seq)
	)`,

	// auth — single-tenant web login (see internal/auth; times are unix epoch).
	`CREATE TABLE IF NOT EXISTS auth_credentials (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		password_hash TEXT NOT NULL,
		created_at BIGINT NOT NULL,
		updated_at BIGINT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS auth_sessions (
		id TEXT PRIMARY KEY,
		created_at BIGINT NOT NULL,
		expires_at BIGINT NOT NULL,
		last_seen_at BIGINT NOT NULL,
		ip TEXT,
		user_agent TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS auth_events (
		id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
		ts BIGINT NOT NULL,
		event TEXT NOT NULL,
		ip TEXT,
		user_agent TEXT,
		detail TEXT
	)`,

	// indexes
	`CREATE INDEX IF NOT EXISTS idx_transactions_fund_code ON transactions(fund_code)`, `CREATE INDEX IF NOT EXISTS idx_transactions_trade_time ON transactions(trade_time)`,
	`CREATE INDEX IF NOT EXISTS idx_nav_history_fund ON nav_history(fund_code)`,
	`CREATE INDEX IF NOT EXISTS idx_nav_history_date ON nav_history(date)`,
	`CREATE INDEX IF NOT EXISTS idx_nav_history_fund_date ON nav_history(fund_code, date DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_dca_plans_active_portfolio ON dca_plans(active, portfolio_id)`,
	`CREATE INDEX IF NOT EXISTS idx_dca_exec_plan_date ON dca_plan_executions(plan_id, trade_date)`,
	`CREATE INDEX IF NOT EXISTS idx_fund_holdings_fund ON fund_holdings(fund_code)`,
	`CREATE INDEX IF NOT EXISTS idx_portfolio_snapshot_portfolio ON portfolio_snapshot(portfolio_id)`,
	`CREATE INDEX IF NOT EXISTS idx_dca_plans_fund ON dca_plans(fund_code)`,
	`CREATE INDEX IF NOT EXISTS idx_agent_confirmations_tool ON agent_confirmations(tool)`,
	`CREATE INDEX IF NOT EXISTS idx_agent_audit_events_tool ON agent_audit_events(tool)`,
	`CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires ON auth_sessions(expires_at)`,
	`CREATE INDEX IF NOT EXISTS idx_auth_events_ts ON auth_events(ts)`,
}
