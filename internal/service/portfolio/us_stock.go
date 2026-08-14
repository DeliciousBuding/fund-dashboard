package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	usStockSnapFreshFor  = 15 * time.Minute
	maxUSStockSnapCache  = 200
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

func (s Service) GetUSStock(ctx context.Context, opts USStockOptions) (USStockReport, error) {
	symbol := strings.ToUpper(strings.TrimSpace(opts.Symbol))
	if len(symbol) > 32 {
		symbol = symbol[:32]
	}
	report := USStockReport{
		Symbol:           symbol,
		DecisionBoundary: "facts_only",
		SideEffects:      "none",
		ExternalFetch:    "not_performed",
	}
	if symbol == "" {
		report.Error = "no_data"
		report.Message = "symbol is required"
		return report, nil
	}

	quote, err := s.queryCachedUSStockQuote(ctx, symbol)
	if err != nil {
		return USStockReport{}, err
	}
	report.Quote = quote

	if opts.IncludeHistory {
		history, err := s.queryCachedUSStockHistory(ctx, symbol, normalizeUSStockRange(opts.Range))
		if err != nil {
			return USStockReport{}, err
		}
		report.History = history
	}

	profile, err := s.queryUSStockProfile(ctx, symbol)
	if err != nil {
		return USStockReport{}, err
	}
	report.Profile = profile

	hasHistory := report.History != nil && report.History.Count > 0
	// Production stock_realtime often only has price/change_pct/volume (#99).
	// Re-fetch Yahoo when OHLC incomplete or history requested but empty.
	needRefresh := report.Quote == nil && !hasHistory
	if report.Quote != nil && usStockQuoteIncomplete(report.Quote) {
		needRefresh = true
	}
	if opts.IncludeHistory && !hasHistory {
		needRefresh = true
	}
	if !needRefresh {
		return report, nil
	}

	// Prefer process-local full snapshot (OHLC+history) before Yahoo (#99).
	var snap datasource.StockSnapshot
	err = nil
	if cached, ok := loadUSStockSnapCache(symbol); ok {
		snap = cached
		report.ExternalFetch = "memory_cache"
	} else {
		// Yahoo refresh-on-read (#98/#99), best-effort upsert quote+kline.
		snap, err = fetchStockSnapshotFn(ctx, symbol, normalizeUSStockRange(opts.Range), true)
	}
	if err != nil {
		// Keep partial cache if we already have a price; only hard no_data when empty.
		if report.Quote == nil && !hasHistory {
			report.Error = "no_data"
			report.Message = "upstream_unavailable"
		}
		return report, nil
	}
	if report.ExternalFetch != "memory_cache" {
		report.ExternalFetch = "yahoo_chart"
		storeUSStockSnapCache(symbol, snap)
	}
	report.Error = ""
	report.Message = ""
	report.Quote = &USStockQuote{
		Name:          clampPortfolioText(snap.Name, 200),
		Price:         snap.Price,
		PreviousClose: snap.PreviousClose,
		Change:        snap.Change,
		ChangePct:     snap.ChangePct,
		Open:          snap.Open,
		High:          snap.High,
		Low:           snap.Low,
		Volume:        snap.Volume,
		Currency:      clampPortfolioText(snap.Currency, 16),
		MarketTime:    snap.MarketTime.UTC().Format(time.RFC3339),
	}
	if len(snap.History) > 0 {
		pts := make([]USStockHistoryPoint, 0, len(snap.History))
		for _, p := range snap.History {
			pts = append(pts, USStockHistoryPoint{Date: p.Date, Close: p.Close, ChangePct: p.ChangePct})
		}
		report.History = &USStockHistory{
			Range:     normalizeUSStockRange(opts.Range),
			Count:     len(pts),
			FirstDate: pts[0].Date,
			LastDate:  pts[len(pts)-1].Date,
			Data:      pts,
		}
	}
	effects := []string{}
	if n, err := s.upsertUSStockSnapshot(ctx, snap); err == nil && n > 0 {
		effects = append(effects, "stock_realtime_upsert")
	}
	if n, err := s.upsertUSStockHistory(ctx, snap); err == nil && n > 0 {
		effects = append(effects, "stock_kline_upsert")
	}
	if len(effects) > 0 {
		report.SideEffects = strings.Join(effects, ",")
	}
	return report, nil
}

