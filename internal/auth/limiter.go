package auth

import (
	"sync"
	"time"
)

// Limiter is an in-memory login rate limiter: per-key lockout after repeated
// failures plus a global sliding window. It never sleeps the request; locked
// callers get a fast 429 with Retry-After.
//
// Lockout is escalating: each consecutive lockout trip multiplies the duration
// by two (15m → 30m → 1h → …) capped at MaxLockout (24h); a successful login
// resets the strike count. Design: docs/design/06-security-hardening.md §2.2.
type Limiter struct {
	mu          sync.Mutex
	now         func() time.Time
	fails       map[string][]time.Time
	lockedUntil map[string]time.Time
	lockStrikes map[string]int
	globalFails []time.Time

	// MaxFails per key within FailWindow before Lockout triggers.
	MaxFails int
	// FailWindow is the per-key failure counting window.
	FailWindow time.Duration
	// Lockout is the base lockout duration (doubled per strike, capped at maxLockout).
	Lockout time.Duration
	// GlobalPerHour caps failures across all keys (credential stuffing guard).
	GlobalPerHour int
}

// maxLockout caps the escalating lockout duration (design §2.2).
const maxLockout = 24 * time.Hour

func NewLimiter(now func() time.Time) *Limiter {
	return &Limiter{
		now:           now,
		fails:         make(map[string][]time.Time),
		lockedUntil:   make(map[string]time.Time),
		lockStrikes:   make(map[string]int),
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

// Failure records a failed attempt; tripping MaxFails locks the key with an
// escalated duration (Lockout << strikes, capped at maxLockout).
func (l *Limiter) Failure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.globalFails = append(prune(l.globalFails, now.Add(-time.Hour)), now)
	windowStart := now.Add(-l.FailWindow)
	l.fails[key] = append(prune(l.fails[key], windowStart), now)
	if len(l.fails[key]) >= l.MaxFails {
		l.lockStrikes[key]++
		l.lockedUntil[key] = now.Add(escalatedLockout(l.Lockout, l.lockStrikes[key]))
		l.fails[key] = nil
	}
}

// Success clears a key's failure state and resets the escalation strike count
// (a successful login restores the base lockout for the next trip).
func (l *Limiter) Success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
	delete(l.lockedUntil, key)
	delete(l.lockStrikes, key)
}

// escalatedLockout computes Lockout << (strikes-1), saturating at maxLockout.
// A plain shift would overflow int64 for large strikes, so it iterates with a
// cap check instead — duration never wraps or goes negative.
func escalatedLockout(base time.Duration, strikes int) time.Duration {
	if strikes <= 1 || base <= 0 {
		return base
	}
	d := base
	for i := 1; i < strikes; i++ {
		if d >= maxLockout/2 {
			return maxLockout
		}
		d *= 2
	}
	return d
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
