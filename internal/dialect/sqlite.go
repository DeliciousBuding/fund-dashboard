package dialect

import (
	"context"
	"database/sql"
	"fmt"
	"os"
)

// SQLite implements Dialect for the modernc.org/sqlite driver.
type SQLite struct {
	db *sql.DB
}

func (d *SQLite) Name() string     { return NameSQLite }
func (d *SQLite) IsPostgres() bool { return false }

func (d *SQLite) DaysSinceExpr(dateColumn string) string {
	// julianday('now') is UTC, which is the baseline PostgreSQL must match.
	return fmt.Sprintf("CAST(julianday('now') - julianday(%s) AS INTEGER)", dateColumn)
}

func (d *SQLite) HasColumn(ctx context.Context, table, column string) (bool, error) {
	rows, err := d.db.QueryContext(ctx, "PRAGMA table_info("+QuoteIdentifier(table)+")")
	if err != nil {
		return false, fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType sql.NullString
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, fmt.Errorf("scan %s columns: %w", table, err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("%s column rows: %w", table, err)
	}
	return false, nil
}

func (d *SQLite) ListUserTables(ctx context.Context) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		  AND name NOT LIKE 'sqlite_%'
		ORDER BY name
		LIMIT 500
	`)
	if err != nil {
		return nil, fmt.Errorf("list user tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("scan user table: %w", err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("user table rows: %w", err)
	}
	return tables, nil
}

func (d *SQLite) DatabaseSizeBytes(ctx context.Context) (int64, error) {
	rows, err := d.db.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return 0, fmt.Errorf("database_list: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var seq int
		var name string
		var file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return 0, fmt.Errorf("scan database_list: %w", err)
		}
		if name != "main" || file == "" {
			continue
		}
		info, err := os.Stat(file)
		if err == nil {
			return info.Size(), nil
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("database_list rows: %w", err)
	}

	pageCount, err := querySingleInt(ctx, d.db, "PRAGMA page_count")
	if err != nil {
		return 0, fmt.Errorf("page_count: %w", err)
	}
	pageSize, err := querySingleInt(ctx, d.db, "PRAGMA page_size")
	if err != nil {
		return 0, fmt.Errorf("page_size: %w", err)
	}
	return int64(pageCount * pageSize), nil
}

func querySingleInt(ctx context.Context, db *sql.DB, query string) (int, error) {
	var value int
	if err := db.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}
