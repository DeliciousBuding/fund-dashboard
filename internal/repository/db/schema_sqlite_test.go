package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestEnsureSQLiteSchemaCreatesAllTablesIdempotently(t *testing.T) {
	ctx := context.Background()
	dbi, err := Open(ctx, Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "first.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dbi.Close()

	for i := 0; i < 2; i++ {
		if err := EnsureSQLiteSchema(ctx, dbi); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}

	var tables int
	if err := dbi.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
	`).Scan(&tables); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if tables < 20 {
		t.Fatalf("tables = %d, want >= 20", tables)
	}

	for _, name := range []string{
		"transactions", "portfolio_snapshot", "nav_history", "dca_plans",
		"dca_plan_executions", "source_events", "fund_details", "fund_holdings",
		"indices", "stock_realtime", "stock_kline_cache", "stock_profile",
		"summary_by_fund", "sector_map", "crawl_log", "qa_report",
		"ant_current_positions", "ant_summary_by_fund", "ant_transactions_normalized",
		"ant_transactions_raw", "portfolio_definitions", "fund_status",
	} {
		var one int
		if err := dbi.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?
		`, name).Scan(&one); err != nil {
			t.Fatalf("check table %s: %v", name, err)
		}
		if one != 1 {
			t.Fatalf("table %q missing after EnsureSQLiteSchema", name)
		}
	}

	// spot-check key indexes
	for _, idx := range []string{
		"idx_transactions_fund_code", "idx_nav_history_fund", "idx_dca_plans_active_portfolio",
		"idx_dca_exec_plan_date", "idx_portfolio_snapshot_portfolio", "idx_transactions_order_fund_unique",
	} {
		var one int
		if err := dbi.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?
		`, idx).Scan(&one); err != nil {
			t.Fatalf("check index %s: %v", idx, err)
		}
		if one != 1 {
			t.Fatalf("index %q missing after EnsureSQLiteSchema", idx)
		}
	}
}

func TestEnsureSQLiteSchemaWriteRoundTrip(t *testing.T) {
	ctx := context.Background()
	dbi, err := Open(ctx, Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "roundtrip.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dbi.Close()
	if err := EnsureSQLiteSchema(ctx, dbi); err != nil {
		t.Fatalf("EnsureSQLiteSchema: %v", err)
	}

	if _, err := dbi.ExecContext(ctx, `
		INSERT INTO transactions (order_id, trade_time, direction, fund_code, confirm_amount, settlement_days)
		VALUES ('T1', '2026-08-30T09:00:00Z', 'buy', '019173', 100, 2)
	`); err != nil {
		t.Fatalf("insert transaction: %v", err)
	}
	var count int
	if err := dbi.QueryRowContext(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&count); err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	if count != 1 {
		t.Fatalf("transactions count = %d, want 1", count)
	}

	// portfolio_snapshot composite PK (fund_code, portfolio_id) accepts two rows
	// for the same fund under different portfolios.
	for _, pid := range []int{1, 2} {
		if _, err := dbi.ExecContext(ctx, `
			INSERT INTO portfolio_snapshot (fund_code, held_shares, total_cost, portfolio_id)
			VALUES ('019173', 10, -100, ?)
		`, pid); err != nil {
			t.Fatalf("insert snapshot pid=%d: %v", pid, err)
		}
	}
	// but not the same (fund_code, portfolio_id) twice
	if _, err := dbi.ExecContext(ctx, `
		INSERT INTO portfolio_snapshot (fund_code, held_shares, total_cost, portfolio_id)
		VALUES ('019173', 11, -110, 1)
	`); err == nil {
		t.Fatalf("expected composite PK violation for duplicate (fund_code, portfolio_id)")
	}
}

func TestEnsureSQLiteSchemaNoSideEffectsOnLegacyDB(t *testing.T) {
	ctx := context.Background()
	dbi, err := Open(ctx, Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "legacy.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dbi.Close()

	// legacy Python-era shapes: single-column portfolio_snapshot PK and a
	// transactions table that already violates (order_id, fund_code) uniqueness.
	// The snapshot table carries the indexed portfolio_id column: indexes are
	// mandatory since versioning landed, so a legacy table missing an indexed
	// column now fails boot (pinned by TestEnsureSQLiteSchemaEnforcesIndexes);
	// this fixture represents a DB the mandatory indexes can still build on.
	// intentionally legacy schema for pre-versioning single-PK snapshot shape
	if _, err := dbi.ExecContext(ctx, `
		CREATE TABLE portfolio_snapshot (
			fund_code TEXT PRIMARY KEY,
			fund_name TEXT,
			held_shares REAL,
			portfolio_id INTEGER NOT NULL DEFAULT 1
		)
	`); err != nil {
		t.Fatalf("create legacy snapshot: %v", err)
	}
	// intentionally legacy schema for pre-uniqueness transactions shape
	if _, err := dbi.ExecContext(ctx, `
		CREATE TABLE transactions (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id TEXT,
			trade_time TEXT,
			fund_code TEXT,
			confirm_amount REAL
		);
		INSERT INTO transactions (order_id, trade_time, fund_code, confirm_amount)
		VALUES ('dup', '2026-01-01', '019173', 1), ('dup', '2026-01-01', '019173', 2);
	`); err != nil {
		t.Fatalf("create legacy transactions: %v", err)
	}

	if err := EnsureSQLiteSchema(ctx, dbi); err != nil {
		t.Fatalf("EnsureSQLiteSchema on legacy DB must not fail: %v", err)
	}

	// legacy single-column PK survives (no re-create of the table).
	if _, err := dbi.ExecContext(ctx, `
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares)
		VALUES ('A', 'x', 1), ('B', 'y', 2)
	`); err != nil {
		t.Fatalf("insert into legacy snapshot: %v", err)
	}
	if _, err := dbi.ExecContext(ctx, `
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares)
		VALUES ('A', 'zzz', 3)
	`); err == nil {
		t.Fatalf("legacy snapshot single-column PK was replaced; duplicate fund_code accepted")
	}
	// duplicate legacy transactions rows still present, count intact
	var rows int
	if err := dbi.QueryRowContext(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&rows); err != nil {
		t.Fatalf("count legacy transactions: %v", err)
	}
	if rows != 2 {
		t.Fatalf("legacy transactions count = %d, want 2 (no unique index enforced)", rows)
	}
	// reads still work on the untouched legacy transactions shape
	var ddl string
	if err := dbi.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'transactions'
	`).Scan(&ddl); err != nil {
		t.Fatalf("read transactions ddl: %v", err)
	}
	if !strings.Contains(ddl, "seq INTEGER PRIMARY KEY AUTOINCREMENT") {
		t.Fatalf("legacy transactions DDL changed: %s", ddl)
	}
}

