package httpapi

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimiter is an in-memory per-key token bucket (design
// docs/design/06-security-hardening.md §2.3). Rate is tokens/second, burst is
// the bucket capacity. Allow never sleeps — a full bucket returns the retry
// duration and handlers respond 429 immediately.
//
// Single-instance / single-tenant deployment: restart resets all buckets,
// which is acceptable. Idle buckets are swept every 5 minutes so the map
// cannot grow unbounded.
type RateLimiter struct {
	mu        sync.Mutex
	now       func() time.Time
	rate      float64 // tokens per second
	burst     float64
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// sweepInterval is how often idle buckets are garbage-collected.
const sweepInterval = 5 * time.Minute

// NewRateLimiter builds a limiter replenishing ratePerMinute tokens per minute
// with burst capacity.
func NewRateLimiter(ratePerMinute, burst float64) *RateLimiter {
	if ratePerMinute <= 0 {
		ratePerMinute = 1
	}
	if burst <= 0 {
		burst = 1
	}
	return &RateLimiter{
		now:       time.Now,
		rate:      ratePerMinute / 60,
		burst:     burst,
		buckets:   map[string]*bucket{},
		lastSweep: time.Time{}, // zero: first Allow triggers an immediate sweep
	}
}

// Allow consumes one token for key. On success ok=true. On failure ok=false and
// retryAfter is how long the caller must wait for at least one token.
func (rl *RateLimiter) Allow(key string) (retryAfter time.Duration, ok bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.now()
	rl.sweep(now)

	b, exists := rl.buckets[key]
	if !exists {
		b = &bucket{tokens: rl.burst, last: now}
		rl.buckets[key] = b
	}
	// Refill from elapsed time.
	if d := now.Sub(b.last).Seconds(); d > 0 {
		b.tokens = math.Min(rl.burst, b.tokens+d*rl.rate)
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return 0, true
	}
	// Need (1-tokens) tokens at rate rate → seconds until allowed.
	wait := (1 - b.tokens) / rl.rate
	return time.Duration(wait * float64(time.Second)), false
}

// sweep deletes buckets that have drained to zero tokens (idle keys). Runs at
// most every sweepInterval; called under rl.mu.
func (rl *RateLimiter) sweep(now time.Time) {
	if now.Sub(rl.lastSweep) < sweepInterval {
		return
	}
	rl.lastSweep = now
	for key, b := range rl.buckets {
		if b.tokens <= 0 {
			delete(rl.buckets, key)
		}
	}
}

// RateLimit is the middleware form: 429 {"error":"rate_limited"} + Retry-After
// when the key's bucket is drained. keyFn derives the per-request key (client
// IP, bearer-hash, etc.).
func RateLimit(rl *RateLimiter, keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			retryAfter, ok := rl.Allow(keyFn(r))
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
				WriteJSON(w, http.StatusTooManyRequests, map[string]any{"error": "rate_limited"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitExpensive applies a second, stricter bucket only to the listed
// paths (design 06 §2.3: 60/min for crawl/report/export/DCA/adjust heavy
// endpoints). It composes inside the API group after the global per-IP limiter,
// so heavy requests consume both buckets. keyFn is the same per-IP key as the
// global layer.
func rateLimitExpensive(rl *RateLimiter, paths []string, keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	byPath := make(map[string]bool, len(paths))
	for _, p := range paths {
		byPath[p] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if byPath[r.URL.Path] {
				if retryAfter, ok := rl.Allow(keyFn(r)); !ok {
					w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
					WriteJSON(w, http.StatusTooManyRequests, map[string]any{"error": "rate_limited"})
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// expensiveAPIPaths are the heavy endpoints that get the 60/min extra bucket.
// /api/export/* is represented by the actual export route; /api/transactions/import,
// /api/reports, /api/dca/run and /api/portfolio/adjust-position are matched exactly.
var expensiveAPIPaths = []string{
	"/api/transactions/import",
	"/api/reports",
	"/api/export/transactions-xlsx",
	"/api/dca/run",
	"/api/portfolio/adjust-position",
}
