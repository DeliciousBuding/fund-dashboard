package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/dialect"
	_ "modernc.org/sqlite"
)

// ── version table + migration list pins ────────────────────────────────────

// The two dialect lists must stay aligned: same versions, same names, in
// order. A step that has nothing to do on one dialect is a record-only no-op
// there, but the version sets must never diverge.
func TestMigrationListsAreAlignedAcrossDialects(t *testing.T) {
	if len(sqliteMigrations) != len(pgMigrations) {
		t.Fatalf("migration count drifted: sqlite=%d pg=%d", len(sqliteMigrations), len(pgMigrations))
	}
	for i, sm := range sqliteMigrations {
		pm := pgMigrations[i]
		if sm.version != pm.version || sm.name != pm.name {
			t.Fatalf("migration %d drifted: sqlite=%s pg=%s", i, sm.id(), pm.id())
		}
		if sm.version != i+1 {
			t.Fatalf("sqlite migration %d has version %d, want dense numbering from 1", i, sm.version)
		}
	}
}

// nav_schema.go's duplicate-column single source moved here with the nav
// backfill (ported from jobs/nav_schema_dupcol_test.go): the migration code
// must classify ADD COLUMN races via dialect.IsDuplicateColumnError, never an
// inline substring match.
func TestNavMigrationUsesDialectDuplicateColumnSource(t *testing.T) {
	raw, err := os.ReadFile("migrate.go")
	if err != nil {
		t.Fatalf("read migrate.go: %v", err)
	}
	src := string(raw)
	if strings.Contains(src, "isDuplicateColumnErr") {
		t.Fatal("migrate.go still defines/uses local isDuplicateColumnErr")
	}
	if !strings.Contains(src, "dialect.IsDuplicateColumnError") {
		t.Fatal("migrate.go must call dialect.IsDuplicateColumnError")
	}
}

// ── fresh database: one shot creates everything and records every version ──

func TestEnsureSQLiteSchemaFreshDBRecordsAllMigrations(t *testing.T) {
	ctx := context.Background()
	dbi, err := Open(ctx, Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fresh.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dbi.Close()

	if err := EnsureSQLiteSchema(ctx, dbi); err != nil {
		t.Fatalf("EnsureSQLiteSchema: %v", err)
	}

	rows, err := dbi.QueryContext(ctx, `SELECT version, name FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	defer rows.Close()
	var versions []int
	var names []string
	for rows.Next() {
		var v int
		var name string
		if err := rows.Scan(&v, &name); err != nil {
			t.Fatalf("scan schema_migrations: %v", err)
		}
		versions = append(versions, v)
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("schema_migrations rows: %v", err)
	}
	wantNames := []string{
		"baseline_schema",
		"transactions_column_defaults",
		"portfolio_snapshot_composite_pk",
		"nav_history_security_columns",
		"transactions_order_fund_unique_index",
	}
	if len(versions) != len(wantNames) {
		t.Fatalf("recorded versions = %v, want %d dense rows", versions, len(wantNames))
	}
	for i, want := range wantNames {
		if versions[i] != i+1 || names[i] != want {
			t.Fatalf("schema_migrations row %d = (%d, %s), want (%d, %s)", i, versions[i], names[i], i+1, want)
		}
	}

	// A fresh database must also end up structurally complete: the baseline
	// tables and the enforced indexes all exist (spot checks; full table list
	// is pinned by TestEnsureSQLiteSchemaCreatesAllTablesIdempotently).
	for _, obj := range []struct{ kind, name string }{
		{"table", "transactions"}, {"table", "nav_history"}, {"table", "schema_migrations"},
		{"index", "idx_portfolio_snapshot_portfolio"}, {"index", "idx_transactions_order_fund_unique"},
	} {
		var one int
		if err := dbi.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?
		`, obj.kind, obj.name).Scan(&one); err != nil {
			t.Fatalf("check %s %s: %v", obj.kind, obj.name, err)
		}
		if one != 1 {
			t.Fatalf("%s %q missing after EnsureSQLiteSchema on fresh DB", obj.kind, obj.name)
		}
	}
}

// ── legacy database: replay-safe probe-and-record ───────────────────────────

