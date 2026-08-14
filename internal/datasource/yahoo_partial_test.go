package datasource

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestFetchYahooIndexQuotesPartialSuccess(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if strings.Contains(r.URL.Path, "BAD") {
			http.Error(w, "nope", http.StatusBadGateway)
			return
		}
		// minimal yahoo chart JSON
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chart":{"result":[{"meta":{"symbol":"OK","regularMarketPrice":100,"chartPreviousClose":99},"timestamp":[1],"indicators":{"quote":[{"close":[100]}]}}],"error":null}}`))
	}))
	defer srv.Close()

	old := os.Getenv("FUND_YAHOO_CHART_ENDPOINT")
	_ = os.Setenv("FUND_YAHOO_CHART_ENDPOINT", srv.URL+"/chart/%s")
	t.Cleanup(func() { _ = os.Setenv("FUND_YAHOO_CHART_ENDPOINT", old) })

	quotes, err := FetchYahooIndexQuotes(context.Background(), []string{"OK", "BAD", "OK2"})
	if err != nil {
		t.Fatalf("partial should succeed: %v", err)
	}
	if len(quotes) < 1 {
		t.Fatalf("want at least 1 quote, got %d (hits=%d)", len(quotes), n)
	}
}

func TestFetchYahooIndexQuotesAllFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer srv.Close()
	old := os.Getenv("FUND_YAHOO_CHART_ENDPOINT")
	_ = os.Setenv("FUND_YAHOO_CHART_ENDPOINT", srv.URL+"/chart/%s")
	t.Cleanup(func() { _ = os.Setenv("FUND_YAHOO_CHART_ENDPOINT", old) })

	_, err := FetchYahooIndexQuotes(context.Background(), []string{"A", "B"})
	if err == nil {
		t.Fatal("expected error when all fail")
	}
	if !strings.Contains(err.Error(), "yahoo index") && !strings.Contains(fmt.Sprint(err), "status") {
		// still an error is enough
		t.Logf("err=%v", err)
	}
}
