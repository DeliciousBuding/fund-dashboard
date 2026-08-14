// Package sqlitedb provides a tested, safe SQLite connection helper with read-only
// and read-write modes, busy timeout, foreign_keys pragma, and clear error messages.
package sqlitedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

type Options struct {
	Path     string
	ReadOnly bool
}

func Open(ctx context.Context, opts Options) (*sql.DB, error) {
	dbPath := strings.TrimSpace(opts.Path)
	if dbPath == "" {
		return nil, errors.New("db path is required")
	}
	if opts.ReadOnly {
		if _, err := os.Stat(dbPath); err != nil {
			return nil, fmt.Errorf("stat db path: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := configure(ctx, db, opts); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func configure(ctx context.Context, db *sql.DB, opts Options) error {
	for _, stmt := range []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	// Match production open path (internal/repository/db): explicit WAL so a
	// fresh DB or restored dump does not stay on DELETE journal + FULL sync.
	// Read-only opens skip mode changes (may need write to create -wal/-shm).
	if !opts.ReadOnly {
		for _, stmt := range []string{
			"PRAGMA journal_mode=WAL",
			"PRAGMA synchronous=NORMAL",
		} {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("%s: %w", stmt, err)
			}
		}
	}
	if opts.ReadOnly {
		if _, err := db.ExecContext(ctx, "PRAGMA query_only=ON"); err != nil {
			return fmt.Errorf("PRAGMA query_only=ON: %w", err)
		}
	}
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sqlite: %w", err)
	}
	return nil
}
