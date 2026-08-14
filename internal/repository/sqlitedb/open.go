// Package sqlitedb is a thin compatibility alias over internal/repository/db.
//
// It exists only so test code can keep the shorter sqlitedb.Open / sqlitedb.Options
// spelling; the actual SQLite open logic (PRAGMAs, WAL, read-only mode) lives in
// one place: internal/repository/db.
package sqlitedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
)

// Options mirrors the subset of db.Options used by tests.
type Options struct {
	Path     string
	ReadOnly bool
}

// Open opens a SQLite database using the shared db package.
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
	return db.Open(ctx, db.Options{
		Driver:     "sqlite",
		SQLitePath: dbPath,
		ReadOnly:   opts.ReadOnly,
	})
}
