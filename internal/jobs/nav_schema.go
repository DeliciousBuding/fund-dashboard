package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"log/slog"
)

// ensureNavSchema is a best-effort SQLite/legacy column backfill, run once per refresher.
// Production Azure PG already has these columns via EnsurePGSchema; only missing columns are ALTERed.
func (r *PriceRefresher) ensureNavSchema(ctx context.Context) {
	r.navSchemaOnce.Do(func() {
		r.navSchemaErr = r.ensureNavSchemaOnce(ctx)
		if r.navSchemaErr != nil {
			slog.Error("ensure nav_history schema", "error", r.navSchemaErr)
		}
	})
}

func (r *PriceRefresher) ensureNavSchemaOnce(ctx context.Context) error {
	cols, err := navHistoryColumns(ctx, r.db)
	if err != nil {
		return err
	}

	var firstErr error
	if _, ok := cols["daily_change_pct"]; !ok {
		if err := addNavHistoryColumn(ctx, r.db, "daily_change_pct", "REAL DEFAULT 0"); err != nil {
			firstErr = err
		}
	}
	if _, ok := cols["security_type"]; !ok {
		if err := addNavHistoryColumn(ctx, r.db, "security_type", "TEXT DEFAULT 'fund'"); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// navHistoryColumns returns lowercased column names for nav_history.
// SQLite PRAGMA first (local/CI); fall back to PG information_schema.
func navHistoryColumns(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	out := map[string]struct{}{}

	if err := ctx.Err(); err != nil {
		return out, err
	}
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(nav_history)`)
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
		if len(out) > 0 {
			return out, nil
		}
	} else if ctx.Err() != nil {
		return out, ctx.Err()
	} else {
		// PRAGMA fails on PG (or missing table) — fall through to information_schema.
		slog.Debug("nav_history PRAGMA table_info failed; trying information_schema", "error", err)
	}

	rows, err = db.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'nav_history'
	`)
	if err != nil {
		return out, fmt.Errorf("list nav_history columns: %w", err)
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

func addNavHistoryColumn(ctx context.Context, db *sql.DB, name, def string) error {
	stmt := fmt.Sprintf("ALTER TABLE nav_history ADD COLUMN %s %s", name, def)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		// Concurrent/race: column may already exist after probe; treat as success.
		if isDuplicateColumnErr(err) {
			return nil
		}
		slog.Error("alter nav_history", "column", name, "error", err)
		return fmt.Errorf("add column %s: %w", name, err)
	}
	return nil
}

func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column") ||
		strings.Contains(msg, "already exists")
}

