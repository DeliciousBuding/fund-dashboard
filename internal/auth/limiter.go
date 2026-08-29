package auth

import (
	"sync"
	"time"
)

// Limiter is an in-memory login rate limiter: per-key lockout after repeated
// failures plus a global sliding window. It never sleeps the request; locked
// callers get a fast 429 with Retry-After.
type Limiter struct {
	mu          sync.Mutex
	now         func() time.Time
	fails       map[string][]time.Time
	lockedUntil map[string]time.Time
	globalFails []time.Time

	// MaxFails per key within FailWindow before Lockout triggers.
	MaxFails int
	// FailWindow is the per-key failure counting window.
	FailWindow time.Duration
	// Lockout is how long a key stays locked after tripping MaxFails.
	Lockout time.Duration
	// GlobalPerHour caps failures across all keys (credential stuffing guard).
	GlobalPerHour int
}

func NewLimiter(now func() time.Time) *Limiter {
	return &Limiter{
		now:           now,
		fails:         make(map[string][]time.Time),
		lockedUntil:   make(map[string]time.Time),
		MaxFails:      5,
		FailWindow:    15 * time.Minute,
		Lockout:       15 * time.Minute,
		GlobalPerHour: 20,
	}
}

// Allow reports whether an attempt for key is allowed right now, and if not,
// how long the caller should wait before retrying.
func (l *Limiter) Allow(key string) (retryAfter time.Duration, allowed bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	if until, locked := l.lockedUntil[key]; locked {
		if now.Before(until) {
			return until.Sub(now), false
		}
		delete(l.lockedUntil, key)
	}
	l.pruneLocked(now)

	l.globalFails = prune(l.globalFails, now.Add(-time.Hour))
	if len(l.globalFails) >= l.GlobalPerHour {
		oldest := l.globalFails[0]
		return oldest.Add(time.Hour).Sub(now), false
	}

	windowStart := now.Add(-l.FailWindow)
	l.fails[key] = prune(l.fails[key], windowStart)
	if len(l.fails[key]) >= l.MaxFails {
		// MaxFails failures inside the window but lock expired → fresh window.
		l.fails[key] = nil
	}
	return 0, true
}

// Failure records a failed attempt; tripping MaxFails locks the key.
func (l *Limiter) Failure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.globalFails = append(prune(l.globalFails, now.Add(-time.Hour)), now)
	windowStart := now.Add(-l.FailWindow)
	l.fails[key] = append(prune(l.fails[key], windowStart), now)
	if len(l.fails[key]) >= l.MaxFails {
		l.lockedUntil[key] = now.Add(l.Lockout)
		l.fails[key] = nil
	}
}

// Success clears a key's failure state (successful login resets the budget).
func (l *Limiter) Success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
	delete(l.lockedUntil, key)
}

func (l *Limiter) pruneLocked(now time.Time) {
	for key, until := range l.lockedUntil {
		if !now.Before(until) {
			delete(l.lockedUntil, key)
		}
	}
}

func prune(times []time.Time, cutoff time.Time) []time.Time {
	kept := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	return kept
}
