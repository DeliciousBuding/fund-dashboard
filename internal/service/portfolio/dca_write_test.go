package portfolio

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	db "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
)

func TestUpsertAndDisableDCAPlan(t *testing.T) {
	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS dca_plans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fund_code TEXT NOT NULL,
		fund_name TEXT,
		amount REAL NOT NULL,
		frequency TEXT NOT NULL,
		weekday_mask TEXT NOT NULL,
		trade_type TEXT NOT NULL,
		portfolio_id INTEGER NOT NULL,
		start_date TEXT NOT NULL,
		end_date TEXT,
		active INTEGER NOT NULL,
		source TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	svc := NewService(db)
	res, err := svc.UpsertDCAPlan(context.Background(), UpsertDCAPlanInput{
		FundCode: "019173", FundName: "Test", Amount: 100, Frequency: "weekday", PortfolioID: 1,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !res.OK || res.Plan.ID <= 0 || res.Plan.FundCode != "019173" || res.Plan.Active != 1 {
		t.Fatalf("upsert result=%+v", res)
	}
	id := res.Plan.ID
	res, err = svc.UpsertDCAPlan(context.Background(), UpsertDCAPlanInput{
		ID: id, FundCode: "019173", Amount: 200, Frequency: "weekday", PortfolioID: 1,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if res.Plan.Amount != 200 {
		t.Fatalf("amount=%v", res.Plan.Amount)
	}
	dis, err := svc.DisableDCAPlan(context.Background(), id)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if !dis.Updated {
		t.Fatalf("disable not updated")
	}
	plans, err := svc.ListDCAPlans(context.Background(), ListDCAPlansOptions{ActiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plans {
		if p.ID == id {
			t.Fatalf("disabled plan still active in active-only list")
		}
	}
}

func TestUpsertDCAPlanRejectsBadActiveAndFrequency(t *testing.T) {
	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE dca_plans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fund_code TEXT, fund_name TEXT, amount REAL, frequency TEXT, weekday_mask TEXT,
		trade_type TEXT, portfolio_id INTEGER, start_date TEXT, end_date TEXT, active INTEGER,
		source TEXT, created_at TEXT, updated_at TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	svc := NewService(db)
	bad := 2
	if _, err := svc.UpsertDCAPlan(context.Background(), UpsertDCAPlanInput{FundCode: "019173", Amount: 100, Active: &bad}); err == nil {
		t.Fatal("expected active reject")
	}
	if _, err := svc.UpsertDCAPlan(context.Background(), UpsertDCAPlanInput{FundCode: "019173", Amount: 100, Frequency: strings.Repeat("w", 33)}); err == nil {
		t.Fatal("expected frequency reject")
	}
}
