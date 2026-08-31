package jobs

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"sync"
	"time"

	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

// CST is Asia/Shanghai (UTC+8).
var cst = time.FixedZone("CST", 8*60*60)

// DCARunner materializes due DCA plans into the local ledger.
// Implemented by portfolio.Service.RunDCAAutoInvest; interface keeps jobs testable.
type DCARunner interface {
	RunDCAAutoInvest(ctx context.Context, in portfoliosvc.RunDCAAutoInvestInput) (portfoliosvc.RunDCAAutoInvestResult, error)
}

// AuthEventSweeper deletes expired auth audit rows (implemented by
// auth.Service.SweepAuthEvents — 180d retention, design 06 §2.2 清扫).
type AuthEventSweeper interface {
	SweepAuthEvents(ctx context.Context, cutoffEpoch int64) (int64, error)
}

// JobStatus is the runtime snapshot of one scheduler job, exposed via
// StatusSnapshot to the system workspace API (design 06 §2.6). Times are unix
// epoch seconds.
type JobStatus struct {
	Name      string `json:"name"`
	Schedule  string `json:"schedule"`
	LastRun   int64  `json:"last_run,omitempty"`
	LastError string `json:"last_error,omitempty"`
	NextRun   int64  `json:"next_run"`
}

// jobRuntime tracks one job's execution record under jobsMu.
type jobRuntime struct {
	name     string
	schedule string
	lastRun  int64
	lastErr  string
}

// Scheduler runs background jobs on a schedule. It uses the PriceRefresher
// for price updates, optional DCARunner for auto-invest materialization,
// HoldingsRefresher for Saturday holdings crawl, and the database handle for WAL maintenance.
//
// The ticker fires every 5 minutes, but each scheduled window runs at most once
// per calendar day (CST) via lastRun + durable crawl_log claims.
type Scheduler struct {
	refresher      *PriceRefresher
	holdings       *HoldingsRefresher
	dca            DCARunner
	indicesRefresh func(ctx context.Context) (int, error)
	db             *sql.DB
	authSweep      AuthEventSweeper

	mu           sync.Mutex
	jobsMu       sync.RWMutex
	jobs         map[string]*jobRuntime
	stopCh       chan struct{}
	ticker       ticker
	lastRun      map[string]string // job -> window id (usually YYYY-MM-DD CST)
	rootCtx      context.Context
	rootCancel   context.CancelFunc
	startupTimer *time.Timer
	stopped      bool
}

// jobDefinitions lists the tracked jobs (name + schedule description) so
// StatusSnapshot always reports the full calendar surface. nextRunEpoch
// computes the next window for each.
func jobDefinitions() []jobRuntime {
	return []jobRuntime{
		{name: "startup_refresh", schedule: "startup catch-up stale_only (once per CST day)"},
		{name: "price_dca", schedule: "daily 20:00 CST all_held + DCA weekdays"},
		{name: "holdings", schedule: "Saturday 10:00 CST holdings once/day"},
		{name: "wal", schedule: "daily 03:00 CST WAL + expired-state sweep"},
	}
}

// nextRunEpoch picks the next scheduled window (unix seconds) for the given
// job, or 0 when the job has no recurring window (startup_refresh runs only
// once per process start).
func nextRunEpoch(job string, now time.Time) int64 {
	now = now.In(cst)
	target := func(hour int, day time.Weekday) time.Time {
		t := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, cst)
		if day >= 0 { // weekly constraint (holdings: Saturday)
			delta := (int(day) - int(t.Weekday()) + 7) % 7
			t = t.AddDate(0, 0, delta)
		}
		if !t.After(now) {
			if day >= 0 {
				t = t.AddDate(0, 0, 7) // next same weekday
			} else {
				t = t.AddDate(0, 0, 1) // next day
			}
		}
		return t
	}
	switch job {
	case "holdings":
		return target(10, time.Saturday).Unix()
	case "price_dca":
		return target(20, -1).Unix()
	case "wal":
		return target(3, -1).Unix()
	default:
		return 0 // startup_refresh — no recurring next run
	}
}

