package jobs

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"
	"time"

	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	_ "modernc.org/sqlite"
)

type neverTicker struct{}

func (neverTicker) Chan() <-chan time.Time { return make(chan time.Time) }
func (neverTicker) Stop()                  {}

func TestSchedulerStopCancelsStartupCatchUp(t *testing.T) {
	s := &Scheduler{lastRun: map[string]string{}}
	s.mu.Lock()
	s.stopCh = make(chan struct{})
	s.ticker = neverTicker{}
	s.rootCtx, s.rootCancel = context.WithCancel(context.Background())
	var ran atomic.Bool
	s.startupTimer = time.AfterFunc(80*time.Millisecond, func() {
		if s.isStopped() {
			return
		}
		ran.Store(true)
	})
	s.mu.Unlock()

	s.Stop()
	time.Sleep(150 * time.Millisecond)
	if ran.Load() {
		t.Fatal("startup catch-up ran after Stop")
	}
}

func TestJobContextCanceledAfterStop(t *testing.T) {
	s := &Scheduler{lastRun: map[string]string{}}
	s.mu.Lock()
	s.stopCh = make(chan struct{})
	s.ticker = neverTicker{}
	s.rootCtx, s.rootCancel = context.WithCancel(context.Background())
	s.mu.Unlock()

	ctx, cancel := s.jobContext(time.Minute)
	defer cancel()
	s.Stop()
	select {
	case <-ctx.Done():
		// expected
	case <-time.After(200 * time.Millisecond):
		t.Fatal("job context not canceled after Stop")
	}
}

type blockingTicker struct{ ch chan time.Time }

func (b blockingTicker) Chan() <-chan time.Time { return b.ch }
func (b blockingTicker) Stop()                  {}

type blockingDCARunner struct {
	started chan struct{}
	release chan struct{}
}

func (r blockingDCARunner) RunDCAAutoInvest(_ context.Context, _ portfoliosvc.RunDCAAutoInvestInput) (portfoliosvc.RunDCAAutoInvestResult, error) {
	close(r.started)
	<-r.release
	return portfoliosvc.RunDCAAutoInvestResult{}, nil
}

func TestStopWaitsForInFlightLoopToExit(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	runner := blockingDCARunner{started: make(chan struct{}), release: make(chan struct{})}
	s := NewScheduler(NewPriceRefresher(db), db).
		WithDCARunner(runner).
		WithMarketIndicesRefresher(func(context.Context) (int, error) { return 0, nil })

	ticks := make(chan time.Time, 1)
	ticks <- time.Date(2026, 7, 15, 20, 0, 0, 0, cst) // Wednesday 20:00 -> DCA window
	s.mu.Lock()
	s.stopCh = make(chan struct{})
	s.done = make(chan struct{})
	s.ticker = blockingTicker{ch: ticks}
	s.rootCtx, s.rootCancel = context.WithCancel(context.Background())
	s.mu.Unlock()

	go func() {
		defer close(s.done)
		s.loop(s.stopCh)
	}()

	select {
	case <-runner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("background loop never entered the DCA tick")
	}

	stopReturned := make(chan struct{})
	go func() {
		s.Stop()
		close(stopReturned)
	}()

	// The in-flight runner ignores cancellation, so Stop must not report
	// completion while the background loop is still executing the tick.
	select {
	case <-stopReturned:
		t.Fatal("Stop returned before the background loop exited")
	case <-time.After(100 * time.Millisecond):
	}

	close(runner.release)
	select {
	case <-stopReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after the background loop exited")
	}
}
