package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddlewareGeneratesAndEchoes(t *testing.T) {
	var seen string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	// No inbound header → generate.
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Header().Get("X-Request-Id") == "" {
		t.Fatalf("expected generated X-Request-Id header")
	}
	if seen == "" || seen != res.Header().Get("X-Request-Id") {
		t.Fatalf("context id = %q, header = %q", seen, res.Header().Get("X-Request-Id"))
	}

	// Inbound header → preserve.
	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("X-Request-Id", "client-req-123")
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Header().Get("X-Request-Id") != "client-req-123" {
		t.Fatalf("header = %q, want client-req-123", res.Header().Get("X-Request-Id"))
	}
	if seen != "client-req-123" {
		t.Fatalf("context id = %q, want client-req-123", seen)
	}
}

func TestRouterHealthIncludesRequestIDHeader(t *testing.T) {
	router := NewRouter(testCfg())
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("X-Request-Id", "health-check-id")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if res.Header().Get("X-Request-Id") != "health-check-id" {
		t.Fatalf("X-Request-Id = %q, want health-check-id", res.Header().Get("X-Request-Id"))
	}
}
