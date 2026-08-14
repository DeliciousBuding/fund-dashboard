package datasource

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCapPricePointsKeepsRecent(t *testing.T) {
	pts := make([]PricePoint, 0, maxNAVHistoryPoints+10)
	for i := 0; i < maxNAVHistoryPoints+10; i++ {
		pts = append(pts, PricePoint{
			Date:  time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format("2006-01-02"),
			Price: 1.0,
		})
	}
	got := capPricePoints(pts)
	if len(got) != maxNAVHistoryPoints {
		t.Fatalf("len=%d want %d", len(got), maxNAVHistoryPoints)
	}
	if got[0].Date != pts[10].Date {
		t.Fatalf("first=%s want %s", got[0].Date, pts[10].Date)
	}
}

func TestParseNetWorthTrendCaps(t *testing.T) {
	var b strings.Builder
	b.WriteString("var Data_netWorthTrend = [")
	n := maxNAVHistoryPoints + 5
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		ms := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).UnixMilli()
		fmt.Fprintf(&b, `{"x":%d,"y":1.23}`, ms)
	}
	b.WriteString("];")
	pts := parseNetWorthTrend(b.String())
	if len(pts) != maxNAVHistoryPoints {
		t.Fatalf("len=%d want %d", len(pts), maxNAVHistoryPoints)
	}
}
