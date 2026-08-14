package portfolio

import (
	"context"
	"testing"
)

func TestServiceCompareFundsReturnsRiskReturnFacts(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
			VALUES ('CMP1', 'Compare One', 'test', 'fund', 'US');
		INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
			VALUES
			('CMP1-BUY', '2025-06-01T00:00:00Z', '2025-06-02', '用户买入', 'buy', 'CMP1', 'Compare One', 100, 100, 0, -100, 100, 1),
			('CMP1-DIV', '2026-06-01T00:00:00Z', '2026-06-02', '现金分红', 'dividend', 'CMP1', 'Compare One', 10, 0, 0, 10, 0, 1);
		INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type)
			VALUES
			('CMP1', '2026-06-01', 1.00, 0, 'fund'),
			('CMP1', '2026-06-02', 1.10, 0, 'fund'),
			('CMP1', '2026-06-03', 1.20, 0, 'fund'),
			('CMP1', '2026-06-04', 1.00, 0, 'fund'),
			('CMP1', '2026-06-05', 1.30, 0, 'fund'),
			('CMP1', '2026-06-06', 1.40, 0, 'fund'),
			('CMP1', '2026-06-07', 1.50, 0, 'fund'),
			('CMP1', '2026-06-08', 1.60, 0, 'fund'),
			('CMP1', '2026-06-09', 1.70, 0, 'fund'),
			('CMP1', '2026-06-10', 1.80, 0, 'fund');
	`); err != nil {
		t.Fatalf("seed compare fixture: %v", err)
	}

	service := NewService(db)
	results, err := service.CompareFunds(context.Background(), []string{"CMP1", "MISSING"}, 1)
	if err != nil {
		t.Fatalf("CompareFunds returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results length = %d, want 2: %#v", len(results), results)
	}

	first := results[0]
	if first.Code != "CMP1" || first.Name != "Compare One" || first.Market != "US" {
		t.Fatalf("identity = %#v, want CMP1 Compare One US", first)
	}
	if first.XIRR == nil || *first.XIRR != 90 {
		t.Fatalf("XIRR = %v, want 90", first.XIRR)
	}
	if first.Volatility == nil || *first.Volatility != 178.99 {
		t.Fatalf("Volatility = %v, want 178.99", first.Volatility)
	}
	if first.Sharpe == nil || *first.Sharpe != 0.5028 {
		t.Fatalf("Sharpe = %v, want 0.5028", first.Sharpe)
	}
	if first.MaxDrawdown == nil || *first.MaxDrawdown != 16.67 {
		t.Fatalf("MaxDrawdown = %v, want 16.67", first.MaxDrawdown)
	}
	if first.Calmar == nil || *first.Calmar != 5.3989 {
		t.Fatalf("Calmar = %v, want 5.3989", first.Calmar)
	}
	if first.DecisionBoundary != "facts_only" {
		t.Fatalf("DecisionBoundary = %q, want facts_only", first.DecisionBoundary)
	}

	missing := results[1]
	if missing.Code != "MISSING" || missing.Name != "MISSING" || missing.Market != "" {
		t.Fatalf("missing identity = %#v, want code fallback", missing)
	}
	if missing.XIRR != nil || missing.Volatility != nil || missing.Sharpe != nil ||
		missing.MaxDrawdown != nil || missing.Calmar != nil {
		t.Fatalf("missing metrics = %#v, want nil metrics", missing)
	}
}
