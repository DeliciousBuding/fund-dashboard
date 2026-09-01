package snapshot

import (
	"context"
	"database/sql"
	"math"
	"testing"

	_ "modernc.org/sqlite"
)

func openRecalcDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, q := range []string{
		`CREATE TABLE portfolio_snapshot (
			fund_code TEXT PRIMARY KEY,
			fund_name TEXT,
			held_shares REAL,
			total_cost REAL,
			latest_nav REAL,
			current_value REAL,
			unrealized_pnl REAL,
			pnl_pct REAL,
			security_type TEXT,
			portfolio_id INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE transactions (
			fund_code TEXT,
			fund_name TEXT,
			signed_share_change REAL,
			signed_cash_flow REAL
		)`,
		`CREATE TABLE nav_history (
			fund_code TEXT,
			date TEXT,
			unit_nav REAL
		)`,
		`CREATE TABLE fund_details (
			fund_code TEXT PRIMARY KEY,
			fund_name TEXT,
			security_type TEXT
		)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func seedRecalc(t *testing.T, db *sql.DB, stmts []string) {
	t.Helper()
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
}

type snapRow struct {
	HeldShares   float64
	TotalCost    float64
	LatestNav    sql.NullFloat64
	CurrentValue float64
	Unrealized   float64
	PnlPct       float64
	FundName     string
	SecurityType string
}

func readSnapshotRow(t *testing.T, db *sql.DB, code string) snapRow {
	t.Helper()
	var row snapRow
	if err := db.QueryRow(`
		SELECT held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct,
			COALESCE(fund_name, ''), COALESCE(security_type, '')
		FROM portfolio_snapshot WHERE fund_code = ?
	`, code).Scan(&row.HeldShares, &row.TotalCost, &row.LatestNav, &row.CurrentValue,
		&row.Unrealized, &row.PnlPct, &row.FundName, &row.SecurityType); err != nil {
		t.Fatalf("read snapshot row: %v", err)
	}
	return row
}

func closeEnough(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestRecalcFullModeLedgerMath(t *testing.T) {
	cases := []struct {
		name string
		code string
		seed []string
		want snapRow
	}{
		{
			name: "single buy",
			code: "F1",
			seed: []string{
				`INSERT INTO fund_details VALUES ('F1', 'Example Fund', 'fund')`,
				`INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow) VALUES ('F1', 'Example Fund', 100, -1000)`,
				`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('F1', '2026-01-02', 1.5)`,
			},
			want: snapRow{
				HeldShares: 100, TotalCost: -1000,
				LatestNav:    sql.NullFloat64{Float64: 1.5, Valid: true},
				CurrentValue: 150, Unrealized: -850, PnlPct: -85,
				FundName: "Example Fund", SecurityType: "fund",
			},
		},
		{
			name: "full round trip zeroes value and pnl",
			code: "F1",
			seed: []string{
				`INSERT INTO fund_details VALUES ('F1', 'Example Fund', 'fund')`,
				`INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow) VALUES ('F1', 'Example Fund', 100, -1000)`,
				`INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow) VALUES ('F1', 'Example Fund', -100, 990)`,
				`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('F1', '2026-01-02', 1.5)`,
			},
			want: snapRow{
				HeldShares: 0, TotalCost: -10,
				LatestNav:    sql.NullFloat64{Float64: 1.5, Valid: true},
				CurrentValue: 0, Unrealized: 0, PnlPct: 0,
				FundName: "Example Fund", SecurityType: "fund",
			},
		},
		{
			name: "float dust below threshold is not a holding",
			code: "F1",
			seed: []string{
				`INSERT INTO fund_details VALUES ('F1', 'Dust Fund', 'fund')`,
				`INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow) VALUES ('F1', 'Dust Fund', 0.0000001, -0.000001)`,
			},
			want: snapRow{
				HeldShares: 0, TotalCost: -0.000001,
				CurrentValue: 0, Unrealized: 0, PnlPct: 0,
				FundName: "Dust Fund", SecurityType: "fund",
			},
		},
		{
			name: "identity falls back to transaction fund_name",
			code: "F9",
			seed: []string{
				`INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow) VALUES ('F9', 'Tx Name', 10, -100)`,
			},
			want: snapRow{
				HeldShares: 10, TotalCost: -100,
				CurrentValue: 0, Unrealized: -100, PnlPct: -100,
				FundName: "Tx Name", SecurityType: "fund",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openRecalcDB(t)
			seedRecalc(t, db, tc.seed)
			if err := Recalc(context.Background(), db, tc.code, ModeFull); err != nil {
				t.Fatal(err)
			}
			got := readSnapshotRow(t, db, tc.code)
			if !closeEnough(got.HeldShares, tc.want.HeldShares) {
				t.Errorf("held_shares = %v, want %v", got.HeldShares, tc.want.HeldShares)
			}
			if !closeEnough(got.TotalCost, tc.want.TotalCost) {
				t.Errorf("total_cost = %v, want %v", got.TotalCost, tc.want.TotalCost)
			}
			if got.LatestNav.Valid != tc.want.LatestNav.Valid ||
				(got.LatestNav.Valid && !closeEnough(got.LatestNav.Float64, tc.want.LatestNav.Float64)) {
				t.Errorf("latest_nav = %v, want %v", got.LatestNav, tc.want.LatestNav)
			}
			if !closeEnough(got.CurrentValue, tc.want.CurrentValue) {
				t.Errorf("current_value = %v, want %v", got.CurrentValue, tc.want.CurrentValue)
			}
			if !closeEnough(got.Unrealized, tc.want.Unrealized) {
				t.Errorf("unrealized_pnl = %v, want %v", got.Unrealized, tc.want.Unrealized)
			}
			if !closeEnough(got.PnlPct, tc.want.PnlPct) {
				t.Errorf("pnl_pct = %v, want %v", got.PnlPct, tc.want.PnlPct)
			}
			if got.FundName != tc.want.FundName {
				t.Errorf("fund_name = %q, want %q", got.FundName, tc.want.FundName)
			}
			if got.SecurityType != tc.want.SecurityType {
				t.Errorf("security_type = %q, want %q", got.SecurityType, tc.want.SecurityType)
			}
		})
	}
}

func TestRecalcLightModePreservesIdentityAndLatestNAV(t *testing.T) {
	db := openRecalcDB(t)
	seedRecalc(t, db, []string{
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
		 VALUES ('F1', 'KeepName', 5, -50, 2.5, 12.5, -37.5, -75, 'stock', 1)`,
		`INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow) VALUES ('F1', 'Tx Name', 100, -1000)`,
	})

	if err := RecalcForPortfolio(context.Background(), db, "F1", 1, ModeLight); err != nil {
		t.Fatal(err)
	}
	got := readSnapshotRow(t, db, "F1")
	if got.FundName != "KeepName" || got.SecurityType != "stock" {
		t.Fatalf("light mode clobbered identity: %+v", got)
	}
	if !got.LatestNav.Valid || !closeEnough(got.LatestNav.Float64, 2.5) {
		t.Fatalf("light mode clobbered latest_nav: %+v", got.LatestNav)
	}
	if !closeEnough(got.HeldShares, 100) || !closeEnough(got.TotalCost, -1000) {
		t.Fatalf("light mode shares/cost wrong: %+v", got)
	}
}

