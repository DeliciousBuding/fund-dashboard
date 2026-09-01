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
	"time"

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
	// Bound connection staleness so a restarted PG container or a NAT-dropped
	// idle socket is recycled within minutes instead of surfacing as errors
	// on the first request afterwards.
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

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

// Compile-time checks: session reset, health probes, and argument checking
// all route through the wrapper instead of silently bypassing it.
var (
	_ driver.SessionResetter   = (*rebindConn)(nil)
	_ driver.Pinger            = (*rebindConn)(nil)
	_ driver.NamedValueChecker = (*rebindConn)(nil)
	_ driver.Validator         = (*rebindConn)(nil)
)

func (c *rebindConn) Prepare(query string) (driver.Stmt, error) {
	q, err := rebind(query)
	if err != nil {
		return nil, err
	}
	return c.inner.Prepare(q)
}

func (c *rebindConn) Close() error {
	return c.inner.Close()
}

func (c *rebindConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

// ExecContext intercepts direct execution and rebinds ? → $N.
func (c *rebindConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	q, err := rebind(query)
	if err != nil {
		return nil, err
	}
	if execer, ok := c.inner.(driver.ExecerContext); ok {
		return execer.ExecContext(ctx, q, args)
	}
	return nil, driver.ErrSkip
}

// QueryContext intercepts direct query and rebinds ? → $N.
func (c *rebindConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q, err := rebind(query)
	if err != nil {
		return nil, err
	}
	if queryer, ok := c.inner.(driver.QueryerContext); ok {
		return queryer.QueryContext(ctx, q, args)
	}
	return nil, driver.ErrSkip
}

// PrepareContext intercepts prepared statement creation and rebinds.
func (c *rebindConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	q, err := rebind(query)
	if err != nil {
		return nil, err
	}
	if preparer, ok := c.inner.(driver.ConnPrepareContext); ok {
		return preparer.PrepareContext(ctx, q)
	}
	return c.inner.Prepare(q)
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

// IsValid forwards the inner connection's liveness signal. Without this,
// database/sql cannot drop a connection that pgx already marked dead after an
// error, so the wrapper would hold broken connections in the pool until the
// next round-trip fails.
func (c *rebindConn) IsValid() bool {
	if v, ok := c.inner.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

// Ping forwards health probes to the inner pgx connection. Without this,
// database/sql falls back to a "SELECT 1" round-trip that cannot surface
// pgx's driver.ErrBadConn mapping for locally closed sockets, so the pool
// would recycle dead connections less efficiently.
func (c *rebindConn) Ping(ctx context.Context) error {
	if pinger, ok := c.inner.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

// CheckNamedValue forwards pgx's permissive argument checking. Without this,
// database/sql applies driver.DefaultParameterConverter, which rejects every
// type outside its small builtin set (int64, float64, bool, []byte, string,
// time.Time) even though pgx converts them natively - e.g. int, uint64, or
// []string arguments for ANY($1).
func (c *rebindConn) CheckNamedValue(nv *driver.NamedValue) error {
	if checker, ok := c.inner.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(nv)
	}
	// Mirror database/sql's own fallback: apply the default value converter.
	v, err := driver.DefaultParameterConverter.ConvertValue(nv.Value)
	if err != nil {
		return err
	}
	nv.Value = v
	return nil
}

// rebind replaces ? positional placeholders with $1, $2, ... $N.
//
// The scanner understands the PostgreSQL lexical constructs that may contain a
// ? so that literal content is never rewritten as a parameter:
//
//   - 'single-quoted strings' with a doubled ” as an escaped quote
//   - "double-quoted identifiers" with a doubled "" as an escaped quote
//   - -- line comments (run to the end of the line)
//   - /* block comments */ (nested, as PostgreSQL permits)
//   - $tag$ dollar-quoted strings (tag may be empty: $$)
//
// Unterminated constructs are rejected with a descriptive error instead of
// silently mis-rewriting a ? that the server would treat as literal content.
// Backslash escapes in E'...' strings are not modelled: this codebase does not
// emit E-strings, and treating them as ordinary strings is conservative except
// when the body contains a backslash-escaped quote.
func rebind(query string) (string, error) {
	// Fast path: no ? present, nothing to do. Also lets $n parameter
	// references and $tag$ queries (such as the DO $$ migration block)
	// pass through untouched.
	if !strings.ContainsRune(query, '?') {
		return query, nil
	}

	var buf strings.Builder
	buf.Grow(len(query) + 16)
	n := 0
	for i := 0; i < len(query); {
		switch {
		case query[i] == '\'':
			j, err := scanSQLString(&buf, query, i)
			if err != nil {
				return "", err
			}
			i = j
		case query[i] == '"':
			j, err := scanQuotedIdent(&buf, query, i)
			if err != nil {
				return "", err
			}
			i = j
		case i+1 < len(query) && query[i] == '-' && query[i+1] == '-':
			i = scanLineComment(&buf, query, i)
		case i+1 < len(query) && query[i] == '/' && query[i+1] == '*':
			j, err := scanBlockComment(&buf, query, i)
			if err != nil {
				return "", err
			}
			i = j
		case query[i] == '$':
			if tag, ok := dollarQuoteTagAt(query, i); ok {
				j, err := scanDollarQuote(&buf, query, i, tag)
				if err != nil {
					return "", err
				}
				i = j
				continue
			}
			buf.WriteByte(query[i])
			i++
		case query[i] == '?':
			n++
			fmt.Fprintf(&buf, "$%d", n)
			i++
		default:
			buf.WriteByte(query[i])
			i++
		}
	}
	return buf.String(), nil
}

// scanSQLString copies a '...'-quoted string literal beginning at i (the
// opening quote) and returns the index just past its closing quote. A doubled
// ” is an escaped quote and stays inside the string.
func scanSQLString(buf *strings.Builder, query string, i int) (int, error) {
	start := i
	i++
	for i < len(query) {
		if query[i] != '\'' {
			i++
			continue
		}
		if i+1 < len(query) && query[i+1] == '\'' {
			i += 2
			continue
		}
		i++
		buf.WriteString(query[start:i])
		return i, nil
	}
	return start, fmt.Errorf("rebind: unterminated string literal at offset %d", start)
}

// scanQuotedIdent copies a "..."-quoted identifier beginning at i (the opening
// quote) and returns the index just past its closing quote. A doubled "" is an
// escaped quote and stays inside the identifier.
func scanQuotedIdent(buf *strings.Builder, query string, i int) (int, error) {
	start := i
	i++
	for i < len(query) {
		if query[i] != '"' {
			i++
			continue
		}
		if i+1 < len(query) && query[i+1] == '"' {
			i += 2
			continue
		}
		i++
		buf.WriteString(query[start:i])
		return i, nil
	}
	return start, fmt.Errorf("rebind: unterminated quoted identifier at offset %d", start)
}

// scanLineComment copies a -- comment through the end of the current line and
// returns the index of the newline (or len(query)) so scanning resumes there.
func scanLineComment(buf *strings.Builder, query string, i int) int {
	for i < len(query) && query[i] != '\n' {
		buf.WriteByte(query[i])
		i++
	}
	return i
}

// scanBlockComment copies a /* ... */ block comment beginning at i and returns
// the index just past the closing */. Nesting is honoured: PostgreSQL treats
// nested block comments as comments.
func scanBlockComment(buf *strings.Builder, query string, i int) (int, error) {
	start := i
	depth := 0
	for i < len(query) {
		switch {
		case i+1 < len(query) && query[i] == '/' && query[i+1] == '*':
			i += 2
			depth++
		case i+1 < len(query) && query[i] == '*' && query[i+1] == '/':
			i += 2
			depth--
			if depth == 0 {
				buf.WriteString(query[start:i])
				return i, nil
			}
		default:
			i++
		}
	}
	return start, fmt.Errorf("rebind: unterminated block comment at offset %d", start)
}

// scanDollarQuote copies a $tag$...$tag$ dollar-quoted string beginning at i
// and returns the index just past the closing delimiter. A ? inside the body
// is literal content and is copied verbatim.
func scanDollarQuote(buf *strings.Builder, query string, i int, tag string) (int, error) {
	delim := "$" + tag + "$"
	body := i + len(delim)
	j := strings.Index(query[body:], delim)
	if j < 0 {
		return i, fmt.Errorf("rebind: unterminated dollar-quoted string %s at offset %d", delim, i)
	}
	end := body + j + len(delim)
	buf.WriteString(query[i:end])
	return end, nil
}

// dollarQuoteTagAt reports whether query[i:] begins a PostgreSQL dollar-quote
// delimiter and returns its tag. A tag is empty ($$) or a plain identifier.
// $n positional parameter references are not dollar-quote delimiters.
func dollarQuoteTagAt(query string, i int) (string, bool) {
	if i >= len(query) || query[i] != '$' {
		return "", false
	}
	if i+1 < len(query) && query[i+1] == '$' {
		return "", true
	}
	j := i + 1
	if j >= len(query) || !isDollarTagStart(query[j]) {
		return "", false
	}
	j++
	for j < len(query) && isDollarTagPart(query[j]) {
		j++
	}
	if j < len(query) && query[j] == '$' {
		return query[i+1 : j], true
	}
	return "", false
}

func isDollarTagStart(c byte) bool {
	return c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func isDollarTagPart(c byte) bool {
	return isDollarTagStart(c) || ('0' <= c && c <= '9')
}
