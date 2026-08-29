package httpapi

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer catches handler panics, logs the stack with the request id, and
// answers with the standard JSON error shape instead of dropping the
// connection. Registered first in the global chain so every later middleware
// and handler is covered.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("handler panic",
					"request_id", RequestIDFromContext(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"panic", rec,
					"stack", string(debug.Stack()),
				)
				// Best effort: headers may already be written (e.g. SSE streams).
				WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
