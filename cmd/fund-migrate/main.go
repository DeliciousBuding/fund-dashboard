// fund-migrate — 一次性工具：把 SQLite 业务库全表导入 PostgreSQL。
//
// 用法（在服务器上一次性运行；目标库由本工具先建 schema 再导入）：
//
//	fund-migrate --sqlite /path/fund.db --dsn "$FUND_PG_DSN"
//
// 语义：
//   - 目标库必须为空（全新部署）；幂等（重复运行不炸，INSERT 冲突按 DO NOTHING）。
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
	flag.Parse()
	if *sqlitePath == "" || *dsn == "" {
		log.Fatal("usage: fund-migrate --sqlite /path/fund.db --dsn postgres://...")
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

	log.Printf("迁移开始：%d 张业务表（排除 auth/agent 与系统表）", len(tables))
	for _, table := range tables {
		if err := migrateTable(ctx, src, dst, table); err != nil {
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

func migrateTable(ctx context.Context, src, dst *sql.DB, table string) error {
	srcCols, err := sqliteColumns(ctx, src, table)
	if err != nil {
		return fmt.Errorf("sqlite columns: %w", err)
	}
	dstCols, err := pgColumns(ctx, dst, table)
	if err != nil {
		return fmt.Errorf("pg columns: %w", err)
	}
	if len(dstCols) == 0 {
		log.Printf("  %-28s 跳过（PG 无此表）", table)
		return nil
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

	if _, err := dst.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s`, dialect.QuoteIdentifier(table))); err != nil {
		return fmt.Errorf("clear target before import: %w", err)
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

func pgColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	out := map[string]bool{}
	// SQLite 目标（行为 pin 测试）走 PRAGMA；生产目标固定走 PG information_schema。
	if rows, err := db.QueryContext(ctx,
		fmt.Sprintf(`PRAGMA table_info(%s)`, dialect.QuoteIdentifier(table))); err == nil {
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				return nil, err
			}
			out[name] = true
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if len(out) > 0 {
			return out, nil
		}
	}
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
