package admin

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestFreshnessReportsHealthyWhenNAVDataIsRecent(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// Seed a held fund with recent NAV and a transaction.
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES ('FRESH1', 'Fresh Fund', 'fund')`,
	); err != nil {
		t.Fatalf("seed fund_details: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES ('FRESH1', 'Fresh Fund', 10, -10, 1, 10, 0, 0, 'fund', 1)`,
	); err != nil {
		t.Fatalf("seed portfolio_snapshot: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('FRESH1', date('now'), 1.0)`,
	); err != nil {
		t.Fatalf("seed nav_history: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days) VALUES ('TX001', '2025-01-01', '2025-01-03', '用户买入', 'buy', 'FRESH1', 'Fresh Fund', 100, 80, 0.15, -100, 80, 2)`,
	); err != nil {
		t.Fatalf("seed transactions: %v", err)
	}

	svc := NewService(db)
	report, err := svc.GetFreshness(context.Background())
	if err != nil {
		t.Fatalf("GetFreshness returned error: %v", err)
	}

	if report.Health != "fresh" {
		t.Fatalf("Health = %q, want fresh", report.Health)
	}
	if report.AnomalyCount != 0 {
		t.Fatalf("AnomalyCount = %d, want 0", report.AnomalyCount)
	}
	if len(report.StaleSecurities) != 0 {
		t.Fatalf("StaleSecurities = %d, want 0", len(report.StaleSecurities))
	}
	if len(report.MissingNAVSecurities) != 0 {
		t.Fatalf("MissingNAVSecurities = %d, want 0", len(report.MissingNAVSecurities))
	}
	if len(report.WatchlistMissingNAVSecurities) != 0 {
		t.Fatalf("WatchlistMissingNAVSecurities = %d, want 0", len(report.WatchlistMissingNAVSecurities))
	}
	if report.LastNAVDate == nil {
		t.Fatalf("LastNAVDate = nil, want non-nil")
	}
	if *report.LastNAVDate == "" {
		t.Fatalf("LastNAVDate = \"\", want non-empty")
	}
}

func TestFreshnessDetectsStaleNAV(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// Insert a held fund with NAV data older than 4 days (stalePriceDays threshold).
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES ('STALE1', 'Stale Fund', 'fund')`,
	); err != nil {
		t.Fatalf("seed stale fund_details: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES ('STALE1', 'Stale Fund', 10, -10, 1, 10, 0, 0, 'fund', 1)`,
	); err != nil {
		t.Fatalf("seed stale portfolio_snapshot: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('STALE1', date('now', '-10 days'), 1.0)`,
	); err != nil {
		t.Fatalf("seed stale nav_history: %v", err)
	}

	svc := NewService(db)
	report, err := svc.GetFreshness(context.Background())
	if err != nil {
		t.Fatalf("GetFreshness returned error: %v", err)
	}

	if len(report.StaleSecurities) == 0 {
		t.Fatalf("StaleSecurities = 0, want at least 1 stale security")
	}
	found := false
	for _, s := range report.StaleSecurities {
		if s.Code == "STALE1" {
			found = true
			if s.StaleDays < 5 {
				t.Fatalf("StaleDays for STALE1 = %d, want >= 5", s.StaleDays)
			}
		}
	}
	if !found {
		t.Fatalf("STALE1 not found in StaleSecurities: %#v", report.StaleSecurities)
	}
}

func TestFreshnessDetectsMissingNAV(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// Insert a held fund with no nav_history entry.
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES ('MISSING1', 'Missing NAV Fund', 'fund')`,
	); err != nil {
		t.Fatalf("seed missing fund_details: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES ('MISSING1', 'Missing NAV Fund', 10, -10, 1, 10, 0, 0, 'fund', 1)`,
	); err != nil {
		t.Fatalf("seed missing portfolio_snapshot: %v", err)
	}

	svc := NewService(db)
	report, err := svc.GetFreshness(context.Background())
	if err != nil {
		t.Fatalf("GetFreshness returned error: %v", err)
	}

	found := false
	for _, item := range report.MissingNAVSecurities {
		if item.Code == "MISSING1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("MISSING1 not found in MissingNAVSecurities: %#v", report.MissingNAVSecurities)
	}
	if report.Health != "degraded" {
		t.Fatalf("Health = %q, want degraded", report.Health)
	}
}