func TestEnsureSQLiteSchemaLegacyDBMigratesAndRecordsWithoutError(t *testing.T) {
	ctx := context.Background()
	dbi, err := Open(ctx, Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "legacy.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dbi.Close()

	// Old-way shapes:
	//   - portfolio_snapshot with the legacy single-column PK (ci-seed defect)
	//     but WITH the indexed portfolio_id column (a database that lacks it
	//     now fails boot by the accepted mandatory-index contract — pinned by
	//     TestEnsureSQLiteSchemaEnforcesIndexes);
	//   - transactions predating the (order_id, fund_code) uniqueness rule,
	//     with duplicate rows already on disk;
	//   - nav_history predating the security-type era (no daily_change_pct /
	//     security_type columns).
	// Deliberately legacy shapes — EnsureSQLiteSchema must evolve, not rebuild,
	// these tables.
	stmts := []string{
		`CREATE TABLE portfolio_snapshot (
			fund_code TEXT PRIMARY KEY,
			fund_name TEXT,
			held_shares REAL,
			portfolio_id INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE transactions (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id TEXT,
			trade_time TEXT,
			fund_code TEXT,
			confirm_amount REAL
		)`,
		`INSERT INTO transactions (order_id, trade_time, fund_code, confirm_amount)
			VALUES ('dup', '2026-01-01', '019173', 1), ('dup', '2026-01-01', '019173', 2)`,
		`CREATE TABLE nav_history (
			fund_code TEXT,
			date TEXT,
			unit_nav REAL,
			PRIMARY KEY (fund_code, date)
		)`,
	}
	for i, stmt := range stmts {
		if _, err := dbi.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("legacy stmt %d: %v", i, err)
		}
	}

	// First boot after upgrading from a pre-versioning build: every migration
	// must probe, do only the missing work, and record — never error.
	if err := EnsureSQLiteSchema(ctx, dbi); err != nil {
		t.Fatalf("EnsureSQLiteSchema on legacy DB: %v", err)
	}

	var versions int
	if err := dbi.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if versions != len(sqliteMigrations) {
		t.Fatalf("recorded versions = %d, want %d", versions, len(sqliteMigrations))
	}

	// nav_history was evolved in place: the security-era columns now exist and
	// accept defaulted writes.
	var cols int
	if err := dbi.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('nav_history')
		WHERE name IN ('daily_change_pct', 'security_type')
	`).Scan(&cols); err != nil {
		t.Fatalf("probe nav_history columns: %v", err)
	}
	if cols != 2 {
		t.Fatalf("nav_history security-era columns = %d, want 2", cols)
	}
	if _, err := dbi.ExecContext(ctx, `
		INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('019173', '2026-09-01', 1.5)
	`); err != nil {
		t.Fatalf("insert into migrated nav_history: %v", err)
	}
	var secType string
	if err := dbi.QueryRowContext(ctx,
		`SELECT security_type FROM nav_history WHERE fund_code = '019173'`,
	).Scan(&secType); err != nil {
		t.Fatalf("read back nav security_type: %v", err)
	}
	if secType != "fund" {
		t.Fatalf("security_type = %q, want the DEFAULT 'fund'", secType)
	}

	// The legacy structures themselves were never rebuilt: portfolio_snapshot
	// keeps its single-column PK and the duplicate transactions rows survive
	// (the unique index was best-effort, as designed).
	if _, err := dbi.ExecContext(ctx, `
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares) VALUES ('A', 'x', 1)
	`); err != nil {
		t.Fatalf("insert into legacy snapshot: %v", err)
	}
	if _, err := dbi.ExecContext(ctx, `
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares) VALUES ('A', 'zzz', 3)
	`); err == nil {
		t.Fatal("legacy snapshot single-column PK was replaced; duplicate fund_code accepted")
	}
	var rows int
	if err := dbi.QueryRowContext(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&rows); err != nil {
		t.Fatalf("count legacy transactions: %v", err)
	}
	if rows != 2 {
		t.Fatalf("legacy transactions count = %d, want 2 (unique index best-effort)", rows)
	}
	var ddl string
	if err := dbi.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'transactions'
	`).Scan(&ddl); err != nil {
		t.Fatalf("read transactions ddl: %v", err)
	}
	if !strings.Contains(ddl, "seq INTEGER PRIMARY KEY AUTOINCREMENT") {
		t.Fatalf("legacy transactions DDL changed: %s", ddl)
	}

	// Second boot: the version gate short-circuits every migration — nothing
	// re-executes, nothing errors, and the recorded set is unchanged.
	if err := EnsureSQLiteSchema(ctx, dbi); err != nil {
		t.Fatalf("second EnsureSQLiteSchema on migrated DB: %v", err)
	}
	var versions2 int
	if err := dbi.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&versions2); err != nil {
		t.Fatalf("recount schema_migrations: %v", err)
	}
	if versions2 != versions {
		t.Fatalf("schema_migrations grew on replay: %d -> %d", versions, versions2)
	}
	// ...and the legacy shapes are still intact after the replay.
	var ddl2 string
	if err := dbi.QueryRowContext(ctx, `
		SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'transactions'
	`).Scan(&ddl2); err != nil {
		t.Fatalf("reread transactions ddl: %v", err)
	}
	if ddl2 != ddl {
		t.Fatalf("transactions DDL changed on replay:\n before: %s\n after:  %s", ddl, ddl2)
	}
}

