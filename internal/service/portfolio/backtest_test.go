package portfolio

import (
	"context"
	"encoding/json"
	"testing"
)

func TestServiceRunBacktestSimulatesDCAWithoutSideEffects(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO fund_details (fund_code, fund_name, fund_type, security_type, market)
			VALUES ('BT1', 'Backtest Fund', 'test', 'fund', 'CN');
		INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type)
			VALUES
			('BT1', '2026-01-01', 1.0, 0, 'fund'),
			('BT1', '2026-01-15', 1.2, 0, 'fund'),
			('BT1', '2026-02-01', 2.0, 0, 'fund'),
			('BT1', '2026-03-01', 1.5, 0, 'fund');
	`); err != nil {
		t.Fatalf("seed backtest navs: %v", err)
	}

	service := NewService(db)
	result, err := service.RunBacktest(context.Background(), BacktestOptions{
		FundCode:   "BT1",
		Strategy:   "dca",
		StartDate:  "2026-01-01",
		BaseAmount: 100,
	})
	if err != nil {
		t.Fatalf("RunBacktest returned error: %v", err)
	}

	if result.FundCode != "BT1" ||
		result.Strategy != "dca" ||
		result.StartDate != "2026-01-01" ||
		result.EndDate != "2026-03-01" ||
		result.BaseAmount != 100 ||
		result.TotalInvested != 300 ||
		result.FinalValue != 25 ||
		result.DecisionBoundary != "facts_only" ||
		result.SideEffects != "none" {
		t.Fatalf("result summary = %#v, want TS-compatible DCA facts", result)
	}
	if len(result.Trades) != 3 || len(result.Timeline) != 4 {
		t.Fatalf("trades/timeline length = %d/%d, want 3/4", len(result.Trades), len(result.Timeline))
	}
	firstTrade := result.Trades[0]
	if firstTrade.Date != "2026-01-01" ||
		firstTrade.Action != "buy" ||
		firstTrade.Price != 1 ||
		firstTrade.Shares != 100 ||
		firstTrade.Amount != 100 ||
		firstTrade.Reason != "定期定额买入 (DCA)" {
		t.Fatalf("first trade = %#v, want first monthly DCA buy", firstTrade)
	}
	last := result.Timeline[len(result.Timeline)-1]
	if last.Date != "2026-03-01" ||
		last.SharesHeld != 216.6667 ||
		last.Cash != -300 ||
		last.EquityValue != 325 ||
		last.TotalValue != 25 ||
		last.TotalInvested != 300 {
		t.Fatalf("last timeline point = %#v, want TS-compatible DCA point", last)
	}
	if result.Comparison.LumpSum.Invested != 1200 ||
		result.Comparison.LumpSum.FinalValue != 1800 ||
		result.Comparison.LumpSum.ReturnPct != 50 ||
		result.Comparison.DCA.Invested != 300 ||
		result.Comparison.DCA.FinalValue != 325 ||
		result.Comparison.DCA.ReturnPct != 8.33 {
		t.Fatalf("comparison = %#v, want lump-sum and DCA comparison facts", result.Comparison)
	}
}

func TestServiceRunBacktestReturnsNoData(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	service := NewService(db)
	result, err := service.RunBacktest(context.Background(), BacktestOptions{
		FundCode:   "MISSING",
		Strategy:   "dca",
		StartDate:  "2026-01-01",
		BaseAmount: 100,
	})
	if err != nil {
		t.Fatalf("RunBacktest returned error: %v", err)
	}
	if result.Error != "no_data" || result.FundCode != "MISSING" {
		t.Fatalf("result = %#v, want no_data error", result)
	}
}

func TestRunBacktestKeepsExtremeLossJSONSafe(t *testing.T) {
	result := runBacktestFromNAVs([]backtestNAV{
		{Date: "2026-01-01", FundCode: "LOSS", UnitNAV: 1.0},
		{Date: "2026-02-01", FundCode: "LOSS", UnitNAV: 0.1},
	}, BacktestOptions{
		FundCode:   "LOSS",
		Strategy:   "dca",
		StartDate:  "2026-01-01",
		BaseAmount: 100,
	})

	if _, err := json.Marshal(result); err != nil {
		t.Fatalf("backtest result should remain JSON-safe under extreme loss: %v; result=%#v", err, result)
	}
}