func TestFreshnessDecisionBoundaryIsReadOnly(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	svc := NewService(db)
	report, err := svc.GetFreshness(context.Background())
	if err != nil {
		t.Fatalf("GetFreshness returned error: %v", err)
	}

	if report.DecisionBoundary != "read_only" {
		t.Fatalf("DecisionBoundary = %q, want read_only", report.DecisionBoundary)
	}
}

func TestFreshnessOutputContainsNoTradingRecommendations(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// Seed a stale fund so the actionable field is populated.
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES ('STALE1', 'Stale Fund', 'fund')`,
	); err != nil {
		t.Fatalf("seed fund_details: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES ('STALE1', 'Stale Fund', 10, -10, 1, 10, 0, 0, 'fund', 1)`,
	); err != nil {
		t.Fatalf("seed portfolio_snapshot: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('STALE1', date('now', '-10 days'), 1.0)`,
	); err != nil {
		t.Fatalf("seed nav_history: %v", err)
	}

	svc := NewService(db)
	report, err := svc.GetFreshness(context.Background())
	if err != nil {
		t.Fatalf("GetFreshness returned error: %v", err)
	}

	if strings.Contains(report.Actionable, "买入") || strings.Contains(report.Actionable, "卖出") {
		t.Fatalf("Actionable contains trading language: %q", report.Actionable)
	}
	if strings.Contains(report.Actionable, "buy") || strings.Contains(report.Actionable, "sell") {
		t.Fatalf("Actionable contains trading language: %q", report.Actionable)
	}
	if strings.Contains(report.Actionable, "建议扣款") {
		t.Fatalf("Actionable contains investment decision language: %q", report.Actionable)
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}

	for _, stmt := range adminFixtureStatements {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			db.Close()
			t.Fatalf("exec fixture statement %q: %v", stmt, err)
		}
	}
	return db
}

var adminFixtureStatements = []string{
	`CREATE TABLE fund_details (
		fund_code TEXT PRIMARY KEY,
		fund_name TEXT,
		fund_type TEXT,
		security_type TEXT DEFAULT 'fund',
		market TEXT DEFAULT ''
	)`,
	`CREATE TABLE nav_history (
		fund_code TEXT,
		date TEXT,
		unit_nav REAL,
		daily_change_pct REAL DEFAULT 0,
		security_type TEXT DEFAULT 'fund'
	)`,
	`CREATE TABLE portfolio_snapshot (
			fund_code TEXT NOT NULL,
		fund_name TEXT,
		held_shares REAL,
		total_cost REAL,
		latest_nav REAL,
		current_value REAL,
		unrealized_pnl REAL,
		pnl_pct REAL,
		security_type TEXT DEFAULT 'fund',
			portfolio_id INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (fund_code, portfolio_id)
		)`,
	`CREATE TABLE transactions (
		seq INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id TEXT,
		trade_time TEXT,
		confirm_date TEXT,
		trade_type TEXT,
		direction TEXT,
		fund_code TEXT,
		fund_name TEXT,
		confirm_amount REAL,
		confirm_share REAL,
		fee REAL,
		signed_cash_flow REAL,
		signed_share_change REAL,
		settlement_days INTEGER
	)`,
	`CREATE TABLE dca_plans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fund_code TEXT NOT NULL,
		fund_name TEXT,
		amount REAL NOT NULL,
		frequency TEXT NOT NULL DEFAULT 'weekday',
		weekday_mask TEXT NOT NULL DEFAULT '1,2,3,4,5',
		trade_type TEXT NOT NULL DEFAULT '',
		portfolio_id INTEGER NOT NULL DEFAULT 1,
		start_date TEXT NOT NULL,
		end_date TEXT,
		active INTEGER NOT NULL DEFAULT 1,
		source TEXT NOT NULL DEFAULT 'manual',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`,
}
