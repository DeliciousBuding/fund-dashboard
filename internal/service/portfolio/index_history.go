package portfolio

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/chinatime"
	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
)

// Index history cache (process-local) — Yahoo 429 protection for NasdaqOverview polls (#95).
const (
	indexHistoryFreshFor = 30 * time.Minute
	maxIndexHistoryCache = 200
)

type indexHistoryCacheEntry struct {
	fetched time.Time
	history datasource.IndexHistory
}

var (
	indexHistoryMu    sync.RWMutex
	indexHistoryCache = map[string]indexHistoryCacheEntry{}
	fetchHistoryFn    = datasource.FetchYahooIndexHistory
	indexHistoryNowFn = time.Now
)

// ResetIndexHistoryCache clears process cache (tests).
func ResetIndexHistoryCache() {
	indexHistoryMu.Lock()
	defer indexHistoryMu.Unlock()
	indexHistoryCache = map[string]indexHistoryCacheEntry{}
}

type IndexHistoryReport struct {
	Symbol           string                    `json:"symbol"`
	Count            int                       `json:"count"`
	Range            string                    `json:"range"`
	Data             []IndexHistoryPointReport `json:"data"`
	DecisionBoundary string                    `json:"decision_boundary,omitempty"`
	SideEffects      string                    `json:"side_effects,omitempty"`
	ExternalFetch    string                    `json:"external_fetch,omitempty"`
	Source           string                    `json:"source,omitempty"`
	Error            string                    `json:"error,omitempty"`
	Message          string                    `json:"message,omitempty"`
}

type IndexHistoryPointReport struct {
	Date      string  `json:"date"`
	Close     float64 `json:"close"`
	ChangePct float64 `json:"change_pct"`
}

type IndexLiveReport struct {
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	Market           string  `json:"market"`
	Price            float64 `json:"price"`
	ChangePct        float64 `json:"change_pct"`
	ChangeAmt        float64 `json:"change_amt"`
	UpdatedAt        string  `json:"updated_at"`
	Source           string  `json:"source,omitempty"`
	DecisionBoundary string  `json:"decision_boundary,omitempty"`
	SideEffects      string  `json:"side_effects,omitempty"`
	ExternalFetch    string  `json:"external_fetch,omitempty"`
	Error            string  `json:"error,omitempty"`
	Message          string  `json:"message,omitempty"`
}

// GetIndexHistory returns Yahoo chart history for SPA NasdaqOverview (#95).
func (s Service) GetIndexHistory(ctx context.Context, code, rangeKey, interval string) (IndexHistoryReport, error) {
	symbol := datasource.NormalizeIndexSymbol(code)
	// Normalize before cache key so raw query strings cannot pollute memory (#228).
	rangeKey = datasource.NormalizeYahooRange(rangeKey)
	interval = datasource.NormalizeYahooInterval(interval)
	report := IndexHistoryReport{
		Symbol:           symbol,
		Range:            rangeKey,
		DecisionBoundary: "facts_only",
		SideEffects:      "none",
		ExternalFetch:    "not_performed",
	}
	if symbol == "" {
		report.Error = "no_data"
		report.Message = "code is required"
		return report, nil
	}

	cacheKey := symbol + "|" + rangeKey + "|" + interval
	if hist, ok := loadIndexHistoryCache(cacheKey); ok {
		return historyToReport(hist, "memory_cache"), nil
	}

	hist, err := fetchHistoryFn(ctx, symbol, rangeKey, interval)
	if err != nil {
		// Soft-fail: return empty facts envelope rather than hard 502 when possible.
		if stale, ok := loadAnyIndexHistoryCache(cacheKey); ok {
			out := historyToReport(stale, "memory_cache_stale")
			out.Message = "upstream_unavailable"
			return out, nil
		}
		return IndexHistoryReport{}, err
	}
	storeIndexHistoryCache(cacheKey, hist)
	return historyToReport(hist, "yahoo_chart"), nil
}

