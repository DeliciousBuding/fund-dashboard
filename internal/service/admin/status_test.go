package admin

import (
	"context"
	"testing"
)

func TestGetStatusByCodeReportsRangeAndPosition(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market) VALUES
			('019173', 'Nasdaq Fund', 'QDII', 'fund', 'US');
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES
			('019173', 'Nasdaq Fund', 10, -10, 1.2, 12, 2, 20, 'fund', 1);
		INSERT INTO nav_history (fund_code, date, unit_nav) VALUES
			('019173', '2026-07-10', 1.0),
			('019173', '2026-07-16', 1.2);
		INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days) VALUES
			('TX001', '2026-07-01', '2026-07-03', '用户买入', 'buy', '019173', 'Nasdaq Fund', 100, 90, 0.1, -100, 90, 2);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := NewService(db)
	report, err := svc.GetStatusByCode(context.Background(), "019173")
	if err != nil {
		t.Fatalf("GetStatusByCode: %v", err)
	}

	if report.Code != "019173" {
		t.Fatalf("Code = %q, want 019173", report.Code)
	}
	if report.Name == nil || *report.Name != "Nasdaq Fund" {
		t.Fatalf("Name = %v, want Nasdaq Fund", report.Name)
	}
	if report.Transactions.N != 1 {
		t.Fatalf("Transactions.N = %d, want 1", report.Transactions.N)
	}
	if report.NAV.N != 2 {
		t.Fatalf("NAV.N = %d, want 2", report.NAV.N)
	}
	if report.NAV.First == nil || *report.NAV.First != "2026-07-10" {
		t.Fatalf("NAV.First = %v, want 2026-07-10", report.NAV.First)
	}
	if report.NAV.Last == nil || *report.NAV.Last != "2026-07-16" {
		t.Fatalf("NAV.Last = %v, want 2026-07-16", report.NAV.Last)
	}
	if report.Position.HeldShares != 10 {
		t.Fatalf("Position.HeldShares = %v, want 10", report.Position.HeldShares)
	}
	if report.Position.TotalCost == nil || *report.Position.TotalCost != -10 {
		t.Fatalf("Position.TotalCost = %v, want -10", report.Position.TotalCost)
	}
}

func TestGetStatusByCodeNormalizesShortNumericCode(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES ('019173', 'Nasdaq Fund', 'fund');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := NewService(db)
	report, err := svc.GetStatusByCode(context.Background(), "19173")
	if err != nil {
		t.Fatalf("GetStatusByCode: %v", err)
	}
	if report.Code != "019173" {
		t.Fatalf("Code = %q, want normalized 019173", report.Code)
	}
}
