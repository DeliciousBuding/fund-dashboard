package portfolio

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
	_ "modernc.org/sqlite"
)

func TestServiceGetUSStockReadsCachedQuoteHistoryAndProfile(t *testing.T) {
	ResetUSStockSnapCache()
	t.Cleanup(ResetUSStockSnapCache)
	db := openUSStockFixture(t)
	defer db.Close()

	report, err := NewService(db).GetUSStock(context.Background(), USStockOptions{
		Symbol:         "aapl",
		Range:          "1y",
		IncludeHistory: true,
	})
	if err != nil {
		t.Fatalf("GetUSStock returned error: %v", err)
	}

	if report.Symbol != "AAPL" ||
		report.DecisionBoundary != "facts_only" ||
		report.SideEffects != "none" ||
		report.ExternalFetch != "not_performed" {
		t.Fatalf("report metadata = %#v", report)
	}
	if report.Quote == nil ||
		report.Quote.Name != "Apple Inc." ||
		report.Quote.Price != 198.25 ||
		report.Quote.ChangePct != 1.2 ||
		report.Quote.Currency != "USD" ||
		report.Quote.MarketTime != "2026-06-18 20:00:00" {
		t.Fatalf("quote = %#v, want cached AAPL quote", report.Quote)
	}
	if report.Profile == nil ||
		report.Profile.Sector != "Technology" ||
		report.Profile.Industry != "Consumer Electronics" ||
		report.Profile.MarketCap == nil ||
		*report.Profile.MarketCap != 3000000000000 {
		t.Fatalf("profile = %#v, want cached AAPL profile", report.Profile)
	}
	if report.History == nil ||
		report.History.Range != "1y" ||
		report.History.Count != 2 ||
		report.History.FirstDate != "2026-06-17" ||
		report.History.LastDate != "2026-06-18" {
		t.Fatalf("history summary = %#v, want newest-first cached history", report.History)
	}
	if report.History.Data[0].Date != "2026-06-18" ||
		report.History.Data[0].Close != 198.25 ||
		report.History.Data[0].ChangePct != 1.2 {
		t.Fatalf("first history point = %#v, want newest row", report.History.Data[0])
	}
}

func TestServiceGetUSStockReturnsNoData(t *testing.T) {
	db := openUSStockFixture(t)
	defer db.Close()

	report, err := NewService(db).GetUSStock(context.Background(), USStockOptions{
		Symbol:         "missing",
		Range:          "1y",
		IncludeHistory: true,
	})
	if err != nil {
		t.Fatalf("GetUSStock returned error: %v", err)
	}
	if report.Symbol != "MISSING" ||
		report.Error != "no_data" ||
		report.DecisionBoundary != "facts_only" ||
		report.SideEffects != "none" ||
		report.ExternalFetch != "not_performed" {
		t.Fatalf("report = %#v, want facts-only no_data", report)
	}
}

