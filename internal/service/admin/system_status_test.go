package admin

import (
	"context"
	"testing"
	"time"
)

func TestGetSystemStatusReportsCounts(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO fund_details (fund_code, fund_name, security_type) VALUES
			('019173', 'Nasdaq Fund', 'fund'),
			('00700', 'Tencent', 'stock');
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id) VALUES
			('019173', 'Nasdaq Fund', 10, -10, 1, 10, 0, 0, 'fund', 1);
		INSERT INTO nav_history (fund_code, date, unit_nav) VALUES
			('019173', '2026-07-15', 1.0),
			('019173', '2026-07-16', 1.1);
		INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days) VALUES
			('TX001', '2026-07-01', '2026-07-03', '用户买入', 'buy', '019173', 'Nasdaq Fund', 100, 90, 0.1, -100, 90, 2);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := NewServiceWithDriver(db, "sqlite")
	report, err := svc.GetSystemStatus(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("GetSystemStatus: %v", err)
	}

	if report.Transactions.Count != 1 {
		t.Fatalf("Transactions.Count = %d, want 1", report.Transactions.Count)
	}
	if report.NAV.Count != 2 {
		t.Fatalf("NAV.Count = %d, want 2", report.NAV.Count)
	}
	if report.NAV.Funds != 1 {
		t.Fatalf("NAV.Funds = %d, want 1", report.NAV.Funds)
	}
	if report.Portfolio.HeldFunds != 1 {
		t.Fatalf("Portfolio.HeldFunds = %d, want 1", report.Portfolio.HeldFunds)
	}
	if report.Securities.Total != 2 {
		t.Fatalf("Securities.Total = %d, want 2", report.Securities.Total)
	}
	if report.Securities.Funds != 1 {
		t.Fatalf("Securities.Funds = %d, want 1", report.Securities.Funds)
	}
	if report.Securities.Stocks != 1 {
		t.Fatalf("Securities.Stocks = %d, want 1", report.Securities.Stocks)
	}
	if report.Anomalies.Count != 0 {
		t.Fatalf("Anomalies.Count = %d, want 0", report.Anomalies.Count)
	}
	if report.DecisionBoundary != "read_only" {
		t.Fatalf("DecisionBoundary = %q, want read_only", report.DecisionBoundary)
	}
}

func TestGetSystemStatusEmptyDatabase(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	svc := NewServiceWithDriver(db, "sqlite")
	report, err := svc.GetSystemStatus(context.Background(), time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("GetSystemStatus on empty db: %v", err)
	}

	if report.Transactions.Count != 0 || report.NAV.Count != 0 || report.Portfolio.HeldFunds != 0 {
		t.Fatalf("expected zero counts on empty db, got tx=%d nav=%d held=%d",
			report.Transactions.Count, report.NAV.Count, report.Portfolio.HeldFunds)
	}
}
