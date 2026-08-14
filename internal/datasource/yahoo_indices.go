package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// IndexQuote is a market index snapshot from an external provider.
type IndexQuote struct {
	Code      string
	Name      string
	Market    string
	Price     float64
	ChangePct float64
	Change    float64
	UpdatedAt time.Time
}

// IndexHistoryPoint is one OHLCV-close observation for an index series.
type IndexHistoryPoint struct {
	Date      string
	Close     float64
	ChangePct float64
}

// IndexHistory is a Yahoo chart history series.
type IndexHistory struct {
	Symbol string
	Range  string
	Points []IndexHistoryPoint
}

// DefaultUSIndexSymbols are the SPA MarketTicker core US indices.
// DefaultIndexSymbols are all MarketTicker indices including CN/HK (#100).
var DefaultIndexSymbols = []string{"^NDX", "^GSPC", "^DJI", "^IXIC", "^HSI", "000001.SS", "399001.SZ", "399006.SZ"}

// DefaultUSIndexSymbols is kept for backward compatibility.
var DefaultUSIndexSymbols = DefaultIndexSymbols

// defaultIndexNames is the Yahoo shortName fallback when meta is empty (#164).
// English Yahoo-style labels — SPA MarketTicker display uses market.index.* i18n.
var defaultIndexNames = map[string]string{
	"^NDX":      "NASDAQ-100",
	"^GSPC":     "S&P 500",
	"^DJI":      "Dow Jones Industrial Average",
	"^IXIC":     "NASDAQ Composite",
	"^HSI":      "HANG SENG INDEX",
	"000001.SS": "SSE Composite Index",
	"399001.SZ": "Shenzhen Component",
	"399006.SZ": "ChiNext",
}

// NormalizeIndexSymbol maps SPA short codes (NDX) to Yahoo symbols (^NDX).
func NormalizeIndexSymbol(raw string) string {
	code := strings.TrimSpace(raw)
	if code == "" {
		return ""
	}
	// URL may pass %5E as ^ already decoded by chi.
	upper := strings.ToUpper(code)
	aliases := map[string]string{
		"NDX":      "^NDX",
		"^NDX":     "^NDX",
		"GSPC":     "^GSPC",
		"^GSPC":    "^GSPC",
		"DJI":      "^DJI",
		"^DJI":     "^DJI",
		"IXIC":     "^IXIC",
		"^IXIC":    "^IXIC",
		"HSI":      "^HSI",
		"^HSI":     "^HSI",
		"SH000001": "000001.SS",
		"SZ399001": "399001.SZ",
		"SZ399006": "399006.SZ",
	}
	if v, ok := aliases[upper]; ok {
		return v
	}
	if strings.HasPrefix(code, "^") {
		out := strings.ToUpper(code[:1]) + strings.ToUpper(code[1:])
		if len(out) > 32 {
			return out[:32]
		}
		return out
	}
	// Keep original casing for unknown symbols (Yahoo is case-sensitive for some).
	if len(code) > 32 {
		return code[:32]
	}
	return code
}

// FetchYahooIndexQuotes loads latest quotes for the given Yahoo symbols.
func FetchYahooIndexQuotes(ctx context.Context, symbols []string) ([]IndexQuote, error) {
	if len(symbols) == 0 {
		symbols = append([]string{}, DefaultIndexSymbols...)
	}
	template := os.Getenv("FUND_YAHOO_CHART_ENDPOINT")
	if template == "" {
		template = "https://query1.finance.yahoo.com/v8/finance/chart/%s?range=5d&interval=1d"
	}
	client := &http.Client{Timeout: 8 * time.Second}
	out := make([]IndexQuote, 0, len(symbols))
	var firstErr error
	for _, symbol := range symbols {
		if err := ctx.Err(); err != nil {
			if len(out) > 0 {
				return out, nil
			}
			return nil, err
		}
		quote, err := fetchOneYahooIndex(ctx, client, template, symbol)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("yahoo index %s: %w", symbol, err)
			}
			// Partial success: keep collecting other symbols (#247).
			continue
		}
		out = append(out, quote)
	}
	if len(out) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("no index quotes returned")
	}
	return out, nil
}

