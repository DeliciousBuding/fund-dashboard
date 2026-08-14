package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLogEmitsStructuredFields(t *testing.T) {
	var captured []string
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&captureWriter{lines: &captured}, nil)))

	h := RequestID(AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hi"))
	})))

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("User-Agent", "vitest")
	req.Header.Set("X-Request-Id", "rid-test")
	req.Header.Set("Authorization", "Bearer sk-should-not-appear")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d", rec.Code)
	}
	joined := strings.Join(captured, "\n")
	for _, want := range []string{"http_request", "rid-test", "GET", "/api/health", "status=418"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("log missing %q in %q", want, joined)
		}
	}
	if strings.Contains(joined, "Authorization") || strings.Contains(joined, "sk-should-not-appear") {
		t.Fatalf("log leaked auth material: %s", joined)
	}
}

func TestAccessLogSkipsStaticAssets(t *testing.T) {
	var captured []string
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&captureWriter{lines: &captured}, nil)))

	h := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/assets/index.js", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if len(captured) != 0 {
		t.Fatalf("expected no log for static asset, got %v", captured)
	}
}

type captureWriter struct{ lines *[]string }

func (c *captureWriter) Write(p []byte) (int, error) {
	*c.lines = append(*c.lines, string(p))
	return len(p), nil
}
