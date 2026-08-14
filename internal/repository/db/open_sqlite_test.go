package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenSQLiteSetsWALOnWritable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fund.db")
	db, err := Open(context.Background(), Options{Driver: "sqlite", SQLitePath: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var mode string
	if err := db.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if strings.ToLower(mode) != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}

	var sync int
	if err := db.QueryRowContext(context.Background(), "PRAGMA synchronous").Scan(&sync); err != nil {
		t.Fatalf("PRAGMA synchronous: %v", err)
	}
	// NORMAL == 1
	if sync != 1 {
		t.Fatalf("synchronous = %d, want 1 (NORMAL)", sync)
	}
}

func TestOpenSQLiteReadOnlyDoesNotRequireWALModeChange(t *testing.T) {
	// Create a writable DB first so the file exists for RO open.
	path := filepath.Join(t.TempDir(), "fund.db")
	w, err := Open(context.Background(), Options{Driver: "sqlite", SQLitePath: path})
	if err != nil {
		t.Fatalf("writable Open: %v", err)
	}
	if _, err := w.ExecContext(context.Background(), "CREATE TABLE t (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	_ = w.Close()

	ro, err := Open(context.Background(), Options{Driver: "sqlite", SQLitePath: path, ReadOnly: true})
	if err != nil {
		t.Fatalf("read-only Open: %v", err)
	}
	defer ro.Close()

	_, err = ro.ExecContext(context.Background(), "INSERT INTO t(id) VALUES (1)")
	if err == nil {
		t.Fatalf("expected write rejection under query_only")
	}
}
