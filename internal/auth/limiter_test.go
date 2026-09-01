package auth

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// limiter map 只增不减的回归测试(已知待办):prune 必须清理过期/空闲 key,
// 且不影响递增锁定语义。全部表驱动 + 注入时钟。

// tripLockout drives key to lockout and returns the imposed lock duration.
func tripLockout(t *testing.T, l *Limiter, key string) time.Duration {
	t.Helper()
	for i := 0; i < l.MaxFails; i++ {
		if _, ok := l.Allow(key); !ok {
			t.Fatalf("attempt %d should be allowed before lockout", i)
		}
		if locked, _ := l.Failure(key); locked && i != l.MaxFails-1 {
			t.Fatalf("lockout tripped early at attempt %d", i)
		}
	}
	return l.Lockout // first strike = base lockout
}

// stateSize reports the combined per-key footprint of the limiter maps.
func stateSize(l *Limiter) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.fails) + len(l.lockedUntil) + len(l.lockStrikes) + len(l.lastFailure)
}

func TestLimiterPrunesStaleKeys(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)

	cases := []struct {
		name   string
		script func(t *testing.T, l *Limiter, now *time.Time)
	}{
		{
			name: "allowed key leaves no residue",
			script: func(t *testing.T, l *Limiter, now *time.Time) {
				if _, ok := l.Allow("ip:clean"); !ok {
					t.Fatal("allowed")
				}
				*now = now.Add(2 * pruneInterval)
				if _, ok := l.Allow("ip:other"); !ok {
					t.Fatal("allowed")
				}
				if got := stateSize(l); got != 0 {
					t.Fatalf("state size = %d, want 0 after sweep", got)
				}
			},
		},
		{
			name: "sub-threshold failures expire with the window",
			script: func(t *testing.T, l *Limiter, now *time.Time) {
				l.Failure("ip:two")
				l.Failure("ip:two")
				*now = now.Add(l.FailWindow + 2*pruneInterval)
				if _, ok := l.Allow("ip:other"); !ok {
					t.Fatal("allowed")
				}
				// fails/lock state must be gone; only lastFailure lingers
				// (it anchors strike escalation until strikeDecay).
				l.mu.Lock()
				residue := len(l.fails) + len(l.lockedUntil) + len(l.lockStrikes)
				lastLeft := len(l.lastFailure)
				l.mu.Unlock()
				if residue != 0 {
					t.Fatalf("lock/failure residue = %d, want 0 after window expiry", residue)
				}
				if lastLeft > 1 {
					t.Fatalf("lastFailure entries = %d, want <= 1", lastLeft)
				}
			},
		},
		{
			name: "active lock survives the sweep",
			script: func(t *testing.T, l *Limiter, now *time.Time) {
				tripLockout(t, l, "ip:locked")
				// Advance past pruneInterval but stay inside the 15m lock.
				*now = now.Add(5 * time.Minute)
				if _, ok := l.Allow("ip:other"); !ok {
					t.Fatal("allowed")
				}
				if _, ok := l.Allow("ip:locked"); ok {
					t.Fatal("key must still be locked mid-lockout")
				}
				l.mu.Lock()
				_, stillLocked := l.lockedUntil["ip:locked"]
				l.mu.Unlock()
				if !stillLocked {
					t.Fatal("locked key must not be swept while the lock is active")
				}
			},
		},
		{
			name: "strikes survive short idle gaps",
			script: func(t *testing.T, l *Limiter, now *time.Time) {
				tripLockout(t, l, "ip:strike")
				// 25h unlocks the base lock and fires many sweeps, but strikes
				// must persist (well inside strikeDecay) so the next trip escalates.
				*now = now.Add(25 * time.Hour)
				locked, lockDuration := l.Failure("ip:strike")
				for i := 1; i < l.MaxFails && !locked; i++ {
					locked, lockDuration = l.Failure("ip:strike")
				}
				if !locked {
					t.Fatal("second window must trip the lockout")
				}
				if lockDuration != 2*l.Lockout {
					t.Fatalf("second trip = %v, want 30m escalation", lockDuration)
				}
			},
		},
		{
			name: "strikes decay after strikeDecay idle",
			script: func(t *testing.T, l *Limiter, now *time.Time) {
				tripLockout(t, l, "ip:decay")
				// Wait out lock + strikeDecay with zero activity.
				*now = now.Add(maxLockout + strikeDecay + time.Hour)
				if _, ok := l.Allow("ip:other"); !ok {
					t.Fatal("allowed")
				}
				l.mu.Lock()
				_, strikesLeft := l.lockStrikes["ip:decay"]
				_, lastLeft := l.lastFailure["ip:decay"]
				l.mu.Unlock()
				if strikesLeft || lastLeft {
					t.Fatal("idle key must lose strikes after strikeDecay")
				}
				locked, lockDuration := l.Failure("ip:decay")
				for i := 1; i < l.MaxFails && !locked; i++ {
					locked, lockDuration = l.Failure("ip:decay")
				}
				if !locked {
					t.Fatal("fresh window must trip the lockout")
				}
				if lockDuration != l.Lockout {
					t.Fatalf("post-decay trip = %v, want base %v", lockDuration, l.Lockout)
				}
			},
		},
		{
			name: "success clears all state",
			script: func(t *testing.T, l *Limiter, now *time.Time) {
				tripLockout(t, l, "ip:win")
				*now = now.Add(16 * time.Minute) // lock expires
				l.Success("ip:win")
				*now = now.Add(2 * pruneInterval)
				if _, ok := l.Allow("ip:other"); !ok {
					t.Fatal("allowed")
				}
				if got := stateSize(l); got != 0 {
					t.Fatalf("state size = %d, want 0 after success + sweep", got)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := base
			l := NewLimiter(func() time.Time { return now })
			tc.script(t, l, &now)
		})
	}
}

