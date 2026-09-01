// Package dialect encapsulates SQL dialect differences between the supported
// databases (SQLite and PostgreSQL). Business code depends on the Dialect
// interface instead of comparing a driver string; the driver name is only
// resolved once in New.
package dialect

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Driver names, resolved once in New. Kept as exported constants so call sites
// that still thread a driver name can refer to them without magic strings.
const (
	NameSQLite   = "sqlite"
	NamePostgres = "pg"
)

// Dialect abstracts the database-specific behavior that cannot be unified by
// placeholder rebinding alone: date arithmetic, schema introspection and
// database-level metrics. Every method that depends on the underlying database
// flavor lives here so callers stay driver-agnostic.
type Dialect interface {
	// Name returns the canonical driver name (NameSQLite or NamePostgres).
	Name() string
	// IsPostgres reports whether the dialect is PostgreSQL. Used only where a
	// whole strategy (not a single query) differs, e.g. integrity checks.
	IsPostgres() bool
	// DaysSinceExpr returns an integer SQL expression for the number of whole
	// days between "now" and the given date column/expression. Both dialects
	// anchor the calculation to UTC so results do not drift with the server
	// timezone; PostgreSQL matches SQLite julianday("now") truncation
	// semantics.
	DaysSinceExpr(dateColumn string) string
	// HasColumn reports whether table contains column.
	HasColumn(ctx context.Context, table, column string) (bool, error)
	// ListUserTables returns the user-visible table names, sorted and bounded.
	ListUserTables(ctx context.Context) ([]string, error)
	// DatabaseSizeBytes returns the on-disk size of the database in bytes.
	DatabaseSizeBytes(ctx context.Context) (int64, error)
}

// NewChecked resolves a driver name into the matching Dialect and fails on
// unknown non-empty names instead of silently falling back to SQLite. An empty
// or whitespace-only name still defaults to SQLite, preserving the historical
// zero-config default. Production assembly paths should prefer NewChecked.
func NewChecked(name string, db *sql.DB) (Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", NameSQLite:
		return &SQLite{db: db}, nil
	case NamePostgres:
		return &Postgres{db: db}, nil
	default:
		return nil, fmt.Errorf("unsupported dialect driver %q (want %q or %q)", name, NameSQLite, NamePostgres)
	}
}

// New resolves a driver name into the matching Dialect. Unknown or empty names
// fall back to SQLite, preserving the historical fail-open default. Callers
// that must fail on unknown drivers should use NewChecked instead.
func New(name string, db *sql.DB) Dialect {
	d, err := NewChecked(name, db)
	if err != nil {
		return &SQLite{db: db}
	}
	return d
}

// QuoteIdentifier quotes an identifier with SQL-standard double quotes. Both
// SQLite and PostgreSQL accept this form, so it is shared rather than a
// per-dialect method.
func QuoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
