// Package testutil provides shared helpers for Go tests (temp SQLite, schema exec).
// Prefer this over copy-pasting sql.Open + PRAGMA setup in each _test.go.
package testutil

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/repository/sqlitedb"
)

// OpenTempDB opens a writable SQLite database in t.TempDir() with production-like
// PRAGMAs (via sqlitedb.Open: busy_timeout, foreign_keys, WAL, synchronous=NORMAL).
// Caller must Close (typically defer db.Close()).
func OpenTempDB(t testing.TB) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "fund.db")
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: dbPath})
	if err != nil {
		t.Fatalf("testutil.OpenTempDB: %v", err)
	}
	return db
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
	db := OpenTempDB(t)
	ExecStatements(t, db, stmts)
	return db
}
