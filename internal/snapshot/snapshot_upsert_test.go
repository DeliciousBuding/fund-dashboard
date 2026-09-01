package snapshot

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"
)

func TestIsUniqueViolation(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"sqlite single pk", errors.New("UNIQUE constraint failed: portfolio_snapshot.fund_code"), true},
		{"sqlite composite pk", errors.New("constraint failed: UNIQUE constraint failed: portfolio_snapshot.fund_code, portfolio_snapshot.portfolio_id (2067)"), true},
		{"pg duplicate key", errors.New(`ERROR: duplicate key value violates unique constraint "portfolio_snapshot_pkey" (SQLSTATE 23505)`), true},
		{"not null is not unique", errors.New("NOT NULL constraint failed: portfolio_snapshot.held_shares"), false},
		{"busy is not unique", errors.New("database is locked"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUniqueViolation(tc.err); got != tc.want {
				t.Fatalf("isUniqueViolation(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestRecalcConcurrentFirstWriteSingleRow runs many concurrent Recalc calls
// for one previously absent code against a production-shaped composite-PK
// SQLite DB. Every call must converge on exactly one row: the bounded
// UPDATE→INSERT retry absorbs the unique-constraint loser instead of surfacing
// a PK error. This pins the first-write fix deterministically (assertions are
// exact, no timing-dependent pass).
func TestRecalcConcurrentFirstWriteSingleRow(t *testing.T) {
	dbPath := filepath.ToSlash(filepath.Join(t.TempDir(), "race.db"))
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(8)

	for _, stmt := range []string{
		`CREATE TABLE portfolio_snapshot (
			fund_code TEXT NOT NULL,
			fund_name TEXT,
			held_shares REAL,
			total_cost REAL,
			latest_nav REAL,
			current_value REAL,
			unrealized_pnl REAL,
			pnl_pct REAL,
			security_type TEXT DEFAULT 'fund',
			portfolio_id INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (fund_code, portfolio_id)
		)`,
		`CREATE TABLE transactions (
			fund_code TEXT,
			fund_name TEXT,
			signed_share_change REAL,
			signed_cash_flow REAL
		)`,
		`CREATE TABLE nav_history (fund_code TEXT, date TEXT, unit_nav REAL)`,
		`CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT, security_type TEXT)`,
		`INSERT INTO fund_details VALUES ('F1', 'Race Fund', 'fund')`,
		`INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow) VALUES ('F1', 'Race Fund', 100, -1000)`,
		`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('F1', '2026-01-02', 1.5)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	const workers = 16
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- Recalc(context.Background(), db, "F1", ModeFull)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent first write returned error: %v", err)
		}
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM portfolio_snapshot WHERE fund_code = 'F1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("portfolio_snapshot rows = %d, want exactly 1", count)
	}
	row := readSnapshotRow(t, db, "F1")
	if row.FundName != "Race Fund" || row.SecurityType != "fund" {
		t.Fatalf("identity wrong: %+v", row)
	}
	if !closeEnough(row.HeldShares, 100) || !closeEnough(row.TotalCost, -1000) {
		t.Fatalf("ledger math wrong: %+v", row)
	}
	if !row.LatestNav.Valid || !closeEnough(row.LatestNav.Float64, 1.5) {
		t.Fatalf("latest nav wrong: %+v", row.LatestNav)
	}
}

type zeroRowsResult struct{}

func (zeroRowsResult) LastInsertId() (int64, error) { return 0, nil }
func (zeroRowsResult) RowsAffected() (int64, error) { return 0, nil }

// fakeUpdateOnceQuerier delegates everything to a real DB but reports the
// first portfolio_snapshot UPDATE as affecting zero rows. That forces Recalc
// onto the INSERT branch while the DB already contains the row, deterministically
// reproducing the concurrent-first-write unique violation and its retry.
type fakeUpdateOnceQuerier struct {
	db      *sql.DB
	updates int
}

func (q *fakeUpdateOnceQuerier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return q.db.QueryRowContext(ctx, query, args...)
}

func (q *fakeUpdateOnceQuerier) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if strings.Contains(query, "UPDATE portfolio_snapshot") {
		q.updates++
		if q.updates == 1 {
			return zeroRowsResult{}, nil
		}
	}
	return q.db.ExecContext(ctx, query, args...)
}

// TestRecalcInsertConflictRetriesUpdate pins the retry contract: UPDATE misses,
// INSERT hits the PK, and the retry converges through a second UPDATE instead
// of returning the raw unique violation to the caller.
func TestRecalcInsertConflictRetriesUpdate(t *testing.T) {
	db := openRecalcDB(t)
	seedRecalc(t, db, []string{
		`INSERT INTO fund_details VALUES ('F1', 'Retry Fund', 'fund')`,
		`INSERT INTO transactions (fund_code, fund_name, signed_share_change, signed_cash_flow) VALUES ('F1', 'Retry Fund', 100, -1000)`,
		`INSERT INTO nav_history (fund_code, date, unit_nav) VALUES ('F1', '2026-01-02', 1.5)`,
		`INSERT INTO portfolio_snapshot (fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
		 VALUES ('F1', 'Stale', 1, -1, 0, 0, 0, 0, 'fund', 1)`,
	})

	q := &fakeUpdateOnceQuerier{db: db}
	if err := Recalc(context.Background(), q, "F1", ModeFull); err != nil {
		t.Fatalf("recalc with insert conflict: %v", err)
	}
	if q.updates < 2 {
		t.Fatalf("expected UPDATE retry after INSERT conflict, got %d UPDATE executions", q.updates)
	}
	row := readSnapshotRow(t, db, "F1")
	if row.FundName != "Retry Fund" {
		t.Fatalf("fund_name = %q, want identity resolved", row.FundName)
	}
	if !closeEnough(row.HeldShares, 100) || !closeEnough(row.TotalCost, -1000) {
		t.Fatalf("ledger math wrong after retry: %+v", row)
	}
	if !row.LatestNav.Valid || !closeEnough(row.LatestNav.Float64, 1.5) {
		t.Fatalf("latest nav wrong after retry: %+v", row.LatestNav)
	}
}