// Production Azure PG stock_realtime is minimal (no open/high/low/market) (#93).
func TestServiceGetUSStockReadsProductionPGShape(t *testing.T) {
	ResetUSStockSnapCache()
	t.Cleanup(ResetUSStockSnapCache)
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE stock_realtime (
			code TEXT PRIMARY KEY,
			name TEXT,
			price REAL,
			change_pct REAL,
			volume INTEGER,
			updated_at TEXT
		)`,
		`CREATE TABLE stock_kline_cache (
			code TEXT,
			period TEXT DEFAULT 'daily',
			date TEXT,
			open REAL,
			high REAL,
			low REAL,
			close REAL,
			volume INTEGER,
			PRIMARY KEY (code, period, date)
		)`,
		`CREATE TABLE stock_profile (
			code TEXT PRIMARY KEY,
			name TEXT,
			sector TEXT,
			market TEXT DEFAULT 'US',
			industry TEXT,
			market_cap REAL,
			pe REAL,
			description TEXT
		)`,
		`INSERT INTO stock_realtime (code, name, price, change_pct, volume, updated_at)
			VALUES ('AAPL', 'Apple Inc.', 198.25, 1.2, 45000000, '2026-07-17 12:00:00')`,
		`INSERT INTO stock_kline_cache (code, period, date, open, high, low, close, volume) VALUES
			('AAPL', 'daily', '2026-07-17', 196.5, 199.0, 195.8, 198.25, 45000000),
			('AAPL', 'daily', '2026-07-16', 194.0, 196.2, 193.7, 195.9, 41000000)`,
		`INSERT INTO stock_profile (code, name, sector, market, industry, market_cap, pe, description)
			VALUES ('AAPL', 'Apple Inc.', 'Technology', 'US', 'Consumer Electronics', 3000000000000, 31.2, 'Consumer hardware')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}

	// Minimal PG cache is incomplete (no OHLC) → Yahoo refresh (#99). Stub network.
	fetchStockSnapshotFn = func(_ context.Context, symbol, rangeKey string, withHistory bool) (datasource.StockSnapshot, error) {
		return datasource.StockSnapshot{
			Symbol: symbol, Name: "Apple Inc.", Price: 198.25, PreviousClose: 195.9,
			Change: 2.35, ChangePct: 1.2, Open: 196.5, High: 199, Low: 195.8, Volume: 45e6,
			Currency: "USD", MarketTime: time.Date(2026, 7, 17, 20, 0, 0, 0, time.UTC),
			History: []datasource.IndexHistoryPoint{
				{Date: "2026-07-16", Close: 195.9, ChangePct: 0.7},
				{Date: "2026-07-17", Close: 198.25, ChangePct: 1.2},
			},
		}, nil
	}
	t.Cleanup(func() { fetchStockSnapshotFn = datasource.FetchYahooStockSnapshot })

	report, err := NewService(db).GetUSStock(context.Background(), USStockOptions{
		Symbol:         "AAPL",
		IncludeHistory: true,
	})
	if err != nil {
		t.Fatalf("GetUSStock: %v", err)
	}
	if report.Error != "" {
		t.Fatalf("unexpected error: %#v", report)
	}
	if report.ExternalFetch != "yahoo_chart" {
		t.Fatalf("ExternalFetch=%q want yahoo_chart for incomplete PG cache", report.ExternalFetch)
	}
	if report.Quote == nil || report.Quote.Price != 198.25 || report.Quote.Open != 196.5 {
		t.Fatalf("quote = %#v", report.Quote)
	}
	if report.History == nil || report.History.Count != 2 {
		t.Fatalf("history = %#v", report.History)
	}
}

func openUSStockFixture(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE stock_realtime (
			code TEXT,
			market TEXT,
			name TEXT,
			price REAL,
			open REAL,
			high REAL,
			low REAL,
			change_pct REAL,
			change_amt REAL,
			volume REAL,
			amount REAL,
			turnover REAL,
			pe REAL,
			total_mv REAL,
			circ_mv REAL,
			high52 REAL,
			low52 REAL,
			currency TEXT DEFAULT '',
			updated_at TEXT DEFAULT (datetime('now')),
			PRIMARY KEY (code, market)
		)`,
		`CREATE TABLE stock_kline_cache (
			code TEXT,
			market TEXT,
			date TEXT,
			open REAL,
			close REAL,
			high REAL,
			low REAL,
			volume REAL,
			amount REAL,
			amplitude REAL,
			change_pct REAL,
			turnover_rate REAL,
			PRIMARY KEY (code, market, date)
		)`,
		`CREATE TABLE stock_profile (
			code TEXT,
			name TEXT,
			market TEXT,
			sector TEXT,
			industry TEXT,
			market_cap REAL,
			pe REAL,
			description TEXT,
			PRIMARY KEY (code, market)
		)`,
		`INSERT INTO stock_realtime (code, market, name, price, open, high, low, change_pct, change_amt, volume, amount, pe, total_mv, high52, low52, currency, updated_at)
			VALUES ('AAPL', 'US', 'Apple Inc.', 198.25, 196.5, 199.0, 195.8, 1.2, 2.35, 45000000, 8900000000, 31.2, 3000000000000, 205.0, 160.0, 'USD', '2026-06-18 20:00:00')`,
		`INSERT INTO stock_kline_cache (code, market, date, open, close, high, low, volume, change_pct) VALUES
			('AAPL', 'US', '2026-06-18', 196.5, 198.25, 199.0, 195.8, 45000000, 1.2),
			('AAPL', 'US', '2026-06-17', 194.0, 195.9, 196.2, 193.7, 41000000, 0.7)`,
		`INSERT INTO stock_profile (code, name, market, sector, industry, market_cap, pe, description)
			VALUES ('AAPL', 'Apple Inc.', 'US', 'Technology', 'Consumer Electronics', 3000000000000, 31.2, 'Consumer hardware and services')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			db.Close()
			t.Fatalf("exec fixture: %v", err)
		}
	}
	return db
}

