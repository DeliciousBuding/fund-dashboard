package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// 通用 API 限流(design 06 §2.3):令牌桶 per-key、429 + Retry-After、不睡眠。

func TestRateLimiterAllowsBurstThenRejectsWithRetryAfter(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	rl := NewRateLimiter(2, 2) // 2 次/分钟,burst 2
	rl.now = func() time.Time { return now }

	if _, ok := rl.Allow("k:1"); !ok {
		t.Fatal("burst token 1 should pass")
	}
	if _, ok := rl.Allow("k:1"); !ok {
		t.Fatal("burst token 2 should pass")
	}
	retryAfter, ok := rl.Allow("k:1")
	if ok {
		t.Fatal("3rd request must be rejected")
	}
	// 令牌桶需要 1 个 token:rate=2/60 per sec → 30s
	if retryAfter <= 0 || retryAfter > 31*time.Second {
		t.Fatalf("retryAfter = %v, want ~30s", retryAfter)
	}

	// 30 秒后再试:补充 1 个 token → 通过。
	now = now.Add(30 * time.Second)
	if _, ok := rl.Allow("k:1"); !ok {
		t.Fatal("after refill the request must pass")
	}

	// 不同 key 不受影响(per-key)。
	if _, ok := rl.Allow("k:2"); !ok {
		t.Fatal("other key must have its own bucket")
	}
}

func TestRateLimiterSweepsIdleBuckets(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	rl := NewRateLimiter(2, 2)
	rl.now = func() time.Time { return now }
	// 耗干一个桶。
	rl.Allow("idle:1")
	rl.Allow("idle:1")
	if _, ok := rl.Allow("idle:1"); ok {
		t.Fatal("expected drained")
	}
	// 推进超过 sweep 间隔:此时 idle:1 已按 2/分钟 补满到 burst(等价于全新桶)
	// → 作为空闲桶被删除;新 touch 的桶存活。
	now = now.Add(5*time.Minute + time.Second)
	rl.Allow("fresh:1")
	rl.mu.Lock()
	_, idleExists := rl.buckets["idle:1"]
	freshExists := rl.buckets["fresh:1"] != nil
	rl.mu.Unlock()
	if idleExists {
		t.Fatal("fully replenished (idle) bucket must be swept")
	}
	if !freshExists {
		t.Fatal("recently touched bucket must survive the sweep")
	}
}

func TestRateLimiterSweepKeepsPartiallyRefilledBuckets(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	rl := NewRateLimiter(1, 60) // 1 次/分钟、burst 60 → 补满需 60 分钟
	rl.now = func() time.Time { return now }
	for i := 0; i < 60; i++ {
		if _, ok := rl.Allow("hot:1"); !ok {
			t.Fatalf("burst token %d should pass", i)
		}
	}
	if _, ok := rl.Allow("hot:1"); ok {
		t.Fatal("61st request must be rejected")
	}
	// 推进超过 sweep 间隔:桶只补了约 5 个 token,未到 burst → 必须保留,
	// 否则限流中的攻击者等一次 sweep 就能原地满血复活。
	now = now.Add(5*time.Minute + time.Second)
	if _, ok := rl.Allow("cold:1"); !ok {
		t.Fatal("cold key must pass and trigger the sweep")
	}
	rl.mu.Lock()
	_, exists := rl.buckets["hot:1"]
	rl.mu.Unlock()
	if !exists {
		t.Fatal("partially refilled bucket must survive the sweep")
	}
	// 限流状态延续:约 5 个补充 token → 接下来只放行约 5 个,随后继续拒绝
	// (若桶被误删重建,会一次性放行整个 60 burst)。
	passed := 0
	for i := 0; i < 10; i++ {
		if _, ok := rl.Allow("hot:1"); ok {
			passed++
		}
	}
	if passed == 0 {
		t.Fatal("refilled tokens must be usable")
	}
	if passed >= 10 {
		t.Fatalf("drained bucket regained %d tokens, want ~5 after 5min at 1/min", passed)
	}
}
func TestRateLimitMiddlewareReturns429WithRetryAfter(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 每分钟 1 次,burst 1;第二个请求必 429
	handler := RateLimit(rl, func(r *http.Request) string { return "ip:1" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if i == 0 {
			if rr.Code != http.StatusNoContent {
				t.Fatalf("first = %d, want 204", rr.Code)
			}
			continue
		}
		if rr.Code != http.StatusTooManyRequests {
			t.Fatalf("second = %d, want 429", rr.Code)
		}
		if rr.Header().Get("Retry-After") == "" {
			t.Fatal("429 must carry Retry-After")
		}
		if rr.Body.String() != `{"error":"rate_limited"}`+"\n" {
			t.Fatalf("body = %q", rr.Body.String())
		}
	}
}

func TestRouterAPIRateLimitRejectsAfterBurst(t *testing.T) {
	// 真实路由 burst 60(NewRouter 内部固定):前 60 个请求通过,第 61 个 429。
	router := NewRouter(testCfg())
	saw429 := false
	for i := 0; i < 62; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			saw429 = true
			if rr.Header().Get("Retry-After") == "" {
				t.Fatal("429 must carry Retry-After")
			}
			break
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("attempt %d = %d, want 200", i, rr.Code)
		}
	}
	if !saw429 {
		t.Fatal("expected 429 after burst of 60")
	}
}

func TestMCPAuthFailureDoesNotBurnRateLimit(t *testing.T) {
	// 限流挂在 MCPAuth 之后(chi 顺序):401 请求不计费 —— 65 次无 Bearer 全部
	// 401,绝无 429(若计费,第 61 个会 429)。
	db := openPortfolioHTTPFixture(t)
	t.Cleanup(func() { db.Close() })
	router := NewRouter(testCfg(), WithDB(db))
	for i := 0; i < 65; i++ {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401 (must not be rate limited)", i, rr.Code)
		}
	}
}
