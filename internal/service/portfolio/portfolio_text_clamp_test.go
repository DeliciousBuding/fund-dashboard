package portfolio

import (
	"strings"
	"testing"
)

func TestClampPortfolioText(t *testing.T) {
	if clampPortfolioText("  ab  ", 10) != "ab" {
		t.Fatal("trim")
	}
	long := strings.Repeat("Z", 100)
	got := clampPortfolioText(long, 10)
	if len([]rune(got)) != 10 {
		t.Fatalf("len=%d", len([]rune(got)))
	}
}

func TestClampPortfolioTextIndexLimits(t *testing.T) {
	if n := len([]rune(clampPortfolioText(strings.Repeat("C", 100), 32))); n != 32 {
		t.Fatalf("code %d", n)
	}
	if n := len([]rune(clampPortfolioText(strings.Repeat("N", 200), 120))); n != 120 {
		t.Fatalf("name %d", n)
	}
}

func TestClampPortfolioTextPublicBounds(t *testing.T) {
	if n := len([]rune(clampPortfolioText(strings.Repeat("D", 1000), 500))); n != 500 {
		t.Fatalf("description %d", n)
	}
	if n := len([]rune(clampPortfolioText(strings.Repeat("S", 100), 64))); n != 64 {
		t.Fatalf("sector %d", n)
	}
}

func TestClampPortfolioTextSearchAndProfile(t *testing.T) {
	if n := len([]rune(clampPortfolioText(strings.Repeat("P", 800), 500))); n != 500 {
		t.Fatalf("profile desc %d", n)
	}
	if n := len([]rune(clampPortfolioText(strings.Repeat("A", 100), 64))); n != 64 {
		t.Fatalf("allocation key %d", n)
	}
}