// The version gate is what makes replay a no-op: a database whose version
// table already holds every version must be left completely untouched, even
// when its structure is still legacy (the operator's problem becomes an
// explicit repair, not a silent rewrite on every boot).
func TestEnsureSQLiteSchemaVersionGateSkipsAppliedMigrations(t *testing.T) {
	ctx := context.Background()
	dbi, err := Open(ctx, Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "gated.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dbi.Close()

	// Legacy nav_history, but the version table claims everything applied.
	for _, stmt := range []string{
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL, PRIMARY KEY (fund_code, date))`,
		createSchemaMigrationsSQLite,
		`INSERT INTO schema_migrations (version, name) VALUES (1, 'baseline_schema')`,
		`INSERT INTO schema_migrations (version, name) VALUES (2, 'transactions_column_defaults')`,
		`INSERT INTO schema_migrations (version, name) VALUES (3, 'portfolio_snapshot_composite_pk')`,
		`INSERT INTO schema_migrations (version, name) VALUES (4, 'nav_history_security_columns')`,
		`INSERT INTO schema_migrations (version, name) VALUES (5, 'transactions_order_fund_unique_index')`,
	} {
		if _, err := dbi.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// intentionally legacy schema: nav_history must stay untouched because its
	// migration version is already recorded
	if err := EnsureSQLiteSchema(ctx, dbi); err != nil {
		t.Fatalf("EnsureSQLiteSchema: %v", err)
	}
	var cols int
	if err := dbi.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('nav_history')
		WHERE name IN ('daily_change_pct', 'security_type')
	`).Scan(&cols); err != nil {
		t.Fatalf("probe nav_history columns: %v", err)
	}
	if cols != 0 {
		t.Fatalf("migration 0004 ran despite recorded version; nav_history gained %d columns", cols)
	}
}

// ── accepted behavior change: enforced indexes ─────────────────────────────

// A legacy DB missing a column that a mandatory index references must fail
// startup loudly (previously: boot continued forever without the index, with
// one warn on first boot). Accepted when versioning was introduced.
func TestEnsureSQLiteSchemaEnforcesIndexes(t *testing.T) {
	ctx := context.Background()
	dbi, err := Open(ctx, Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "noidx.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer dbi.Close()

	// intentionally legacy schema: portfolio_snapshot lacks the portfolio_id
	// column referenced by idx_portfolio_snapshot_portfolio
	if _, err := dbi.ExecContext(ctx, `
		CREATE TABLE portfolio_snapshot (
			fund_code TEXT PRIMARY KEY,
			fund_name TEXT,
			held_shares REAL
		)
	`); err != nil {
		t.Fatalf("create legacy snapshot: %v", err)
	}

	err = EnsureSQLiteSchema(ctx, dbi)
	if err == nil {
		t.Fatal("EnsureSQLiteSchema must fail when a mandatory index cannot be created")
	}
	if !strings.Contains(err.Error(), "idx_portfolio_snapshot_portfolio") {
		t.Fatalf("error = %q, want mention of the failing index", err)
	}
	// The failed step is not recorded, so a repaired DB migrates cleanly.
	if _, err := dbi.ExecContext(ctx, `ALTER TABLE portfolio_snapshot ADD COLUMN portfolio_id INTEGER NOT NULL DEFAULT 1`); err != nil {
		t.Fatalf("repair legacy snapshot: %v", err)
	}
	// intentionally legacy schema retained: the repair above restores only the
	// missing indexed column
	if err := EnsureSQLiteSchema(ctx, dbi); err != nil {
		t.Fatalf("EnsureSQLiteSchema after repair: %v", err)
	}
}

// ── nav_history column backfill details (migration 0004) ───────────────────

func TestEnsureSQLiteSchemaNavBackfillToleratesDuplicateColumnRace(t *testing.T) {
	// dialect.IsDuplicateColumnError must classify both driver wordings so the
	// probe/ALTER race stays a success on SQLite and PG.
	for _, msg := range []string{"duplicate column name: security_type", `column "security_type" of relation "nav_history" already exists`, "SQLSTATE 42701"} {
		if !dialect.IsDuplicateColumnError(errString(msg)) {
			t.Fatalf("IsDuplicateColumnError(%q) = false, want true", msg)
		}
	}
	if dialect.IsDuplicateColumnError(errString("no such table: nav_history")) {
		t.Fatal("IsDuplicateColumnError must not match unrelated errors")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
