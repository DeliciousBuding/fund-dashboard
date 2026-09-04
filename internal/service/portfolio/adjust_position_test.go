package portfolio

import (
	"context"
	"path/filepath"
	"testing"

	db "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
	dbpkg "github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
	"github.com/DeliciousBuding/fund-dashboard/internal/snapshot"
)

func TestAdjustPositionOverridesShares(t *testing.T) {
	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := dbpkg.EnsureSQLiteSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO fund_details (fund_code, fund_name, security_type, market) VALUES ('019173','Test','fund','CN')`,
		`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('019173','2026-07-01',1.5)`,
		`INSERT INTO transactions (order_id, trade_time, confirm_date, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share, fee, signed_cash_flow, signed_share_change, settlement_days)
			VALUES ('buy1','2026-06-01T10:00:00+08:00','2026-06-01','用户买入','buy','019173','Test',120,100,0,-120,100,1)`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
			VALUES ('019173','Test',100,-120,1.5,150,30,25,'fund',1)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	svc := NewService(db)
	res, err := svc.AdjustPosition(context.Background(), AdjustPositionInput{Code: "019173", Shares: 50})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Shares != 50 {
		t.Fatalf("%+v", res)
	}
	if res.DeltaShares != -50 {
		t.Fatalf("delta=%v want -50", res.DeltaShares)
	}
	var shares, value, pnl float64
	if err := db.QueryRow(`SELECT held_shares, current_value, unrealized_pnl FROM portfolio_snapshot WHERE fund_code='019173'`).Scan(&shares, &value, &pnl); err != nil {
		t.Fatal(err)
	}
	// 50 * 1.5 = 75; cost still -120 (cash-neutral adjust); unrealized = 75 + (-120) = -45
	if shares != 50 || value != 75 || pnl != -45 {
		t.Fatalf("shares=%.1f value=%.1f pnl=%.1f", shares, value, pnl)
	}

	// Durability: recalcSnapshotLight (as price refresh does) must keep 50 shares.
	if err := snapshot.RecalcForPortfolio(context.Background(), db, "019173", 1, snapshot.ModeLight); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT held_shares FROM portfolio_snapshot WHERE fund_code='019173'`).Scan(&shares); err != nil {
		t.Fatal(err)
	}
	if shares != 50 {
		t.Fatalf("after recalc held_shares=%.1f want 50", shares)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(1) FROM transactions WHERE order_id LIKE 'adj-%'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("balancing txs=%d want 1", n)
	}
}

func TestAdjustPositionRejectsHugeShares(t *testing.T) {
	db, err := db.Open(context.Background(), db.Options{Driver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "fund.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := dbpkg.EnsureSQLiteSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO fund_details (fund_code, fund_name) VALUES ('019173','Test')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	svc := NewService(db)
	if _, err := svc.AdjustPosition(context.Background(), AdjustPositionInput{Code: "019173", Shares: 1e9 + 1}); err == nil {
		t.Fatal("expected shares too large")
	}
	if _, err := svc.AdjustPosition(context.Background(), AdjustPositionInput{Code: "019173", Shares: 1, Reason: string(make([]byte, 201))}); err == nil {
		t.Fatal("expected reason too long")
	}
}
