package chinatime

import (
	"testing"
	"time"
)

func TestLocIsChinaMarketZone(t *testing.T) {
	if Loc == nil {
		t.Fatal("chinatime.Loc is nil")
	}
	// A modern instant with unambiguous +08:00 offset (no DST since 1991).
	instant := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	want := "2026-09-02 08:00:00"
	if got := instant.In(Loc).Format("2006-01-02 15:04:05"); got != want {
		t.Fatalf("instant in Loc = %q, want %q", got, want)
	}
	_, offset := instant.In(Loc).Zone()
	if offset != 8*3600 {
		t.Fatalf("Loc offset = %d seconds, want %d", offset, 8*3600)
	}
}

func TestFallbackFixedZoneEquivalentForModernDates(t *testing.T) {
	fallback := time.FixedZone("CST", 8*3600)
	for _, year := range []int{1992, 2026, 2100} {
		instant := time.Date(year, 6, 15, 12, 0, 0, 0, time.UTC)
		if instant.In(Loc).Format(time.RFC3339) != instant.In(fallback).Format(time.RFC3339) {
			t.Fatalf("year %d: Loc and fixed +08:00 fallback disagree: %q vs %q",
				year, instant.In(Loc).Format(time.RFC3339), instant.In(fallback).Format(time.RFC3339))
		}
	}
}
