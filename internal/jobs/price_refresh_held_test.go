package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestGetHeldSecuritiesListsAllUnderLimit(t *testing.T) {
	db := heldListTestDB(t)
	defer db.Close()
	seedHeldRows(t, db, 3)

	r := NewPriceRefresher(db)
	got, err := r.getHeldSecurities(context.Background())
	if err != nil {
		t.Fatalf("getHeldSecurities: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
}

func TestGetHeldSecuritiesCapsAtLimit(t *testing.T) {
	db := heldListTestDB(t)
	defer db.Close()
	seedHeldRows(t, db, heldSecuritiesMaxCodes+1)

	r := NewPriceRefresher(db)
	got, err := r.getHeldSecurities(context.Background())
	if err != nil {
		t.Fatalf("getHeldSecurities: %v", err)
	}
	if len(got) != heldSecuritiesMaxCodes {
		t.Fatalf("len=%d want cap %d", len(got), heldSecuritiesMaxCodes)
	}
}

// TestGetHeldSecuritiesTruncationObservable is a source-level guard in the same
// style as TestRecalcAllTruncationObservable: the LIMIT+1 probe and warning
// must stay wired so an oversized ledger can never silently drop held codes.
func TestGetHeldSecuritiesTruncationObservable(t *testing.T) {
	raw, err := os.ReadFile("price_refresh.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	if !strings.Contains(src, "heldSecuritiesMaxCodes+1") && !strings.Contains(src, "heldSecuritiesMaxCodes + 1") {
		t.Fatal("expected LIMIT max+1 probe in getHeldSecurities")
	}
	if !strings.Contains(src, `"held securities code list truncated"`) {
		t.Fatal("expected truncation warning log in getHeldSecurities")
	}
	if !strings.Contains(src, "capCodes(codes, heldSecuritiesMaxCodes)") {
		t.Fatal("expected capCodes applied to held codes")
	}
}

func heldListTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE portfolio_snapshot (fund_code TEXT PRIMARY KEY, held_shares REAL DEFAULT 0)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	return db
}

func seedHeldRows(t *testing.T, db *sql.DB, n int) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.Prepare(`INSERT INTO portfolio_snapshot (fund_code, held_shares) VALUES (?, 1)`)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for i := 0; i < n; i++ {
		if _, err := stmt.Exec(fmt.Sprintf("H%05d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
