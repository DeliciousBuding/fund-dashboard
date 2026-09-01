package admin

import "testing"

func TestClampPortfolioID(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, 1},
		{-5, 1},
		{1, 1},
		{2, 2},
		{999, 999},
		{1000, 1000},
		{1001, 1000},
		{5000, 1000},
	}
	for _, tc := range cases {
		if got := clampPortfolioID(tc.in); got != tc.want {
			t.Fatalf("clampPortfolioID(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