func usStockQuoteIncomplete(q *USStockQuote) bool {
	if q == nil {
		return true
	}
	// Minimal production stock_realtime only stores price/change/volume (#99).
	// If OHLC are all zero, force Yahoo refresh. previous_close alone is not required
	// for cache hit (often not a DB column).
	if q.Price > 0 && q.Open <= 0 && q.High <= 0 && q.Low <= 0 {
		return true
	}
	return false
}

func (s Service) upsertUSStockSnapshot(ctx context.Context, snap datasource.StockSnapshot) (int, error) {
	hasTable, err := s.tableExists(ctx, "stock_realtime")
	if err != nil || !hasTable {
		return 0, err
	}
	cols, err := s.tableColumns(ctx, "stock_realtime")
	if err != nil {
		return 0, err
	}
	has := func(name string) bool { _, ok := cols[name]; return ok }
	updated := snap.MarketTime.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
	// Prefer rich Bun-era columns when present; else minimal production shape.
	if has("market") && has("open") && has("high") && has("low") {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO stock_realtime (code, market, name, price, open, high, low, change_pct, change_amt, volume, currency, updated_at)
			VALUES (?, 'US', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(code, market) DO UPDATE SET
				name=excluded.name, price=excluded.price, open=excluded.open, high=excluded.high, low=excluded.low,
				change_pct=excluded.change_pct, change_amt=excluded.change_amt, volume=excluded.volume,
				currency=excluded.currency, updated_at=excluded.updated_at
		`, snap.Symbol, snap.Name, snap.Price, snap.Open, snap.High, snap.Low, snap.ChangePct, snap.Change, snap.Volume, snap.Currency, updated)
	} else if has("market") {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO stock_realtime (code, market, name, price, change_pct, volume, updated_at)
			VALUES (?, 'US', ?, ?, ?, ?, ?)
			ON CONFLICT(code, market) DO UPDATE SET
				name=excluded.name, price=excluded.price, change_pct=excluded.change_pct,
				volume=excluded.volume, updated_at=excluded.updated_at
		`, snap.Symbol, snap.Name, snap.Price, snap.ChangePct, snap.Volume, updated)
	} else {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO stock_realtime (code, name, price, change_pct, volume, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(code) DO UPDATE SET
				name=excluded.name, price=excluded.price, change_pct=excluded.change_pct,
				volume=excluded.volume, updated_at=excluded.updated_at
		`, snap.Symbol, snap.Name, snap.Price, snap.ChangePct, snap.Volume, updated)
	}
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (s Service) upsertUSStockHistory(ctx context.Context, snap datasource.StockSnapshot) (int, error) {
	if len(snap.History) == 0 {
		return 0, nil
	}
	kind, stmt, err := s.preparedKlineUpsert(ctx)
	if err != nil {
		return 0, err
	}
	if kind == klineUpsertNone || stmt == nil {
		return 0, nil
	}
	n := 0
	for _, pt := range snap.History {
		var execErr error
		switch kind {
		case klineUpsertMarketPeriod:
			_, execErr = stmt.ExecContext(ctx, snap.Symbol, pt.Date, pt.Close, pt.ChangePct)
		case klineUpsertMarket:
			_, execErr = stmt.ExecContext(ctx, snap.Symbol, pt.Date, pt.Close, pt.ChangePct)
		case klineUpsertPeriod:
			_, execErr = stmt.ExecContext(ctx, snap.Symbol, pt.Date, pt.Close)
		default:
			return 0, nil
		}
		if execErr != nil {
			return n, execErr
		}
		n++
	}
	return n, nil
}

func (s Service) queryCachedUSStockQuote(ctx context.Context, symbol string) (*USStockQuote, error) {
	hasTable, err := s.tableExists(ctx, "stock_realtime")
	if err != nil || !hasTable {
		return nil, err
	}
	// Production Azure PG shape (#93): code, name, price, change_pct, volume, updated_at.
	// Optional Bun-era columns (open/high/low/change_amt/currency/market) are selected only when present.
	cols, err := s.tableColumns(ctx, "stock_realtime")
	if err != nil {
		return nil, err
	}
	has := func(name string) bool { _, ok := cols[name]; return ok }

	// Avoid PG-only casts so SQLite fixtures keep working.
	selectCols := []string{"COALESCE(name, '')", "price", "change_pct", "volume", "COALESCE(CAST(updated_at AS TEXT), '')"}
	scanOpen, scanHigh, scanLow, scanChangeAmt, scanCurrency := false, false, false, false, false
	if has("open") {
		selectCols = append(selectCols, "open")
		scanOpen = true
	}
	if has("high") {
		selectCols = append(selectCols, "high")
		scanHigh = true
	}
	if has("low") {
		selectCols = append(selectCols, "low")
		scanLow = true
	}
	if has("change_amt") {
		selectCols = append(selectCols, "change_amt")
		scanChangeAmt = true
	}
	if has("currency") {
		selectCols = append(selectCols, "COALESCE(currency, '')")
		scanCurrency = true
	}

	where := "code = ?"
	args := []any{symbol}
	if has("market") {
		where += " AND (market = 'US' OR market = '' OR market IS NULL)"
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM stock_realtime
		WHERE %s
		LIMIT 1
	`, strings.Join(selectCols, ", "), where)

	row := s.db.QueryRowContext(ctx, query, args...)
	var quote USStockQuote
	var price, changePct, volume sql.NullFloat64
	var updated string
	dest := []any{&quote.Name, &price, &changePct, &volume, &updated}
	var open, high, low, changeAmt sql.NullFloat64
	var currency string
	if scanOpen {
		dest = append(dest, &open)
	}
	if scanHigh {
		dest = append(dest, &high)
	}
	if scanLow {
		dest = append(dest, &low)
	}
	if scanChangeAmt {
		dest = append(dest, &changeAmt)
	}
	if scanCurrency {
		dest = append(dest, &currency)
	}
	err = row.Scan(dest...)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query cached us stock quote: %w", err)
	}
	quote.Price = nullableFloat64Value(price)
	quote.ChangePct = nullableFloat64Value(changePct)
	quote.Volume = nullableFloat64Value(volume)
	quote.MarketTime = updated
	if scanOpen {
		quote.Open = nullableFloat64Value(open)
	}
	if scanHigh {
		quote.High = nullableFloat64Value(high)
	}
	if scanLow {
		quote.Low = nullableFloat64Value(low)
	}
	if scanChangeAmt {
		quote.Change = nullableFloat64Value(changeAmt)
	}
	if scanCurrency && currency != "" {
		quote.Currency = currency
	} else {
		quote.Currency = "USD"
	}
	return &quote, nil
}

