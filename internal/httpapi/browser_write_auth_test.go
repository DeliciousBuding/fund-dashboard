package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// BrowserWriteAuth is the browser-mutation guard: session cookie preferred,
// legacy EdgeKey fallback while enabled; Origin allowlist + CSRF header on
// unsafe methods.

func TestWriteAuthRejectsMissingEverything(t *testing.T) {
	h := BrowserWriteAuth(nil, testEdgeKey, true, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/transactions", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rr.Code)
	}
}

func TestWriteAuthEdgeKeyAllowsSameOriginBrowser(t *testing.T) {
	origins := []string{"https://fund.example"}
	h := BrowserWriteAuth(nil, testEdgeKey, true, origins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/transactions", nil)
	req.Header.Set(edgeKeyHeader, testEdgeKey)
	req.Header.Set("Origin", "https://fund.example")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204 body=%s", rr.Code, rr.Body.String())
	}
}

func TestWriteAuthEdgeKeyRejectsCrossSite(t *testing.T) {
	h := BrowserWriteAuth(nil, testEdgeKey, true, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/transactions", nil)
	req.Header.Set(edgeKeyHeader, testEdgeKey)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403 body=%s", rr.Code, rr.Body.String())
	}
}

func TestWriteAuthEdgeKeyRejectsBadOriginWithoutSecFetch(t *testing.T) {
	h := BrowserWriteAuth(nil, testEdgeKey, true, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/transactions", nil)
	req.Header.Set(edgeKeyHeader, testEdgeKey)
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rr.Code)
	}
}

func TestWriteAuthEdgeKeyAllowsNonBrowserNoOrigin(t *testing.T) {
	// curl / ops tooling with EdgeKey and no Origin header.
	h := BrowserWriteAuth(nil, testEdgeKey, true, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/transactions", nil)
	req.Header.Set(edgeKeyHeader, testEdgeKey)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rr.Code)
	}
}

func TestWriteAuthEdgeKeyAllowsLocalhostOrigin(t *testing.T) {
	h := BrowserWriteAuth(nil, testEdgeKey, true, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/transactions", nil)
	req.Header.Set(edgeKeyHeader, testEdgeKey)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rr.Code)
	}
}

func TestWriteAuthEdgeDisabledRejectsKey(t *testing.T) {
	h := BrowserWriteAuth(nil, testEdgeKey, false, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/transactions", nil)
	req.Header.Set(edgeKeyHeader, testEdgeKey)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("edge disabled status=%d want 401", rr.Code)
	}
}
