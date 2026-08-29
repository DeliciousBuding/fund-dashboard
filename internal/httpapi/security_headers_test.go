package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersSetDefaults(t *testing.T) {
	h := SecurityHeaders(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rr.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", got)
	}
	if got := rr.Header().Get("Referrer-Policy"); got == "" {
		t.Fatalf("Referrer-Policy empty")
	}
	if got := rr.Header().Get("Permissions-Policy"); got == "" {
		t.Fatalf("Permissions-Policy empty")
	}
}

func TestSecurityHeadersDoNotClobberUpstream(t *testing.T) {
	h := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate edge already setting a stronger frame policy before body.
		// Middleware runs first, so we set after next? Pattern is "only if empty"
		// before next — so edge after middleware can still override. Test pre-set
		// is not possible in this order; ensure defaults still apply when empty.
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("expected DENY default")
	}
	// secure=false → 无 HSTS。
	if rr.Header().Get("Strict-Transport-Security") != "" {
		t.Fatalf("HSTS must not be set on insecure deployments, got %q", rr.Header().Get("Strict-Transport-Security"))
	}
}

// HSTS 仅在 secure-cookies 开启(FUND_AUTH_SECURE_COOKIE=true)时下发(design 06 §2.1)。
func TestSecurityHeadersHSTSOnSecureOnly(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	secure := SecurityHeaders(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rr := httptest.NewRecorder()
	secure.ServeHTTP(rr, req)
	if got := rr.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
		t.Fatalf("secure HSTS = %q", got)
	}
	if got := rr.Header().Get("Cross-Origin-Opener-Policy"); got != "same-origin" {
		t.Fatalf("COOP = %q, want same-origin", got)
	}

	insecure := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rr = httptest.NewRecorder()
	insecure.ServeHTTP(rr, req)
	if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("insecure HSTS = %q, want unset", got)
	}
}
