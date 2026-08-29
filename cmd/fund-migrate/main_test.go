package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
)

// migrateTable 是核心路径；SQLite 作目标验证交集/序/幂等逻辑
// （真正目标为 PG，见 jp1 部署 POSTCHECK；此处 pin 行为不变）。
func openTestDBs(t *testing.T) (src *sql.DB, dst *sql.DB) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.db")
	dstPath := filepath.Join(dir, "dst.db")

	var err error
	src, err = db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: srcPath})
	if err != nil {
		t.Fatal(err)
	}
	dst, err = db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: dstPath})
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE t1 (id INTEGER PRIMARY KEY, name TEXT, extra TEXT)`,
		`INSERT INTO t1 (id, name, extra) VALUES (1, 'a', 'x'), (2, 'b', 'y')`,
		`CREATE TABLE t2 (id INTEGER PRIMARY KEY, val REAL)`,
		`INSERT INTO t2 (id, val) VALUES (1, 1.5)`,
	} {
		if _, err := src.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed source: %v", err)
		}
	}
	// 目标表只含部分列（模拟 PG DDL 交集语义）
	for _, stmt := range []string{
		`CREATE TABLE t1 (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE t2 (id INTEGER PRIMARY KEY, val REAL)`,
	} {
		if _, err := dst.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed target: %v", err)
		}
	}
	return src, dst
}

func TestMigrateTableIntersectsColumnsAndCounts(t *testing.T) {
	ctx := context.Background()
	src, dst := openTestDBs(t)
	defer src.Close()
	defer dst.Close()

	if err := migrateTable(ctx, src, dst, "t1"); err != nil {
		t.Fatalf("migrate t1: %v", err)
	}
	if err := migrateTable(ctx, src, dst, "t2"); err != nil {
		t.Fatalf("migrate t2: %v", err)
	}

	var rows, cols int
	if err := dst.QueryRowContext(ctx, `SELECT COUNT(*) FROM t1`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("t1 rows = %d, want 2", rows)
	}
	q := `SELECT COUNT(*) FROM pragma_table_info('t1')`
	if err := dst.QueryRowContext(ctx, q).Scan(&cols); err != nil {
		t.Fatal(err)
	}
	if cols != 2 {
		t.Fatalf("t1 cols = %d, want 2 (交集: id,name；extra 无目标列被跳过)", cols)
	}
}

func TestMigrateTableIdempotentCanRerun(t *testing.T) {
	ctx := context.Background()
	src, dst := openTestDBs(t)
	defer src.Close()
	defer dst.Close()

	for i := 0; i < 2; i++ {
		if err := migrateTable(ctx, src, dst, "t1"); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	var rows int
	_ = dst.QueryRowContext(ctx, `SELECT COUNT(*) FROM t1`).Scan(&rows)
	if rows != 2 {
		t.Fatalf("idempotent rerun rows = %d, want 2", rows)
	}
}

func TestSrcTablesExcludesAuthAndInternal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	src, err := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: filepath.Join(dir, "x.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	for _, stmt := range []string{
		`CREATE TABLE fund_details (x TEXT)`,
		`CREATE TABLE auth_sessions (x TEXT)`,
		`CREATE TABLE agent_audit_events (x TEXT)`,
	} {
		if _, err := src.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	tables, err := srcTables(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0] != "fund_details" {
		t.Fatalf("tables = %v, want [fund_details]", tables)
	}
}
