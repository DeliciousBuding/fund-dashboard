package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
)

// schemaMetaCache holds process-lifetime table/column probe results.
// Production schema is stable after startup; invalidate only on process restart.
// Shared via pointer so value-copied Service receivers share one cache.
type schemaMetaCache struct {
	existsMu sync.Mutex
	exists   map[string]bool

	colsMu sync.Mutex
	cols   map[string]map[string]struct{}
}

type klineUpsertKind int

const (
	klineUpsertNone klineUpsertKind = iota
	klineUpsertMarketPeriod
	klineUpsertMarket
	klineUpsertPeriod
)

func newSchemaMetaCache() *schemaMetaCache {
	return &schemaMetaCache{
		exists: make(map[string]bool),
		cols:   make(map[string]map[string]struct{}),
	}
}

// quoteSQLiteIdent double-quotes a SQLite identifier (defense-in-depth for PRAGMA).
func quoteSQLiteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

// tableExists returns whether name is a table. Result is cached for process life on success.
func (s Service) tableExists(ctx context.Context, name string) (bool, error) {
	if s.schema != nil {
		s.schema.existsMu.Lock()
		if v, ok := s.schema.exists[name]; ok {
			s.schema.existsMu.Unlock()
			return v, nil
		}
		s.schema.existsMu.Unlock()
	}

	found, err := s.probeTableExists(ctx, name)
	if err != nil {
		return false, err
	}
	if s.schema != nil {
		s.schema.existsMu.Lock()
		s.schema.exists[name] = found
		s.schema.existsMu.Unlock()
	}
	return found, nil
}

func (s Service) probeTableExists(ctx context.Context, name string) (bool, error) {
	// Cross-dialect table existence check.
	// Try information_schema first (PG), fall through to sqlite_master.
	var found string
	err := s.db.QueryRowContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = ?
	`, name).Scan(&found)
	if err == nil {
		return true, nil
	}
	if err != sql.ErrNoRows {
		// Not a PG info_schema error — try sqlite_master for SQLite
		err2 := s.db.QueryRowContext(ctx, `
			SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?
		`, name).Scan(&found)
		if err2 == nil {
			return true, nil
		}
		if err2 == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check table %s: %w", name, err2)
	}
	return false, nil
}

// tableColumns returns lowercased column name set for the given table.
// Result is cached for process life on success.
func (s Service) tableColumns(ctx context.Context, table string) (map[string]struct{}, error) {
	if s.schema != nil {
		s.schema.colsMu.Lock()
		if v, ok := s.schema.cols[table]; ok {
			s.schema.colsMu.Unlock()
			return v, nil
		}
		s.schema.colsMu.Unlock()
	}

	out, err := s.probeTableColumns(ctx, table)
	if err != nil {
		return nil, err
	}
	if s.schema != nil {
		s.schema.colsMu.Lock()
		s.schema.cols[table] = out
		s.schema.colsMu.Unlock()
	}
	return out, nil
}

func (s Service) probeTableColumns(ctx context.Context, table string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	// SQLite first (local/CI fixtures). On SQLite, PRAGMA table_info on a
	// missing table succeeds with 0 rows (no error) — that means "table
	// absent", not "try PostgreSQL". Only a real PRAGMA error (i.e. a
	// non-SQLite connection) falls through to the information_schema branch.
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, quoteSQLiteIdent(table)))
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				return nil, err
			}
			out[strings.ToLower(name)] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return out, nil
	}
	// PostgreSQL information_schema (production Azure PG).
	rows, err = s.db.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ?
	`, table)
	if err != nil {
		return out, fmt.Errorf("list columns %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[strings.ToLower(name)] = struct{}{}
	}
	return out, rows.Err()
}

// preparedKlineUpsert prepares a stock_kline_cache upsert statement for a single
// call. The caller must Close the returned statement once done. kind is
// klineUpsertNone when the table/shape cannot support history writes.
//
// The statement is prepared per call rather than cached for the process
// lifetime: its lifecycle then stays symmetric with its use, so there is no
// orphaned *sql.Stmt left behind. Schema probing (table/column shape) remains
// cached in schemaMetaCache, and the caller reuses one prepare across all
// history rows of a single upsert.
func (s Service) preparedKlineUpsert(ctx context.Context) (klineUpsertKind, *sql.Stmt, error) {
	return s.buildKlineUpsert(ctx)
}

func (s Service) buildKlineUpsert(ctx context.Context) (klineUpsertKind, *sql.Stmt, error) {
	hasTable, err := s.tableExists(ctx, "stock_kline_cache")
	if err != nil {
		return klineUpsertNone, nil, err
	}
	if !hasTable {
		return klineUpsertNone, nil, nil
	}
	cols, err := s.tableColumns(ctx, "stock_kline_cache")
	if err != nil {
		return klineUpsertNone, nil, err
	}
	has := func(name string) bool { _, ok := cols[name]; return ok }

	var kind klineUpsertKind
	var query string
	switch {
	case has("market") && has("period"):
		kind = klineUpsertMarketPeriod
		query = `
			INSERT INTO stock_kline_cache (code, market, period, date, close, change_pct)
			VALUES (?, 'US', 'daily', ?, ?, ?)
			ON CONFLICT(code, market, period, date) DO UPDATE SET
				close=excluded.close, change_pct=excluded.change_pct`
	case has("market"):
		kind = klineUpsertMarket
		query = `
			INSERT INTO stock_kline_cache (code, market, date, close, change_pct)
			VALUES (?, 'US', ?, ?, ?)
			ON CONFLICT(code, market, date) DO UPDATE SET
				close=excluded.close, change_pct=excluded.change_pct`
	case has("period"):
		kind = klineUpsertPeriod
		query = `
			INSERT INTO stock_kline_cache (code, period, date, close)
			VALUES (?, 'daily', ?, ?)
			ON CONFLICT(code, period, date) DO UPDATE SET close=excluded.close`
	default:
		return klineUpsertNone, nil, nil
	}
	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return klineUpsertNone, nil, fmt.Errorf("prepare stock_kline_cache upsert: %w", err)
	}
	return kind, stmt, nil
}
