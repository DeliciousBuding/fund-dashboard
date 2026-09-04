package admin

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestCheckAlertsSkipsMissingDCAPlansTable keeps the legacy-fixture tolerance:
// a database without dca_plans must still produce a report instead of erroring.
func TestCheckAlertsSkipsMissingDCAPlansTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// intentionally legacy schema for missing-table tolerance: the subject of
	// the test is a database WITHOUT dca_plans, so the production bootstrap
	// (which would create it) must not run here.
	for _, stmt := range []string{
		`CREATE TABLE portfolio_snapshot (fund_code TEXT, fund_name TEXT, held_shares REAL, total_cost REAL, latest_nav REAL, current_value REAL, unrealized_pnl REAL, pnl_pct REAL, security_type TEXT, portfolio_id INTEGER)`,
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT, security_type TEXT, market TEXT)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL, daily_change_pct REAL)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	svc := NewServiceWithDriver(db, "sqlite")
	res, err := svc.CheckAlerts(context.Background(), CheckAlertsInput{})
	if err != nil {
		t.Fatalf("CheckAlerts without dca_plans: %v", err)
	}
	if !res.OK {
		t.Fatal("report not OK")
	}
}

// TestCheckAlertsMissingTableSingleSource guards the single-source reuse: the
// dialect predicate must own missing-table classification instead of inline
// error-string matching in alerts.go.
func TestCheckAlertsMissingTableSingleSource(t *testing.T) {
	raw, err := os.ReadFile("alerts.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	if !strings.Contains(src, "dialect.IsMissingTableError(err)") {
		t.Fatal("expected dialect.IsMissingTableError reuse in CheckAlerts")
	}
	if strings.Contains(src, `"no such table"`) || strings.Contains(src, `"undefined_table"`) {
		t.Fatal("inline missing-table string matching must not remain in alerts.go")
	}
}
