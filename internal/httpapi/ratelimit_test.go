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
	// 推进超过 sweep 间隔 → 已耗干的桶被删除(新 touch 的桶存活)。
	now = now.Add(5*time.Minute + time.Second)
	rl.Allow("fresh:1")
	rl.mu.Lock()
	_, idleExists := rl.buckets["idle:1"]
	freshExists := rl.buckets["fresh:1"] != nil
	rl.mu.Unlock()
	if idleExists {
		t.Fatal("drained bucket must be swept")
	}
	if !freshExists {
		t.Fatal("recently touched bucket must survive the sweep")
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
