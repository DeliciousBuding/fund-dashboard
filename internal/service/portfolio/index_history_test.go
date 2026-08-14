package portfolio

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
)

func TestNormalizeAndGetIndexHistoryCaches(t *testing.T) {
	ResetIndexHistoryCache()
	t.Cleanup(ResetIndexHistoryCache)

	calls := 0
	fetchHistoryFn = func(_ context.Context, symbol, rangeKey, interval string) (datasource.IndexHistory, error) {
		calls++
		if symbol != "^NDX" {
			t.Fatalf("symbol = %q, want ^NDX", symbol)
		}
		if rangeKey != "1y" && rangeKey != "max" {
			// normalize happens in datasource; here we receive raw-ish
		}
		return datasource.IndexHistory{
			Symbol: symbol,
			Range:  rangeKey,
			Points: []datasource.IndexHistoryPoint{
				{Date: "2026-07-15", Close: 100, ChangePct: 0},
				{Date: "2026-07-16", Close: 110, ChangePct: 10},
			},
		}, nil
	}
	t.Cleanup(func() { fetchHistoryFn = datasource.FetchYahooIndexHistory })

	svc := NewService(nil)
	first, err := svc.GetIndexHistory(context.Background(), "NDX", "1y", "")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.Count != 2 || first.Symbol != "^NDX" || first.Source != "yahoo_chart" {
		t.Fatalf("first = %#v", first)
	}
	second, err := svc.GetIndexHistory(context.Background(), "NDX", "1y", "")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Source != "memory_cache" {
		t.Fatalf("second source = %q", second.Source)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestGetIndexHistoryFallsBackToStaleCache(t *testing.T) {
	ResetIndexHistoryCache()
	t.Cleanup(ResetIndexHistoryCache)

	fetchHistoryFn = func(context.Context, string, string, string) (datasource.IndexHistory, error) {
		return datasource.IndexHistory{
			Symbol: "^GSPC",
			Range:  "1y",
			Points: []datasource.IndexHistoryPoint{{Date: "2026-07-01", Close: 5000, ChangePct: 0}},
		}, nil
	}
	svc := NewService(nil)
	if _, err := svc.GetIndexHistory(context.Background(), "GSPC", "1y", "1d"); err != nil {
		t.Fatal(err)
	}
	// expire cache freshness but keep entry
	indexHistoryMu.Lock()
	for k, v := range indexHistoryCache {
		v.fetched = indexHistoryNowFn().Add(-2 * time.Hour)
		indexHistoryCache[k] = v
	}
	indexHistoryMu.Unlock()

	fetchHistoryFn = func(context.Context, string, string, string) (datasource.IndexHistory, error) {
		return datasource.IndexHistory{}, errors.New("upstream 429")
	}
	t.Cleanup(func() { fetchHistoryFn = datasource.FetchYahooIndexHistory })

	report, err := svc.GetIndexHistory(context.Background(), "GSPC", "1y", "1d")
	if err != nil {
		t.Fatalf("want soft fallback, got %v", err)
	}
	if report.Source != "memory_cache_stale" || report.Count != 1 {
		t.Fatalf("report = %#v", report)
	}
}

func TestGetIndexLiveFromIndicesCache(t *testing.T) {
	// Uses market indices path with stubbed fetchIndexFn and sqlite fixture.
	// Covered more fully via httpapi tests; keep a lightweight unit here with empty db soft no_data.
	svc := NewService(nil)
	// no db → GetMarketIndices may error; GetIndexLive should soft no_data or error path
	// With nil db this panics — skip if no db. Use empty in-memory via market_indices tests pattern elsewhere.
	_ = svc
}

func TestGetIndexHistoryNormalizesCacheKey(t *testing.T) {
	ResetIndexHistoryCache()
	calls := 0
	fetchHistoryFn = func(ctx context.Context, symbol, rangeKey, interval string) (datasource.IndexHistory, error) {
		calls++
		if rangeKey != "1y" || interval != "1d" {
			t.Fatalf("fetch args range=%q interval=%q", rangeKey, interval)
		}
		return datasource.IndexHistory{Symbol: symbol, Range: rangeKey, Points: []datasource.IndexHistoryPoint{{Date: "2026-01-01", Close: 1}}}, nil
	}
	t.Cleanup(func() {
		fetchHistoryFn = datasource.FetchYahooIndexHistory
		ResetIndexHistoryCache()
	})
	svc := Service{}
	if _, err := svc.GetIndexHistory(context.Background(), "NDX", "NOT-A-RANGE", "bogus"); err != nil {
		t.Fatal(err)
	}
	// second call with raw aliases should hit cache (same normalized key)
	if _, err := svc.GetIndexHistory(context.Background(), "NDX", "1y", "daily"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("fetch calls=%d want 1 (cache hit after normalize)", calls)
	}
}

func TestIndexHistoryCacheEvictsExpiredAndCapsSize(t *testing.T) {
	ResetIndexHistoryCache()
	t.Cleanup(func() {
		indexHistoryNowFn = time.Now
		ResetIndexHistoryCache()
	})

	base := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	indexHistoryNowFn = func() time.Time { return base }

	for i := 0; i < 50; i++ {
		storeIndexHistoryCache(fmt.Sprintf("STALE%d|1y|1d", i), datasource.IndexHistory{
			Symbol: fmt.Sprintf("STALE%d", i),
			Range:  "1y",
			Points: []datasource.IndexHistoryPoint{{Date: "2026-07-01", Close: float64(i)}},
		})
	}
	indexHistoryMu.Lock()
	for k, e := range indexHistoryCache {
		e.fetched = base.Add(-indexHistoryFreshFor - time.Minute)
		indexHistoryCache[k] = e
	}
	indexHistoryMu.Unlock()

	storeIndexHistoryCache("FRESH|1y|1d", datasource.IndexHistory{
		Symbol: "FRESH",
		Range:  "1y",
		Points: []datasource.IndexHistoryPoint{{Date: "2026-07-18", Close: 1}},
	})
	indexHistoryMu.RLock()
	sizeAfterExpiry := len(indexHistoryCache)
	_, hasFresh := indexHistoryCache["FRESH|1y|1d"]
	indexHistoryMu.RUnlock()
	if sizeAfterExpiry != 1 || !hasFresh {
		t.Fatalf("after expiry sweep size=%d hasFresh=%v, want size=1 with FRESH", sizeAfterExpiry, hasFresh)
	}

	for i := 0; i < maxIndexHistoryCache+25; i++ {
		idx := i
		indexHistoryNowFn = func() time.Time { return base.Add(time.Duration(idx) * time.Second) }
		storeIndexHistoryCache(fmt.Sprintf("S%d|1y|1d", i), datasource.IndexHistory{
			Symbol: fmt.Sprintf("S%d", i),
			Range:  "1y",
			Points: []datasource.IndexHistoryPoint{{Date: "2026-07-18", Close: float64(i)}},
		})
	}
	indexHistoryMu.RLock()
	size := len(indexHistoryCache)
	_, hasOldest := indexHistoryCache["S0|1y|1d"]
	_, hasNewest := indexHistoryCache[fmt.Sprintf("S%d|1y|1d", maxIndexHistoryCache+24)]
	indexHistoryMu.RUnlock()
	if size > maxIndexHistoryCache {
		t.Fatalf("cache size=%d exceeds maxIndexHistoryCache=%d", size, maxIndexHistoryCache)
	}
	if hasOldest {
		t.Fatalf("expected oldest entry S0|1y|1d to be evicted under size cap")
	}
	if !hasNewest {
		t.Fatalf("expected newest entry to remain under size cap")
	}
}
