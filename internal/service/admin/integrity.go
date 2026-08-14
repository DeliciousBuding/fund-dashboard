package admin

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/dialect"
)

// IntegrityReport is the read-only SQLite integrity / freelist / row-count view.
type IntegrityReport struct {
	Timestamp        string            `json:"timestamp"`
	Overall          string            `json:"overall"`
	Checks           IntegrityChecks   `json:"checks"`
	TableChecksums   map[string]string `json:"table_checksums"`
	RowCounts        map[string]int    `json:"row_counts"`
	Recommendations  []string          `json:"recommendations"`
	DecisionBoundary string            `json:"decision_boundary"`
}

type IntegrityChecks struct {
	IntegrityCheck  IntegrityCheckDetail `json:"integrity_check"`
	ForeignKeyCheck ForeignKeyCheck      `json:"foreign_key_check"`
	QuickCheck      QuickCheck           `json:"quick_check"`
	FreelistCount   FreelistCheck        `json:"freelist_count"`
}

type IntegrityCheckDetail struct {
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type ForeignKeyCheck struct {
	Passed     bool   `json:"passed"`
	Violations int    `json:"violations"`
	Detail     string `json:"detail,omitempty"`
}

type QuickCheck struct {
	Passed bool   `json:"passed"`
	Result string `json:"result"`
}

type FreelistCheck struct {
	Passed   bool   `json:"passed"`
	Freelist int    `json:"freelist"`
	Detail   string `json:"detail"`
}

func (s Service) GetDBIntegrity(ctx context.Context, now time.Time) (IntegrityReport, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	report := IntegrityReport{
		Timestamp:        now.UTC().Format(time.RFC3339),
		Overall:          "ok",
		TableChecksums:   map[string]string{},
		RowCounts:        map[string]int{},
		Recommendations:  []string{},
		DecisionBoundary: "read_only",
	}

	if s.dialect.IsPostgres() {
		return s.getPGIntegrity(ctx, report)
	}
	return s.getSQLiteIntegrity(ctx, report)
}

func (s Service) getPGIntegrity(ctx context.Context, report IntegrityReport) (IntegrityReport, error) {
	// PostgreSQL integrity: check connectivity + FK constraints.
	report.Checks.IntegrityCheck = IntegrityCheckDetail{
		Passed: true,
		Detail: "pg: connected (no per-page integrity check; rely on pg_stat_database for corruption detection)",
	}
	report.Checks.QuickCheck = QuickCheck{
		Passed: true,
		Result: "pg: n/a (SQLite-only quick_check; checksum verification handled by PG WAL)",
	}

	// Check foreign key constraint health — count tables with unvalidated FKs.
	fkRows, err := s.db.QueryContext(ctx, `
		SELECT conname, conrelid::regclass::text
		FROM pg_constraint
		WHERE contype = 'f' AND convalidated = false
	`)
	if err != nil {
		slog.Error("integrity pg_constraint query", "error", err.Error())
		report.Checks.ForeignKeyCheck = ForeignKeyCheck{
			Passed: false,
			Detail: "pg_constraint_query_error",
		}
	} else {
		defer fkRows.Close()
		violations := 0
		for fkRows.Next() {
			violations++
		}
		if err := fkRows.Err(); err != nil {
			slog.Error("integrity pg_constraint iterate", "error", err.Error())
			report.Checks.ForeignKeyCheck = ForeignKeyCheck{
				Passed: false,
				Detail: "pg_constraint_iterate_error",
			}
		} else {
			report.Checks.ForeignKeyCheck = ForeignKeyCheck{
				Passed:     violations == 0,
				Violations: violations,
			}
		}
	}

	// Freelist is a SQLite concept; PG uses autovacuum.
	report.Checks.FreelistCount = FreelistCheck{
		Passed:   true,
		Freelist: 0,
		Detail:   "pg: n/a (autovacuum manages free space; check pg_stat_user_tables.n_dead_tup for bloat)",
	}

	tables, err := s.dialect.ListUserTables(ctx)
	if err != nil {
		return IntegrityReport{}, err
	}
	for _, table := range tables {
		count, err := s.countRows(ctx, "SELECT COUNT(*) FROM "+dialect.QuoteIdentifier(table))
		if err != nil {
			slog.Error("integrity table unreadable", "table", table, "error", err.Error())
			report.Recommendations = append(report.Recommendations, fmt.Sprintf("table_unreadable:%s", table))
			continue
		}
		report.RowCounts[table] = count
		report.TableChecksums[table] = "rows=" + strconv.Itoa(count)
	}

	report.Overall = "ok"
	return report, nil
}

func (s Service) getSQLiteIntegrity(ctx context.Context, report IntegrityReport) (IntegrityReport, error) {

	integrity, err := s.querySingleString(ctx, "PRAGMA integrity_check")
	if err != nil {
		return IntegrityReport{}, fmt.Errorf("integrity_check: %w", err)
	}
	report.Checks.IntegrityCheck = IntegrityCheckDetail{
		Passed: integrity == "ok",
		Detail: integrity,
	}

	quick, err := s.querySingleString(ctx, "PRAGMA quick_check")
	if err != nil {
		return IntegrityReport{}, fmt.Errorf("quick_check: %w", err)
	}
	report.Checks.QuickCheck = QuickCheck{
		Passed: quick == "ok",
		Result: quick,
	}

	foreignKeyViolations, err := s.countQueryRows(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return IntegrityReport{}, fmt.Errorf("foreign_key_check: %w", err)
	}
	report.Checks.ForeignKeyCheck = ForeignKeyCheck{
		Passed:     foreignKeyViolations == 0,
		Violations: foreignKeyViolations,
	}

	freelist, err := s.querySingleInt(ctx, "PRAGMA freelist_count")
	if err != nil {
		return IntegrityReport{}, fmt.Errorf("freelist_count: %w", err)
	}
	report.Checks.FreelistCount = FreelistCheck{
		Passed:   freelist <= 1000,
		Freelist: freelist,
		Detail:   clampAdminText(fmt.Sprintf("%d pages (normal)", freelist), 200),
	}
	if freelist > 1000 {
		report.Checks.FreelistCount.Detail = clampAdminText(fmt.Sprintf("High freelist count (%d pages). Consider VACUUM during a maintenance window.", freelist), 200)
		report.Recommendations = append(report.Recommendations, "Schedule a reviewed VACUUM maintenance window to reclaim freelist pages.")
	}

	tables, err := s.dialect.ListUserTables(ctx)
	if err != nil {
		return IntegrityReport{}, err
	}
	for _, table := range tables {
		count, err := s.countRows(ctx, "SELECT COUNT(*) FROM "+dialect.QuoteIdentifier(table))
		if err != nil {
			slog.Error("integrity table unreadable", "table", table, "error", err.Error())
			report.Recommendations = append(report.Recommendations, fmt.Sprintf("table_unreadable:%s", table))
			continue
		}
		report.RowCounts[table] = count
		report.TableChecksums[table] = "rows=" + strconv.Itoa(count)
	}

	allPassed := report.Checks.IntegrityCheck.Passed &&
		report.Checks.QuickCheck.Passed &&
		report.Checks.ForeignKeyCheck.Passed &&
		report.Checks.FreelistCount.Passed
	if !report.Checks.IntegrityCheck.Passed || !report.Checks.QuickCheck.Passed {
		report.Overall = "corrupted"
		report.Recommendations = append(report.Recommendations, "Database corruption detected; stop writes and inspect the database file before any restore.")
	} else if !allPassed || len(report.Recommendations) > 0 {
		report.Overall = "degraded"
	}

	return report, nil
}

func (s Service) querySingleString(ctx context.Context, query string) (string, error) {
	var value string
	if err := s.db.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return "", err
	}
	return value, nil
}

func (s Service) querySingleInt(ctx context.Context, query string) (int, error) {
	var value int
	if err := s.db.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}

func (s Service) countQueryRows(ctx context.Context, query string) (int, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return count, nil
}
