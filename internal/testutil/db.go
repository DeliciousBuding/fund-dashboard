// Package testutil provides shared helpers for Go tests (temp SQLite, schema exec).
// Prefer this over copy-pasting sql.Open + PRAGMA setup in each _test.go.
package testutil

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	db "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
)

// OpenTempDB opens a writable SQLite database in t.TempDir() with production-like
// PRAGMAs (via db.Open: busy_timeout, foreign_keys, WAL, synchronous=NORMAL).
// The handle is closed automatically at test cleanup (registered after
// TempDir's own cleanup, so LIFO releases the WAL file before the directory
// is removed on Windows); callers may still Close early.
func OpenTempDB(t testing.TB) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "fund.db")
	opened, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: dbPath})
	if err != nil {
		t.Fatalf("testutil.OpenTempDB: %v", err)
	}
	t.Cleanup(func() { _ = opened.Close() })
	return opened
}

// ExecStatements runs DDL/DML statements in order; fails the test on first error.
func ExecStatements(t testing.TB, db *sql.DB, stmts []string) {
	t.Helper()
	ctx := context.Background()
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("testutil.ExecStatements %q: %v", stmt, err)
		}
	}
}

// OpenTempDBWithSchema opens a temp DB and applies statements (schema + seed).
func OpenTempDBWithSchema(t testing.TB, stmts []string) *sql.DB {
	t.Helper()
	opened := OpenTempDB(t)
	ExecStatements(t, opened, stmts)
	return opened
}

// OpenTempDBWithProductionSchema opens a temp SQLite database with the
// production schema applied via db.EnsureSQLiteSchema — the exact boot path a
// fresh install takes, including the schema_migrations version table and
// enforced indexes.
//
// Prefer this over hand-written CREATE TABLE fixtures: tests then run on the
// same schema EnsureSchema builds in production, so a production DDL drift
// turns the affected tests red instead of hiding behind test-only tables.
// Keep hand-written DDL only where the test deliberately constructs a
// legacy/defective schema (migration probes, missing-column tolerance) and
// mark it with a "// intentionally legacy schema for <reason>" comment.
func OpenTempDBWithProductionSchema(t testing.TB) *sql.DB {
	t.Helper()
	opened := OpenTempDB(t)
	if err := db.EnsureSQLiteSchema(context.Background(), opened); err != nil {
		t.Fatalf("testutil.OpenTempDBWithProductionSchema: %v", err)
	}
	return opened
}
