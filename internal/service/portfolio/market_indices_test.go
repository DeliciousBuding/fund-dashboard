package portfolio

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
	_ "modernc.org/sqlite"
)

func TestServiceGetMarketIndicesReadsFreshCachedRows(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	indicesNowFn = func() time.Time { return now }
	t.Cleanup(func() { indicesNowFn = time.Now })
	fresh := now.Add(-30 * time.Minute).Format("2006-01-02 15:04:05")
	for _, stmt := range []string{
		`CREATE TABLE indices (
			code TEXT PRIMARY KEY,
			name TEXT,
			market TEXT,
			price REAL,
			change_pct REAL,
			change_amt REAL,
			updated_at TEXT DEFAULT (datetime('now'))
		)`,
		`INSERT INTO indices (code, name, market, price, change_pct, change_amt, updated_at) VALUES
			('^GSPC', '标普500', 'US', 5600.5, 0.42, 23.5, '` + fresh + `'),
			('^NDX', '纳斯达克100', 'US', 19888.2, 1.25, 245.8, '` + fresh + `')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec fixture: %v", err)
		}
	}

	// Should not call upstream when cache is fresh.
	fetchIndexFn = func(context.Context, []string) ([]datasource.IndexQuote, error) {
		t.Fatal("unexpected fetch for fresh cache")
		return nil, nil
	}
	t.Cleanup(func() { fetchIndexFn = datasource.FetchYahooIndexQuotes })

	report, err := NewService(db).GetMarketIndices(context.Background())
	if err != nil {
		t.Fatalf("GetMarketIndices returned error: %v", err)
	}
	if report.Count != 2 ||
		report.DecisionBoundary != "facts_only" ||
		report.SideEffects != "none" ||
		report.ExternalFetch != "not_performed" {
		t.Fatalf("report metadata = %#v", report)
	}
	if report.Indices["^GSPC"].Name != "标普500" ||
		report.Indices["^GSPC"].Price != 5600.5 ||
		report.Indices["^GSPC"].ChangePct != 0.42 {
		t.Fatalf("GSPC row = %#v", report.Indices["^GSPC"])
	}
}

func TestServiceGetMarketIndicesRefreshesStaleCache(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	for _, stmt := range []string{
		`CREATE TABLE indices (
			code TEXT PRIMARY KEY,
			name TEXT,
			market TEXT,
			price REAL,
			change_pct REAL,
			change_amt REAL,
			updated_at TEXT
		)`,
		`INSERT INTO indices (code, name, market, price, change_pct, change_amt, updated_at) VALUES
			('^GSPC', '标普500', 'US', 1000, 0, 0, '2026-07-03 04:33:00')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec fixture: %v", err)
		}
	}

	fetchIndexFn = func(context.Context, []string) ([]datasource.IndexQuote, error) {
		return []datasource.IndexQuote{{
			Code:      "^GSPC",
			Name:      "S&P 500",
			Market:    "US",
			Price:     7500.25,
			ChangePct: 1.1,
			Change:    80,
			UpdatedAt: time.Date(2026, 7, 17, 4, 0, 0, 0, time.UTC),
		}}, nil
	}
	t.Cleanup(func() { fetchIndexFn = datasource.FetchYahooIndexQuotes })

	report, err := NewService(db).GetMarketIndices(context.Background())
	if err != nil {
		t.Fatalf("GetMarketIndices: %v", err)
	}
	if report.ExternalFetch != "yahoo_chart" {
		t.Fatalf("ExternalFetch = %q, want yahoo_chart", report.ExternalFetch)
	}
	if report.Indices["^GSPC"].Price != 7500.25 {
		t.Fatalf("price = %v, want refreshed 7500.25", report.Indices["^GSPC"].Price)
	}
	if report.SideEffects != "indices_cache_upsert" {
		t.Fatalf("SideEffects = %q", report.SideEffects)
	}
}

