package portfolio

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestListDCAPlansSoftLimit(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE dca_plans (
		id INTEGER PRIMARY KEY,
		fund_code TEXT,
		fund_name TEXT,
		amount REAL,
		frequency TEXT,
		weekday_mask TEXT,
		trade_type TEXT,
		portfolio_id INTEGER,
		start_date TEXT,
		end_date TEXT,
		active INTEGER,
		source TEXT,
		created_at TEXT,
		updated_at TEXT
	)`); err != nil {
		t.Fatal(err)
	}
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
