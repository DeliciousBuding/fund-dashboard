package portfolio

import "testing"

func TestClampPortfolioID(t *testing.T) {
	if clampPortfolioID(0) != 1 || clampPortfolioID(-3) != 1 {
		t.Fatal("non-positive")
	}
	if clampPortfolioID(1001) != 1000 {
		t.Fatal("cap")
	}
	if clampPortfolioID(7) != 7 {
		t.Fatal("pass")
	}
}
