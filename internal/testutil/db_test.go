package testutil

import (
	"testing"
)

func TestOpenTempDBWithSchema(t *testing.T) {
	db := OpenTempDBWithSchema(t, []string{
		`CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO t (id, name) VALUES (1, 'ok')`,
	})
	defer db.Close()

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM t`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count=%d want 1", n)
	}

	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" && mode != "WAL" {
		// sqlitedb returns lower-case "wal" typically
		if mode != "wal" {
			t.Fatalf("journal_mode=%q want wal", mode)
		}
	}
}
