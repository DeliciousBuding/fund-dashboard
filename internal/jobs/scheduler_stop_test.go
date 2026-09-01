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

type fakeTimer struct{ ch chan time.Time }

func (f fakeTimer) Chan() <-chan time.Time { return f.ch }
func (f fakeTimer) Stop() bool             { return true }

func TestSchedulerStopCancelsStartupCatchUp(t *testing.T) {
	s := &Scheduler{lastRun: map[string]string{}}
	var ran atomic.Bool
	ticks := make(chan time.Time, 1)
	s.mu.Lock()
	s.stopCh = make(chan struct{})
	s.ticker = neverTicker{}
	s.rootCtx, s.rootCancel = context.WithCancel(context.Background())
	s.startupTimer = fakeTimer{ch: ticks}
	s.startupRefresh = func(context.Context) (int, int, error) {
		ran.Store(true)
		return 0, 0, nil
	}
	s.mu.Unlock()

	s.startStartupCatchUp(s.stopCh, s.startupTimer)
	s.Stop()
	// A late tick must not run the catch-up: Stop canceled the context and
	// joined the startup goroutine before returning.
	ticks <- time.Now().In(cst)
	time.Sleep(100 * time.Millisecond)
	if ran.Load() {
		t.Fatal("startup catch-up ran after Stop")
	}
}

func TestStopWaitsForInFlightStartupCatchUp(t *testing.T) {
	s := &Scheduler{lastRun: map[string]string{}}
	started := make(chan struct{})
	release := make(chan struct{})
	ticks := make(chan time.Time, 1)
	ticks <- time.Now().In(cst)

	s.mu.Lock()
	s.stopCh = make(chan struct{})
	s.ticker = neverTicker{}
	s.rootCtx, s.rootCancel = context.WithCancel(context.Background())
	s.startupTimer = fakeTimer{ch: ticks}
	s.startupRefresh = func(context.Context) (int, int, error) {
		close(started)
		<-release
		return 0, 0, nil
	}
	s.mu.Unlock()

	s.startStartupCatchUp(s.stopCh, s.startupTimer)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("startup catch-up never entered the blocking refresh")
	}

	stopReturned := make(chan struct{})
	go func() {
		s.Stop()
		close(stopReturned)
	}()
	select {
	case <-stopReturned:
		t.Fatal("Stop returned while the startup catch-up was still in flight")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	select {
	case <-stopReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after the startup catch-up exited")
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
