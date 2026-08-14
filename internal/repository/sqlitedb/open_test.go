package sqlitedb

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/repository/sqlitecompat"
	_ "modernc.org/sqlite"
)

func TestOpenRejectsEmptyPath(t *testing.T) {
	_, err := Open(context.Background(), Options{})
	if err == nil {
		t.Fatalf("Open returned nil error, want required path error")
	}
	if !strings.Contains(err.Error(), "db path is required") {
		t.Fatalf("error = %q, want required path", err.Error())
	}
}

func TestOpenReadOnlyRejectsMissingPath(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.db")

	_, err := Open(context.Background(), Options{Path: missingPath, ReadOnly: true})
	if err == nil {
		t.Fatalf("Open returned nil error, want missing path error")
	}
	if !strings.Contains(err.Error(), "stat db path") {
		t.Fatalf("error = %q, want stat db path", err.Error())
	}
}

func TestOpenReadOnlyAllowsReadsAndRejectsWrites(t *testing.T) {
	dbPath := createFixture(t)

	db, err := Open(context.Background(), Options{Path: dbPath, ReadOnly: true})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM fund_details").Scan(&count); err != nil {
		t.Fatalf("read query returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("fund_details count = %d, want 1", count)
	}

	_, err = db.ExecContext(context.Background(), "INSERT INTO fund_details (fund_code, fund_name) VALUES ('000001', 'blocked')")
	if err == nil {
		t.Fatalf("write returned nil error, want query-only rejection")
	}
}

func TestOpenReadWriteCreatesDatabaseAndWorksWithCompatibilityCheck(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fund.db")

	db, err := Open(context.Background(), Options{Path: dbPath})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	for _, stmt := range minimalSchemaStatements {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec schema statement %q: %v", stmt, err)
		}
	}
	var journalMode string
	if err := db.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	report, err := sqlitecompat.CheckCompatibility(context.Background(), dbPath, sqlitecompat.RequiredTables)
	if err != nil {
		t.Fatalf("CheckCompatibility returned error: %v", err)
	}
	if len(report.MissingTables) != 0 {
		t.Fatalf("missing tables = %v, want none", report.MissingTables)
	}
}

func createFixture(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "fund.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer db.Close()

	for _, stmt := range append(minimalSchemaStatements,
		`INSERT INTO fund_details (fund_code, fund_name) VALUES ('019173', 'Nasdaq 100 QDII C')`,
	) {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec fixture statement %q: %v", stmt, err)
		}
	}
	return dbPath
}

var minimalSchemaStatements = []string{
	`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT)`,
	`CREATE TABLE transactions (seq INTEGER PRIMARY KEY AUTOINCREMENT, fund_code TEXT)`,
	`CREATE TABLE nav_history (fund_code TEXT, date TEXT, nav REAL, PRIMARY KEY (fund_code, date))`,
	`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, shares REAL, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,
	`CREATE TABLE fund_holdings (fund_code TEXT, stock_code TEXT, weight REAL)`,
	`CREATE TABLE source_events (id INTEGER PRIMARY KEY AUTOINCREMENT, fund_code TEXT)`,
	`CREATE TABLE dca_plans (id INTEGER PRIMARY KEY AUTOINCREMENT, fund_code TEXT)`,
}