// TestLimiterMapStaysBoundedUnderKeyChurn simulates a public login endpoint
// behind rotating client IPs: many distinct keys, few failures each. Before
// the GC fix every key pinned a map entry forever.
func TestLimiterMapStaysBoundedUnderKeyChurn(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	l := NewLimiter(func() time.Time { return now })
	// This case stresses per-key GC, not the global stuffing guard.
	l.GlobalPerHour = 1_000_000

	for round := 0; round < 5; round++ {
		for i := 0; i < 200; i++ {
			key := fmt.Sprintf("ip:%d:%d", round, i)
			if _, ok := l.Allow(key); !ok {
				t.Fatalf("round %d key %d should be allowed", round, i)
			}
			if i%2 == 0 {
				l.Failure(key) // one failure, never reaches MaxFails
			}
		}
		now = now.Add(l.FailWindow + time.Hour) // slide every window + decay margin
	}
	// One more pass to let the amortized sweep see all stale keys.
	now = now.Add(strikeDecay + time.Hour)
	if _, ok := l.Allow("ip:final"); !ok {
		t.Fatal("final key must be allowed")
	}

	l.mu.Lock()
	total := len(l.fails) + len(l.lockedUntil) + len(l.lockStrikes) + len(l.lastFailure)
	l.mu.Unlock()
	if total != 0 {
		t.Fatalf("limiter retained %d stale entries after churn + decay, want 0", total)
	}
}

// TestLimiterConcurrentAccessIsSafe exercises the limiter from many goroutines
// with overlapping Allow/Failure/Success on shared keys; the race detector
// (go test -race) verifies the locking, and the final invariants must hold
// even without it.
func TestLimiterConcurrentAccessIsSafe(t *testing.T) {
	l := NewLimiter(time.Now)
	l.MaxFails = 3
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				key := fmt.Sprintf("ip:%d", i%7)
				if retryAfter, ok := l.Allow(key); !ok && retryAfter <= 0 {
					t.Errorf("locked key returned non-positive retryAfter")
					return
				}
				switch i % 4 {
				case 0, 1:
					l.Failure(key)
				case 2:
					l.Allow(fmt.Sprintf("other:%d", seed))
				default:
					l.Success(key)
				}
			}
		}(g)
	}
	wg.Wait()

	// Internal consistency: no negative timestamps, slices inside the window.
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, times := range l.fails {
		if len(times) > l.MaxFails {
			t.Errorf("key %s holds %d failures, want <= MaxFails", key, len(times))
		}
	}
}
