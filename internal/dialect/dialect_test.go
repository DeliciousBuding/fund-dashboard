package dialect

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNew(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"sqlite", "sqlite", NameSQLite},
		{"pg", "pg", NamePostgres},
		{"pg-upper", "PG", NamePostgres},
		{"empty defaults to sqlite", "", NameSQLite},
		{"unknown defaults to sqlite", "mysql", NameSQLite},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := New(c.in, nil)
			if d.Name() != c.want {
				t.Fatalf("New(%q).Name() = %q, want %q", c.in, d.Name(), c.want)
			}
		})
	}
}

func TestDaysSinceExpr(t *testing.T) {
	col := "MAX(nh.date)"

	sqlite := New(NameSQLite, nil)
	got := sqlite.DaysSinceExpr(col)
	if got != "CAST(julianday('now') - julianday(MAX(nh.date)) AS INTEGER)" {
		t.Fatalf("sqlite DaysSinceExpr = %q", got)
	}

	pg := New(NamePostgres, nil)
	got = pg.DaysSinceExpr(col)
	if got != "CAST(EXTRACT(EPOCH FROM NOW()) / 86400 - EXTRACT(EPOCH FROM MAX(nh.date)::timestamp) / 86400 AS INTEGER)" {
		t.Fatalf("pg DaysSinceExpr = %q", got)
	}
}

func TestIsPostgres(t *testing.T) {
	if New(NameSQLite, nil).IsPostgres() {
		t.Fatal("sqlite should not report postgres")
	}
	if !New(NamePostgres, nil).IsPostgres() {
		t.Fatal("pg should report postgres")
	}
}

func TestQuoteIdentifier(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"fund_code", `"fund_code"`},
		{`weird"name`, `"weird""name"`},
		{"", `""`},
	}
	for _, c := range cases {
		if got := QuoteIdentifier(c.in); got != c.want {
			t.Fatalf("QuoteIdentifier(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSQLiteHasColumn(t *testing.T) {
	db := newSQLiteDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE sample (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	d := New(NameSQLite, db)

	found, err := d.HasColumn(context.Background(), "sample", "name")
	if err != nil {
		t.Fatalf("HasColumn: %v", err)
	}
	if !found {
		t.Fatal("expected column 'name' to exist")
	}

	found, err = d.HasColumn(context.Background(), "sample", "missing")
	if err != nil {
		t.Fatalf("HasColumn missing: %v", err)
	}
	if found {
		t.Fatal("expected column 'missing' to not exist")
	}
}

func TestSQLiteListUserTables(t *testing.T) {
	db := newSQLiteDB(t)
	defer db.Close()

	for _, stmt := range []string{
		`CREATE TABLE alpha (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE beta (id INTEGER PRIMARY KEY)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	d := New(NameSQLite, db)
	tables, err := d.ListUserTables(context.Background())
	if err != nil {
		t.Fatalf("ListUserTables: %v", err)
	}

	// Internal sqlite_* tables must be excluded.
	for _, table := range tables {
		if len(table) >= 7 && table[:7] == "sqlite_" {
			t.Fatalf("internal table leaked: %q", table)
		}
	}

	if !containsString(tables, "alpha") || !containsString(tables, "beta") {
		t.Fatalf("expected alpha and beta, got %v", tables)
	}
}

func TestSQLiteDatabaseSizeBytes(t *testing.T) {
	db := newSQLiteDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE payload (id INTEGER PRIMARY KEY, blob TEXT)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO payload (blob) VALUES ('x')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	d := New(NameSQLite, db)
	size, err := d.DatabaseSizeBytes(context.Background())
	if err != nil {
		t.Fatalf("DatabaseSizeBytes: %v", err)
	}
	if size <= 0 {
		t.Fatalf("expected positive database size, got %d", size)
	}
}

func newSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()
	// Use a temp file (not :memory:) so PRAGMA database_list reports a real
	// path, exercising the os.Stat branch in DatabaseSizeBytes.
	db, err := sql.Open("sqlite", "file:"+t.TempDir()+"/test.db")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
