package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/chinatime"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

// dcaBackfillLookbackDays bounds how far back a missed DCA due date is
// compensated: the backfill evaluates the 7 natural days before today (CST),
// excluding today itself (today still belongs exclusively to the 20:00 window,
// so normal same-day behavior is unchanged). 7 days covers every weekday of the
// preceding week, so any weekday_mask slot missed while the process was down
// (restart, deploy, host suspend) is retried at least once; beyond a week a
// missed order is treated as an intentionally paused plan rather than silently
// auto-executed, and NAV drift between due date and execution grows with age.
// Package constant on purpose — internal/config is owned by another change
// line this round.
const dcaBackfillLookbackDays = 7

// dcaBackfillJob is the claim key for the daily backfill pass. It maps to the
// durable crawl_log row __sched_dca_backfill (see schedulerClaimCode), so the
// once-per-CST-day guarantee survives process restarts, while the in-memory
// lastRun map keeps co-located ticks from re-checking within one process.
const dcaBackfillJob = "dca_backfill"

// runDCABackfillWindow is the tick-level catch-up: the first tick between
// 06:00 and 09:59 CST each day claims the day and replays missed due dates.
// A morning band (instead of a single hour) tolerates wake-ups from host
// suspend anywhere in the morning; weekend ticks claim but immediately no-op
// inside runDCABackfill (DCA stays a weekday decision).
func (s *Scheduler) runDCABackfillWindow(now time.Time) {
	windowDay := now.In(chinatime.Loc).Format("2006-01-02")
	if !s.claimWindow(dcaBackfillJob, windowDay) {
		return
	}
	slog.Info("dca backfill window", "window", "06:00-09:59 CST daily catch-up", "as_of", windowDay)
	ctx, cancel := s.jobContext(10 * time.Minute)
	defer cancel()
	if err := s.runDCABackfill(ctx, now); err != nil {
		slog.Error("dca backfill failed", "as_of", windowDay, "error", err)
	}
}

// runDCABackfill replays every due date in [now-7d, now-1d] (CST natural days,
// oldest first) through the existing RunDCAAutoInvest materialization. Per-date
// idempotency is owned entirely by the service (order_id = DCA-<plan>-<due date>
// + WHERE NOT EXISTS claim insert + dca_plan_executions ledger in one
// transaction), so a replayed date either materializes once with the due date —
// not the execution date — stamped on the order, or is skipped as duplicate.
// Dates that do not match the plan's weekday_mask, fall outside the plan's
// start/end window, or have no NAV are skipped by the same gating the 20:00
// window uses; nothing here invents a second idempotency mechanism. Weekday
// gate matches the 20:00 materialization policy: a financial decision — never
// on Saturday/Sunday (a missed Friday is replayed Monday, still inside the
// 7-day window). Per-date errors are logged and do not block the remaining
// dates; the first error is returned for window-level visibility.
func (s *Scheduler) runDCABackfill(ctx context.Context, now time.Time) error {
	if s.dca == nil {
		return nil
	}
	now = now.In(chinatime.Loc)
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return nil
	}
	// Single-user deployment: default portfolio only (same as runDCAMaterialization).
	const portfolioID = 1
	var firstErr error
	for i := dcaBackfillLookbackDays; i >= 1; i-- {
		due := now.AddDate(0, 0, -i).Format("2006-01-02")
		res, err := s.dca.RunDCAAutoInvest(ctx, portfoliosvc.RunDCAAutoInvestInput{
			AsOf:        due,
			PortfolioID: portfolioID,
			DryRun:      false,
		})
		if err != nil {
			slog.Error("dca backfill date failed", "due_date", due, "portfolio_id", portfolioID, "error", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if res.Executed > 0 {
			slog.Info("dca backfill executed", "due_date", due, "portfolio_id", portfolioID,
				"executed", res.Executed, "skipped", res.Skipped)
		} else {
			slog.Debug("dca backfill date clean", "due_date", due, "skipped", res.Skipped)
		}
	}
	return firstErr
}
