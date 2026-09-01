package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/chinatime"
	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
	"golang.org/x/sync/singleflight"
)

// indicesStaleAfter controls refresh-on-read for MarketTicker (#92).
const indicesStaleAfter = 6 * time.Hour

// Overridable in tests.
var (
	indicesNowFn   = time.Now
	fetchIndexFn   = datasource.FetchYahooIndexQuotes
	indexSymbolsFn = func() []string { return append([]string{}, datasource.DefaultIndexSymbols...) }
	// Coalesce concurrent stale-cache refreshes (HTTP/MCP/SSE stampede).
	indicesRefreshSF singleflight.Group
)

type MarketIndicesReport struct {
	Indices          map[string]MarketIndexQuote `json:"indices"`
	Count            int                         `json:"count"`
	DecisionBoundary string                      `json:"decision_boundary"`
	SideEffects      string                      `json:"side_effects"`
	ExternalFetch    string                      `json:"external_fetch"`
	Error            string                      `json:"error,omitempty"`
	Message          string                      `json:"message,omitempty"`
}

type MarketIndexQuote struct {
	Name      string  `json:"name"`
	Market    string  `json:"market"`
	Price     float64 `json:"price"`
	ChangePct float64 `json:"change_pct"`
	Change    float64 `json:"change"`
	UpdatedAt string  `json:"updated_at"`
}

func (s Service) GetMarketIndices(ctx context.Context) (MarketIndicesReport, error) {
	report, err := s.readMarketIndices(ctx)
	if err != nil {
		return MarketIndicesReport{}, err
	}
	if !s.indicesNeedRefresh(report) {
		return report, nil
	}

	// Coalesce concurrent refresh-on-read; waiters re-read cache after the leader finishes.
	v, refreshErr, _ := indicesRefreshSF.Do("market-indices", func() (any, error) {
		// Re-check under the singleflight leader in case another waiter already refreshed.
		cur, err := s.readMarketIndices(ctx)
		if err != nil {
			return 0, err
		}
		if !s.indicesNeedRefresh(cur) {
			return 0, nil
		}
		// Bound upstream wait so a hung Yahoo call cannot pin all readers.
		rctx, cancel := context.WithTimeout(ctx, 12*time.Second)
		defer cancel()
		return s.RefreshMarketIndices(rctx)
	})
	n, ok := v.(int)
	if !ok {
		slog.Warn("market indices singleflight returned unexpected type", "type", fmt.Sprintf("%T", v))
		n = 0
	}
	if refreshErr != nil {
		slog.Warn("market indices refresh failed; serving cache", "error", refreshErr)
		report.ExternalFetch = "yahoo_chart_failed_using_cache"
		if report.Count == 0 {
			report.Error = "no_data"
			// Stable client code only — full error stays in slog (#247).
			// Prefer upstream_unavailable over empty-cache prose when refresh failed.
			report.Message = "upstream_unavailable"
		}
		return report, nil
	}
	report, err = s.readMarketIndices(ctx)
	if err != nil {
		return MarketIndicesReport{}, err
	}
	if n > 0 {
		report.ExternalFetch = "yahoo_chart"
		report.SideEffects = "indices_cache_upsert"
	} else if report.Count > 0 {
		// Leader or co-waiter found cache already fresh after coalesce.
		report.ExternalFetch = "not_performed"
	}
	return report, nil
}

// RefreshMarketIndices fetches Yahoo quotes and upserts the indices table.
func (s Service) RefreshMarketIndices(ctx context.Context) (int, error) {
	hasTable, err := s.tableExists(ctx, "indices")
	if err != nil {
		return 0, err
	}
	if !hasTable {
		return 0, fmt.Errorf("indices table not found")
	}
	quotes, err := fetchIndexFn(ctx, indexSymbolsFn())
	if err != nil {
		return 0, err
	}
	if len(quotes) == 0 {
		return 0, fmt.Errorf("no index quotes returned")
	}
	n := 0
	for _, q := range quotes {
		updated := q.UpdatedAt.In(chinatime.Loc).Format("2006-01-02 15:04:05")
		if q.UpdatedAt.IsZero() {
			updated = indicesNowFn().In(chinatime.Loc).Format("2006-01-02 15:04:05")
		}
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO indices (code, name, market, price, change_pct, change_amt, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(code) DO UPDATE SET
				name = excluded.name,
				market = excluded.market,
				price = excluded.price,
				change_pct = excluded.change_pct,
				change_amt = excluded.change_amt,
				updated_at = excluded.updated_at
		`, q.Code, q.Name, q.Market, q.Price, q.ChangePct, q.Change, updated)
		if err != nil {
			return n, fmt.Errorf("upsert index %s: %w", q.Code, err)
		}
		n++
	}
	slog.Info("market indices refreshed", "rows", n)
	return n, nil
}

func (s Service) readMarketIndices(ctx context.Context) (MarketIndicesReport, error) {
	report := MarketIndicesReport{
		Indices:          map[string]MarketIndexQuote{},
		DecisionBoundary: "facts_only",
		SideEffects:      "none",
		ExternalFetch:    "not_performed",
	}
	hasTable, err := s.tableExists(ctx, "indices")
	if err != nil {
		return MarketIndicesReport{}, err
	}
	if !hasTable {
		report.Error = "no_data"
		report.Message = "indices table not found"
		return report, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			code,
			COALESCE(name, ''),
			COALESCE(market, ''),
			price,
			change_pct,
			change_amt,
			COALESCE(updated_at, '')
		FROM indices
		ORDER BY code
		LIMIT 64
	`)
	if err != nil {
		return MarketIndicesReport{}, fmt.Errorf("query market indices: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var code string
		var item MarketIndexQuote
		var price sql.NullFloat64
		var changePct sql.NullFloat64
		var change sql.NullFloat64
		if err := rows.Scan(&code, &item.Name, &item.Market, &price, &changePct, &change, &item.UpdatedAt); err != nil {
			return MarketIndicesReport{}, fmt.Errorf("scan market index: %w", err)
		}
		code = clampPortfolioText(code, 32)
		item.Name = clampPortfolioText(item.Name, 120)
		item.Market = clampPortfolioText(item.Market, 32)
		item.UpdatedAt = clampPortfolioText(item.UpdatedAt, 40)
		item.Price = nullableFloat64Value(price)
		item.ChangePct = nullableFloat64Value(changePct)
		item.Change = nullableFloat64Value(change)
		report.Indices[code] = item
	}
	if err := rows.Err(); err != nil {
		return MarketIndicesReport{}, fmt.Errorf("market index rows: %w", err)
	}
	report.Count = len(report.Indices)
	if report.Count == 0 {
		report.Error = "no_data"
		report.Message = "indices table has no cached rows"
	}
	return report, nil
}

func (s Service) indicesNeedRefresh(report MarketIndicesReport) bool {
	if report.Count == 0 {
		return true
	}
	newest := time.Time{}
	for _, item := range report.Indices {
		ts := parseIndexUpdatedAt(item.UpdatedAt)
		if ts.After(newest) {
			newest = ts
		}
	}
	if newest.IsZero() {
		return true
	}
	return indicesNowFn().Sub(newest) > indicesStaleAfter
}

func parseIndexUpdatedAt(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, chinatime.Loc); err == nil {
			return t
		}
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}
