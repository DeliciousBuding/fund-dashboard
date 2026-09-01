// Package jobs provides background job orchestration: price refresh,
// DCA materialization, integrity checks, and their scheduling.
package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
	"github.com/DeliciousBuding/fund-dashboard/internal/dialect"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	"github.com/DeliciousBuding/fund-dashboard/internal/snapshot"
)

// PriceRefresher updates nav_history from external price sources.
// It depends on the datasource.PriceSource interface, not on concrete
// implementations — swap eastmoney for yahoo or a test stub freely.
type PriceRefresher struct {
	db            *sql.DB
	driver        string // "sqlite" (default) or "pg" — freshness dialect
	sources       map[datasource.SecurityType]datasource.PriceSource
	navSchemaOnce sync.Once
	navSchemaErr  error
}

// NewPriceRefresher creates a PriceRefresher. At least one source must
// be provided for datasource.TypeFund; TypeStock is optional.
func NewPriceRefresher(db *sql.DB, opts ...PriceRefresherOption) *PriceRefresher {
	r := &PriceRefresher{
		db:      db,
		driver:  "sqlite",
		sources: map[datasource.SecurityType]datasource.PriceSource{},
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// PriceRefresherOption configures a PriceRefresher.
type PriceRefresherOption func(*PriceRefresher)

// WithSource registers a PriceSource for a given security type.
func WithSource(secType datasource.SecurityType, src datasource.PriceSource) PriceRefresherOption {
	return func(r *PriceRefresher) {
		r.sources[secType] = src
	}
}

// WithDBDriver sets the SQL dialect hint for freshness queries ("sqlite" or "pg").
func WithDBDriver(driver string) PriceRefresherOption {
	return func(r *PriceRefresher) {
		if strings.TrimSpace(driver) != "" {
			r.driver = strings.ToLower(strings.TrimSpace(driver))
		}
	}
}

// RefreshResult summarizes a price refresh run.
type RefreshResult struct {
	Securities int    `json:"securities"`
	Added      int    `json:"added"`
	Latest     string `json:"latest"`
}

// RefreshSecurity fetches and persists price history for a single security.
// When local nav_history already has rows, only points on/after the latest date
// are upserted (upstream may still return a full series — we drop the old tail).
func (r *PriceRefresher) RefreshSecurity(ctx context.Context, code string, secType datasource.SecurityType) (*RefreshResult, error) {
	src, ok := r.sources[secType]
	if !ok {
		return nil, fmt.Errorf("no source registered for type %s", secType)
	}

	points, err := src.FetchHistory(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("refresh %s: %w", code, err)
	}
	if len(points) == 0 {
		return &RefreshResult{Securities: 1, Added: 0, Latest: "none"}, nil
	}

	r.ensureNavSchema(ctx)
	if r.navSchemaErr != nil {
		return nil, fmt.Errorf("nav_history schema: %w", r.navSchemaErr)
	}

	// Incremental upsert: keep last known date inclusive so same-day NAV corrections apply.
	if since, ok, err := latestNavDate(ctx, r.db, code); err != nil {
		return nil, err
	} else if ok {
		filtered := filterPointsSince(points, since)
		if len(filtered) == 0 {
			slog.Debug("price refresh up-to-date", "code", code, "latest", since)
			return &RefreshResult{Securities: 1, Added: 0, Latest: since}, nil
		}
		points = filtered
	}

	added, err := upsertNavHistory(ctx, r.db, r.driver, code, string(secType), points)
	if err != nil {
		return nil, err
	}

	latest := points[len(points)-1].Date
	if err := snapshot.Recalc(ctx, r.db, code, snapshot.ModeFull); err != nil {
		return nil, fmt.Errorf("recalc snapshot %s: %w", code, err)
	}
	slog.Info("price refresh", "code", code, "type", secType, "added", added, "latest", latest)
	return &RefreshResult{Securities: 1, Added: added, Latest: latest}, nil
}

// RefreshAllHeld refreshes every security with held_shares > 0 in portfolio_snapshot.
// Prefer RefreshStaleHeld for scheduled jobs; keep this for explicit full crawl.
func (r *PriceRefresher) RefreshAllHeld(ctx context.Context) (securities, totalAdded int, err error) {
	held, err := r.getHeldSecurities(ctx)
	if err != nil {
		return 0, 0, err
	}
	if len(held) == 0 {
		slog.Info("no held securities to refresh")
		return 0, 0, nil
	}

	return r.RefreshCodes(ctx, held)
}

// RefreshStaleHeld refreshes only held securities with missing or stale NAV
// (same selection as admin/MCP stale_only). Fresh holdings are skipped entirely.
func (r *PriceRefresher) RefreshStaleHeld(ctx context.Context) (securities, totalAdded int, err error) {
	svc := adminsvc.NewServiceWithDriver(r.db, r.driver)
	report, err := svc.GetFreshness(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("freshness for stale refresh: %w", err)
	}
	codes := heldRefreshCodes(report)
	if len(codes) == 0 {
		slog.Info("price refresh stale_only: nothing to do")
		return 0, 0, nil
	}
	slog.Info("price refresh stale_only", "codes", len(codes))
	return r.RefreshCodes(ctx, codes)
}

// RefreshCodes refreshes an explicit list of security codes (order preserved).
func (r *PriceRefresher) RefreshCodes(ctx context.Context, codes []string) (securities, totalAdded int, err error) {
	if len(codes) == 0 {
		return 0, 0, nil
	}
	attempted := 0
	for i, code := range codes {
		if err := ctx.Err(); err != nil {
			return securities, totalAdded, err
		}
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		attempted++
		added, _, cerr := r.CrawlCode(ctx, code)
		if cerr != nil {
			slog.Error("price refresh failed", "code", code, "error", cerr)
			continue
		}
		totalAdded += added
		securities++
		if i < len(codes)-1 {
			if err := sleepContext(ctx, 1500*time.Millisecond); err != nil {
				return securities, totalAdded, err
			}
		}
	}
	slog.Info("price refresh complete", "securities", securities, "new_rows", totalAdded, "requested", len(codes))
	// Total-failure must surface: per-code errors are logged and soft-skipped for
	// partial-crawl parity, but a run where every attempted security failed is
	// an error, not a successful crawl.
	if attempted > 0 && securities == 0 {
		return securities, totalAdded, fmt.Errorf("price refresh failed for all %d attempted securities", attempted)
	}
	return securities, totalAdded, nil
}

// heldRefreshCodes merges stale + missing held NAV codes (admin/MCP parity).
func heldRefreshCodes(report adminsvc.FreshnessReport) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(report.StaleSecurities)+len(report.MissingNAVSecurities))
	for _, item := range report.StaleSecurities {
		code := adminsvc.NormalizeSecurityCode(item.Code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	for _, item := range report.MissingNAVSecurities {
		code := adminsvc.NormalizeSecurityCode(item.Code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func latestNavDate(ctx context.Context, db *sql.DB, code string) (string, bool, error) {
	var d sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT MAX(date) FROM nav_history WHERE fund_code = ?
	`, code).Scan(&d)
	if err != nil {
		return "", false, fmt.Errorf("latest nav date %s: %w", code, err)
	}
	if !d.Valid || strings.TrimSpace(d.String) == "" {
		return "", false, nil
	}
	return d.String, true, nil
}

// filterPointsSince keeps points with date >= since (lexicographic YYYY-MM-DD).
func filterPointsSince(points []datasource.PricePoint, since string) []datasource.PricePoint {
	if since == "" {
		return points
	}
	out := make([]datasource.PricePoint, 0, len(points))
	for _, p := range points {
		if p.Date >= since {
			out = append(out, p)
		}
	}
	return out
}

// ── helpers ─────────────────────────────────────────────────────────────────

// getHeldSecurities lists held codes (held_shares > 0.001, same filter as the
// SPA). The security type is deliberately not carried: CrawlCode re-resolves it
// from portfolio_snapshot/fund_details at crawl time, so caching it here was
// dead state.
func (r *PriceRefresher) getHeldSecurities(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT fund_code
		FROM portfolio_snapshot WHERE held_shares > 0.001
		LIMIT 5000
	`)
	if err != nil {
		return nil, fmt.Errorf("getHeldSecurities: %w", err)
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

func upsertNavHistory(ctx context.Context, db *sql.DB, driver, code, secType string, points []datasource.PricePoint) (int, error) {
	// Soft-cap series length defense-in-depth (#241).
	if len(points) > 5000 {
		points = points[len(points)-5000:]
	}
	if len(points) == 0 {
		return 0, nil
	}
	// One transaction for the whole series: avoids per-row autocommit fsync
	// (especially painful if journal_mode is not WAL). Holdings upsert already
	// uses BeginTx; keep NAV parity.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin nav_history upsert: %w", err)
	}
	defer tx.Rollback()

	// Only count rows that are newly inserted or whose values actually change.
	// Plain ON CONFLICT DO UPDATE makes PG RowsAffected=1 for no-op rewrites (#87).
	conflictTarget, err := navUpsertConflictTarget(driver)
	if err != nil {
		return 0, err
	}
	insert, err := tx.PrepareContext(ctx, fmt.Sprintf(`
		INSERT INTO nav_history (date, fund_code, unit_nav, daily_change_pct, security_type)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT%s DO UPDATE SET
			unit_nav = excluded.unit_nav,
			daily_change_pct = excluded.daily_change_pct,
			security_type = excluded.security_type
		WHERE nav_history.unit_nav IS DISTINCT FROM excluded.unit_nav
			OR nav_history.daily_change_pct IS DISTINCT FROM excluded.daily_change_pct
			OR COALESCE(nav_history.security_type, '') IS DISTINCT FROM COALESCE(excluded.security_type, '')
	`, conflictTarget))
	if err != nil {
		return 0, err
	}
	defer insert.Close()

	added := 0
	for _, p := range points {
		res, err := insert.ExecContext(ctx, p.Date, code, p.Price, p.ChangePct, secType)
		if err != nil {
			return 0, fmt.Errorf("insert nav_history(%s, %s): %w", code, p.Date, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("rows affected nav_history(%s, %s): %w", code, p.Date, err)
		}
		added += int(n)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit nav_history upsert: %w", err)
	}
	return added, nil
}

// CrawlAllHeld is the MCP/admin-facing alias for RefreshAllHeld.
func (r *PriceRefresher) CrawlAllHeld(ctx context.Context) (securities int, added int, err error) {
	return r.RefreshAllHeld(ctx)
}

// CrawlCode refreshes one security code. Type is inferred from portfolio_snapshot or fund_details, default fund.
func (r *PriceRefresher) CrawlCode(ctx context.Context, code string) (added int, latest string, err error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return 0, "", fmt.Errorf("code is required")
	}
	if len(code) > 32 {
		return 0, "", fmt.Errorf("code too long")
	}
	secType := datasource.TypeFund
	var raw string
	err = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(security_type, 'fund') FROM portfolio_snapshot WHERE fund_code = ?
		UNION ALL
		SELECT COALESCE(security_type, 'fund') FROM fund_details WHERE fund_code = ?
		LIMIT 1
	`, code, code).Scan(&raw)
	if err == nil && raw != "" {
		secType = datasource.SecurityType(raw)
	} else if err != nil && err != sql.ErrNoRows {
		return 0, "", err
	}
	result, err := r.RefreshSecurity(ctx, code, secType)
	if err != nil {
		return 0, "", err
	}
	return result.Added, result.Latest, nil
}

// recalcAllMaxCodes bounds one RecalcAllSnapshots batch. Production currently
// has ~61 distinct fund codes, so 5000 is defense-in-depth rather than a
// workload number; detection fetches limit+1 rows (see capRecalcCodes) so an
// oversized ledger is reported instead of silently dropping codes.
const recalcAllMaxCodes = 5000

// capRecalcCodes keeps the first limit codes and reports how many rows were
// dropped, converting silent LIMIT truncation into an observable boundary.
// Split out so the cut point is unit-tested without a 5001-row fixture.
func capRecalcCodes(list []string, limit int) ([]string, int) {
	if len(list) <= limit {
		return list, 0
	}
	return list[:limit], len(list) - limit
}

// RecalcSnapshot is the exported maintenance entrypoint used by MCP/admin.
func RecalcSnapshot(ctx context.Context, db *sql.DB, code string) error {
	return snapshot.Recalc(ctx, db, code, snapshot.ModeFull)
}

// RecalcAllSnapshots rebuilds portfolio_snapshot for every distinct fund_code in transactions.
// Soft-fails per code (logs each failure). Hard errors (list/scan) return err with failed=nil.
// Per-code failures return err=nil and failed_codes so admin/MCP can expose status partial|error.
func RecalcAllSnapshots(ctx context.Context, db *sql.DB) (codes int, failed []string, err error) {
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT fund_code FROM transactions WHERE fund_code IS NOT NULL AND fund_code <> '' LIMIT ?`, recalcAllMaxCodes+1)
	if err != nil {
		return 0, nil, fmt.Errorf("list codes for snapshot recalc: %w", err)
	}
	defer rows.Close()
	var list []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return 0, nil, err
		}
		list = append(list, code)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}
	var dropped int
	list, dropped = capRecalcCodes(list, recalcAllMaxCodes)
	if dropped > 0 {
		// Silent LIMIT truncation made observable: one extra row is fetched
		// (LIMIT max+1), so its presence proves codes were left unprocessed.
		// Production has ~61 distinct fund codes, but a pathological ledger must
		// never silently skip tail codes.
		slog.Warn("recalc snapshots code list truncated",
			"limit", recalcAllMaxCodes,
			"processed", len(list),
			"at_least_dropped", dropped,
		)
	}
	for _, code := range list {
		if err := ctx.Err(); err != nil {
			logRecalcPartial(codes, failed)
			if len(failed) > 0 {
				// Surface completed work + failures; cancel is already in ctx for callers.
				return codes, failed, nil
			}
			return codes, failed, err
		}
		if err := snapshot.Recalc(ctx, db, code, snapshot.ModeFull); err != nil {
			slog.Error("recalc snapshot failed", "code", code, "error", err)
			failed = append(failed, code)
			continue
		}
		codes++
	}
	if len(failed) > 0 {
		logRecalcPartial(codes, failed)
	}
	return codes, failed, nil
}

// logRecalcPartial records ok/failed counts and the full failed code list for ops.
func logRecalcPartial(ok int, failed []string) {
	if len(failed) == 0 {
		return
	}
	sample := failed
	if len(sample) > 8 {
		sample = sample[:8]
	}
	slog.Error("recalc snapshots finished with failures",
		"ok", ok,
		"failed", len(failed),
		"sample", strings.Join(sample, ","),
		"failed_codes", failed,
	)
}

// RecalcAllStatus maps ok/failed counts to crawl-nav-style status strings.
// complete | partial | error (all attempted codes failed).
func RecalcAllStatus(ok int, failed []string) string {
	if len(failed) == 0 {
		return "complete"
	}
	if ok == 0 {
		return "error"
	}
	return "partial"
}

// navUpsertConflictTarget returns the unique-column list matching the
// nav_history PRIMARY KEY for each dialect. The PK column order differs:
// SQLite (fund_code, date) vs PostgreSQL (date, fund_code), and PostgreSQL
// rejects a conflict target that does not match an existing unique index.
func navUpsertConflictTarget(driver string) (string, error) {
	d, err := dialect.NewChecked(driver, nil)
	if err != nil {
		return "", err
	}
	if d.IsPostgres() {
		return "(date, fund_code)", nil
	}
	return "(fund_code, date)", nil
}

// sleepContext waits d or returns early when ctx is canceled (#247).
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