// FetchYahooIndexHistory loads historical closes for one index symbol.
// rangeKey examples: 1d,5d,1mo,3mo,6mo,1y,2y,5y,ytd,max (Yahoo style).
// interval examples: 1d, 60m (empty → 1d).
func FetchYahooIndexHistory(ctx context.Context, symbol, rangeKey, interval string) (IndexHistory, error) {
	symbol = NormalizeIndexSymbol(symbol)
	if symbol == "" {
		return IndexHistory{}, fmt.Errorf("symbol is required")
	}
	rangeKey = normalizeYahooRange(rangeKey)
	interval = normalizeYahooInterval(interval)
	template := os.Getenv("FUND_YAHOO_HISTORY_ENDPOINT")
	if template == "" {
		template = "https://query1.finance.yahoo.com/v8/finance/chart/%s?range=%s&interval=%s"
	}
	endpoint := fmt.Sprintf(template, url.PathEscape(symbol), url.QueryEscape(rangeKey), url.QueryEscape(interval))
	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return IndexHistory{}, err
	}
	req.Header.Set("User-Agent", "fund-dashboard/1.0")
	res, err := client.Do(req)
	if err != nil {
		return IndexHistory{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return IndexHistory{}, fmt.Errorf("status %d", res.StatusCode)
	}
	var payload yahooChartHistoryResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&payload); err != nil {
		return IndexHistory{}, err
	}
	if len(payload.Chart.Result) == 0 {
		return IndexHistory{}, fmt.Errorf("empty chart result")
	}
	result := payload.Chart.Result[0]
	timestamps := result.Timestamp
	var closes []*float64
	if len(result.Indicators.Quote) > 0 {
		closes = result.Indicators.Quote[0].Close
	}
	points := make([]IndexHistoryPoint, 0, len(timestamps))
	prev := 0.0
	for i, ts := range timestamps {
		if i >= len(closes) {
			break
		}
		if closes[i] == nil {
			continue
		}
		close := *closes[i]
		// Yahoo uses nulls for missing bars; skip non-positive closes.
		if close <= 0 {
			continue
		}
		changePct := 0.0
		if prev > 0 {
			changePct = (close - prev) / prev * 100
		}
		// For daily bars use date; for intraday include time UTC.
		date := time.Unix(ts, 0).UTC().Format("2006-01-02")
		if interval != "1d" && interval != "1wk" && interval != "1mo" {
			date = time.Unix(ts, 0).UTC().Format("2006-01-02 15:04")
		}
		points = append(points, IndexHistoryPoint{
			Date:      date,
			Close:     close,
			ChangePct: changePct,
		})
		prev = close
	}
	if len(points) > 5000 {
		points = points[len(points)-5000:]
	}
	if len(points) == 0 {
		return IndexHistory{}, fmt.Errorf("no history points")
	}
	return IndexHistory{Symbol: symbol, Range: rangeKey, Points: points}, nil
}

func fetchOneYahooIndex(ctx context.Context, client *http.Client, template, symbol string) (IndexQuote, error) {
	symbol = NormalizeIndexSymbol(symbol)
	endpoint := fmt.Sprintf(template, url.PathEscape(symbol))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return IndexQuote{}, err
	}
	req.Header.Set("User-Agent", "fund-dashboard/1.0")
	res, err := client.Do(req)
	if err != nil {
		return IndexQuote{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return IndexQuote{}, fmt.Errorf("status %d", res.StatusCode)
	}
	var payload yahooChartResponse
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&payload); err != nil {
		return IndexQuote{}, err
	}
	if len(payload.Chart.Result) == 0 {
		return IndexQuote{}, fmt.Errorf("empty chart result")
	}
	meta := payload.Chart.Result[0].Meta
	price := meta.RegularMarketPrice
	prev := meta.ChartPreviousClose
	if prev <= 0 {
		prev = meta.PreviousClose
	}
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
		name = defaultIndexNames[symbol]
	}
	if name == "" {
		name = symbol
	}
	market := indexMarket(symbol)
	return IndexQuote{
		Code:      symbol,
		Name:      name,
		Market:    market,
		Price:     price,
		ChangePct: changePct,
		Change:    change,
		UpdatedAt: updated,
	}, nil
}

// indexMarket returns the market region for an index symbol.
func indexMarket(symbol string) string {
	switch {
	case strings.HasPrefix(symbol, "^") && symbol != "^HSI":
		return "US"
	case strings.HasSuffix(symbol, ".SS") || strings.HasSuffix(symbol, ".SZ"):
		return "CN"
	case symbol == "^HSI" || strings.HasSuffix(symbol, ".HK"):
		return "HK"
	default:
		return "US"
	}
}

// NormalizeYahooRange is the exported whitelist for SPA/MCP range query params (#228).
func NormalizeYahooRange(raw string) string {
	return normalizeYahooRange(raw)
}

// NormalizeYahooInterval is the exported whitelist for SPA/MCP interval query params (#228).
func NormalizeYahooInterval(raw string) string {
	return normalizeYahooInterval(raw)
}

func normalizeYahooRange(raw string) string {
	r := strings.ToLower(strings.TrimSpace(raw))
	// Bound absurd inputs before switch (#228).
	if len(r) > 16 {
		return "1y"
	}
	switch r {
	case "", "1y":
		return "1y"
	case "1m", "1mo":
		return "1mo"
	case "3m", "3mo":
		return "3mo"
	case "6m", "6mo":
		return "6mo"
	case "1d", "5d", "ytd", "max", "2y", "5y", "10y":
		return r
	case "all", "tx":
		return "max"
	default:
		// Unknown → safe default (was: return raw unbounded string).
		return "1y"
	}
}

func normalizeYahooInterval(raw string) string {
	i := strings.ToLower(strings.TrimSpace(raw))
	switch i {
	case "", "1d", "daily":
		return "1d"
	case "60m", "1h", "60min":
		return "60m"
	case "1wk", "1mo", "5m", "15m", "30m":
		return i
	default:
		return "1d"
	}
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency           string  `json:"currency"`
				Symbol             string  `json:"symbol"`
				ShortName          string  `json:"shortName"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
				PreviousClose      float64 `json:"previousClose"`
				RegularMarketTime  int64   `json:"regularMarketTime"`
			} `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
}

type yahooChartHistoryResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol string `json:"symbol"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []*float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}
