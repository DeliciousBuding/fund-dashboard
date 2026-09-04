package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/DeliciousBuding/fund-dashboard/internal/dialect"
)

// Schema evolution used to be scattered: two hand-written CREATE TABLE IF NOT
// EXISTS lists (schema_sqlite.go / schema_pg.go) plus three probe-and-ALTER
// repairs (transactions defaults, portfolio_snapshot PK, nav_history columns)
// living in two different packages. Nothing recorded which steps a database
// had already received, index creation was best-effort, and dual-dialect drift
// was prevented only by tests, not by mechanism.
//
// This file collapses schema evolution into one numbered, append-only
// migration list per dialect, applied by ensureSchema and recorded in a
// schema_migrations version table (same shape on SQLite and PostgreSQL).
//
// Rules every migration must follow:
//
//   - Idempotent AND replay-safe against legacy databases that already are at
//     the target structure but have no version-table rows (pre-versioning
//     databases): each apply probes before mutating, so replaying it on an
//     already-migrated database must skip the work and only record the
//     version. This is what makes the first boot after upgrading from a
//     pre-versioning build a no-op.
//   - Append-only: versions are never edited or removed. New steps get the
//     next number and are added to both dialect lists (a step that has nothing
//     to do on one dialect is a record-only no-op there) so the version sets
//     stay aligned across dialects.
//   - A migration failure fails startup. The two exceptions are documented on
//     the migrations themselves: the PG catalog repairs keep their historical
//     warn-and-continue contract, and the (order_id, fund_code) unique index
//     cannot be forced onto legacy rows that already violate it.
//
// A crash between apply and record is safe: the next boot replays the step,
// and every apply is idempotent by construction.

// migration is one numbered schema evolution step.
type migration struct {
	version int
	name    string
	// apply executes the step. Must be idempotent and replay-safe (see package
	// comment). Returning an error fails startup.
	apply func(ctx context.Context, db *sql.DB) error
}

func (m migration) id() string { return fmt.Sprintf("%04d_%s", m.version, m.name) }

// execStatements runs an ordered DDL list, failing on the first error. The
// error carries the failing statement's head so operators can tell which
// table or index could not be created (e.g. a mandatory index referencing a
// column a legacy table lacks).
func execStatements(ctx context.Context, db *sql.DB, stmts []string) error {
	for i, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			head := stmt
			if idx := strings.IndexAny(head, "\n("); idx > 0 {
				head = head[:idx]
			}
			return fmt.Errorf("stmt %d (%s): %w", i, strings.TrimSpace(head), err)
		}
	}
	return nil
}

// baselineApply returns a migration apply that executes an ordered statement
// list with fatal errors. Statement lists stay creation-only (CREATE TABLE /
// CREATE INDEX IF NOT EXISTS — pinned by schema_pg_test.go), so running them
// on a database that already has the objects is a no-op.
func baselineApply(stmts []string) func(context.Context, *sql.DB) error {
	return func(ctx context.Context, db *sql.DB) error {
		return execStatements(ctx, db, stmts)
	}
}

// recordOnlyApply is a migration that has nothing to do on this dialect but is
// recorded so version sets stay aligned across dialects. The step it mirrors
// on the other dialect is named in the reason.
func recordOnlyApply(reason string) func(context.Context, *sql.DB) error {
	return func(context.Context, *sql.DB) error {
		slog.Debug("schema migration record-only on this dialect", "reason", reason)
		return nil
	}
}

// bestEffortApply wraps an apply so a failure logs a warning instead of
// failing startup. Used only for the transactions(order_id, fund_code) unique
// index: legacy conversion legs legitimately share an order_id across two
// fund_codes, so the index can be permanently un-creatable on data imported
// before the uniqueness rule. Import/DCA defend with WHERE NOT EXISTS.
func bestEffortApply(what string, apply func(context.Context, *sql.DB) error) func(context.Context, *sql.DB) error {
	return func(ctx context.Context, db *sql.DB) error {
		if err := apply(ctx, db); err != nil {
			// Behavior change note: regular indexes are enforced by their
			// migrations (failure = boot failure). Only this unique index stays
			// best-effort, decided and accepted when versioning was introduced:
			// legacy duplicate rows would otherwise brick every boot with no
			// in-product repair path.
			slog.Warn("best-effort schema step skipped; will not be retried", "step", what, "error", err)
			return nil
		}
		return nil
	}
}

