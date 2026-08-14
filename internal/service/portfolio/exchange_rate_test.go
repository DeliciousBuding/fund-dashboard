package portfolio

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGetExchangeRateCachesSuccessfulFetch(t *testing.T) {
	ResetExchangeRateCache()
	t.Cleanup(ResetExchangeRateCache)

	calls := 0
	prevDo := exchangeRateHTTPDo
	exchangeRateHTTPDo = func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Header.Get("User-Agent") == "" {
			t.Fatal("expected User-Agent header")
		}
		body := `{"chart":{"result":[{"meta":{"regularMarketPrice":7.25,"regularMarketTime":1783439100}}]}}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	}
	t.Cleanup(func() { exchangeRateHTTPDo = prevDo })
	prevEndpoint := exchangeRateEndpointFn
	exchangeRateEndpointFn = func() string { return "http://example.invalid/fx" }
	t.Cleanup(func() { exchangeRateEndpointFn = prevEndpoint })

	svc := NewService(nil)
	first, err := svc.GetExchangeRate(context.Background())
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.Rate != 7.25 || first.Source != "yahoo_chart" {
		t.Fatalf("first = %#v", first)
	}
	second, err := svc.GetExchangeRate(context.Background())
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Source != "memory_cache" || second.Rate != 7.25 {
		t.Fatalf("second = %#v, want memory_cache", second)
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls)
	}
}

func TestGetExchangeRateFallsBackToLastGoodOnUpstreamError(t *testing.T) {
	ResetExchangeRateCache()
	t.Cleanup(ResetExchangeRateCache)

	storeExchangeRate(ExchangeRateReport{
		From: "USD", To: "CNY", Rate: 7.11, UpdatedAt: "2026-07-17T00:00:00Z",
	})
	fxCache.mu.Lock()
	fxCache.fetched = exchangeRateNowFn().Add(-2 * time.Hour)
	fxCache.mu.Unlock()

	prevDo := exchangeRateHTTPDo
	exchangeRateHTTPDo = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 429,
			Body:       io.NopCloser(strings.NewReader("rate limited")),
			Header:     make(http.Header),
		}, nil
	}
	t.Cleanup(func() { exchangeRateHTTPDo = prevDo })
	prevEndpoint := exchangeRateEndpointFn
	exchangeRateEndpointFn = func() string { return "http://example.invalid/fx" }
	t.Cleanup(func() { exchangeRateEndpointFn = prevEndpoint })

	report, err := NewService(nil).GetExchangeRate(context.Background())
	if err != nil {
		t.Fatalf("expected soft fallback, got err %v", err)
	}
	if report.Rate != 7.11 || report.Source != "memory_cache_stale" {
		t.Fatalf("report = %#v, want last-good stale cache", report)
	}
}

func TestGetExchangeRateErrorsWhenNoCacheAndUpstreamFails(t *testing.T) {
	ResetExchangeRateCache()
	t.Cleanup(ResetExchangeRateCache)

	prevDo := exchangeRateHTTPDo
	exchangeRateHTTPDo = func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 429,
			Body:       io.NopCloser(strings.NewReader("rate limited")),
			Header:     make(http.Header),
		}, nil
	}
	t.Cleanup(func() { exchangeRateHTTPDo = prevDo })
	prevEndpoint := exchangeRateEndpointFn
	exchangeRateEndpointFn = func() string { return "http://example.invalid/fx" }
	t.Cleanup(func() { exchangeRateEndpointFn = prevEndpoint })

	_, err := NewService(nil).GetExchangeRate(context.Background())
	if err == nil {
		t.Fatal("expected error when no cache")
	}
}
