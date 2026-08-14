package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
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
