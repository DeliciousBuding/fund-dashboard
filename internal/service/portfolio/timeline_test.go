package portfolio

import (
	"context"
	"testing"
)

func TestServiceGetTimelineReturnsRowsSortedByDate(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	service := NewService(db)
	timeline, err := service.GetTimeline(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetTimeline returned error: %v", err)
	}
	if len(timeline) == 0 {
		t.Fatalf("timeline is empty, want rows")
	}

	for i := 1; i < len(timeline); i++ {
		if timeline[i].Date < timeline[i-1].Date {
			t.Fatalf("timeline is not sorted at %d: %q before %q", i, timeline[i].Date, timeline[i-1].Date)
		}
	}
}

func TestServiceGetTimelineCarriesForwardLatestNAV(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	service := NewService(db)
	timeline, err := service.GetTimeline(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetTimeline returned error: %v", err)
	}

	aug := findTimelineEntry(t, timeline, "2024-08-01")
	if aug.TotalValue != 415.18 {
		t.Fatalf("2024-08-01 total value = %.2f, want 415.18", aug.TotalValue)
	}
	if aug.TotalCost != 400 {
		t.Fatalf("2024-08-01 total cost = %.2f, want 400", aug.TotalCost)
	}
	if aug.PNL != 15.18 {
		t.Fatalf("2024-08-01 pnl = %.2f, want 15.18", aug.PNL)
	}
	if aug.PNLPct != 3.8 {
		t.Fatalf("2024-08-01 pnl pct = %.2f, want 3.8", aug.PNLPct)
	}

	jan := findTimelineEntry(t, timeline, "2025-01-05")
	if jan.TotalValue != 549.29 {
		t.Fatalf("2025-01-05 total value = %.2f, want 549.29", jan.TotalValue)
	}
	if jan.TotalCost != 500 {
		t.Fatalf("2025-01-05 total cost = %.2f, want 500", jan.TotalCost)
	}
	if jan.PNL != 49.29 {
		t.Fatalf("2025-01-05 pnl = %.2f, want 49.29", jan.PNL)
	}
	if jan.PNLPct != 9.86 {
		t.Fatalf("2025-01-05 pnl pct = %.2f, want 9.86", jan.PNLPct)
	}
}

func TestServiceGetTimelineReturnsEmptyForPortfolioWithoutNAV(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()

	service := NewService(db)
	timeline, err := service.GetTimeline(context.Background(), 2)
	if err != nil {
		t.Fatalf("GetTimeline returned error: %v", err)
	}
	if len(timeline) != 0 {
		t.Fatalf("timeline length = %d, want 0 for portfolio without matching NAV rows", len(timeline))
	}
}

func findTimelineEntry(t *testing.T, timeline []TimelineEntry, date string) TimelineEntry {
	t.Helper()
	for _, row := range timeline {
		if row.Date == date {
			return row
		}
	}
	t.Fatalf("timeline entry %s not found in %#v", date, timeline)
	return TimelineEntry{}
}
