package admin

import (
	"context"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestVerifyAllClearWhenDataIsClean(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// Seed a clean held fund: has nav_history, positive shares, settlement_days set.
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES ('CLEAN1', 'Clean Fund', 'fund')`,
	); err != nil {
		t.Fatalf("seed fund_details: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES ('CLEAN1', 'Clean Fund', 10, -10, 1, 10, 0, 0, 'fund', 1)`,
	); err != nil {
		t.Fatalf("seed portfolio_snapshot: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('CLEAN1', date('now'), 1.0)`,
	); err != nil {
		t.Fatalf("seed nav_history: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days) VALUES ('TX001', '2025-01-01', '2025-01-03', '用户买入', 'buy', 'CLEAN1', 'Clean Fund', 100, 80, 0.15, -100, 80, 2)`,
	); err != nil {
		t.Fatalf("seed transactions: %v", err)
	}

	svc := NewServiceWithDriver(db, "sqlite")
	report, err := svc.VerifyData(context.Background())
	if err != nil {
		t.Fatalf("VerifyData returned error: %v", err)
	}

	if !report.OK {
		t.Fatalf("OK = false, want true; issues=%#v", report.Issues)
	}
	if len(report.Issues) != 1 || report.Issues[0] != "all clear" {
		t.Fatalf("Issues = %#v, want [\"all clear\"]", report.Issues)
	}
	if len(report.Details.SecuritiesWithoutNAV) != 0 {
		t.Fatalf("SecuritiesWithoutNAV = %d, want 0", len(report.Details.SecuritiesWithoutNAV))
	}
	if len(report.Details.NegativePositions) != 0 {
		t.Fatalf("NegativePositions = %d, want 0", len(report.Details.NegativePositions))
	}
	if report.Details.MissingSettlement != 0 {
		t.Fatalf("MissingSettlement = %d, want 0", report.Details.MissingSettlement)
	}
}

func TestVerifyDetectsNegativePositions(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES ('NEG1', 'Negative Fund', -5, 10, 1, -5, -15, 0, 'fund', 1)`,
	); err != nil {
		t.Fatalf("seed negative portfolio_snapshot: %v", err)
	}

	svc := NewServiceWithDriver(db, "sqlite")
	report, err := svc.VerifyData(context.Background())
	if err != nil {
		t.Fatalf("VerifyData returned error: %v", err)
	}

	if report.OK {
		t.Fatalf("OK = true, want false; issues=%#v", report.Issues)
	}
	issues := strings.Join(report.Issues, " ")
	if !strings.Contains(issues, "negative positions") {
		t.Fatalf("Issues = %#v, want substring \"negative positions\"", report.Issues)
	}
	if len(report.Details.NegativePositions) != 1 {
		t.Fatalf("NegativePositions count = %d, want 1", len(report.Details.NegativePositions))
	}
	if report.Details.NegativePositions[0].Code != "NEG1" {
		t.Fatalf("NegativePositions[0].Code = %q, want NEG1", report.Details.NegativePositions[0].Code)
	}
	if report.Details.NegativePositions[0].Shares != -5 {
		t.Fatalf("NegativePositions[0].Shares = %f, want -5", report.Details.NegativePositions[0].Shares)
	}
}

func TestVerifyDetectsMissingSettlementDays(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days) VALUES ('TX001', '2025-01-01', '2025-01-03', '用户买入', 'buy', 'FUND1', 'Fund 1', 100, 80, 0.15, -100, 80, NULL)`,
	); err != nil {
		t.Fatalf("seed transaction: %v", err)
	}

	svc := NewServiceWithDriver(db, "sqlite")
	report, err := svc.VerifyData(context.Background())
	if err != nil {
		t.Fatalf("VerifyData returned error: %v", err)
	}

	if report.OK {
		t.Fatalf("OK = true, want false; issues=%#v", report.Issues)
	}
	issues := strings.Join(report.Issues, " ")
	if !strings.Contains(issues, "missing settlement_days") {
		t.Fatalf("Issues = %#v, want substring \"missing settlement_days\"", report.Issues)
	}
	if report.Details.MissingSettlement == 0 {
		t.Fatalf("MissingSettlement = %d, want > 0", report.Details.MissingSettlement)
	}
}

func TestVerifyDetectsSecuritiesWithoutNAV(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// A held fund with no nav_history entry.
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES ('NONAV1', 'No NAV Fund', 'fund')`,
	); err != nil {
		t.Fatalf("seed fund_details: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES ('NONAV1', 'No NAV Fund', 10, -10, 1, 10, 0, 0, 'fund', 1)`,
	); err != nil {
		t.Fatalf("seed portfolio_snapshot: %v", err)
	}

	svc := NewServiceWithDriver(db, "sqlite")
	report, err := svc.VerifyData(context.Background())
	if err != nil {
		t.Fatalf("VerifyData returned error: %v", err)
	}

	if report.OK {
		t.Fatalf("OK = true, want false; issues=%#v", report.Issues)
	}
	issues := strings.Join(report.Issues, " ")
	if !strings.Contains(issues, "missing NAV") {
		t.Fatalf("Issues = %#v, want substring \"missing NAV\"", report.Issues)
	}
	if len(report.Details.SecuritiesWithoutNAV) != 1 {
		t.Fatalf("SecuritiesWithoutNAV count = %d, want 1", len(report.Details.SecuritiesWithoutNAV))
	}
	if report.Details.SecuritiesWithoutNAV[0] != "NONAV1" {
		t.Fatalf("SecuritiesWithoutNAV[0] = %q, want NONAV1", report.Details.SecuritiesWithoutNAV[0])
	}
}

func TestVerifyDecisionBoundaryIsFactsOnly(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	svc := NewServiceWithDriver(db, "sqlite")
	report, err := svc.VerifyData(context.Background())
	if err != nil {
		t.Fatalf("VerifyData returned error: %v", err)
	}

	if report.DecisionBoundary != "facts_only" {
		t.Fatalf("DecisionBoundary = %q, want facts_only", report.DecisionBoundary)
	}
}
