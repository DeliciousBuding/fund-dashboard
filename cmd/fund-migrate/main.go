// fund-migrate — 一次性工具：把 SQLite 业务库全表导入 PostgreSQL。
//
// 用法（在服务器上一次性运行；目标库由本工具先建 schema 再导入）：
//
//	fund-migrate --sqlite /path/fund.db --dsn "$FUND_PG_DSN"
//	fund-migrate --sqlite /path/fund.db --dsn "$FUND_PG_DSN" --force
//
// 语义：
//   - 目标库必须为空（全新部署）。启动时预检所有待迁移表：任一目标表非空即报错
//     退出，且不会清空/写入任何数据；显式传 --force 才允许覆盖非空目标。
//   - --force 下每表在一个事务内先 DELETE 再导入（原子替换）：源读取或插入任一步
//     失败即回滚，目标表保留迁移前的数据，不会出现「清空到一半」的状态。
//   - 非 force 导入不清空目标；INSERT 冲突按 DO NOTHING 跳过，保留表级幂等语义。
//   - 表清单取自 SQLite；只迁移业务表（排除 sqlite_* 系统表、auth_* / agent_*
//     会话与事件——新部署从头建鉴权面）。
//   - 按列交集导入（源列 ∩ 目标列），未知列跳过，缺列补默认值，避免 legacy 与
//     PG DDL 的形状差异阻塞上线。
//   - 每表一个事务；结束时打印每表行数 + 源/目标对账。
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/dialect"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
)

// 迁移目标：业务表全集（schema_pg.go 有对应 DDL；源库缺的表自动跳过）。
// auth_* / agent_* 为新鉴权面自建，不迁移。
var excludedTables = map[string]bool{
	"auth_credentials": true, "auth_sessions": true, "auth_events": true,
	"agent_audit_events": true, "agent_confirmations": true,
}

func main() {
	sqlitePath := flag.String("sqlite", "", "path to source SQLite database (required)")
	dsn := flag.String("dsn", "", "postgres DSN (required)")
	force := flag.Bool("force", false, "allow migrating into a non-empty target (each table is atomically replaced)")
	flag.Parse()
	if *sqlitePath == "" || *dsn == "" {
		log.Fatal("usage: fund-migrate --sqlite /path/fund.db --dsn postgres://... [--force]")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// 1. 打开源（只读；WAL/mvcc 一致性由 sqlite 驱动保证）
	src, err := db.Open(ctx, db.Options{Driver: "sqlite", SQLitePath: *sqlitePath, ReadOnly: true})
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer src.Close()

	// 2. 打开目标并建 schema
	dst, err := db.Open(ctx, db.Options{Driver: "pg", DSN: *dsn})
	if err != nil {
		log.Fatalf("open pg: %v", err)
	}
	defer dst.Close()
	if err := db.EnsurePGSchema(ctx, dst); err != nil {
		log.Fatalf("ensure pg schema: %v", err)
	}

	tables, err := srcTables(ctx, src)
	if err != nil {
		log.Fatalf("list sqlite tables: %v", err)
	}

	// 3. 预检目标：非空即拒绝（全新部署语义），避免把已有数据清掉。
	//    目标库固定是 PG；显式 --force 才走原子替换路径。
	nonEmpty, err := nonEmptyTargetTables(ctx, dst, dialect.NamePostgres, tables)
	if err != nil {
		log.Fatalf("preflight target tables: %v", err)
	}
	if len(nonEmpty) > 0 && !*force {
		log.Fatalf("目标库非空（%s）：按全新部署语义拒绝覆盖；确认要重跑请显式传 --force（每表原子替换）",
			strings.Join(nonEmpty, ", "))
	}
	if *force {
		log.Printf("--force：允许覆盖非空目标；每表在事务内先 DELETE 再导入，失败自动回滚")
	}

	log.Printf("迁移开始：%d 张业务表（排除 auth/agent 与系统表）", len(tables))
	for _, table := range tables {
		if err := migrateTable(ctx, src, dst, dialect.NamePostgres, table, *force); err != nil {
			log.Fatalf("迁移 %s 失败: %v", table, err)
		}
	}
	log.Println("迁移完成")
}

// srcTables lists non-internal, non-excluded SQLite tables.
func srcTables(ctx context.Context, src *sql.DB) ([]string, error) {
	rows, err := src.QueryContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if !excludedTables[name] {
			out = append(out, name)
		}
	}
	return out, rows.Err()
}

