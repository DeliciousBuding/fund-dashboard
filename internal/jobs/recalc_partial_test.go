package jobs

import "testing"

func TestRecalcAllStatus(t *testing.T) {
	cases := []struct {
		ok     int
		failed []string
		want   string
	}{
		{5, nil, "complete"},
		{5, []string{}, "complete"},
		{3, []string{"A", "B"}, "partial"},
		{0, []string{"A"}, "error"},
	}
	for _, tc := range cases {
		got := RecalcAllStatus(tc.ok, tc.failed)
		if got != tc.want {
			t.Fatalf("ok=%d failed=%v got %q want %q", tc.ok, tc.failed, got, tc.want)
		}
	}
}

func TestLogRecalcPartial_NoPanic(t *testing.T) {
	logRecalcPartial(0, nil)
	logRecalcPartial(2, []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"})
}
