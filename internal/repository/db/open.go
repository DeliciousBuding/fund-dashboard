// Package db provides database driver abstraction supporting both SQLite and PostgreSQL.
//
// The "pg" driver is a thin wrapper around the pgx stdlib driver that translates
// ? placeholders to $1, $2, ... so that the entire application can use a single
// placeholder style regardless of the backing database.
//
// Register:
//
//	import _ "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
//
// Usage:
//
//	sqlitedb := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: "data/fund.db"})
//	pgdb := db.Open(ctx, db.Options{Driver: "pg", DSN: "postgres://..."})
package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite" // register "sqlite" driver
)

func init() {
	sql.Register("pg", &rebindDriver{})
}

// Options configures the database connection.
type Options struct {
	// Driver selects the database driver. Must be "sqlite" or "pg".
	// Default: "sqlite" if SQLitePath is set, "pg" if DSN is set.
	Driver string

	// SQLitePath is the file path for the SQLite database. Only used when Driver="sqlite".
	SQLitePath string

	// DSN is the PostgreSQL connection string. Only used when Driver="pg".
	// Format: postgres://user:pass@host:5432/dbname?sslmode=require
	DSN string

	// ReadOnly opens the database in read-only mode (SQLite only).
	ReadOnly bool
}

// Open opens a database connection based on the provided options.
func Open(ctx context.Context, opts Options) (*sql.DB, error) {
	if opts.Driver == "" {
		if opts.DSN != "" {
			opts.Driver = "pg"
		} else {
			opts.Driver = "sqlite"
		}
	}

	switch opts.Driver {
	case "sqlite":
		return openSQLite(ctx, opts)
	case "pg":
		return openPG(ctx, opts)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", opts.Driver)
	}
}

func openSQLite(ctx context.Context, opts Options) (*sql.DB, error) {
	// Keep SQLite PRAGMAs aligned with sqlitedb.Open (production open path).
	dbPath := strings.TrimSpace(opts.SQLitePath)
	if dbPath == "" {
		return nil, errors.New("sqlite path is required")
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Single writer for SQLite. Concurrent readers require a separate read pool
	// (not enabled here); docs claim WAL for durability + non-blocking readers
	// once multi-conn is introduced.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Always-safe session defaults.
	for _, stmt := range []string{
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("%s: %w", stmt, err)
		}
	}
	// Explicit WAL on writable opens so fresh DBs / dump restores match runbooks
	// (docs/STATE + scheduler wal_checkpoint). Read-only opens skip mode changes
	// (journal_mode=WAL may need write access to create -wal/-shm).
	if !opts.ReadOnly {
		for _, stmt := range []string{
			"PRAGMA journal_mode=WAL",
			"PRAGMA synchronous=NORMAL",
		} {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				_ = db.Close()
				return nil, fmt.Errorf("%s: %w", stmt, err)
			}
		}
	}
	if opts.ReadOnly {
		if _, err := db.ExecContext(ctx, "PRAGMA query_only=ON"); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("PRAGMA query_only=ON: %w", err)
		}
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

func openPG(ctx context.Context, opts Options) (*sql.DB, error) {
	if strings.TrimSpace(opts.DSN) == "" {
		return nil, errors.New("pg DSN is required")
	}

	// sql.Open uses the registered "pg" driver (rebindDriver) which
	// translates ? → $N transparently for the entire application.
	db, err := sql.Open("pg", opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("open pg: %w", err)
	}
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping pg: %w", err)
	}

	return db, nil
}

// ── ? → $N rebinding driver ──────────────────────────────────────────────

// rebindDriver wraps the pgx stdlib driver so that application-level ?
// placeholders are translated to the $1, $2, ... that PostgreSQL expects.
// This allows the entire codebase to use ? placeholders regardless of driver.
type rebindDriver struct{}

func (d *rebindDriver) Open(dsn string) (driver.Conn, error) {
	inner, err := stdlib.GetDefaultDriver().Open(dsn)
	if err != nil {
		return nil, err
	}
	return &rebindConn{inner: inner}, nil
}

// rebindConn wraps a native pgx connection and rebinds queries.
type rebindConn struct {
	inner driver.Conn
}

// Compile-time check: pool checkout can reset session state via the wrapper.
var _ driver.SessionResetter = (*rebindConn)(nil)

func (c *rebindConn) Prepare(query string) (driver.Stmt, error) {
	return c.inner.Prepare(rebind(query))
}

func (c *rebindConn) Close() error {
	return c.inner.Close()
}

func (c *rebindConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

// ExecContext intercepts direct execution and rebinds ? → $N.
func (c *rebindConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if execer, ok := c.inner.(driver.ExecerContext); ok {
		return execer.ExecContext(ctx, rebind(query), args)
	}
	return nil, driver.ErrSkip
}

// QueryContext intercepts direct query and rebinds ? → $N.
func (c *rebindConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if queryer, ok := c.inner.(driver.QueryerContext); ok {
		return queryer.QueryContext(ctx, rebind(query), args)
	}
	return nil, driver.ErrSkip
}

// PrepareContext intercepts prepared statement creation and rebinds.
func (c *rebindConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if preparer, ok := c.inner.(driver.ConnPrepareContext); ok {
		return preparer.PrepareContext(ctx, rebind(query))
	}
	return c.inner.Prepare(rebind(query))
}

// BeginTx delegates to the inner connection.
func (c *rebindConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if beginner, ok := c.inner.(driver.ConnBeginTx); ok {
		return beginner.BeginTx(ctx, opts)
	}
	return nil, errors.New("pg: underlying driver does not implement driver.ConnBeginTx")
}

// ResetSession delegates pool checkout reset to the inner pgx connection when available.
// Without this, database/sql may skip session reset and leak SET/temp/advisory state across checkouts.
func (c *rebindConn) ResetSession(ctx context.Context) error {
	if rs, ok := c.inner.(driver.SessionResetter); ok {
		return rs.ResetSession(ctx)
	}
	return nil
}

// rebind replaces ? positional placeholders with $1, $2, ... $N.
// Single-quoted SQL string literals are respected: ? inside '...' is left alone,
// and a doubled quote (two single quotes) does not end the string. Dollar-quoting
// and double-quoted identifiers are not specially handled.
func rebind(query string) string {
	// Fast path: no ? present.
	if !strings.ContainsRune(query, '?') {
		return query
	}

	var buf strings.Builder
	buf.Grow(len(query) + 16)
	n := 0
	inString := false
	// Iterate by index so we can look ahead for '' escaped quotes.
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if inString {
			buf.WriteByte(ch)
			if ch == '\'' {
				// '' inside a string is an escaped quote; stay in string.
				if i+1 < len(query) && query[i+1] == '\'' {
					buf.WriteByte(query[i+1])
					i++
					continue
				}
				inString = false
			}
			continue
		}
		switch ch {
		case '\'':
			inString = true
			buf.WriteByte(ch)
		case '?':
			n++
			fmt.Fprintf(&buf, "$%d", n)
		default:
			buf.WriteByte(ch)
		}
	}
	return buf.String()
}