func TestUSStockSnapCacheEvictsExpiredAndCapsSize(t *testing.T) {
	ResetUSStockSnapCache()
	t.Cleanup(func() {
		usStockSnapNowFn = time.Now
		ResetUSStockSnapCache()
	})

	base := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	usStockSnapNowFn = func() time.Time { return base }

	// Seed many stale entries under the write lock path via store + backdating.
	for i := 0; i < 50; i++ {
		storeUSStockSnapCache(fmt.Sprintf("STALE%d", i), datasource.StockSnapshot{Symbol: fmt.Sprintf("STALE%d", i), Price: float64(i)})
	}
	usStockSnapMu.Lock()
	for k, e := range usStockSnapCache {
		e.fetched = base.Add(-usStockSnapFreshFor - time.Minute)
		usStockSnapCache[k] = e
	}
	usStockSnapMu.Unlock()

	// One store should sweep all expired entries.
	storeUSStockSnapCache("FRESH", datasource.StockSnapshot{Symbol: "FRESH", Price: 1})
	usStockSnapMu.RLock()
	sizeAfterExpiry := len(usStockSnapCache)
	_, hasFresh := usStockSnapCache["FRESH"]
	usStockSnapMu.RUnlock()
	if sizeAfterExpiry != 1 || !hasFresh {
		t.Fatalf("after expiry sweep size=%d hasFresh=%v, want size=1 with FRESH", sizeAfterExpiry, hasFresh)
	}

	// Over-cap with unique fresh keys; each store advances now so oldest is well-defined.
	for i := 0; i < maxUSStockSnapCache+25; i++ {
		idx := i
		usStockSnapNowFn = func() time.Time { return base.Add(time.Duration(idx) * time.Second) }
		storeUSStockSnapCache(fmt.Sprintf("S%d", i), datasource.StockSnapshot{Symbol: fmt.Sprintf("S%d", i), Price: float64(i)})
	}
	usStockSnapMu.RLock()
	size := len(usStockSnapCache)
	_, hasOldest := usStockSnapCache["S0"]
	_, hasNewest := usStockSnapCache[fmt.Sprintf("S%d", maxUSStockSnapCache+24)]
	usStockSnapMu.RUnlock()
	if size > maxUSStockSnapCache {
		t.Fatalf("cache size=%d exceeds maxUSStockSnapCache=%d", size, maxUSStockSnapCache)
	}
	if hasOldest {
		t.Fatalf("expected oldest entry S0 to be evicted under size cap")
	}
	if !hasNewest {
		t.Fatalf("expected newest entry to remain under size cap")
	}
}

func TestGetUSStockRefreshesIncompleteCache(t *testing.T) {
	ResetUSStockSnapCache()
	t.Cleanup(ResetUSStockSnapCache)
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE stock_realtime (
			code TEXT PRIMARY KEY,
			name TEXT,
			price REAL,
			change_pct REAL,
			volume INTEGER,
			updated_at TEXT
		)`,
		`CREATE TABLE stock_kline_cache (
			code TEXT,
			period TEXT DEFAULT 'daily',
			date TEXT,
			close REAL,
			PRIMARY KEY (code, period, date)
		)`,
		`INSERT INTO stock_realtime (code, name, price, change_pct, volume, updated_at)
			VALUES ('AAPL', 'Apple Inc.', 300, 1.0, 1000, '2026-07-17 00:00:00')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	fetchStockSnapshotFn = func(_ context.Context, symbol, rangeKey string, withHistory bool) (datasource.StockSnapshot, error) {
		return datasource.StockSnapshot{
			Symbol: symbol, Name: "Apple Inc.", Price: 333.26, PreviousClose: 330,
			Change: 3.26, ChangePct: 0.99, Open: 328, High: 335, Low: 327, Volume: 5e7,
			Currency: "USD", MarketTime: time.Date(2026, 7, 17, 20, 0, 0, 0, time.UTC),
			History: []datasource.IndexHistoryPoint{
				{Date: "2026-07-16", Close: 330, ChangePct: 0},
				{Date: "2026-07-17", Close: 333.26, ChangePct: 0.99},
			},
		}, nil
	}
	t.Cleanup(func() { fetchStockSnapshotFn = datasource.FetchYahooStockSnapshot })

	report, err := NewService(db).GetUSStock(context.Background(), USStockOptions{Symbol: "AAPL", IncludeHistory: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.ExternalFetch != "yahoo_chart" {
		t.Fatalf("ExternalFetch=%q", report.ExternalFetch)
	}
	if report.Quote == nil || report.Quote.Open != 328 || report.Quote.PreviousClose != 330 {
		t.Fatalf("quote=%#v", report.Quote)
	}
	if report.History == nil || report.History.Count != 2 {
		t.Fatalf("history=%#v", report.History)
	}
}