// ticker abstracts time.Ticker for testability.
type ticker interface {
	Chan() <-chan time.Time
	Stop()
}

type realTicker struct{ *time.Ticker }

func (t *realTicker) Chan() <-chan time.Time { return t.Ticker.C }
func (t *realTicker) Stop()                  { t.Ticker.Stop() }

// NewScheduler creates a Scheduler. It does not start automatically —
// call Start() to begin periodic execution.
func NewScheduler(refresher *PriceRefresher, db *sql.DB) *Scheduler {
	svc := portfoliosvc.NewService(db)
	return &Scheduler{
		refresher:      refresher,
		holdings:       NewHoldingsRefresher(db),
		dca:            svc,
		indicesRefresh: svc.RefreshMarketIndices,
		db:             db,
		lastRun:        map[string]string{},
		jobs:           map[string]*jobRuntime{},
	}
}

// WithDCARunner overrides the DCA materialization runner (tests / custom wiring).
func (s *Scheduler) WithDCARunner(r DCARunner) *Scheduler {
	s.dca = r
	return s
}

// WithMarketIndicesRefresher overrides the best-effort indices cache refresh
// (tests / custom wiring). Default is portfolio.Service.RefreshMarketIndices.
func (s *Scheduler) WithMarketIndicesRefresher(fn func(ctx context.Context) (int, error)) *Scheduler {
	s.indicesRefresh = fn
	return s
}

// WithAuthEventSweeper wires the auth audit retention sweep (auth.Service —
// deletes auth_events rows older than 180d in the 03:00 window).
func (s *Scheduler) WithAuthEventSweeper(sweeper AuthEventSweeper) *Scheduler {
	s.authSweep = sweeper
	return s
}

// recordJob updates the runtime record after a job attempt. now is the window
// time (the injectable tick time in tests); a nil error clears last_error.
func (s *Scheduler) recordJob(job string, now time.Time, err error) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	if s.jobs == nil {
		s.jobs = map[string]*jobRuntime{}
	}
	rt, ok := s.jobs[job]
	if !ok {
		rt = &jobRuntime{name: job}
		s.jobs[job] = rt
	}
	rt.lastRun = now.Unix()
	if err != nil {
		rt.lastErr = err.Error()
	} else {
		rt.lastErr = ""
	}
}

// StatusSnapshot returns the tracked jobs with last run / error and the next
// scheduled window (design 06 §2.6). Safe for concurrent use; job times are
// unix epoch seconds.
func (s *Scheduler) StatusSnapshot() []JobStatus {
	now := time.Now()
	s.jobsMu.RLock()
	// Defs are rebuilt per call so the schedule text stays current while the
	// stored runtime records are looked up by name.
	out := make([]JobStatus, 0, 4)
	for _, def := range jobDefinitions() {
		snap := JobStatus{Name: def.name, Schedule: def.schedule, NextRun: nextRunEpoch(def.name, now)}
		if rt := s.jobs[def.name]; rt != nil {
			snap.LastRun = rt.lastRun
			snap.LastError = rt.lastErr
		}
		out = append(out, snap)
	}
	s.jobsMu.RUnlock()
	return out
}

// Start begins periodic execution. Safe to call multiple times.
//   - Startup: optional catch-up refresh after 30s, once per CST day (durable).
//   - Every 5 min: check if we're in a scheduled window (weekday 20:00 CST, etc.).
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopCh != nil {
		return // already started
	}

	s.stopped = false
	s.stopCh = make(chan struct{})
	s.ticker = &realTicker{time.NewTicker(5 * time.Minute)}

	s.rootCtx, s.rootCancel = context.WithCancel(context.Background())
	// Startup catch-up after a short delay (once per CST day; skips same-day redeploys).
	s.startupTimer = time.AfterFunc(30*time.Second, func() {
		s.runStartupCatchUp(time.Now().In(cst))
	})

	go s.loop(s.stopCh)
	slog.Info("scheduler started", "schedule", "startup catch-up stale_only once/day, daily 20:00 CST price full-refresh held + DCA weekdays, Saturdays 10:00 CST holdings once/day, daily 03:00 CST WAL once/day")
}

