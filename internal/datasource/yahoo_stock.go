package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// StockSnapshot is a Yahoo chart snapshot for a single US equity symbol.
type StockSnapshot struct {
	Symbol        string
	Name          string
	Price         float64
	PreviousClose float64
	Change        float64
	ChangePct     float64
	Open          float64
	High          float64
	Low           float64
	Volume        float64
	Currency      string
	MarketTime    time.Time
	History       []IndexHistoryPoint // reuse point type (date/close/change_pct)
}

// FetchYahooStockSnapshot loads quote (+ optional daily history) from Yahoo chart API.
func FetchYahooStockSnapshot(ctx context.Context, symbol, rangeKey string, withHistory bool) (StockSnapshot, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return StockSnapshot{}, fmt.Errorf("symbol is required")
	}
	if rangeKey == "" {
		rangeKey = "1y"
	}
	rangeKey = normalizeYahooRange(rangeKey)
	interval := "1d"
	template := os.Getenv("FUND_YAHOO_STOCK_ENDPOINT")
	if template == "" {
		template = "https://query1.finance.yahoo.com/v8/finance/chart/%s?range=%s&interval=%s"
	}
	endpoint := fmt.Sprintf(template, url.PathEscape(symbol), url.QueryEscape(rangeKey), url.QueryEscape(interval))
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return StockSnapshot{}, err
	}
	req.Header.Set("User-Agent", "fund-dashboard/1.0")
	res, err := client.Do(req)
	if err != nil {
		return StockSnapshot{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return StockSnapshot{}, fmt.Errorf("status %d", res.StatusCode)
	}
	body, err := readBodyLimited(res.Body, 4<<20)
	if err != nil {
		return StockSnapshot{}, err
	}
	var payload yahooStockChartResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return StockSnapshot{}, err
	}
	if len(payload.Chart.Result) == 0 {
		return StockSnapshot{}, fmt.Errorf("empty chart result")
	}
	result := payload.Chart.Result[0]
	meta := result.Meta
	price := meta.RegularMarketPrice
	// Prefer true prior-day close. chartPreviousClose on long ranges (1y) is the
	// first-bar anchor, not yesterday's close (#102).
	prev := meta.PreviousClose
	change := 0.0
	changePct := 0.0
	if prev > 0 {
		change = price - prev
		changePct = change / prev * 100
	}
	updated := time.Now().UTC()
	if meta.RegularMarketTime > 0 {
		updated = time.Unix(meta.RegularMarketTime, 0).UTC()
	}
	name := meta.ShortName
	if name == "" {
		name = meta.Symbol
	}
	if name == "" {
		name = symbol
	}
	snap := StockSnapshot{
		Symbol:        symbol,
		Name:          name,
		Price:         price,
		PreviousClose: prev,
		Change:        change,
		ChangePct:     changePct,
		Open:          meta.RegularMarketOpen,
		High:          meta.RegularMarketDayHigh,
		Low:           meta.RegularMarketDayLow,
		Volume:        float64(meta.RegularMarketVolume),
		Currency:      meta.Currency,
		MarketTime:    updated,
	}
	if snap.Currency == "" {
		snap.Currency = "USD"
	}
	// Fill OHL from last daily bar if meta day fields empty.
	if withHistory && len(result.Timestamp) > 0 && len(result.Indicators.Quote) > 0 {
		q := result.Indicators.Quote[0]
		points := make([]IndexHistoryPoint, 0, len(result.Timestamp))
		prevClose := 0.0
		for i, ts := range result.Timestamp {
			if i >= len(q.Close) || q.Close[i] == nil {
				continue
			}
			c := *q.Close[i]
			if c <= 0 {
				continue
			}
			cp := 0.0
			if prevClose > 0 {
				cp = (c - prevClose) / prevClose * 100
			}
			points = append(points, IndexHistoryPoint{
				Date:      time.Unix(ts, 0).UTC().Format("2006-01-02"),
				Close:     c,
				ChangePct: cp,
			})
			prevClose = c
		}
		if len(points) > 5000 {
			points = points[len(points)-5000:]
		}
		snap.History = points
		if len(points) > 0 {
			last := points[len(points)-1]
			// Prefer last bar OHLC when meta open/high/low missing.
			if snap.Open <= 0 && len(q.Open) > 0 && q.Open[len(q.Open)-1] != nil {
				snap.Open = *q.Open[len(q.Open)-1]
			}
			if snap.High <= 0 && len(q.High) > 0 && q.High[len(q.High)-1] != nil {
				snap.High = *q.High[len(q.High)-1]
			}
			if snap.Low <= 0 && len(q.Low) > 0 && q.Low[len(q.Low)-1] != nil {
				snap.Low = *q.Low[len(q.Low)-1]
			}
			if snap.Price <= 0 {
				snap.Price = last.Close
			}
			// Derive previous close from penultimate daily bar when meta missing (#102).
			if snap.PreviousClose <= 0 && len(points) >= 2 {
				snap.PreviousClose = points[len(points)-2].Close
			}
			// Last resort: chartPreviousClose (only safe on short ranges).
			if snap.PreviousClose <= 0 && meta.ChartPreviousClose > 0 {
				snap.PreviousClose = meta.ChartPreviousClose
			}
			if snap.PreviousClose > 0 {
				snap.Change = snap.Price - snap.PreviousClose
				snap.ChangePct = snap.Change / snap.PreviousClose * 100
			}
		}
	}
	if snap.Price <= 0 {
		return StockSnapshot{}, fmt.Errorf("no price for %s", symbol)
	}
	// Without history path, still avoid chartPreviousClose unless nothing else.
	if snap.PreviousClose <= 0 && meta.ChartPreviousClose > 0 {
		snap.PreviousClose = meta.ChartPreviousClose
		snap.Change = snap.Price - snap.PreviousClose
		snap.ChangePct = snap.Change / snap.PreviousClose * 100
	}
	return snap, nil
}

type yahooStockChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency             string  `json:"currency"`
				Symbol               string  `json:"symbol"`
				ShortName            string  `json:"shortName"`
				RegularMarketPrice   float64 `json:"regularMarketPrice"`
				ChartPreviousClose   float64 `json:"chartPreviousClose"`
				PreviousClose        float64 `json:"previousClose"`
				RegularMarketTime    int64   `json:"regularMarketTime"`
				RegularMarketOpen    float64 `json:"regularMarketOpen"`
				RegularMarketDayHigh float64 `json:"regularMarketDayHigh"`
				RegularMarketDayLow  float64 `json:"regularMarketDayLow"`
				RegularMarketVolume  int64   `json:"regularMarketVolume"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}