func TestRecalcLightModeInsertResolvesIdentityWithNullNAV(t *testing.T) {
	db := openRecalcDB(t)
	seedRecalc(t, db, []string{
		`INSERT INTO fund_details VALUES ('F2', 'Light Fund', 'stock')`,
		`INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow) VALUES ('F2', 'Light Fund', 10, -20)`,
	})

	if err := RecalcForPortfolio(context.Background(), db, "F2", 7, ModeLight); err != nil {
		t.Fatal(err)
	}
	got := readSnapshotRow(t, db, "F2")
	if got.FundName != "Light Fund" || got.SecurityType != "stock" {
		t.Fatalf("light insert identity wrong: %+v", got)
	}
	if got.LatestNav.Valid {
		t.Fatalf("light insert should keep latest_nav NULL, got %+v", got.LatestNav)
	}
	if !closeEnough(got.HeldShares, 10) || !closeEnough(got.TotalCost, -20) {
		t.Fatalf("light insert shares/cost wrong: %+v", got)
	}
}

func TestNullIfZero(t *testing.T) {
	cases := []struct {
		in    float64
		want  any
		isNil bool
	}{
		{0, nil, true},
		{1.5, 1.5, false},
		{-0.0, nil, true},
	}
	for _, tc := range cases {
		got := nullIfZero(tc.in)
		if tc.isNil {
			if got != nil {
				t.Fatalf("nullIfZero(%v) = %v, want nil", tc.in, got)
			}
			continue
		}
		if v, ok := got.(float64); !ok || !closeEnough(v, tc.want.(float64)) {
			t.Fatalf("nullIfZero(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
