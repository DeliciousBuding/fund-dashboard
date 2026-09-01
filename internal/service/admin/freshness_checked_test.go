package admin

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewServiceWithDriverCheckedRejectsUnknownDriver(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := NewServiceWithDriverChecked(db, "unknown-dialect"); err == nil {
		t.Fatal("NewServiceWithDriverChecked(unknown) = nil error, want fail-closed")
	}

	svc, err := NewServiceWithDriverChecked(db, "sqlite")
	if err != nil {
		t.Fatalf("NewServiceWithDriverChecked(sqlite): %v", err)
	}
	if svc.dialect == nil {
		t.Fatal("sqlite service has nil dialect")
	}
	if svc.dialect.IsPostgres() {
		t.Fatal("sqlite service reported postgres dialect")
	}

	pg, err := NewServiceWithDriverChecked(db, "pg")
	if err != nil {
		t.Fatalf("NewServiceWithDriverChecked(pg): %v", err)
	}
	if !pg.dialect.IsPostgres() {
		t.Fatal("pg service did not report postgres dialect")
	}
}

func TestNewServiceWithDriverLegacyFailsClosed(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	svc := NewServiceWithDriver(db, "sqlite")
	if svc.dialect == nil {
		t.Fatal("legacy sqlite construction must not fail")
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("legacy constructor with unknown driver must panic instead of silently falling back")
			}
		}()
		NewServiceWithDriver(db, "unknown-dialect")
	}()
}

// TestAdminListQueriesUseSharedLimits keeps the shared constants wired: read
// admin list payloads must not silently reintroduce hardcoded LIMIT values.
func TestAdminListQueriesUseSharedLimits(t *testing.T) {
	for _, name := range []string{"freshness.go", "verify.go", "holdings_coverage.go", "alerts.go"} {
		src := readAdminSource(t, name)
		if strings.Contains(src, "LIMIT 5000") {
			t.Fatalf("%s still hardcodes LIMIT 5000", name)
		}
		if !strings.Contains(src, "adminListMaxRows") {
			t.Fatalf("%s no longer references adminListMaxRows", name)
		}
	}
	for _, name := range []string{"dashboard.go", "system_status.go"} {
		src := readAdminSource(t, name)
		if strings.Contains(src, "LIMIT 20") {
			t.Fatalf("%s still hardcodes LIMIT 20", name)
		}
		if !strings.Contains(src, "maxRecentAnomalies") {
			t.Fatalf("%s no longer references maxRecentAnomalies", name)
		}
	}
}

func readAdminSource(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// NewServiceWithDriver must stay a fail-closed shell over the checked
// constructor: any reintroduction of the old silent SQLite fallback is a
// data-integrity hazard (wrong dialect SQL against PG).
func TestNewServiceWithDriverRoutesThroughChecked(t *testing.T) {
	raw, err := os.ReadFile("freshness.go")
	if err != nil {
		t.Fatalf("read freshness.go: %v", err)
	}
	src := string(raw)
	if !strings.Contains(src, "func NewServiceWithDriver(db *sql.DB, driver string) Service {\n\tsvc, err := NewServiceWithDriverChecked(db, driver)") {
		t.Fatal("NewServiceWithDriver must delegate to NewServiceWithDriverChecked")
	}
	if strings.Contains(src, "return Service{db: db}") && strings.Contains(src, "dialect.MustNew") {
		t.Fatal("legacy unchecked dialect construction resurfaced")
	}
}
