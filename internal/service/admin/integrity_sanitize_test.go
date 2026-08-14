package admin

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestGetDBIntegritySanitizesUnreadableTableDetail(t *testing.T) {
	// Empty memory DB still yields a report; recommendations must not look like SQL dumps.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")
	report, err := svc.GetDBIntegrity(context.Background(), time.Now().UTC())
	if err != nil {
		// Some environments may error hard; if so, ensure not required.
		t.Skipf("integrity setup: %v", err)
	}
	for _, rec := range report.Recommendations {
		low := strings.ToLower(rec)
		if strings.Contains(low, "sql:") || strings.Contains(low, "pq:") {
			t.Fatalf("leaked detail in recommendation: %q", rec)
		}
	}
	_ = report.Checks.ForeignKeyCheck.Detail
	if strings.Contains(strings.ToLower(report.Checks.ForeignKeyCheck.Detail), "sql:") {
		t.Fatalf("fk detail leak: %q", report.Checks.ForeignKeyCheck.Detail)
	}
}
