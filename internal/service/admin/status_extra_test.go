package admin

import (
	"context"
	"strings"
	"testing"
)

func TestGetStatusByCodeUnknownSecurity(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	report, err := svc.GetStatusByCode(context.Background(), " xyz ")
	if err != nil {
		t.Fatalf("GetStatusByCode: %v", err)
	}
	if report.Code != "XYZ" {
		t.Fatalf("Code = %q, want normalized XYZ", report.Code)
	}
	if report.Name != nil || report.Type != nil || report.SecurityType != nil || report.Market != nil {
		t.Fatalf("identity must be nil for unknown code: name=%v type=%v security=%v market=%v",
			report.Name, report.Type, report.SecurityType, report.Market)
	}
	if report.Transactions.N != 0 || report.NAV.N != 0 {
		t.Fatalf("ranges = tx %+v nav %+v, want zero", report.Transactions, report.NAV)
	}
	if report.Transactions.First != nil || report.Transactions.Last != nil || report.NAV.First != nil || report.NAV.Last != nil {
		t.Fatalf("range bounds must be nil when empty")
	}
	if report.Position.HeldShares != 0 {
		t.Fatalf("held_shares = %v, want 0", report.Position.HeldShares)
	}
	if len(report.Trading) != 0 {
		t.Fatalf("trading = %v, want empty", report.Trading)
	}
}

func TestGetStatusByCodePositionNullFields(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
		VALUES ('019173', 'Null Fund', 5, NULL, NULL, NULL, NULL, NULL, 'fund', 1)
	`); err != nil {
		t.Fatalf("seed null position: %v", err)
	}

	report, err := svc.GetStatusByCode(context.Background(), "019173")
	if err != nil {
		t.Fatalf("GetStatusByCode: %v", err)
	}
	if report.Position.HeldShares != 5 {
		t.Fatalf("held_shares = %v, want 5", report.Position.HeldShares)
	}
	if report.Position.TotalCost != nil || report.Position.CurrentValue != nil ||
		report.Position.UnrealizedPNL != nil || report.Position.PNLPct != nil {
		t.Fatalf("null fields must stay nil: total=%v current=%v unrealized=%v pnl_pct=%v",
			report.Position.TotalCost, report.Position.CurrentValue, report.Position.UnrealizedPNL, report.Position.PNLPct)
	}
}

func TestQueryTradingReportsFundStatus(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE fund_status (fund_code TEXT, purchase_status TEXT, redemption_status TEXT);
		INSERT INTO fund_status (fund_code, purchase_status, redemption_status) VALUES
			('019173', '开放', '开放'),
			('019174', '暂停', NULL);
	`); err != nil {
		t.Fatalf("seed fund_status: %v", err)
	}

	svc := NewServiceWithDriver(db, "sqlite")
	trading, err := svc.queryTrading(context.Background(), "019173")
	if err != nil {
		t.Fatalf("queryTrading: %v", err)
	}
	if len(trading) != 2 || trading["purchase_status"] != "开放" || trading["redemption_status"] != "开放" {
		t.Fatalf("trading = %v, want both statuses", trading)
	}

	trading, err = svc.queryTrading(context.Background(), "019174")
	if err != nil {
		t.Fatalf("queryTrading partial: %v", err)
	}
	if len(trading) != 1 || trading["purchase_status"] != "暂停" {
		t.Fatalf("trading = %v, want only purchase_status", trading)
	}
	if _, ok := trading["redemption_status"]; ok {
		t.Fatalf("redemption_status = %v, want omitted for NULL", trading["redemption_status"])
	}

	trading, err = svc.queryTrading(context.Background(), "MISSING")
	if err != nil {
		t.Fatalf("queryTrading missing row: %v", err)
	}
	if len(trading) != 0 {
		t.Fatalf("trading = %v, want empty for missing row", trading)
	}
}

func TestTableExistsSQLiteFallback(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	svc := NewServiceWithDriver(db, "sqlite")

	exists, err := svc.tableExists(context.Background(), "transactions")
	if err != nil {
		t.Fatalf("tableExists transactions: %v", err)
	}
	if !exists {
		t.Fatal("transactions should exist")
	}
	exists, err = svc.tableExists(context.Background(), "fund_status")
	if err != nil {
		t.Fatalf("tableExists fund_status: %v", err)
	}
	if exists {
		t.Fatal("fund_status should not exist in the base fixture")
	}
}

func TestNormalizeSecurityCodeTable(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{" 019173 ", "019173"},
		{"019173", "019173"},
		{"173", "000173"},
		{"1", "000001"},
		{"123456", "123456"},
		{"1234567890123456789012345678901234567890", "12345678901234567890123456789012"},
		{"abc", "ABC"},
		{"nasdaq-fund-0001", "NASDAQ-FUND-0001"},
		{strings.Repeat("x", 40), strings.Repeat("X", 32)},
	}
	for _, tc := range cases {
		if got := NormalizeSecurityCode(tc.in); got != tc.want {
			t.Fatalf("NormalizeSecurityCode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
