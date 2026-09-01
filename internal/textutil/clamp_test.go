package textutil

import (
	"strings"
	"testing"
)

func TestClamp(t *testing.T) {
	if got := Clamp("  ab  ", 10); got != "ab" {
		t.Fatalf("trim: got %q", got)
	}
	long := strings.Repeat("Z", 50)
	if got := Clamp(long, 10); got != strings.Repeat("Z", 10) {
		t.Fatalf("byte-bound cut: got %q", got)
	}
	cjk := strings.Repeat("基", 20) // 20 runes, 60 bytes
	if got := Clamp(cjk, 8); len([]rune(got)) != 8 {
		t.Fatalf("rune-safe cut: got %d runes", len([]rune(got)))
	}
	if got := Clamp("short", 0); got != "short" {
		t.Fatalf("max<=0 must trim only: got %q", got)
	}
	if got := Clamp("   ", 10); got != "" {
		t.Fatalf("whitespace only: got %q", got)
	}
}
