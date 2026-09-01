package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/dialect"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
)

// migrateTable 是核心路径；SQLite 作目标验证交集/序/幂等逻辑
// （真正目标为 PG，见生产部署 POSTCHECK；此处 pin 行为不变）。
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

	if err := migrateTable(ctx, src, dst, dialect.NameSQLite, "t1", false); err != nil {
		t.Fatalf("migrate t1: %v", err)
	}
	if err := migrateTable(ctx, src, dst, dialect.NameSQLite, "t2", false); err != nil {
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

// 非 force 导入不清空目标；重复运行靠 ON CONFLICT DO NOTHING 保持幂等。
func TestMigrateTableIdempotentCanRerun(t *testing.T) {
	ctx := context.Background()
	src, dst := openTestDBs(t)
	defer src.Close()
	defer dst.Close()

	for i := 0; i < 2; i++ {
		if err := migrateTable(ctx, src, dst, dialect.NameSQLite, "t1", false); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	var rows int
	_ = dst.QueryRowContext(ctx, `SELECT COUNT(*) FROM t1`).Scan(&rows)
	if rows != 2 {
		t.Fatalf("idempotent rerun rows = %d, want 2", rows)
	}
}

// 回归：目标缺表必须按「无此表跳过」处理，而不是落入错误的分流分支报错。
func TestMigrateTableMissingTargetTableSkipsWithoutError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	src, err := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: filepath.Join(dir, "src.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: filepath.Join(dir, "dst.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	if _, err := src.ExecContext(ctx, `CREATE TABLE t1 (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := src.ExecContext(ctx, `INSERT INTO t1 (id, name) VALUES (1, 'a')`); err != nil {
		t.Fatal(err)
	}
	// dst 完全不建 t1：迁移应静默跳过而非报「no such table: information_schema.columns」。
	if err := migrateTable(ctx, src, dst, dialect.NameSQLite, "t1", false); err != nil {
		t.Fatalf("missing target table should skip, got: %v", err)
	}
	var n int
	if err := dst.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='t1'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("target t1 unexpectedly created (n=%d)", n)
	}
}

// --force 覆盖路径：先清空再导入，结果应为源表的完整镜像。
func TestMigrateTableReplaceOverwritesTarget(t *testing.T) {
	ctx := context.Background()
	src, dst := openTestDBs(t)
	defer src.Close()
	defer dst.Close()

	if _, err := dst.ExecContext(ctx, `INSERT INTO t1 (id, name) VALUES (99, 'old')`); err != nil {
		t.Fatal(err)
	}
	if err := migrateTable(ctx, src, dst, dialect.NameSQLite, "t1", true); err != nil {
		t.Fatalf("replace migrate: %v", err)
	}

	var rows, old int
	if err := dst.QueryRowContext(ctx, `SELECT COUNT(*) FROM t1`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("after replace rows = %d, want 2", rows)
	}
	if err := dst.QueryRowContext(ctx, `SELECT COUNT(*) FROM t1 WHERE id = 99`).Scan(&old); err != nil {
		t.Fatal(err)
	}
	if old != 0 {
		t.Fatalf("stale target row survived replace (id=99 count=%d)", old)
	}
}

// 回归：--force 的 DELETE 与导入同事务；插入失败回滚后目标保留迁移前数据。
func TestMigrateTableReplaceRollsBackOnInsertError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	src, err := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: filepath.Join(dir, "src.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: filepath.Join(dir, "dst.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	for _, stmt := range []string{
		`CREATE TABLE t1 (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO t1 (id, name) VALUES (1, 'a'), (2, 'bad')`,
	} {
		if _, err := src.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed source: %v", err)
		}
	}
	for _, stmt := range []string{
		`CREATE TABLE t1 (id INTEGER PRIMARY KEY, name TEXT CHECK (name <> 'bad'))`,
		`INSERT INTO t1 (id, name) VALUES (99, 'keep')`,
	} {
		if _, err := dst.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed target: %v", err)
		}
	}

	if err := migrateTable(ctx, src, dst, dialect.NameSQLite, "t1", true); err == nil {
		t.Fatal("want insert error (CHECK violation)")
	}

	// DELETE 在事务内 → 回滚必须恢复迁移前的行，而不是留下空表/半表。
	var rows int
	if err := dst.QueryRowContext(ctx, `SELECT COUNT(*) FROM t1`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("rollback rows = %d, want 1 (original 'keep' row preserved)", rows)
	}
	var name string
	if err := dst.QueryRowContext(ctx, `SELECT name FROM t1 WHERE id = 99`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "keep" {
		t.Fatalf("original row name = %q, want keep", name)
	}
}

func TestNonEmptyTargetTables(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dst, err := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: filepath.Join(dir, "dst.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	for _, stmt := range []string{
		`CREATE TABLE t_empty (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE t_full (id INTEGER PRIMARY KEY, name TEXT)`,
		`INSERT INTO t_full (id, name) VALUES (1, 'x')`,
	} {
		if _, err := dst.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}

	// t_missing 目标无此表：预检必须跳过而不是报错。
	nonEmpty, err := nonEmptyTargetTables(ctx, dst, dialect.NameSQLite, []string{"t_empty", "t_full", "t_missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(nonEmpty) != 1 || nonEmpty[0] != "t_full" {
		t.Fatalf("nonEmpty = %v, want [t_full]", nonEmpty)
	}

	empty, err := nonEmptyTargetTables(ctx, dst, dialect.NameSQLite, []string{"t_empty", "t_missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty preflight = %v, want []", empty)
	}
}

func TestCoerceForTargetBooleanMapping(t *testing.T) {
	tests := []struct {
		name    string
		driver  string
		colType string
		in      any
		want    any
		wantErr bool
	}{
		{"int zero", dialect.NamePostgres, "boolean", int64(0), false, false},
		{"int one", dialect.NamePostgres, "boolean", int64(1), true, false},
		{"bool passthrough", dialect.NamePostgres, "boolean", true, true, false},
		{"nil passthrough", dialect.NamePostgres, "boolean", nil, nil, false},
		{"bad integer", dialect.NamePostgres, "boolean", int64(2), nil, true},
		{"bad type", dialect.NamePostgres, "boolean", "yes", nil, true},
		{"non-boolean passthrough", dialect.NamePostgres, "bigint", int64(7), int64(7), false},
		{"sqlite passthrough", dialect.NameSQLite, "boolean", int64(1), int64(1), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := coerceForTarget(tc.driver, tc.colType, tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("coerceForTarget(%q, %q, %v) = %v, want error", tc.driver, tc.colType, tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("coerceForTarget(%q, %q, %v) error: %v", tc.driver, tc.colType, tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("coerceForTarget(%q, %q, %v) = %v (%T), want %v (%T)",
					tc.driver, tc.colType, tc.in, got, got, tc.want, tc.want)
			}
		})
	}
}

func TestTargetColumnsUnknownDriverErrors(t *testing.T) {
	ctx := context.Background()
	src, dst := openTestDBs(t)
	defer src.Close()
	defer dst.Close()

	if _, err := targetColumns(ctx, dst, "mysql", "t1"); err == nil {
		t.Fatal("want error for unsupported target driver")
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

func TestMigrateTableQuotesReservedAndQuotedIdentifiers(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	src, err := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: filepath.Join(dir, "src.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: filepath.Join(dir, "dst.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	for _, stmt := range []string{
		`CREATE TABLE "order" (id INTEGER PRIMARY KEY, "group" TEXT)`,
		`INSERT INTO "order" (id, "group") VALUES (1, 'a'), (2, 'b')`,
	} {
		if _, err := src.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed source: %v", err)
		}
	}
	if _, err := dst.ExecContext(ctx, `CREATE TABLE "order" (id INTEGER PRIMARY KEY, "group" TEXT)`); err != nil {
		t.Fatal(err)
	}

	if err := migrateTable(ctx, src, dst, dialect.NameSQLite, "order", false); err != nil {
		t.Fatalf("migrate reserved-identifier table: %v", err)
	}
	var n int
	if err := dst.QueryRowContext(ctx, `SELECT COUNT(*) FROM "order"`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rows = %d, want 2", n)
	}
}
