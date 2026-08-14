package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("json marshal failed", "error", err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		if _, writeErr := w.Write([]byte(`{"error":"internal_error"}` + "\n")); writeErr != nil {
			slog.Error("json write failed", "error", writeErr)
		}
		return
	}
	// Match encoding/json.Encoder, which appends a trailing newline.
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, writeErr := w.Write(body); writeErr != nil {
		slog.Error("json write failed", "error", writeErr)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

// writeSafeError maps internal errors to stable client messages for public/semi-public
// handlers. Full error is logged server-side with request_id; clients never see SQL/stack dumps (#202).
func writeSafeError(w http.ResponseWriter, r *http.Request, status int, err error) {
	rid := ""
	if r != nil {
		rid = RequestIDFromContext(r.Context())
	}
	if err != nil {
		slog.Error("http handler error",
			"request_id", rid,
			"status", status,
			"path", safePath(r),
			"error", err.Error(),
		)
	}
	msg := clientErrorMessage(status, err)
	writeError(w, status, msg)
}

func safePath(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.URL.Path
}

func clientErrorMessage(status int, err error) string {
	// 4xx: prefer short stable codes when error already looks intentional.
	if err != nil {
		m := strings.TrimSpace(err.Error())
		low := strings.ToLower(m)
		// Pass through known short domain messages for validation.
		if status >= 400 && status < 500 {
			if looksSafeClientMessage(m, low) {
				return m
			}
			switch status {
			case http.StatusBadRequest:
				return "bad_request"
			case http.StatusUnauthorized:
				return "unauthorized"
			case http.StatusForbidden:
				return "forbidden"
			case http.StatusNotFound:
				return "not_found"
			case http.StatusConflict:
				return "conflict"
			default:
				return "request_failed"
			}
		}
	}
	if status == http.StatusBadGateway {
		return "upstream_unavailable"
	}
	if status >= 500 {
		return "internal_error"
	}
	return "request_failed"
}

func looksSafeClientMessage(m, low string) bool {
	if m == "" || len(m) > 120 {
		return false
	}
	// Technical dumps / SQL / driver noise
	if strings.Contains(low, "sql:") ||
		strings.Contains(low, "pq:") ||
		strings.Contains(low, "postgres") ||
		strings.Contains(low, "driver:") ||
		strings.Contains(low, "stack") ||
		strings.Contains(low, "panic") ||
		strings.Contains(low, "\n") ||
		strings.Contains(low, "at path") ||
		strings.Contains(low, "connection refused") ||
		strings.Contains(low, "timeout") && strings.Contains(low, "dial") {
		return false
	}
	// Prefer code-like messages (snake_case / short words)
	if strings.Contains(m, " ") && !strings.Contains(m, ":") {
		// free-form sentences may still be OK if short and not technical
		if strings.ContainsAny(m, "{}[]") {
			return false
		}
	}
	return true
}
