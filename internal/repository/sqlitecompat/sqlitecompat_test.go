package sqlitecompat

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCheckCompatibilityAcceptsFundDashboardSchemaFixture(t *testing.T) {
	dbPath := createFundDashboardFixture(t)

	report, err := CheckCompatibility(context.Background(), dbPath, RequiredTables)
	if err != nil {
		t.Fatalf("CheckCompatibility returned error: %v", err)
	}

	if report.Driver != "modernc.org/sqlite" {
		t.Fatalf("driver = %q, want modernc.org/sqlite", report.Driver)
	}
	if report.JournalMode != "wal" {
		t.Fatalf("journal mode = %q, want wal", report.JournalMode)
	}
	if report.IntegrityCheck != "ok" {
		t.Fatalf("integrity_check = %q, want ok", report.IntegrityCheck)
	}
	if report.QuickCheck != "ok" {
		t.Fatalf("quick_check = %q, want ok", report.QuickCheck)
	}
	if len(report.MissingTables) != 0 {
		t.Fatalf("missing tables = %v, want none", report.MissingTables)
	}
}

func TestCheckCompatibilityReportsMissingCoreTables(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("create empty fixture: %v", err)
	}

	report, err := CheckCompatibility(context.Background(), dbPath, RequiredTables)
	if err != nil {
		t.Fatalf("CheckCompatibility returned error: %v", err)
	}

	if len(report.MissingTables) == 0 {
		t.Fatalf("missing tables is empty, want core tables reported")
	}
}

func TestCheckCompatibilityReportsProductionShapedColumnsAndIndexes(t *testing.T) {
	dbPath := createProductionShapedFundDashboardFixture(t)

	report, err := CheckCompatibility(context.Background(), dbPath, RequiredTables)
	if err != nil {
		t.Fatalf("CheckCompatibility returned error: %v", err)
	}

	assertHasColumns(t, report.TableColumns["fund_details"], "security_type", "market", "currency", "exchange")
	assertHasColumns(t, report.TableColumns["transactions"], "signed_cash_flow", "signed_share_change", "nav_on_effective_date", "nav_verified")
	assertHasColumns(t, report.TableColumns["nav_history"], "unit_nav", "daily_change_pct", "security_type")
	assertHasColumns(t, report.TableColumns["portfolio_snapshot"], "current_value", "pnl_pct", "security_type", "portfolio_id")
	assertHasColumns(t, report.TableColumns["source_events"], "query", "related_security_code", "is_read", "is_useful", "fetched_at")
	assertHasColumns(t, report.TableColumns["dca_plans"], "frequency", "weekday_mask", "active", "source", "updated_at")

	if got, want := report.PrimaryKeys["nav_history"], []string{"fund_code", "date"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nav_history primary key = %v, want %v", got, want)
	}
	assertHasStrings(t, report.Indexes, "idx_sev_code", "idx_dca_plans_active", "idx_nav_date", "idx_tx_fund_time", "idx_ps_portfolio")
}

func TestCheckWALConcurrentReadWriteAllowsWriterDuringReadTransaction(t *testing.T) {
	dbPath := createProductionShapedFundDashboardFixture(t)

	report, err := CheckWALConcurrentReadWrite(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("CheckWALConcurrentReadWrite returned error: %v", err)
	}

	if report.JournalMode != "wal" {
		t.Fatalf("journal mode = %q, want wal", report.JournalMode)
	}
	if report.ReaderInitialRows == 0 {
		t.Fatalf("reader initial rows = 0, want seeded fixture rows")
	}
	if report.WriterRowsInserted != 1 {
		t.Fatalf("writer inserted rows = %d, want 1", report.WriterRowsInserted)
	}
	if report.ReaderRowsDuringWrite != report.ReaderInitialRows {
		t.Fatalf("reader saw %d rows during write, want snapshot count %d", report.ReaderRowsDuringWrite, report.ReaderInitialRows)
	}
	if report.FinalProbeRows != 1 {
		t.Fatalf("final probe rows = %d, want 1", report.FinalProbeRows)
	}
}

func TestCheckCompatibilityWithExternalDB(t *testing.T) {
	dbPath := os.Getenv("FUND_SQLITE_SPIKE_DB")
	if dbPath == "" {
		t.Skip("set FUND_SQLITE_SPIKE_DB to run against a real copied fund.db")
	}

	report, err := CheckCompatibility(context.Background(), dbPath, RequiredTables)
	if err != nil {
		t.Fatalf("CheckCompatibility(%s) returned error: %v", dbPath, err)
	}
	if report.IntegrityCheck != "ok" {
		t.Fatalf("integrity_check = %q, want ok", report.IntegrityCheck)
	}
	if len(report.MissingTables) != 0 {
		t.Fatalf("missing tables = %v, want none", report.MissingTables)
	}
}

func createFundDashboardFixture(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "fund.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("enable WAL: %v", err)
	}

	for _, stmt := range []string{
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT)`,
		`CREATE TABLE transactions (seq INTEGER PRIMARY KEY AUTOINCREMENT, fund_code TEXT)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, nav REAL, PRIMARY KEY (fund_code, date))`,
		`CREATE TABLE portfolio_snapshot (fund_code TEXT NOT NULL, shares REAL, portfolio_id INTEGER NOT NULL DEFAULT 1, PRIMARY KEY (fund_code, portfolio_id))`,
		`CREATE TABLE fund_holdings (fund_code TEXT, stock_code TEXT, weight REAL)`,
		`CREATE TABLE source_events (id INTEGER PRIMARY KEY AUTOINCREMENT, fund_code TEXT)`,
		`CREATE TABLE dca_plans (id INTEGER PRIMARY KEY AUTOINCREMENT, fund_code TEXT)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec fixture statement %q: %v", stmt, err)
		}
	}

	return dbPath
}

func assertHasColumns(t *testing.T, columns []string, expected ...string) {
	t.Helper()
	assertHasStrings(t, columns, expected...)
}

func assertHasStrings(t *testing.T, values []string, expected ...string) {
	t.Helper()
	have := map[string]bool{}
	for _, value := range values {
		have[value] = true
	}
	for _, value := range expected {
		if !have[value] {
			t.Fatalf("values %v missing %q", values, value)
		}
	}
}
