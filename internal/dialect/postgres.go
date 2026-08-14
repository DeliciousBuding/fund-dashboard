package dialect

import (
	"context"
	"database/sql"
	"fmt"
)

// Postgres implements Dialect for the pgx stdlib driver. Placeholder rebinding
// (? → $N) is handled at connection open time; this type supplies the remaining
// dialect differences (date arithmetic, catalog introspection, sizing).
type Postgres struct {
	db *sql.DB
}

func (d *Postgres) Name() string     { return NamePostgres }
func (d *Postgres) IsPostgres() bool { return true }

func (d *Postgres) DaysSinceExpr(dateColumn string) string {
	return fmt.Sprintf(
		"CAST(EXTRACT(EPOCH FROM NOW()) / 86400 - EXTRACT(EPOCH FROM %s::timestamp) / 86400 AS INTEGER)",
		dateColumn,
	)
}

func (d *Postgres) HasColumn(ctx context.Context, table, column string) (bool, error) {
	var exists bool
	err := d.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = ? AND column_name = ?
		)`, table, column).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("inspect %s columns: %w", table, err)
	}
	return exists, nil
}

func (d *Postgres) ListUserTables(ctx context.Context) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT tablename
		FROM pg_catalog.pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename
		LIMIT 500
	`)
	if err != nil {
		return nil, fmt.Errorf("list pg tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("scan pg table: %w", err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pg table rows: %w", err)
	}
	return tables, nil
}

func (d *Postgres) DatabaseSizeBytes(ctx context.Context) (int64, error) {
	var size int64
	if err := d.db.QueryRowContext(ctx,
		"SELECT pg_database_size(current_database())",
	).Scan(&size); err != nil {
		return 0, fmt.Errorf("pg database size: %w", err)
	}
	return size, nil
}
