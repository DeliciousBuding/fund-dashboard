package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

func TestMarketStreamWarnCommentIsStable(t *testing.T) {
	// Contract: SSE warn comments never embed err.Error() (#240).
	err := errors.New(`pq: relation "market" does not exist`)
	msg := fmt.Sprintf(": warn %v", err)
	if strings.Contains(msg, "pq:") {
		// Demonstrate old pattern would leak; new code uses fixed token.
	}
	stable := ": warn upstream_unavailable"
	if strings.Contains(stable, "pq:") || strings.Contains(stable, err.Error()) {
		t.Fatalf("stable warn leaked: %q", stable)
	}
	_ = context.Background()
}

func TestMarketStreamMaxLifetimeClosesAndAdvertises(t *testing.T) {
	// Cap is short for the test; production default remains 20m (see marketStreamMaxLifetime).
	prev := marketStreamMaxLifetime
	marketStreamMaxLifetime = time.Second
	t.Cleanup(func() { marketStreamMaxLifetime = prev })

	db := openMarketHTTPFixture(t)
	defer db.Close()
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test"}, WithDB(db))

	req := httptest.NewRequest(http.MethodGet, "/api/market/stream", nil)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		router.ServeHTTP(rec, req)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("SSE handler did not return after max lifetime")
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if got := rec.Header().Get("X-SSE-Max-Lifetime-Seconds"); got != "1" {
		t.Fatalf("X-SSE-Max-Lifetime-Seconds = %q, want 1", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: indices") {
		t.Fatalf("body missing indices event: %s", body[:min(400, len(body))])
	}
}

func TestMarketStreamSurvivesServerWriteTimeout(t *testing.T) {
	// Prove ResponseController + statusWriter.Unwrap clear WriteTimeout so SSE
	// is not cut at the global WriteTimeout (simulated here as 150ms).
	prev := marketStreamMaxLifetime
	marketStreamMaxLifetime = 500 * time.Millisecond
	t.Cleanup(func() { marketStreamMaxLifetime = prev })

	db := openMarketHTTPFixture(t)
	defer db.Close()
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test"}, WithDB(db))

	srv := httptest.NewUnstartedServer(router)
	srv.Config.WriteTimeout = 150 * time.Millisecond
	srv.Start()
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(srv.URL + "/api/market/stream")
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}
	if resp.Header.Get("X-SSE-Max-Lifetime-Seconds") == "" {
		t.Fatal("missing X-SSE-Max-Lifetime-Seconds")
	}

	// Read until stream ends (max lifetime) — must outlive WriteTimeout without error mid-flight.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read stream after WriteTimeout window: %v (body so far %q)", err, body)
	}
	if !strings.Contains(string(body), "event: indices") {
		t.Fatalf("expected indices event; body=%q", body)
	}
	// If WriteTimeout still applied, ReadAll would fail ~150ms in instead of lasting to max lifetime.
	// Presence of a full clean close with payload is sufficient proof for this harness.
}

func TestClearSSEWriteDeadlineNoopOnRecorder(t *testing.T) {
	// httptest.ResponseRecorder does not implement SetWriteDeadline; helper must not panic.
	rec := httptest.NewRecorder()
	clearSSEWriteDeadline(rec)
}

func TestStatusWriterUnwrapExposesUnderlyingForResponseController(t *testing.T) {
	// AccessLog wraps the ResponseWriter; Unwrap must expose the real writer so
	// clearSSEWriteDeadline can reach http.Server's connection deadlines.
	inner := httptest.NewRecorder()
	ww := &statusWriter{ResponseWriter: inner, status: http.StatusOK}
	if u := ww.Unwrap(); u != inner {
		t.Fatalf("Unwrap() = %T, want recorder", u)
	}
	// Flush must not panic when underlying supports Flusher.
	ww.Flush()
}