func (s Service) queryCachedUSStockHistory(ctx context.Context, symbol string, historyRange string) (*USStockHistory, error) {
	hasTable, err := s.tableExists(ctx, "stock_kline_cache")
	if err != nil || !hasTable {
		return nil, err
	}
	cols, err := s.tableColumns(ctx, "stock_kline_cache")
	if err != nil {
		return nil, err
	}
	has := func(name string) bool { _, ok := cols[name]; return ok }

	selectCols := []string{"date", "close"}
	scanChange := false
	if has("change_pct") {
		selectCols = append(selectCols, "change_pct")
		scanChange = true
	}
	where := "code = ?"
	args := []any{symbol}
	if has("market") {
		where += " AND (market = 'US' OR market = '' OR market IS NULL)"
	}
	if has("period") {
		where += " AND (period = 'daily' OR period = '' OR period IS NULL)"
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM stock_kline_cache
		WHERE %s
		ORDER BY date DESC
		LIMIT 250
	`, strings.Join(selectCols, ", "), where)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query cached us stock history: %w", err)
	}
	defer rows.Close()

	points := []USStockHistoryPoint{}
	for rows.Next() {
		var point USStockHistoryPoint
		var closeValue sql.NullFloat64
		var changePct sql.NullFloat64
		if scanChange {
			if err := rows.Scan(&point.Date, &closeValue, &changePct); err != nil {
				return nil, fmt.Errorf("scan us stock history: %w", err)
			}
			point.ChangePct = nullableFloat64Value(changePct)
		} else {
			if err := rows.Scan(&point.Date, &closeValue); err != nil {
				return nil, fmt.Errorf("scan us stock history: %w", err)
			}
		}
		point.Close = nullableFloat64Value(closeValue)
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("us stock history rows: %w", err)
	}
	if len(points) == 0 {
		return nil, nil
	}
	// Derive change_pct from adjacent closes when column missing.
	if !scanChange {
		for i := 0; i < len(points)-1; i++ {
			prev := points[i+1].Close
			if prev > 0 {
				points[i].ChangePct = (points[i].Close - prev) / prev * 100
			}
		}
	}
	return &USStockHistory{
		Range:     historyRange,
		Count:     len(points),
		FirstDate: points[len(points)-1].Date,
		LastDate:  points[0].Date,
		Data:      points,
	}, nil
}

func (s Service) queryUSStockProfile(ctx context.Context, symbol string) (*USStockProfile, error) {
	hasTable, err := s.tableExists(ctx, "stock_profile")
	if err != nil || !hasTable {
		return nil, err
	}
	cols, err := s.tableColumns(ctx, "stock_profile")
	if err != nil {
		return nil, err
	}
	has := func(name string) bool { _, ok := cols[name]; return ok }

	selectCols := []string{"COALESCE(sector, '')"}
	if has("industry") {
		selectCols = append(selectCols, "COALESCE(industry, '')")
	} else {
		selectCols = append(selectCols, "''")
	}
	if has("market_cap") {
		selectCols = append(selectCols, "market_cap")
	} else {
		selectCols = append(selectCols, "NULL")
	}
	if has("pe") {
		selectCols = append(selectCols, "pe")
	} else {
		selectCols = append(selectCols, "NULL")
	}
	if has("description") {
		selectCols = append(selectCols, "COALESCE(description, '')")
	} else {
		selectCols = append(selectCols, "''")
	}
	where := "code = ?"
	args := []any{symbol}
	if has("market") {
		where += " AND (market = 'US' OR market = '' OR market IS NULL)"
	}
	query := fmt.Sprintf(`
		SELECT %s
		FROM stock_profile
		WHERE %s
		LIMIT 1
	`, strings.Join(selectCols, ", "), where)

	var profile USStockProfile
	var marketCap sql.NullFloat64
	var pe sql.NullFloat64
	err = s.db.QueryRowContext(ctx, query, args...).Scan(
		&profile.Sector,
		&profile.Industry,
		&marketCap,
		&pe,
		&profile.Description,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query us stock profile: %w", err)
	}
	if marketCap.Valid {
		v := marketCap.Float64
		profile.MarketCap = &v
	}
	if pe.Valid {
		v := pe.Float64
		profile.PE = &v
	}
	profile.Sector = clampPortfolioText(profile.Sector, 64)
	profile.Industry = clampPortfolioText(profile.Industry, 64)
	profile.Description = clampPortfolioText(profile.Description, 500)
	if profile.Sector == "" && profile.Industry == "" && profile.MarketCap == nil && profile.PE == nil && profile.Description == "" {
		return nil, nil
	}
	return &profile, nil
}

func normalizeUSStockRange(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "1y", "1Y":
		return "1y"
	case "6m", "6M":
		return "6m"
	case "3m", "3M":
		return "3m"
	case "1m", "1M":
		return "1m"
	case "5y", "5Y":
		return "5y"
	case "max":
		return "max"
	default:
		return "1y"
	}
}

func nullableFloat64Value(value sql.NullFloat64) float64 {
	if !value.Valid {
		return 0
	}
	return value.Float64
}
