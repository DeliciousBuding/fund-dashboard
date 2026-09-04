package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

// /mcp 认证前 per-IP 粗限流（FUND_MCP_PREAUTH_RPM）：随机 Bearer 洪水在到达
// ECDSA 验签之前就被 429 挡住；认证后的 per-key 精桶（FUND_MCP_RPM）行为不变。

// preAuthTestCfg copies testCfg with a tiny pre-auth rate so the flood tests can
// exhaust the bucket quickly (burst is fixed at 60 inside the router, so the
// 61st request per IP within the minute must 429).
func preAuthTestCfg() config.Config {
	cfg := testCfg()
	cfg.MCPPreAuthRPM = 1
	return cfg
}

// postMCP sends one /mcp POST from an explicit source address with an optional
// Authorization header.
func postMCP(t *testing.T, router http.Handler, remoteAddr, auth string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.RemoteAddr = remoteAddr
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

// TestMCPPreAuthRateLimitRejectsUnauthenticatedFlood pins the pre-auth ordering:
// unauthenticated requests are 401 while the per-IP bucket holds tokens, then
// the limiter answers 429 {"error":"rate_limited"} with Retry-After — the spray
// never reaches the ECDSA verify path.
func TestMCPPreAuthRateLimitRejectsUnauthenticatedFlood(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(preAuthTestCfg(), WithDB(db))

	saw429 := false
	for i := 0; i < 62; i++ {
		res := postMCP(t, router, "192.0.2.10:4444", "Bearer not-a-real-token")
		if res.Code == http.StatusTooManyRequests {
			saw429 = true
			if res.Header().Get("Retry-After") == "" {
				t.Fatal("429 must carry Retry-After")
			}
			if res.Body.String() != `{"error":"rate_limited"}`+"\n" {
				t.Fatalf("429 body = %q, want rate_limited envelope", res.Body.String())
			}
			continue
		}
		if saw429 {
			t.Fatalf("request %d = %d after the limit tripped, want 429", i, res.Code)
		}
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("request %d = %d, want 401 while the bucket still has tokens", i, res.Code)
		}
	}
	if !saw429 {
		t.Fatal("expected 429 once the pre-auth per-IP bucket drained")
	}
}

// TestMCPPreAuthRateLimitBucketIsPerIP pins that draining one source address
// does not touch another: the ceiling is per-IP, so one flood source cannot lock
// out a different client.
func TestMCPPreAuthRateLimitBucketIsPerIP(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(preAuthTestCfg(), WithDB(db))

	for i := 0; i < 60; i++ {
		res := postMCP(t, router, "192.0.2.10:4444", "Bearer nope")
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("attacker request %d = %d, want 401 during the drain", i, res.Code)
		}
	}
	if res := postMCP(t, router, "192.0.2.10:4444", "Bearer nope"); res.Code != http.StatusTooManyRequests {
		t.Fatalf("drained IP status = %d, want 429", res.Code)
	}
	if res := postMCP(t, router, "192.0.2.20:5555", "Bearer nope"); res.Code != http.StatusUnauthorized {
		t.Fatalf("fresh IP status = %d, want 401 (own per-IP bucket, not the drained one)", res.Code)
	}
}

// TestMCPPreAuthRateLimitKeepsPerKeyBucketAfterAuth pins that a legitimate
// operator is unaffected by the coarse layer while under it, and that the
// post-auth per-key bucket (FUND_MCP_RPM) still gates authenticated traffic:
// 61 valid-key requests from 61 different IPs exhaust the key bucket (burst 60)
// while every per-IP pre-auth bucket stays empty.
func TestMCPPreAuthRateLimitKeepsPerKeyBucketAfterAuth(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()
	router := NewRouter(preAuthTestCfg(), WithDB(db))

	var last *httptest.ResponseRecorder
	for i := 1; i <= 61; i++ {
		last = postMCP(t, router, fmt.Sprintf("192.0.2.%d:%d", i, 40000+i), "Bearer "+testAdminKey)
		if i < 61 {
			if last.Code != http.StatusOK {
				t.Fatalf("valid request %d = %d, want 200; body=%s", i, last.Code, last.Body.String())
			}
			continue
		}
		if last.Code != http.StatusTooManyRequests {
			t.Fatalf("61st valid-key request from a fresh IP = %d, want 429 from the per-key bucket", last.Code)
		}
	}
	if last.Header().Get("Retry-After") == "" {
		t.Fatal("per-key 429 must carry Retry-After")
	}
}
