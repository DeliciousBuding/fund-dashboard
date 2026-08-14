package admin

import "testing"

func TestNormalizeSecurityCodeBoundsLength(t *testing.T) {
	long := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	got := NormalizeSecurityCode(long)
	if len(got) != 32 {
		t.Fatalf("len=%d want 32 got %q", len(got), got)
	}
	if got != long[:32] {
		t.Fatalf("got %q", got)
	}
	// numeric pad still works
	if NormalizeSecurityCode("173") != "000173" {
		t.Fatalf("pad: %q", NormalizeSecurityCode("173"))
	}
}
