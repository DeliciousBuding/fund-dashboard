package datasource

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchYahooStockSnapshotPrefersPreviousCloseOverChartAnchor(t *testing.T) {
	// Long-range chartPreviousClose is first-bar anchor (wrong for day change).
	// previousClose empty → derive from penultimate history close (#102).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"chart": {
				"result": [{
					"meta": {
						"currency": "USD",
						"symbol": "AAPL",
						"shortName": "Apple Inc.",
						"regularMarketPrice": 333.26,
						"chartPreviousClose": 210.02,
						"previousClose": 0,
						"regularMarketTime": 1784232001,
						"regularMarketOpen": 328.01,
						"regularMarketDayHigh": 334.68,
						"regularMarketDayLow": 326.79,
						"regularMarketVolume": 50000000
					},
					"timestamp": [1784059200, 1784145600, 1784232000],
					"indicators": {
						"quote": [{
							"open": [320.0, 325.0, 328.01],
							"high": [322.0, 330.0, 334.68],
							"low": [318.0, 324.0, 326.79],
							"close": [321.0, 330.0, 333.26],
							"volume": [1e7, 2e7, 5e7]
						}]
					}
				}]
			}
		}`))
	}))
	defer srv.Close()
	t.Setenv("FUND_YAHOO_STOCK_ENDPOINT", srv.URL+"/%s?range=%s&interval=%s")

	snap, err := FetchYahooStockSnapshot(context.Background(), "AAPL", "1y", true)
	if err != nil {
		t.Fatalf("FetchYahooStockSnapshot: %v", err)
	}
	if snap.PreviousClose != 330.0 {
		t.Fatalf("PreviousClose = %v, want penultimate history close 330", snap.PreviousClose)
	}
	// change = 333.26 - 330 = 3.26; pct ≈ 0.9879
	if snap.Change < 3.0 || snap.Change > 3.5 {
		t.Fatalf("Change = %v, want ~3.26", snap.Change)
	}
	if snap.ChangePct < 0.5 || snap.ChangePct > 2.0 {
		t.Fatalf("ChangePct = %v, want sane ~1%% (not ~58%% from chartPreviousClose)", snap.ChangePct)
	}
	if snap.Price != 333.26 {
		t.Fatalf("Price = %v", snap.Price)
	}
}

func TestFetchYahooStockSnapshotUsesMetaPreviousCloseWhenPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"chart": {
				"result": [{
					"meta": {
						"currency": "USD",
						"symbol": "MSFT",
						"shortName": "Microsoft",
						"regularMarketPrice": 450.0,
						"chartPreviousClose": 100.0,
						"previousClose": 440.0,
						"regularMarketTime": 1784232001,
						"regularMarketOpen": 445.0,
						"regularMarketDayHigh": 452.0,
						"regularMarketDayLow": 443.0,
						"regularMarketVolume": 1000000
					},
					"timestamp": [],
					"indicators": {"quote": [{}]}
				}]
			}
		}`))
	}))
	defer srv.Close()
	t.Setenv("FUND_YAHOO_STOCK_ENDPOINT", srv.URL+"/%s?range=%s&interval=%s")

	snap, err := FetchYahooStockSnapshot(context.Background(), "MSFT", "1y", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if snap.PreviousClose != 440.0 {
		t.Fatalf("PreviousClose = %v, want meta.previousClose 440 (not chartPreviousClose 100)", snap.PreviousClose)
	}
	if snap.ChangePct < 2.0 || snap.ChangePct > 3.0 {
		t.Fatalf("ChangePct = %v, want ~2.27", snap.ChangePct)
	}
}
