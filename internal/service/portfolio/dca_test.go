package portfolio

import (
	"context"
	"testing"
)

func TestServiceComputeDCAAmountUsesCostDeviationMode(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	service := NewService(db)
	plan, err := service.ComputeDCAAmount(context.Background(), ComputeDCAAmountOptions{
		Code:       "019173",
		BaseAmount: 200,
		Mode:       "nav_deviation",
	})
	if err != nil {
		t.Fatalf("ComputeDCAAmount returned error: %v", err)
	}

	if plan.FundCode != "019173" ||
		plan.SecurityType != "fund" ||
		plan.Mode != "nav_deviation" ||
		plan.BaseAmount != 200 ||
		plan.LatestNAV != 1.35 ||
		plan.CostPerShare == nil ||
		*plan.CostPerShare != 1.1169 ||
		plan.DeviationPct == nil ||
		*plan.DeviationPct != 20.87 ||
		plan.DCARate != 0.525 ||
		plan.ActualAmount != 105 ||
		plan.Signal != "decrease" ||
		plan.Range != "decrease" ||
		plan.DecisionBoundary != "facts_only" ||
		plan.SideEffects != "none" {
		t.Fatalf("plan = %#v, want cost-deviation DCA facts", plan)
	}
}

func TestServiceComputeDCAAmountReportsNoPosition(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	service := NewService(db)
	plan, err := service.ComputeDCAAmount(context.Background(), ComputeDCAAmountOptions{
		Code:       "000001",
		BaseAmount: 30,
		Mode:       "nav_deviation",
	})
	if err != nil {
		t.Fatalf("ComputeDCAAmount returned error: %v", err)
	}

	if plan.Error != "no_position" || plan.Message == "" || plan.FundCode != "000001" {
		t.Fatalf("plan = %#v, want no_position error for missing holding", plan)
	}
}

func TestServiceComputeDCAAmountScopesPortfolio(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	// Move the only holding to portfolio 2 — portfolio 1 should see no position.
	if _, err := db.Exec(`UPDATE portfolio_snapshot SET portfolio_id = 2 WHERE fund_code = '019173'`); err != nil {
		t.Fatalf("update portfolio_id: %v", err)
	}

	service := NewService(db)
	planDefault, err := service.ComputeDCAAmount(context.Background(), ComputeDCAAmountOptions{
		Code:       "019173",
		BaseAmount: 200,
		Mode:       "nav_deviation",
		// PortfolioID default 1
	})
	if err != nil {
		t.Fatalf("default portfolio ComputeDCAAmount: %v", err)
	}
	if planDefault.Error != "no_position" {
		t.Fatalf("default portfolio plan = %#v, want no_position after move to portfolio 2", planDefault)
	}

	planP2, err := service.ComputeDCAAmount(context.Background(), ComputeDCAAmountOptions{
		Code:        "019173",
		BaseAmount:  200,
		Mode:        "nav_deviation",
		PortfolioID: 2,
	})
	if err != nil {
		t.Fatalf("portfolio 2 ComputeDCAAmount: %v", err)
	}
	if planP2.Error != "" || planP2.ActualAmount <= 0 {
		t.Fatalf("portfolio 2 plan = %#v, want successful DCA calculation", planP2)
	}
}