// Stop terminates periodic execution.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopCh == nil {
		return
	}
	close(s.stopCh)
	s.ticker.Stop()
	if s.startupTimer != nil {
		s.startupTimer.Stop()
		s.startupTimer = nil
	}
	if s.rootCancel != nil {
		s.rootCancel()
		s.rootCancel = nil
	}
	s.rootCtx = nil
	s.stopCh = nil
	s.stopped = true
	slog.Info("scheduler stopped")
}

func (s *Scheduler) loop(stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		case t := <-s.ticker.Chan():
			s.tick(t.In(cst))
		}
	}
}

func (s *Scheduler) runStartupCatchUp(now time.Time) {
	if s.isStopped() {
		return
	}
	windowDay := now.Format("2006-01-02")
	if !s.claimWindow("startup_refresh", windowDay) {
		slog.Info("startup price refresh skipped", "reason", "already ran today", "as_of", windowDay)
		return
	}
	slog.Info("startup price refresh", "mode", "stale_only", "as_of", windowDay)
	ctx, cancel := s.jobContext(45 * time.Minute)
	defer cancel()
	if ctx.Err() != nil {
		return
	}
	// Scheduled paths only touch missing/stale held NAV; full crawl stays on admin/MCP.
	_, _, err := s.refresher.RefreshStaleHeld(ctx)
	if err != nil {
		slog.Error("startup price refresh failed", "error", err)
	}
	s.recordJob("startup_refresh", now, err)
}

func (s *Scheduler) isStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}

// jobContext derives a timeout from the scheduler root context so Stop cancels in-flight jobs.
func (s *Scheduler) jobContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	s.mu.Lock()
	root := s.rootCtx
	s.mu.Unlock()
	if root == nil {
		return context.WithTimeout(context.Background(), timeout)
	}
	return context.WithTimeout(root, timeout)
}

