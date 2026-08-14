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

// Scheduler runs background jobs on a schedule. It uses the PriceRefresher
// for price updates, optional DCARunner for auto-invest materialization,
// HoldingsRefresher for Saturday holdings crawl, and the database handle for WAL maintenance.
//
// The ticker fires every 5 minutes, but each scheduled window runs at most once
// per calendar day (CST) via lastRun + durable crawl_log claims.
type Scheduler struct {
	refresher *PriceRefresher
	holdings  *HoldingsRefresher
	dca       DCARunner
	db        *sql.DB

	mu           sync.Mutex
	stopCh       chan struct{}
	ticker       ticker
	lastRun      map[string]string // job -> window id (usually YYYY-MM-DD CST)
	rootCtx      context.Context
	rootCancel   context.CancelFunc
	startupTimer *time.Timer
	stopped      bool
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
	return &Scheduler{
		refresher: refresher,
		holdings:  NewHoldingsRefresher(db),
		dca:       portfoliosvc.NewService(db),
		db:        db,
		lastRun:   map[string]string{},
	}
}

// WithDCARunner overrides the DCA materialization runner (tests / custom wiring).
func (s *Scheduler) WithDCARunner(r DCARunner) *Scheduler {
	s.dca = r
	return s
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
	slog.Info("scheduler started", "schedule", "startup catch-up stale_only once/day, weekdays 20:00 CST price stale_only+dca once/day, Saturdays 10:00 CST holdings once/day, daily 03:00 CST WAL once/day")
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
	if _, _, err := s.refresher.RefreshStaleHeld(ctx); err != nil {
		slog.Error("startup price refresh failed", "error", err)
	}
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
	// Weekday 20:00 hour — price refresh + DCA materialization (local ledger), once per day.
	case hour == 20 && day >= time.Monday && day <= time.Friday:
		if !s.claimWindow("price_dca", windowDay) {
			return
		}
		slog.Info("price refresh window", "window", "20:00 CST weekday", "mode", "stale_only", "as_of", windowDay)
		ctx, cancel := s.jobContext(45 * time.Minute)
		defer cancel()
		if _, _, err := s.refresher.RefreshStaleHeld(ctx); err != nil {
			slog.Error("price refresh failed", "error", err)
		}
		// MarketTicker indices cache (#92) — best-effort Yahoo refresh.
		idxCtx, idxCancel := s.jobContext(2 * time.Minute)
		if _, err := portfoliosvc.NewService(s.db).RefreshMarketIndices(idxCtx); err != nil {
			slog.Error("market indices refresh failed", "error", err)
		}
		idxCancel()
		s.runDCAMaterialization(ctx, now)

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
		if err != nil {
			slog.Error("holdings refresh failed", "error", err, "funds", funds, "added", added)
			return
		}
		slog.Info("holdings refresh complete", "funds", funds, "added", added)

	// Daily 03:00 hour — WAL checkpoint (SQLite only), once per day.
	case hour == 3:
		if !s.claimWindow("wal", windowDay) {
			return
		}
		// Probe SQLite before PRAGMA so PG never sees invalid syntax in server logs.
		ctx, cancel := s.jobContext(5 * time.Second)
		defer cancel()
		var probe int
		if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM sqlite_master LIMIT 1`).Scan(&probe); err != nil {
			slog.Debug("WAL checkpoint skipped (non-SQLite driver)")
			return
		}
		if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			slog.Warn("WAL checkpoint failed", "error", err)
		} else {
			slog.Info("WAL checkpoint complete")
		}
	}
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
				slog.Debug("scheduler durable claim insert failed", "job", job, "window", windowID, "error", err)
				return true
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

func (s *Scheduler) runDCAMaterialization(ctx context.Context, now time.Time) {
	if s.dca == nil {
		return
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
		return
	}
	slog.Info("dca materialization complete",
		"as_of", asOf,
		"portfolio_id", portfolioID,
		"executed", res.Executed,
		"skipped", res.Skipped,
		"previewed", res.Previewed,
		"items", len(res.Items),
	)
}
