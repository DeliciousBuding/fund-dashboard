package portfolio

import (
	"sync"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
)

type USStockOptions struct {
	Symbol         string
	Range          string
	IncludeHistory bool
}

type USStockReport struct {
	Symbol           string          `json:"symbol"`
	Quote            *USStockQuote   `json:"quote,omitempty"`
	History          *USStockHistory `json:"history,omitempty"`
	Profile          *USStockProfile `json:"profile,omitempty"`
	DecisionBoundary string          `json:"decision_boundary"`
	SideEffects      string          `json:"side_effects"`
	ExternalFetch    string          `json:"external_fetch"`
	Error            string          `json:"error,omitempty"`
	Message          string          `json:"message,omitempty"`
}

type USStockQuote struct {
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	PreviousClose float64 `json:"previous_close"`
	Change        float64 `json:"change"`
	ChangePct     float64 `json:"change_pct"`
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	Volume        float64 `json:"volume"`
	Currency      string  `json:"currency"`
	MarketTime    string  `json:"market_time"`
}

type USStockHistory struct {
	Range     string                `json:"range"`
	Count     int                   `json:"count"`
	FirstDate string                `json:"first_date,omitempty"`
	LastDate  string                `json:"last_date,omitempty"`
	Data      []USStockHistoryPoint `json:"data"`
}

type USStockHistoryPoint struct {
	Date      string  `json:"date"`
	Close     float64 `json:"close"`
	ChangePct float64 `json:"change_pct"`
}

type USStockProfile struct {
	Sector      string   `json:"sector,omitempty"`
	Industry    string   `json:"industry,omitempty"`
	MarketCap   *float64 `json:"market_cap,omitempty"`
	PE          *float64 `json:"pe,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Overridable in tests.
var fetchStockSnapshotFn = datasource.FetchYahooStockSnapshot

// Process-local full snapshot cache: production stock_realtime cannot store OHLC (#99).
const (
	usStockSnapFreshFor = 15 * time.Minute
	maxUSStockSnapCache = 200
)

type usStockSnapCacheEntry struct {
	fetched time.Time
	snap    datasource.StockSnapshot
}

var (
	usStockSnapMu    sync.RWMutex
	usStockSnapCache = map[string]usStockSnapCacheEntry{}
	usStockSnapNowFn = time.Now
)

func loadUSStockSnapCache(symbol string) (datasource.StockSnapshot, bool) {
	usStockSnapMu.RLock()
	defer usStockSnapMu.RUnlock()
	e, ok := usStockSnapCache[symbol]
	if !ok || usStockSnapNowFn().Sub(e.fetched) > usStockSnapFreshFor {
		return datasource.StockSnapshot{}, false
	}
	return e.snap, true
}

func storeUSStockSnapCache(symbol string, snap datasource.StockSnapshot) {
	usStockSnapMu.Lock()
	defer usStockSnapMu.Unlock()
	now := usStockSnapNowFn()
	usStockSnapCache[symbol] = usStockSnapCacheEntry{fetched: now, snap: snap}

	// Evict expired entries so unique symbols cannot grow the map forever.
	for k, e := range usStockSnapCache {
		if now.Sub(e.fetched) > usStockSnapFreshFor {
			delete(usStockSnapCache, k)
		}
	}
	// Cap max size by dropping oldest fetched entries.
	for len(usStockSnapCache) > maxUSStockSnapCache {
		oldestKey := ""
		var oldestTime time.Time
		first := true
		for k, e := range usStockSnapCache {
			if first || e.fetched.Before(oldestTime) {
				oldestKey = k
				oldestTime = e.fetched
				first = false
			}
		}
		if oldestKey == "" {
			break
		}
		delete(usStockSnapCache, oldestKey)
	}
}

// ResetUSStockSnapCache clears process-local full snapshots (tests).
func ResetUSStockSnapCache() {
	usStockSnapMu.Lock()
	defer usStockSnapMu.Unlock()
	usStockSnapCache = map[string]usStockSnapCacheEntry{}
}
