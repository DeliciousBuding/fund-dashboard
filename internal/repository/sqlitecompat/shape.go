package sqlitecompat

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

func inspectTableShapes(ctx context.Context, db *sql.DB, tables []string) (map[string][]string, map[string][]string, error) {
	tableColumns := make(map[string][]string, len(tables))
	primaryKeys := make(map[string][]string, len(tables))
	for _, table := range tables {
		rows, err := db.QueryContext(ctx, "PRAGMA table_info("+quoteIdentifier(table)+")")
		if err != nil {
			return nil, nil, fmt.Errorf("table info %s: %w", table, err)
		}

		var columns []string
		var pkColumns []pkColumn
		for rows.Next() {
			var cid int
			var name string
			var columnType string
			var notNull int
			var defaultValue any
			var pk int
			if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
				rows.Close()
				return nil, nil, fmt.Errorf("scan table info %s: %w", table, err)
			}
			columns = append(columns, name)
			if pk > 0 {
				pkColumns = append(pkColumns, pkColumn{Name: name, Position: pk})
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("table info rows %s: %w", table, err)
		}
		rows.Close()

		sort.Strings(columns)
		tableColumns[table] = columns
		sort.Slice(pkColumns, func(i, j int) bool {
			return pkColumns[i].Position < pkColumns[j].Position
		})
		for _, column := range pkColumns {
			primaryKeys[table] = append(primaryKeys[table], column.Name)
		}
	}
	return tableColumns, primaryKeys, nil
}

type pkColumn struct {
	Name     string
	Position int
}

func listIndexes(ctx context.Context, db *sql.DB, tables []string) ([]string, error) {
	indexSet := map[string]struct{}{}
	for _, table := range tables {
		rows, err := db.QueryContext(ctx, "PRAGMA index_list("+quoteIdentifier(table)+")")
		if err != nil {
			return nil, fmt.Errorf("index list %s: %w", table, err)
		}
		for rows.Next() {
			var seq int
			var name string
			var unique int
			var origin string
			var partial int
			if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan index list %s: %w", table, err)
			}
			if !strings.HasPrefix(name, "sqlite_autoindex_") {
				indexSet[name] = struct{}{}
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("index list rows %s: %w", table, err)
		}
		rows.Close()
	}

	indexes := make([]string, 0, len(indexSet))
	for index := range indexSet {
		indexes = append(indexes, index)
	}
	sort.Strings(indexes)
	return indexes, nil
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
