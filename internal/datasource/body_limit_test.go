package datasource

import (
	"strings"
	"testing"
)

func TestReadBodyLimitedAtLimit(t *testing.T) {
	want := strings.Repeat("a", 100)
	got, err := readBodyLimited(strings.NewReader(want), 100)
	if err != nil {
		t.Fatalf("at-limit read returned error: %v", err)
	}
	if string(got) != want {
		t.Fatalf("at-limit read = %q, want %q", got, want)
	}
}

func TestReadBodyLimitedExceedsLimit(t *testing.T) {
	_, err := readBodyLimited(strings.NewReader(strings.Repeat("a", 101)), 100)
	if err == nil {
		t.Fatal("expected error when body exceeds limit, got nil")
	}
}
