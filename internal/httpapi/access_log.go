package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// AccessLog logs one structured line per request after completion.
// Does not log Authorization / cookies / bodies (no secret leakage).
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)

		// Skip high-frequency static asset noise; keep SPA shell + API + MCP.
		path := r.URL.Path
		if isStaticAssetPath(path) {
			return
		}

		slog.Info("http_request",
			"request_id", RequestIDFromContext(r.Context()),
			"method", r.Method,
			"path", path,
			"status", ww.status,
			"bytes", ww.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
			"user_agent", truncateUA(r.UserAgent()),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the real connection (SetWriteDeadline for SSE).
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Flush forwards SSE flushes through the access-log wrapper.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func isStaticAssetPath(path string) bool {
	if len(path) >= 8 && path[:8] == "/assets/" {
		return true
	}
	switch path {
	case "/sw.js", "/registerSW.js", "/manifest.json", "/manifest.webmanifest":
		return true
	default:
		return false
	}
}

func truncateUA(ua string) string {
	if len(ua) > 120 {
		return ua[:120]
	}
	return ua
}
