package portfolio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

const defaultExchangeRateEndpoint = "https://query1.finance.yahoo.com/v8/finance/chart/USDCNY=X?range=1d&interval=1d"

// exchangeRateFreshFor controls how long a successful Yahoo quote is reused without re-fetch (#94).
const exchangeRateFreshFor = time.Hour

// Overridable in tests.
var (
	exchangeRateNowFn      = time.Now
	exchangeRateHTTPDo     = defaultExchangeRateHTTPDo
	exchangeRateEndpointFn = func() string {
		if v := os.Getenv("FUND_EXCHANGE_RATE_ENDPOINT"); v != "" {
			return v
		}
		return defaultExchangeRateEndpoint
	}
)

type ExchangeRateReport struct {
	From      string  `json:"from"`
	To        string  `json:"to"`
	Rate      float64 `json:"rate"`
	UpdatedAt string  `json:"updated_at"`
	// Source is optional observability; SPA ignores unknown fields.
	Source string `json:"source,omitempty"`
}

type exchangeRateCache struct {
	mu      sync.RWMutex
	report  ExchangeRateReport
	fetched time.Time
	ok      bool
}

var fxCache exchangeRateCache

// ResetExchangeRateCache clears the process cache (tests).
func ResetExchangeRateCache() {
	fxCache.mu.Lock()
	defer fxCache.mu.Unlock()
	fxCache.report = ExchangeRateReport{}
	fxCache.fetched = time.Time{}
	fxCache.ok = false
}

func (s Service) GetExchangeRate(ctx context.Context) (ExchangeRateReport, error) {
	// Serve fresh in-process cache first — avoids Yahoo 429 from SPA multi-page polls (#94).
	if report, ok := loadFreshExchangeRate(); ok {
		report.Source = "memory_cache"
		return report, nil
	}

	report, err := fetchExchangeRateUpstream(ctx)
	if err == nil {
		storeExchangeRate(report)
		report.Source = "yahoo_chart"
		return report, nil
	}

	// Soft-fail: last good rate is better than blanking SPA badges.
	if stale, ok := loadAnyExchangeRate(); ok {
		slog.Warn("exchange rate upstream failed; serving last-good cache", "error", err)
		stale.Source = "memory_cache_stale"
		return stale, nil
	}
	return ExchangeRateReport{}, err
}

func loadFreshExchangeRate() (ExchangeRateReport, bool) {
	fxCache.mu.RLock()
	defer fxCache.mu.RUnlock()
	if !fxCache.ok {
		return ExchangeRateReport{}, false
	}
	if exchangeRateNowFn().Sub(fxCache.fetched) > exchangeRateFreshFor {
		return ExchangeRateReport{}, false
	}
	return fxCache.report, true
}

func loadAnyExchangeRate() (ExchangeRateReport, bool) {
	fxCache.mu.RLock()
	defer fxCache.mu.RUnlock()
	if !fxCache.ok {
		return ExchangeRateReport{}, false
	}
	return fxCache.report, true
}

func storeExchangeRate(report ExchangeRateReport) {
	fxCache.mu.Lock()
	defer fxCache.mu.Unlock()
	// Don't store Source in cache; callers set it per response path.
	report.Source = ""
	fxCache.report = report
	fxCache.fetched = exchangeRateNowFn()
	fxCache.ok = true
}

func fetchExchangeRateUpstream(ctx context.Context) (ExchangeRateReport, error) {
	endpoint := exchangeRateEndpointFn()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ExchangeRateReport{}, fmt.Errorf("build exchange rate request: %w", err)
	}
	req.Header.Set("User-Agent", "fund-dashboard/1.0")
	res, err := exchangeRateHTTPDo(req)
	if err != nil {
		return ExchangeRateReport{}, fmt.Errorf("fetch exchange rate: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ExchangeRateReport{}, fmt.Errorf("fetch exchange rate: status %d", res.StatusCode)
	}

	var payload yahooExchangeRateResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&payload); err != nil {
		return ExchangeRateReport{}, fmt.Errorf("decode exchange rate: %w", err)
	}
	if len(payload.Chart.Result) == 0 || payload.Chart.Result[0].Meta.RegularMarketPrice <= 0 {
		return ExchangeRateReport{}, fmt.Errorf("exchange rate not found")
	}
	meta := payload.Chart.Result[0].Meta
	updatedAt := time.Unix(meta.RegularMarketTime, 0).UTC().Format(time.RFC3339)
	if meta.RegularMarketTime <= 0 {
		updatedAt = exchangeRateNowFn().UTC().Format(time.RFC3339)
	}
	return ExchangeRateReport{
		From:      "USD",
		To:        "CNY",
		Rate:      meta.RegularMarketPrice,
		UpdatedAt: updatedAt,
	}, nil
}

func defaultExchangeRateHTTPDo(req *http.Request) (*http.Response, error) {
	// Explicit client timeout complements request context (#243).
	client := &http.Client{Timeout: 5 * time.Second}
	return client.Do(req)
}

type yahooExchangeRateResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				RegularMarketTime  int64   `json:"regularMarketTime"`
			} `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
}
