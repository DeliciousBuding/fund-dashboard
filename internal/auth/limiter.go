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
//
// Per-key state is garbage-collected: an amortized sweep (at most once per
// pruneInterval, under the same mutex) drops expired locks, failure windows
// that slid empty, and escalation strikes idle longer than strikeDecay, so the
// maps cannot grow without bound on a public endpoint.
type Limiter struct {
	mu          sync.Mutex
	now         func() time.Time
	fails       map[string][]time.Time
	lockedUntil map[string]time.Time
	lockStrikes map[string]int
	lastFailure map[string]time.Time
	globalFails []time.Time
	lastPrune   time.Time

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

// maxLockStrikes caps the per-key escalation counter. Escalation saturates at
// 8 strikes (maxLockout), so any larger count is equivalent; capping keeps the
// int counter from ever overflowing after years of lockout trips.
const maxLockStrikes = 64

// strikeDecay is how long an idle key keeps its escalation strikes. After this
// long without any failure the key's state is garbage-collected and its next
// lockout trip restarts at the base Lockout. It must exceed maxLockout with
// headroom so an attacker cannot reset escalation by simply waiting out a
// 24h lock; a week of silence is treated as a fresh start.
const strikeDecay = 7 * 24 * time.Hour

// pruneInterval amortizes the full-map garbage collection so Allow/Failure
// stay O(1) per key in the common case. Overridable in tests.
var pruneInterval = time.Minute

func NewLimiter(now func() time.Time) *Limiter {
	return &Limiter{
		now:           now,
		fails:         make(map[string][]time.Time),
		lockedUntil:   make(map[string]time.Time),
		lockStrikes:   make(map[string]int),
		lastFailure:   make(map[string]time.Time),
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
	l.pruneStaleLocked(now)

	l.globalFails = prune(l.globalFails, now.Add(-time.Hour))
	if len(l.globalFails) >= l.GlobalPerHour {
		oldest := l.globalFails[0]
		return oldest.Add(time.Hour).Sub(now), false
	}

	windowStart := now.Add(-l.FailWindow)
	if kept := prune(l.fails[key], windowStart); len(kept) == 0 {
		// No residue for allowed keys: a bare Allow must not pin the map entry.
		delete(l.fails, key)
	} else {
		l.fails[key] = kept
	}
	if len(l.fails[key]) >= l.MaxFails {
		// MaxFails failures inside the window but lock expired → fresh window.
		delete(l.fails, key)
	}
	return 0, true
}

// Failure records a failed attempt. It returns whether this failure tripped
// the lockout (locked=true) and the fresh lock duration, so callers can emit
// exactly one lockout audit event per trip instead of one per rejected retry.
// Tripping MaxFails locks the key with an escalated duration
// (Lockout << strikes, capped at maxLockout).
func (l *Limiter) Failure(key string) (locked bool, lockDuration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.globalFails = append(prune(l.globalFails, now.Add(-time.Hour)), now)
	windowStart := now.Add(-l.FailWindow)
	l.fails[key] = append(prune(l.fails[key], windowStart), now)
	l.lastFailure[key] = now
	if len(l.fails[key]) >= l.MaxFails {
		if l.lockStrikes[key] < maxLockStrikes {
			l.lockStrikes[key]++
		}
		lockDuration = escalatedLockout(l.Lockout, l.lockStrikes[key])
		l.lockedUntil[key] = now.Add(lockDuration)
		delete(l.fails, key)
		l.pruneStaleLocked(now)
		return true, lockDuration
	}
	l.pruneStaleLocked(now)
	return false, 0
}

// Success clears a key's failure state and resets the escalation strike count
// (a successful login restores the base lockout for the next trip).
func (l *Limiter) Success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
	delete(l.lockedUntil, key)
	delete(l.lockStrikes, key)
	delete(l.lastFailure, key)
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

// pruneStaleLocked garbage-collects per-key state at most once per
// pruneInterval so a long-running instance never grows its maps without
// bound. It drops (1) expired locks, (2) failure windows that slid empty, and
// (3) escalation strikes for keys idle longer than strikeDecay. Callers hold
// l.mu.
func (l *Limiter) pruneStaleLocked(now time.Time) {
	if !l.lastPrune.IsZero() && now.Sub(l.lastPrune) < pruneInterval {
		return
	}
	l.lastPrune = now

	for key, until := range l.lockedUntil {
		if !now.Before(until) {
			delete(l.lockedUntil, key)
		}
	}
	windowStart := now.Add(-l.FailWindow)
	for key, times := range l.fails {
		if kept := prune(times, windowStart); len(kept) == 0 {
			delete(l.fails, key)
		} else {
			l.fails[key] = kept
		}
	}
	for key, last := range l.lastFailure {
		if _, activeLock := l.lockedUntil[key]; activeLock {
			continue // locked: strikes must survive until the lock lapses.
		}
		if _, counting := l.fails[key]; counting {
			continue // failures still inside the counting window.
		}
		if now.Sub(last) < strikeDecay {
			continue // recent enough that escalation must carry over.
		}
		delete(l.lastFailure, key)
		delete(l.lockStrikes, key)
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