const createSchemaMigrationsSQLite = `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`

const createSchemaMigrationsPG = `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`

// ensureSchema applies migrations whose version is missing from
// schema_migrations, oldest first, recording each as it lands.
func ensureSchema(ctx context.Context, db *sql.DB, createVersionTable string, migrations []migration) error {
	if _, err := db.ExecContext(ctx, createVersionTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	applied, err := loadAppliedVersions(ctx, db)
	if err != nil {
		return fmt.Errorf("load schema_migrations: %w", err)
	}
	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if err := m.apply(ctx, db); err != nil {
			return fmt.Errorf("migration %s: %w", m.id(), err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`,
			m.version, m.name,
		); err != nil {
			return fmt.Errorf("record migration %s: %w", m.id(), err)
		}
		slog.Info("schema migration applied", "version", m.version, "name", m.name)
	}
	return nil
}

func loadAppliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applied := map[int]bool{}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

// sqliteMigrations is the SQLite evolution list. Baseline 0001 is the
// historical EnsureSQLiteSchema statement list; 0002/0003 are record-only
// mirrors of PG catalog repairs SQLite never needed; 0004 absorbs the
// nav_history column backfill that used to live in jobs/nav_schema.go.
var sqliteMigrations = []migration{
	// Behavior change (accepted when versioning was introduced): the index
	// list below used to be best-effort with a startup warn, so a legacy
	// database missing an indexed column booted forever without that index.
	// Index creation now fails startup instead, so the defect surfaces on the
	// first boot rather than lurking as a silent performance regression.
	{version: 1, name: "baseline_schema", apply: baselineApply(append(append([]string{}, sqliteSchemaTables...), sqliteSchemaIndexes...))},
	// SQLite declares transactions.portfolio_id/security_type defaults in the
	// 0001 CREATE TABLE (and legacy databases received them from the
	// historical ALTER TABLE ... ADD COLUMN), so the PG catalog repair has no
	// SQLite work.
	{version: 2, name: "transactions_column_defaults", apply: recordOnlyApply("defaults declared by baseline CREATE TABLE on sqlite")},
	// SQLite fresh schema already uses the composite PK and legacy single-PK
	// tables are left untouched by design (see schema_sqlite.go doc).
	{version: 3, name: "portfolio_snapshot_composite_pk", apply: recordOnlyApply("sqlite legacy single-PK snapshot tables are not rebuilt")},
	{version: 4, name: "nav_history_security_columns", apply: ensureNavHistoryColumns(sqliteNavHistoryColumnDefs)},
	{version: 5, name: "transactions_order_fund_unique_index", apply: bestEffortApply("sqlite idx_transactions_order_fund_unique", baselineApply([]string{`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_order_fund_unique
		ON transactions(order_id, fund_code)
	`}))},
}

// pgMigrations is the PostgreSQL evolution list. 0001 is the historical
// EnsurePGSchema statement list; 0002/0003 wrap the historical probe-and-ALTER
// catalog repairs verbatim; 0004 absorbs jobs/nav_schema.go.
var pgMigrations = []migration{
	{version: 1, name: "baseline_schema", apply: baselineApply(pgSchemaStatements)},
	// migrateTransactionsDefaults and migratePortfolioSnapshotPK keep their
	// historical contract: a probe or repair failure degrades to a warning and
	// never blocks boot. The migration is recorded either way, matching the
	// pre-versioning behavior where every boot re-probed.
	{version: 2, name: "transactions_column_defaults", apply: func(ctx context.Context, db *sql.DB) error {
		migrateTransactionsDefaults(ctx, db)
		return nil
	}},
	{version: 3, name: "portfolio_snapshot_composite_pk", apply: func(ctx context.Context, db *sql.DB) error {
		migratePortfolioSnapshotPK(ctx, db)
		return nil
	}},
	{version: 4, name: "nav_history_security_columns", apply: ensureNavHistoryColumns(pgNavHistoryColumnDefs)},
	{version: 5, name: "transactions_order_fund_unique_index", apply: bestEffortApply("pg idx_transactions_order_fund_unique", baselineApply([]string{`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_order_fund_unique
		ON transactions(order_id, fund_code)
	`}))},
}

// ── nav_history column backfill (absorbed from internal/jobs/nav_schema.go) ──

// navHistoryColumnDefs maps a missing nav_history column to its ADD COLUMN
// body. The SQLite defs use REAL (modernc SQLite type); PG uses DOUBLE
// PRECISION to match the 0001 CREATE TABLE column type.
var (
	sqliteNavHistoryColumnDefs = map[string]string{
		"daily_change_pct": "REAL DEFAULT 0",
		"security_type":    "TEXT DEFAULT 'fund'",
	}
	pgNavHistoryColumnDefs = map[string]string{
		"daily_change_pct": "DOUBLE PRECISION DEFAULT 0",
		"security_type":    "TEXT DEFAULT 'fund'",
	}
)

// EnsureNavHistoryColumns adds any missing nav_history security-era columns.
// It is the single implementation behind migration 0004 and the lazy
// jobs.PriceRefresher backfill (which exists only for databases/tests that
// never ran EnsureSchema). Both dialects: the PRAGMA probe answers on SQLite
// and falls back to information_schema on PostgreSQL. Duplicate-column races
// are success, via the dialect single source.
func EnsureNavHistoryColumns(ctx context.Context, db *sql.DB) error {
	return ensureNavHistoryColumns(sqliteNavHistoryColumnDefs)(ctx, db)
}

// ensureNavHistoryColumns probes nav_history once and adds only the columns
// the table is missing, so replaying it on a table that already has the
// security-era columns (fresh 0001 tables, migrated legacy tables) is a pure
// probe with no writes.
func ensureNavHistoryColumns(defs map[string]string) func(context.Context, *sql.DB) error {
	return func(ctx context.Context, db *sql.DB) error {
		cols, err := navHistoryColumns(ctx, db)
		if err != nil {
			return fmt.Errorf("list nav_history columns: %w", err)
		}
		var firstErr error
		for _, name := range []string{"daily_change_pct", "security_type"} {
			if _, ok := cols[name]; ok {
				continue
			}
			if err := addNavHistoryColumn(ctx, db, name, defs[name]); err != nil {
				if firstErr == nil {
					firstErr = err
				}
			}
		}
		return firstErr
	}
}

// navHistoryColumns returns the lowercased column names of nav_history.
// SQLite PRAGMA first (local/CI); fall back to PG information_schema.
func navHistoryColumns(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	out := map[string]struct{}{}

	if err := ctx.Err(); err != nil {
		return out, err
	}
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(nav_history)`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				return nil, err
			}
			out[strings.ToLower(name)] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if len(out) > 0 {
			return out, nil
		}
	} else if ctx.Err() != nil {
		return out, ctx.Err()
	} else {
		// PRAGMA fails on PG (or missing table) — fall through to information_schema.
		slog.Debug("nav_history PRAGMA table_info failed; trying information_schema", "error", err)
	}

	rows, err = db.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'nav_history'
	`)
	if err != nil {
		return out, fmt.Errorf("list nav_history columns: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[strings.ToLower(name)] = struct{}{}
	}
	return out, rows.Err()
}

func addNavHistoryColumn(ctx context.Context, db *sql.DB, name, def string) error {
	stmt := fmt.Sprintf("ALTER TABLE nav_history ADD COLUMN %s %s", dialect.QuoteIdentifier(name), def)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		// Concurrent/race: column may already exist after probe; treat as success.
		if dialect.IsDuplicateColumnError(err) {
			return nil
		}
		return fmt.Errorf("add column %s: %w", name, err)
	}
	slog.Info("nav_history column backfilled", "column", name)
	return nil
}
