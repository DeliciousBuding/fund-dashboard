package jobs

import (
	"errors"
	"testing"
	"time"
)

func TestIsMissingTableErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"sqlite", errors.New("SQL logic error: no such table: crawl_log (1)"), true},
		{"postgres", errors.New(`pq: relation "agent_confirmations" does not exist`), true},
		{"legacy", errors.New("undefined_table"), true},
		{"locked", errors.New("database is locked"), false},
		{"constraint", errors.New("UNIQUE constraint failed"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isMissingTableErr(tc.err); got != tc.want {
				t.Fatalf("isMissingTableErr(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestNextRunEpochIndependentOfInputZone(t *testing.T) {
	// The same instant expressed in UTC and in CST must produce the same epoch:
	// calendar math is anchored to the CST fund calendar, not the input zone or
	// the runner's local timezone.
	utc := time.Date(2026, 7, 15, 12, 30, 0, 0, time.UTC)
	inCST := utc.In(cst)
	for _, job := range []string{"price_dca", "holdings", "wal"} {
		if got, want := nextRunEpoch(job, utc), nextRunEpoch(job, inCST); got != want {
			t.Fatalf("%s nextRunEpoch(utc)=%d nextRunEpoch(cst)=%d", job, got, want)
		}
	}
}

func TestNextRunEpochUsesCSTBoundaries(t *testing.T) {
	// 19:59 CST is before the 20:00 window; 20:00 CST itself belongs to the
	// next day. Both must resolve in CST regardless of what the local clock says.
	before := time.Date(2026, 7, 15, 19, 59, 0, 0, cst)
	after := time.Date(2026, 7, 15, 20, 0, 0, 0, cst)
	if got := time.Unix(nextRunEpoch("price_dca", before), 0).In(cst); got.Format("2006-01-02 15:04") != "2026-07-15 20:00" {
		t.Fatalf("next price_dca at 19:59 = %s, want 2026-07-15 20:00 CST", got.Format("2006-01-02 15:04"))
	}
	if got := time.Unix(nextRunEpoch("price_dca", after), 0).In(cst); got.Format("2006-01-02 15:04") != "2026-07-16 20:00" {
		t.Fatalf("next price_dca at 20:00 = %s, want 2026-07-16 20:00 CST", got.Format("2006-01-02 15:04"))
	}
}