// GetIndexLive returns one index quote for SPA (#95).
func (s Service) GetIndexLive(ctx context.Context, code string) (IndexLiveReport, error) {
	symbol := datasource.NormalizeIndexSymbol(code)
	report := IndexLiveReport{
		Code:             symbol,
		DecisionBoundary: "facts_only",
		SideEffects:      "none",
		ExternalFetch:    "not_performed",
	}
	if symbol == "" {
		report.Error = "no_data"
		report.Message = "code is required"
		return report, nil
	}

	// Prefer indices table after refresh-on-read path.
	indices, err := s.GetMarketIndices(ctx)
	if err == nil {
		if q, ok := indices.Indices[symbol]; ok && q.Price > 0 {
			report.Name = clampPortfolioText(q.Name, 120)
			report.Market = clampPortfolioText(q.Market, 32)
			report.Price = q.Price
			report.ChangePct = q.ChangePct
			report.ChangeAmt = q.Change
			report.UpdatedAt = q.UpdatedAt
			report.Source = "indices_cache"
			report.ExternalFetch = indices.ExternalFetch
			report.SideEffects = indices.SideEffects
			return report, nil
		}
	}

	// Fallback: direct Yahoo quote for this symbol only.
	quotes, err := fetchIndexFn(ctx, []string{symbol})
	if err != nil {
		report.Error = "no_data"
		report.Message = "upstream_unavailable"
		return report, nil
	}
	if len(quotes) == 0 {
		report.Error = "no_data"
		report.Message = clampPortfolioText(fmt.Sprintf("no quote for %s", symbol), 120)
		return report, nil
	}
	q := quotes[0]
	report.Name = clampPortfolioText(q.Name, 120)
	report.Market = clampPortfolioText(q.Market, 32)
	report.Price = q.Price
	report.ChangePct = q.ChangePct
	report.ChangeAmt = q.Change
	report.UpdatedAt = q.UpdatedAt.In(chinatime.Loc).Format("2006-01-02 15:04:05")
	report.Source = "yahoo_chart"
	report.ExternalFetch = "yahoo_chart"
	return report, nil
}

func historyToReport(hist datasource.IndexHistory, source string) IndexHistoryReport {
	data := make([]IndexHistoryPointReport, 0, len(hist.Points))
	for _, p := range hist.Points {
		data = append(data, IndexHistoryPointReport{
			Date:      p.Date,
			Close:     p.Close,
			ChangePct: p.ChangePct,
		})
	}
	return IndexHistoryReport{
		Symbol:           hist.Symbol,
		Count:            len(data),
		Range:            hist.Range,
		Data:             data,
		DecisionBoundary: "facts_only",
		SideEffects:      "none",
		ExternalFetch:    source,
		Source:           source,
	}
}

func loadIndexHistoryCache(key string) (datasource.IndexHistory, bool) {
	indexHistoryMu.RLock()
	defer indexHistoryMu.RUnlock()
	entry, ok := indexHistoryCache[key]
	if !ok {
		return datasource.IndexHistory{}, false
	}
	if indexHistoryNowFn().Sub(entry.fetched) > indexHistoryFreshFor {
		return datasource.IndexHistory{}, false
	}
	return entry.history, true
}

func loadAnyIndexHistoryCache(key string) (datasource.IndexHistory, bool) {
	indexHistoryMu.RLock()
	defer indexHistoryMu.RUnlock()
	entry, ok := indexHistoryCache[key]
	if !ok {
		return datasource.IndexHistory{}, false
	}
	return entry.history, true
}

func storeIndexHistoryCache(key string, hist datasource.IndexHistory) {
	indexHistoryMu.Lock()
	defer indexHistoryMu.Unlock()
	now := indexHistoryNowFn()
	indexHistoryCache[key] = indexHistoryCacheEntry{fetched: now, history: hist}

	// Evict expired entries so unique keys cannot grow the map forever.
	for k, e := range indexHistoryCache {
		if now.Sub(e.fetched) > indexHistoryFreshFor {
			delete(indexHistoryCache, k)
		}
	}
	// Cap max size by dropping oldest fetched entries.
	for len(indexHistoryCache) > maxIndexHistoryCache {
		oldestKey := ""
		var oldestTime time.Time
		first := true
		for k, e := range indexHistoryCache {
			if first || e.fetched.Before(oldestTime) {
				oldestKey = k
				oldestTime = e.fetched
				first = false
			}
		}
		if oldestKey == "" {
			break
		}
		delete(indexHistoryCache, oldestKey)
	}
}