func (s *Scheduler) tick(now time.Time) {
	hour, day := now.Hour(), now.Weekday()
	windowDay := now.Format("2006-01-02")

	switch {
	// Daily 20:00 hour — full price refresh of every held security, once per day.
	// Every calendar day (including Sat/Sun): QDII funds publish T+2 and bond funds
	// T+1, so a Saturday/Sunday run still picks up fresh NAVs. Full (not stale_only)
	// refresh keeps last_nav within one window of the upstream even when upstream
	// publishes late or publishes back-dated corrections.
	case hour == 20:
		if !s.claimWindow("price_dca", windowDay) {
			return
		}
		slog.Info("price refresh window", "window", "20:00 CST daily", "mode", "all_held", "as_of", windowDay)
		ctx, cancel := s.jobContext(45 * time.Minute)
		defer cancel()
		var jobErr error
		if _, _, err := s.refresher.RefreshAllHeld(ctx); err != nil {
			slog.Error("price refresh failed", "error", err)
			jobErr = err
		}
		// MarketTicker indices cache (#92) — best-effort Yahoo refresh.
		idxCtx, idxCancel := s.jobContext(2 * time.Minute)
		if _, err := s.indicesRefresh(idxCtx); err != nil {
			slog.Error("market indices refresh failed", "error", err)
			if jobErr == nil {
				jobErr = err
			}
		}
		idxCancel()
		// DCA materialization stays weekday-only (a financial decision — never on
		// Saturday/Sunday; price refresh above is pure data sync).
		if day >= time.Monday && day <= time.Friday {
			if err := s.runDCAMaterialization(ctx, now); err != nil && jobErr == nil {
				jobErr = err
			}
		}
		s.recordJob("price_dca", now, jobErr)

	// Saturday 10:00 hour — fund holdings refresh, once per day.
	case hour == 10 && day == time.Saturday:
		if !s.claimWindow("holdings", windowDay) {
			return
		}
		slog.Info("holdings refresh window", "window", "10:00 CST Saturday", "as_of", windowDay)
		if s.holdings == nil {
			return
		}
		ctx, cancel := s.jobContext(45 * time.Minute)
		defer cancel()
		funds, added, err := s.holdings.CrawlAllHeld(ctx)
		s.recordJob("holdings", now, err)
		if err != nil {
			slog.Error("holdings refresh failed", "error", err, "funds", funds, "added", added)
			return
		}
		slog.Info("holdings refresh complete", "funds", funds, "added", added)

	// Daily 03:00 hour — WAL checkpoint (SQLite only) + expired-state sweep, once per day.
	case hour == 3:
		if !s.claimWindow("wal", windowDay) {
			return
		}
		ctx, cancel := s.jobContext(30 * time.Second)
		defer cancel()
		var jobErr error
		// Probe SQLite before PRAGMA so PG never sees invalid syntax in server logs.
		var probe int
		if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master LIMIT 1`).Scan(&probe); err != nil {
			slog.Debug("WAL checkpoint skipped (non-SQLite driver)")
		} else if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			slog.Warn("WAL checkpoint failed", "error", err)
			jobErr = err
		} else {
			slog.Info("WAL checkpoint complete")
		}
		if err := s.sweepExpiredState(ctx); err != nil && jobErr == nil {
			jobErr = err
		}
		s.recordJob("wal", now, jobErr)
	}
}

// sweepExpiredState piggybacks the daily 03:00 window: expired web sessions,
// agent confirmations expired >7d, audit events older than 90d, and auth
// events older than 180d (design 06 §2.2 — auth_events retention is owned by
// auth.Service via the AuthEventSweeper interface). Best-effort; legacy
// databases may lack the tables. Returns the first non-nil error encountered.
func (s *Scheduler) sweepExpiredState(ctx context.Context) error {
	now := time.Now()
	var firstErr error
	sweeps := []struct {
		table string
		query string
		arg   any
	}{
		{"auth_sessions", `DELETE FROM auth_sessions WHERE expires_at < ?`, now.Unix()},
		{"agent_confirmations", `DELETE FROM agent_confirmations WHERE expires_at < ?`, now.Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)},
		{"agent_audit_events", `DELETE FROM agent_audit_events WHERE created_at < ?`, now.Add(-90 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)},
	}
	for _, sweep := range sweeps {
		res, err := s.db.ExecContext(ctx, sweep.query, sweep.arg)
		if err != nil {
			if strings.Contains(err.Error(), "no such table") {
				slog.Debug("sweep skipped (table absent)", "table", sweep.table)
				continue
			}
			slog.Warn("daily sweep failed", "table", sweep.table, "error", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if deleted, err := res.RowsAffected(); err == nil && deleted > 0 {
			slog.Info("daily sweep", "table", sweep.table, "deleted", deleted)
		}
	}
	// auth_events retention (180d) via the injected auth service — the table is
	// dialect-agnostic but owned by internal/auth, which keeps its DDL together.
	if s.authSweep != nil {
		if deleted, err := s.authSweep.SweepAuthEvents(ctx, now.Add(-180*24*time.Hour).Unix()); err != nil {
			slog.Warn("daily sweep failed", "table", "auth_events", "error", err)
			if firstErr == nil {
				firstErr = err
			}
		} else if deleted > 0 {
			slog.Info("daily sweep", "table", "auth_events", "deleted", deleted)
		}
	}
	return firstErr
}

// claimWindow records a job attempt for the given window id.
// Returns false if this job+window already ran (in-memory or durable crawl_log).
// Does not hold s.mu across claimWindowDurable (which calls jobContext and would deadlock).
func (s *Scheduler) claimWindow(job, windowID string) bool {
	s.mu.Lock()
	if s.lastRun == nil {
		s.lastRun = map[string]string{}
	}
	if s.lastRun[job] == windowID {
		s.mu.Unlock()
		return false
	}
	db := s.db
	s.mu.Unlock()

	// Durable claim survives process restarts / redeploys (best-effort).
	if db != nil && !s.claimWindowDurable(job, windowID) {
		s.mu.Lock()
		s.lastRun[job] = windowID
		s.mu.Unlock()
		return false
	}
	s.mu.Lock()
	s.lastRun[job] = windowID
	s.mu.Unlock()
	return true
}

// claimWindowDurable uses one crawl_log row per job.
// Production crawl_log PRIMARY KEY is fund_code only, so each job needs a distinct fund_code.
func (s *Scheduler) claimWindowDurable(job, windowID string) bool {
	ctx, cancel := s.jobContext(3 * time.Second)
	defer cancel()

	code := schedulerClaimCode(job)
	var existing string
	err := s.db.QueryRowContext(ctx, `SELECT latest_date FROM crawl_log WHERE fund_code = ?`, code).Scan(&existing)
	if err != nil {
		if err == sql.ErrNoRows {
			now := time.Now().In(cst).Format("2006-01-02 15:04:05")
			_, err = s.db.ExecContext(ctx, `
				INSERT INTO crawl_log (fund_code, source, rows_added, latest_date, status, crawled_at)
				VALUES (?, ?, 0, ?, 'ok', ?)
			`, code, job, windowID, now)
			if err != nil {
				msg := strings.ToLower(err.Error())
				if strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined_table") {
					return true
				}
				// race: another process inserted
				var again string
				if err2 := s.db.QueryRowContext(ctx, `SELECT latest_date FROM crawl_log WHERE fund_code = ?`, code).Scan(&again); err2 == nil && again == windowID {
					return false
				}
				slog.Error("scheduler durable claim insert failed", "job", job, "window", windowID, "error", err)
				return false
			}
			return true
		}
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined_table") {
			return true
		}
		// Unknown lookup errors: fail closed (do not claim) to avoid duplicate job runs.
		slog.Error("scheduler durable claim lookup failed", "job", job, "window", windowID, "error", err)
		return false
	}
	if existing == windowID {
		return false
	}
	// Advance claim to a new day for this job.
	now := time.Now().In(cst).Format("2006-01-02 15:04:05")
	_, err = s.db.ExecContext(ctx, `
		UPDATE crawl_log SET source = ?, rows_added = 0, latest_date = ?, status = 'ok', crawled_at = ?
		WHERE fund_code = ?
	`, job, windowID, now, code)
	if err != nil {
		// Fail closed: do not run the job if durable claim could not advance (#201).
		slog.Error("scheduler durable claim update failed", "job", job, "window", windowID, "error", err)
		return false
	}
	return true
}

func schedulerClaimCode(job string) string {
	switch job {
	case "startup_refresh":
		return "__sched_startup_refresh"
	case "price_dca":
		return "__sched_price_dca"
	case "holdings":
		return "__sched_holdings"
	case "wal":
		return "__sched_wal"
	default:
		return "__sched_" + job
	}
}

func (s *Scheduler) runDCAMaterialization(ctx context.Context, now time.Time) error {
	if s.dca == nil {
		return nil
	}
	asOf := now.Format("2006-01-02")
	// Single-user deployment: default portfolio only. Multi-portfolio ledger is deferred
	// (docs/STATE.md residual) — do not invent multi-id loops until ledger scopes txs.
	const portfolioID = 1
	res, err := s.dca.RunDCAAutoInvest(ctx, portfoliosvc.RunDCAAutoInvestInput{
		AsOf:        asOf,
		PortfolioID: portfolioID,
		DryRun:      false,
	})
	if err != nil {
		slog.Error("dca materialization failed", "as_of", asOf, "portfolio_id", portfolioID, "error", err)
		return err
	}
	slog.Info("dca materialization complete",
		"as_of", asOf,
		"portfolio_id", portfolioID,
		"executed", res.Executed,
		"skipped", res.Skipped,
		"previewed", res.Previewed,
		"items", len(res.Items),
	)
	return nil
}
