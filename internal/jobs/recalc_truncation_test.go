package jobs

import (
	"os"
	"strings"
	"testing"
)

func TestCapRecalcCodes(t *testing.T) {
	cases := []struct {
		name  string
		in    []string
		limit int
		keep  int
		drop  int
	}{
		{"nil list", nil, 5, 0, 0},
		{"under limit", []string{"a", "b", "c"}, 5, 3, 0},
		{"exactly at limit", []string{"a", "b", "c", "d", "e"}, 5, 5, 0},
		{"over limit", []string{"a", "b", "c", "d", "e", "f"}, 5, 5, 1},
		{"far over limit", makeCodes(5001), 5000, 5000, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keep, dropped := capRecalcCodes(tc.in, tc.limit)
			if len(keep) != tc.keep || dropped != tc.drop {
				t.Fatalf("capRecalcCodes(len=%d, limit=%d) = (len=%d, dropped=%d), want (%d, %d)",
					len(tc.in), tc.limit, len(keep), dropped, tc.keep, tc.drop)
			}
		})
	}
}

func makeCodes(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "F" + string(rune('A'+i%26)) + string(rune('0'+i/26))
	}
	return out
}

// TestRecalcAllTruncationObservable is a source-level guard in the same style
// as TestSchedulerJobTimeoutsWired: the LIMIT+1 probe and the truncation log
// must stay wired so an oversized ledger can never silently drop codes again.
func TestRecalcAllTruncationObservable(t *testing.T) {
	raw, err := os.ReadFile("price_refresh.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	if !strings.Contains(src, "recalcAllMaxCodes+1") && !strings.Contains(src, "recalcAllMaxCodes + 1") {
		t.Fatal("expected LIMIT max+1 probe in RecalcAllSnapshots")
	}
	if !strings.Contains(src, `"recalc snapshots code list truncated"`) {
		t.Fatal("expected truncation warning log in RecalcAllSnapshots")
	}
	if !strings.Contains(src, "capRecalcCodes(list, recalcAllMaxCodes)") {
		t.Fatal("expected capRecalcCodes applied to the listed codes")
	}
}