// nonEmptyTargetTables returns the business tables that exist in the target and
// already contain rows. Tables absent from the target schema are skipped (the
// import loop skips them too); existing tables are probed with LIMIT 1 so huge
// tables are not fully scanned during the preflight.
func nonEmptyTargetTables(ctx context.Context, dst *sql.DB, targetDriver string, tables []string) ([]string, error) {
	var nonEmpty []string
	for _, table := range tables {
		cols, err := targetColumns(ctx, dst, targetDriver, table)
		if err != nil {
			return nil, fmt.Errorf("inspect target %s: %w", table, err)
		}
		if len(cols) == 0 {
			continue
		}
		var one int
		err = dst.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT 1 FROM %s LIMIT 1`, dialect.QuoteIdentifier(table))).Scan(&one)
		switch {
		case err == sql.ErrNoRows:
			// 空表，符合全新部署前提。
		case err != nil:
			return nil, fmt.Errorf("probe target %s: %w", table, err)
		default:
			nonEmpty = append(nonEmpty, table)
		}
	}
	return nonEmpty, nil
}

func migrateTable(ctx context.Context, src, dst *sql.DB, targetDriver, table string, replace bool) error {
	srcCols, err := sqliteColumns(ctx, src, table)
	if err != nil {
		return fmt.Errorf("sqlite columns: %w", err)
	}
	dstCols, err := targetColumns(ctx, dst, targetDriver, table)
	if err != nil {
		return fmt.Errorf("target columns: %w", err)
	}
	if len(dstCols) == 0 {
		log.Printf("  %-28s 跳过（目标无此表）", table)
		return nil
	}
	// PG 目标需要列类型以做类型适配：SQLite 无 BOOLEAN（following 存 0/1），
	// 直接把 int64 发给 PG 会以 int8 绑定而报「column is of type boolean」。
	var pgTypes map[string]string
	if targetDriver == dialect.NamePostgres {
		pgTypes, err = pgColumnTypes(ctx, dst, table)
		if err != nil {
			return fmt.Errorf("target column types: %w", err)
		}
	}

	// 交集（源序）
	var cols []string
	for _, c := range srcCols {
		if dstCols[c] {
			cols = append(cols, c)
		}
	}
	if len(cols) == 0 {
		log.Printf("  %-28s 跳过（无共同列）", table)
		return nil
	}

	// Quote identifiers against catalog names that would otherwise break or
	// inject SQL (reserved words, spaces, quotes in legacy schemas).
	quotedTable := dialect.QuoteIdentifier(table)
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = dialect.QuoteIdentifier(c)
	}
	colList := strings.Join(quotedCols, ", ")
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	insert := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING`, quotedTable, colList, placeholders)

	rows, err := src.QueryContext(ctx, fmt.Sprintf(`SELECT %s FROM %s`, colList, quotedTable))
	if err != nil {
		return fmt.Errorf("select source: %w", err)
	}
	defer rows.Close()

	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// --force 覆盖路径：DELETE 与导入同一事务，源读取/插入失败时回滚，
	// 目标表保留迁移前的数据（不会出现「清空到一半」的中间状态）。
	if replace {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s`, quotedTable)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("clear target before import: %w", err)
		}
	}
	stmt, err := tx.PrepareContext(ctx, insert)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare insert %s: %w", table, err)
	}
	defer stmt.Close()

	vals := make([]any, len(cols))
	dest := make([]any, len(cols))
	for i := range vals {
		dest[i] = &vals[i]
	}
	count := 0
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("scan source row: %w", err)
		}
		args := make([]any, len(cols))
		for i, v := range vals {
			switch typed := v.(type) {
			case int64:
				args[i] = typed
			case float64:
				args[i] = typed
			case string:
				args[i] = typed
			default:
				args[i] = v
			}
			if targetDriver == dialect.NamePostgres && pgTypes != nil {
				coerced, err := coerceForTarget(targetDriver, pgTypes[cols[i]], args[i])
				if err != nil {
					_ = tx.Rollback()
					return fmt.Errorf("coerce %s.%s row %d: %w", table, cols[i], count, err)
				}
				args[i] = coerced
			}
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert row: %w", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("iterate source: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", table, err)
	}

	verify(ctx, dst, table, count)
	log.Printf("  %-28s %6d 行", table, count)
	return nil
}

// verify checks the destination row count matched the imported count.
func verify(ctx context.Context, dst *sql.DB, table string, want int) {
	var got int
	if err := dst.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, dialect.QuoteIdentifier(table))).Scan(&got); err != nil {
		log.Printf("  %-28s 对账失败: %v", table, err)
		return
	}
	if got != want {
		log.Printf("  ⚠ %-28s 对账不符：导入 %d，目标 %d", table, want, got)
	}
}

func sqliteColumns(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		fmt.Sprintf(`SELECT name FROM pragma_table_info('%s')`, strings.ReplaceAll(table, "'", "''")))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

// targetColumns returns the target table's column set, dispatching on the known
// target driver so a missing table yields an empty set (import skips it) instead
// of leaking into the wrong catalog branch and surfacing a bogus error.
func targetColumns(ctx context.Context, db *sql.DB, targetDriver, table string) (map[string]bool, error) {
	switch targetDriver {
	case dialect.NameSQLite:
		cols, err := sqliteColumns(ctx, db, table)
		if err != nil {
			return nil, err
		}
		out := make(map[string]bool, len(cols))
		for _, c := range cols {
			out[c] = true
		}
		return out, nil
	case dialect.NamePostgres:
		return pgColumns(ctx, db, table)
	default:
		return nil, fmt.Errorf("unsupported target driver %q", targetDriver)
	}
}

// coerceForTarget adapts SQLite-scanned values to the PG column type before
// binding. The known mismatch is SQLite INTEGER 0/1 → PG BOOLEAN; everything
// else passes through unchanged.
func coerceForTarget(targetDriver, colType string, v any) (any, error) {
	if targetDriver != dialect.NamePostgres || v == nil || colType != "boolean" {
		return v, nil
	}
	switch n := v.(type) {
	case bool:
		return n, nil
	case int64:
		switch n {
		case 0:
			return false, nil
		case 1:
			return true, nil
		default:
			return nil, fmt.Errorf("boolean column got integer %d", n)
		}
	default:
		return nil, fmt.Errorf("boolean column got unsupported scanned type %T", v)
	}
}

// pgColumnTypes returns column name → data_type for one PG table.
func pgColumnTypes(ctx context.Context, db *sql.DB, table string) (map[string]string, error) {
	out := map[string]string{}
	rows, err := db.QueryContext(ctx, `
		SELECT column_name, data_type FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = ?`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, err
		}
		out[name] = typ
	}
	return out, rows.Err()
}

func pgColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	out := map[string]bool{}
	rows, err := db.QueryContext(ctx, `
		SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = ?`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}
