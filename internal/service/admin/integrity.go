package admin

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/dialect"
	"github.com/DeliciousBuding/fund-dashboard/internal/snapshot"
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

// maxShareDriftFindings caps how many per-fund drift entries reconcileShareDrift
// appends to report.Recommendations (and warns about) in a single integrity
// run. A personal ledger should never approach this; the cap keeps a
// pathological database from ballooning the read-only report payload.
const maxShareDriftFindings = 20

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
		// NOTE: `violations` here counts unvalidated FK constraints, not actual
		// violating rows (SQLite PRAGMA foreign_key_check reports real rows). The
		// JSON field name is shared with the SQLite branch for API compatibility.
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

	s.reconcileShareDrift(ctx, &report)

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

	s.reconcileShareDrift(ctx, &report)

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

// reconcileShareDrift cross-checks the share ledger against the position
// snapshots, fund by fund: SUM(signed_share_change) from transactions versus
// SUM(held_shares) from portfolio_snapshot (summed across portfolio rows —
// transactions are fund-wide while snapshots are per-portfolio).
//
// Both sides settle to snapshot.RoundShares, the exact 4dp basis Recalc
// persists with, so the comparison can never disagree with the writer about
// what the ledger says. A drift larger than snapshot.HeldSharesDust means the
// stored position no longer matches the trade history: findings are appended
// to report.Recommendations (existing []string field — the response JSON
// shape is unchanged) and warned to the server log.
//
// Best-effort by design: when either table is absent (fresh/legacy database)
// the check is skipped silently, and a query error is logged without failing
// the read-only report — an instrumentation gap must not be reported as data
// corruption. On SQLite, appended recommendations flip Overall to "degraded"
// through the existing len(Recommendations) rule; the PG branch keeps its
// historical "ok" Overall for recommendation-only findings (same asymmetry as
// table_unreadable entries).
func (s Service) reconcileShareDrift(ctx context.Context, report *IntegrityReport) {
	// RowCounts was populated by the caller's table enumeration; when either
	// side of the reconciliation is missing there is nothing to compare.
	if _, ok := report.RowCounts["transactions"]; !ok {
		return
	}
	if _, ok := report.RowCounts["portfolio_snapshot"]; !ok {
		return
	}

	ledger, err := s.shareSumsByFund(ctx, `
		SELECT fund_code, SUM(COALESCE(signed_share_change, 0))
		FROM transactions
		WHERE fund_code IS NOT NULL AND fund_code <> ''
		GROUP BY fund_code
	`)
	if err != nil {
		slog.Warn("integrity share drift ledger query", "error", err.Error())
		return
	}
	snapshots, err := s.shareSumsByFund(ctx, `
		SELECT fund_code, SUM(COALESCE(held_shares, 0))
		FROM portfolio_snapshot
		WHERE fund_code IS NOT NULL AND fund_code <> ''
		GROUP BY fund_code
	`)
	if err != nil {
		slog.Warn("integrity share drift snapshot query", "error", err.Error())
		return
	}

	appended := 0
	totalDrifts := 0
	for _, code := range sortedFundCodes(ledger, snapshots) {
		ledgerShares := snapshot.RoundShares(ledger[code])
		snapShares := snapshot.RoundShares(snapshots[code])
		drift := ledgerShares - snapShares
		if math.Abs(drift) <= snapshot.HeldSharesDust {
			continue
		}
		totalDrifts++
		slog.Warn("integrity share drift detected",
			"fund_code", code,
			"ledger_shares", ledgerShares,
			"snapshot_shares", snapShares,
			"drift", drift)
		if appended < maxShareDriftFindings {
			report.Recommendations = append(report.Recommendations,
				fmt.Sprintf("share_drift:%s ledger_shares=%.4f snapshot_shares=%.4f drift=%.4f",
					clampAdminText(code, 32), ledgerShares, snapShares, drift))
			appended++
		}
	}
	if totalDrifts > appended {
		report.Recommendations = append(report.Recommendations,
			fmt.Sprintf("share_drift_truncated:%d_more_funds_beyond_cap", totalDrifts-appended))
	}
}

// shareSumsByFund runs one grouped share-sum query into a fund->shares map.
func (s Service) shareSumsByFund(ctx context.Context, query string) (map[string]float64, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sums := map[string]float64{}
	for rows.Next() {
		var code string
		var shares float64
		if err := rows.Scan(&code, &shares); err != nil {
			return nil, err
		}
		sums[code] = shares
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sums, nil
}

// sortedFundCodes returns the union of both maps' keys in stable order so the
// report payload and log output are deterministic across runs.
func sortedFundCodes(a, b map[string]float64) []string {
	seen := map[string]bool{}
	codes := []string{}
	for code := range a {
		if !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}
	for code := range b {
		if !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)
	return codes
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
