package portfolio

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
	_ "modernc.org/sqlite"
)

func TestSchemaMetaCache_TableExistsSecondCallUsesCache(t *testing.T) {
	db, err := sql.Open("sqlite", "file:schema-exists-cache?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE stock_profile (code TEXT PRIMARY KEY)`); err != nil {
		db.Close()
		t.Fatalf("create table: %v", err)
	}

	svc := NewService(db)
	ctx := context.Background()

	ok, err := svc.tableExists(ctx, "stock_profile")
	if err != nil {
		t.Fatalf("first tableExists: %v", err)
	}
	if !ok {
		t.Fatalf("stock_profile should exist")
	}
	missing, err := svc.tableExists(ctx, "no_such_table_xyz")
	if err != nil {
		t.Fatalf("first missing tableExists: %v", err)
	}
	if missing {
		t.Fatalf("no_such_table_xyz should not exist")
	}

	// Drop live connection: second probe would fail without process-local cache.
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	ok2, err := svc.tableExists(ctx, "stock_profile")
	if err != nil {
		t.Fatalf("cached tableExists after close: %v", err)
	}
	if !ok2 {
		t.Fatalf("cached stock_profile should still be true")
	}
	missing2, err := svc.tableExists(ctx, "no_such_table_xyz")
	if err != nil {
		t.Fatalf("cached missing tableExists after close: %v", err)
	}
	if missing2 {
		t.Fatalf("cached missing should still be false")
	}
}

func TestSchemaMetaCache_TableColumnsSecondCallUsesCache(t *testing.T) {
	db, err := sql.Open("sqlite", "file:schema-cols-cache?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE stock_realtime (
		code TEXT PRIMARY KEY,
		name TEXT,
		price REAL,
		market TEXT
	)`); err != nil {
		db.Close()
		t.Fatalf("create table: %v", err)
	}

	svc := NewService(db)
	ctx := context.Background()

	cols, err := svc.tableColumns(ctx, "stock_realtime")
	if err != nil {
		t.Fatalf("first tableColumns: %v", err)
	}
	for _, want := range []string{"code", "name", "price", "market"} {
		if _, ok := cols[want]; !ok {
			t.Fatalf("missing column %q in %#v", want, cols)
		}
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	cols2, err := svc.tableColumns(ctx, "stock_realtime")
	if err != nil {
		t.Fatalf("cached tableColumns after close: %v", err)
	}
	if len(cols2) != len(cols) {
		t.Fatalf("cached columns len=%d want %d", len(cols2), len(cols))
	}
	if _, ok := cols2["market"]; !ok {
		t.Fatalf("cached columns missing market: %#v", cols2)
	}
}

func TestSchemaMetaCache_SharedAcrossValueCopy(t *testing.T) {
	db, err := sql.Open("sqlite", "file:schema-share-cache?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE indices (code TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	svc := NewService(db)
	ctx := context.Background()
	if _, err := svc.tableExists(ctx, "indices"); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	// Value-copied receivers (method set) must share the same schema pointer.
	copy := svc
	if copy.schema == nil || copy.schema != svc.schema {
		t.Fatalf("schema cache not shared on value copy")
	}
	if _, ok := copy.schema.exists["indices"]; !ok {
		t.Fatalf("copy did not see cached exists entry")
	}
}

func TestProbeTableColumnsMissingTableOnSQLiteReturnsEmpty(t *testing.T) {
	// PRAGMA table_info on a missing SQLite table succeeds with 0 rows; the
	// probe must return an empty set (table absent) instead of falling through
	// to PostgreSQL's information_schema and 500-ing on SQLite.
	db, err := sql.Open("sqlite", "file:probe-missing-cols?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	svc := NewService(db)
	cols, err := svc.probeTableColumns(context.Background(), "no_such_table_xyz")
	if err != nil {
		t.Fatalf("probe missing table columns: %v (want empty set, no error)", err)
	}
	if len(cols) != 0 {
		t.Fatalf("cols = %#v, want empty set", cols)
	}
}

func TestPreparedKlineUpsert_PerCallPrepareCloseAndWrite(t *testing.T) {
	db, err := sql.Open("sqlite", "file:schema-kline-prep?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE stock_kline_cache (
		code TEXT,
		market TEXT,
		date TEXT,
		close REAL,
		change_pct REAL,
		PRIMARY KEY (code, market, date)
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	svc := NewService(db)
	ctx := context.Background()
	kind1, stmt1, err := svc.preparedKlineUpsert(ctx)
	if err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	if kind1 != klineUpsertMarket || stmt1 == nil {
		t.Fatalf("kind=%v stmt=%v", kind1, stmt1)
	}
	// The statement is owned by the caller: closing the first one must not
	// affect a later prepare, because nothing is cached process-lifetime.
	if err := stmt1.Close(); err != nil {
		t.Fatalf("close first stmt: %v", err)
	}
	kind2, stmt2, err := svc.preparedKlineUpsert(ctx)
	if err != nil {
		t.Fatalf("second prepare after first close: %v", err)
	}
	defer stmt2.Close()
	if kind2 != klineUpsertMarket || stmt2 == nil {
		t.Fatalf("second prepare kind=%v stmt=%v", kind2, stmt2)
	}
	if stmt1 == stmt2 {
		t.Fatalf("expected per-call statements to be independent")
	}

	n, err := svc.upsertUSStockHistory(ctx, datasource.StockSnapshot{
		Symbol: "MSFT",
		History: []datasource.IndexHistoryPoint{
			{Date: "2026-07-01", Close: 400, ChangePct: 1},
			{Date: "2026-07-02", Close: 404, ChangePct: 1},
		},
	})
	if err != nil {
		t.Fatalf("upsert history: %v", err)
	}
	if n != 2 {
		t.Fatalf("upsert n=%d want 2", n)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM stock_kline_cache WHERE code = 'MSFT'`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("row count=%d want 2", count)
	}
}
