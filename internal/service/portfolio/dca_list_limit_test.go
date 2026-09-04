package portfolio

import (
	"context"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

func TestListDCAPlansSoftLimit(t *testing.T) {
	db := testutil.OpenTempDBWithProductionSchema(t)
	// Insert a few rows; LIMIT 5000 must not break listing.
	for i := 1; i <= 3; i++ {
		if _, err := db.Exec(`INSERT INTO dca_plans
			(id, fund_code, fund_name, amount, frequency, weekday_mask, trade_type, portfolio_id, start_date, end_date, active, source, created_at, updated_at)
			VALUES (?, ?, 'n', 100, 'weekly', '1', 'buy', 1, '2026-01-01', NULL, 1, 'test', '2026-01-01', '2026-01-01')`,
			i, "C"+string(rune('0'+i))); err != nil {
			t.Fatal(err)
		}
	}
	svc := NewService(db)
	plans, err := svc.ListDCAPlans(context.Background(), ListDCAPlansOptions{})
	if err != nil {
		t.Fatalf("ListDCAPlans: %v", err)
	}
	if len(plans) != 3 {
		t.Fatalf("got %d plans", len(plans))
	}
}
