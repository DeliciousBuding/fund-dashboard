package admin

import "testing"

func TestClampAdminText(t *testing.T) {
	if clampAdminText("  ab  ", 10) != "ab" {
		t.Fatal("trim")
	}
	long := ""
	for i := 0; i < 100; i++ {
		long += "X"
	}
	got := clampAdminText(long, 10)
	if len([]rune(got)) != 10 {
		t.Fatalf("len=%d", len([]rune(got)))
	}
}
