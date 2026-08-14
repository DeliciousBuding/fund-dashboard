// Package sqlitecompat verifies SQLite compatibility at startup: PRAGMA checks,
// schema introspection, WAL behavior, and core table/column validation.
package sqlitecompat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

var RequiredTables = []string{
	"fund_details",
	"transactions",
	"nav_history",
	"portfolio_snapshot",
	"fund_holdings",
	"source_events",
	"dca_plans",
}

type Report struct {
	Driver          string
	Path            string
	JournalMode     string
	IntegrityCheck  string
	ForeignKeyRows  int
	QuickCheck      string
	PresentTables   []string
	MissingTables   []string
	TableColumns    map[string][]string
	PrimaryKeys     map[string][]string
	Indexes         []string
	SQLiteVersion   string
	DatabaseList    []string
	ReadOnlyWarning string
}

func CheckCompatibility(ctx context.Context, dbPath string, requiredTables []string) (Report, error) {
	if strings.TrimSpace(dbPath) == "" {
		return Report{}, errors.New("db path is required")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return Report{}, fmt.Errorf("stat db path: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return Report{}, fmt.Errorf("open sqlite: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		return Report{}, fmt.Errorf("set busy timeout: %w", err)
	}

	report := Report{
		Driver: "modernc.org/sqlite",
		Path:   dbPath,
	}

	if err := queryScalar(ctx, db, "PRAGMA journal_mode", &report.JournalMode); err != nil {
		return report, err
	}
	report.JournalMode = strings.ToLower(report.JournalMode)

	if err := queryScalar(ctx, db, "PRAGMA integrity_check", &report.IntegrityCheck); err != nil {
		return report, err
	}
	if err := queryScalar(ctx, db, "PRAGMA quick_check", &report.QuickCheck); err != nil {
		return report, err
	}
	if err := queryScalar(ctx, db, "SELECT sqlite_version()", &report.SQLiteVersion); err != nil {
		return report, err
	}

	foreignKeyRows, err := countRows(ctx, db, "PRAGMA foreign_key_check")
	if err != nil {
		return report, err
	}
	report.ForeignKeyRows = foreignKeyRows

	present, err := listTables(ctx, db)
	if err != nil {
		return report, err
	}
	report.PresentTables = present
	report.MissingTables = missingTables(present, requiredTables)

	tableColumns, primaryKeys, err := inspectTableShapes(ctx, db, present)
	if err != nil {
		return report, err
	}
	report.TableColumns = tableColumns
	report.PrimaryKeys = primaryKeys

	indexes, err := listIndexes(ctx, db, present)
	if err != nil {
		return report, err
	}
	report.Indexes = indexes

	databaseList, err := listDatabaseFiles(ctx, db)
	if err != nil {
		return report, err
	}
	report.DatabaseList = databaseList

	return report, nil
}

func queryScalar(ctx context.Context, db *sql.DB, query string, dest *string) error {
	if err := db.QueryRowContext(ctx, query).Scan(dest); err != nil {
		return fmt.Errorf("%s: %w", query, err)
	}
	return nil
}

func countRows(ctx context.Context, db *sql.DB, query string) (int, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", query, err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("%s rows: %w", query, err)
	}
	return count, nil
}

func listTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		  AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("table rows: %w", err)
	}
	return tables, nil
}

func missingTables(present []string, required []string) []string {
	have := make(map[string]struct{}, len(present))
	for _, table := range present {
		have[table] = struct{}{}
	}

	var missing []string
	for _, table := range required {
		if _, ok := have[table]; !ok {
			missing = append(missing, table)
		}
	}
	sort.Strings(missing)
	return missing
}

func listDatabaseFiles(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return nil, fmt.Errorf("database list: %w", err)
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var seq int
		var name string
		var file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return nil, fmt.Errorf("scan database list: %w", err)
		}
		if file != "" {
			files = append(files, file)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("database list rows: %w", err)
	}
	return files, nil
}
