package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEdgeAuthRejectsMissingKey(t *testing.T) {
	h := EdgeAuth(testEdgeKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/transactions", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rr.Code)
	}
}

func TestEdgeAuthAllowsSameOriginBrowser(t *testing.T) {
	h := EdgeAuth(testEdgeKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/transactions", nil)
	req.Header.Set(edgeKeyHeader, testEdgeKey)
	req.Header.Set("Origin", "https://fund.vectorcontrol.tech")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204 body=%s", rr.Code, rr.Body.String())
	}
}

func TestEdgeAuthRejectsCrossSite(t *testing.T) {
	h := EdgeAuth(testEdgeKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestEdgeAuthRejectsBadOriginEvenWithoutSecFetch(t *testing.T) {
	h := EdgeAuth(testEdgeKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestEdgeAuthAllowsNonBrowserNoOrigin(t *testing.T) {
	// curl / ops tooling with EdgeKey and no Origin header.
	h := EdgeAuth(testEdgeKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestEdgeAuthAllowsLocalhostOrigin(t *testing.T) {
	h := EdgeAuth(testEdgeKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/transactions", nil)
	req.Header.Set(edgeKeyHeader, testEdgeKey)
	req.Header.Set("Origin", "http://127.0.0.1:5176")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rr.Code)
	}
}