// TestEnsureSQLiteSchemaAppliesTransactionsColumnDefaults pins dialect parity
// with schema_pg.go: an insert that omits portfolio_id and security_type must
// land on 1 and 'fund'. That is what legacy databases already got from
// ALTER TABLE ... ADD COLUMN portfolio_id INTEGER DEFAULT 1, and what
// admin.Service.ImportTransactions relies on because its column list has never
// included either field.
func TestEnsureSQLiteSchemaAppliesTransactionsColumnDefaults(t *testing.T) {
	ctx := context.Background()
	dbi, err := Open(ctx, Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "defaults.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dbi.Close()
	if err := EnsureSQLiteSchema(ctx, dbi); err != nil {
		t.Fatalf("EnsureSQLiteSchema: %v", err)
	}

	if _, err := dbi.ExecContext(ctx, `
		INSERT INTO transactions (order_id, trade_time, direction, fund_code, confirm_amount)
		VALUES ('pdf-00000-019173', '2026-08-30 09:00:00', 'buy', '019173', 100)
	`); err != nil {
		t.Fatalf("insert transaction omitting portfolio_id/security_type: %v", err)
	}

	var portfolioID int64
	var securityType string
	if err := dbi.QueryRowContext(ctx,
		`SELECT portfolio_id, security_type FROM transactions WHERE order_id = 'pdf-00000-019173'`,
	).Scan(&portfolioID, &securityType); err != nil {
		t.Fatalf("read back defaults: %v", err)
	}
	if portfolioID != 1 {
		t.Fatalf("portfolio_id = %d, want the DEFAULT 1", portfolioID)
	}
	if securityType != "fund" {
		t.Fatalf("security_type = %q, want the DEFAULT 'fund'", securityType)
	}
}