func TestServiceGetMarketIndicesFallsBackOnFetchError(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	for _, stmt := range []string{
		`CREATE TABLE indices (
			code TEXT PRIMARY KEY,
			name TEXT,
			market TEXT,
			price REAL,
			change_pct REAL,
			change_amt REAL,
			updated_at TEXT
		)`,
		`INSERT INTO indices (code, name, market, price, change_pct, change_amt, updated_at) VALUES
			('^NDX', '纳斯达克100', 'US', 29000, 0.5, 100, '2026-07-03 04:33:00')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec fixture: %v", err)
		}
	}

	fetchIndexFn = func(context.Context, []string) ([]datasource.IndexQuote, error) {
		return nil, errors.New("upstream down")
	}
	t.Cleanup(func() { fetchIndexFn = datasource.FetchYahooIndexQuotes })

	report, err := NewService(db).GetMarketIndices(context.Background())
	if err != nil {
		t.Fatalf("GetMarketIndices: %v", err)
	}
	if report.ExternalFetch != "yahoo_chart_failed_using_cache" {
		t.Fatalf("ExternalFetch = %q", report.ExternalFetch)
	}
	if report.Indices["^NDX"].Price != 29000 {
		t.Fatalf("should keep cache price, got %#v", report.Indices["^NDX"])
	}
}

func TestServiceGetMarketIndicesHandlesMissingTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	report, err := NewService(db).GetMarketIndices(context.Background())
	if err != nil {
		t.Fatalf("GetMarketIndices returned error: %v", err)
	}
	if report.Count != 0 || report.Error != "no_data" {
		t.Fatalf("report = %#v, want no_data with missing table", report)
	}
}

func TestServiceGetMarketIndicesClampsAndLimits(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	indicesNowFn = func() time.Time { return now }
	t.Cleanup(func() { indicesNowFn = time.Now })
	fresh := now.Add(-10 * time.Minute).Format("2006-01-02 15:04:05")
	longName := strings.Repeat("N", 200)
	if _, err := db.ExecContext(context.Background(), `CREATE TABLE indices (
		code TEXT PRIMARY KEY, name TEXT, market TEXT, price REAL, change_pct REAL, change_amt REAL, updated_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO indices (code, name, market, price, change_pct, change_amt, updated_at) VALUES (?, ?, ?, 1, 0, 0, ?)`,
		"^GSPC", longName, "US", fresh,
	); err != nil {
		t.Fatal(err)
	}
	fetchIndexFn = func(context.Context, []string) ([]datasource.IndexQuote, error) {
		t.Fatal("unexpected fetch")
		return nil, nil
	}
	t.Cleanup(func() { fetchIndexFn = datasource.FetchYahooIndexQuotes })

	report, err := NewService(db).GetMarketIndices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := report.Indices["^GSPC"].Name
	if len([]rune(got)) != 120 {
		t.Fatalf("name len=%d want 120", len([]rune(got)))
	}
}

func TestServiceGetMarketIndicesSanitizesRefreshError(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), `CREATE TABLE indices (
		code TEXT PRIMARY KEY, name TEXT, market TEXT, price REAL, change_pct REAL, change_amt REAL, updated_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	fetchIndexFn = func(context.Context, []string) ([]datasource.IndexQuote, error) {
		return nil, errors.New("pq: dial tcp 192.0.2.1:5432: connect: connection refused secret=abc")
	}
	t.Cleanup(func() { fetchIndexFn = datasource.FetchYahooIndexQuotes })

	report, err := NewService(db).GetMarketIndices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Message != "upstream_unavailable" {
		t.Fatalf("Message=%q want upstream_unavailable", report.Message)
	}
	if strings.Contains(report.Message, "pq:") || strings.Contains(report.Message, "secret") {
		t.Fatalf("leaked error: %q", report.Message)
	}
}

func TestServiceGetMarketIndicesCoalescesConcurrentRefresh(t *testing.T) {
	// File-backed DB: pure :memory: is connection-private and multi-conn open races.
	dbPath := t.TempDir() + "/indices.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	for _, stmt := range []string{
		`CREATE TABLE indices (
			code TEXT PRIMARY KEY,
			name TEXT,
			market TEXT,
			price REAL,
			change_pct REAL,
			change_amt REAL,
			updated_at TEXT
		)`,
		`INSERT INTO indices (code, name, market, price, change_pct, change_amt, updated_at) VALUES
			('^GSPC', '标普500', 'US', 1000, 0, 0, '2026-07-03 04:33:00')`,
	} {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("exec fixture: %v", err)
		}
	}

	var calls atomic.Int32
	fetchIndexFn = func(ctx context.Context, _ []string) ([]datasource.IndexQuote, error) {
		calls.Add(1)
		time.Sleep(150 * time.Millisecond)
		return []datasource.IndexQuote{{
			Code:      "^GSPC",
			Name:      "S&P 500",
			Market:    "US",
			Price:     7500.25,
			ChangePct: 1.1,
			Change:    80,
			UpdatedAt: time.Date(2026, 7, 17, 4, 0, 0, 0, time.UTC),
		}}, nil
	}
	t.Cleanup(func() { fetchIndexFn = datasource.FetchYahooIndexQuotes })

	svc := NewService(db)
	errCh := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			_, err := svc.GetMarketIndices(context.Background())
			errCh <- err
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("GetMarketIndices: %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1 (singleflight)", calls.Load())
	}
}
