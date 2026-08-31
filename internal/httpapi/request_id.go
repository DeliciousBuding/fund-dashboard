package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

type requestIDContextKey struct{}

// RequestID middleware ensures every request has an X-Request-Id.
// Existing client-provided IDs are kept (trimmed, length-capped); otherwise a
// random 16-byte hex id is generated. The value is written to the response
// header and stored on the request context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := sanitizeRequestID(r.Header.Get("X-Request-Id"))
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request id if present.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

// sanitizeRequestID accepts only printable ASCII (no control chars, no
// whitespace) so a client-supplied X-Request-Id cannot inject into response
// headers or structured logs. Anything unsafe yields "" → a fresh ID is issued.
func sanitizeRequestID(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) > 128 {
		raw = raw[:128]
	}
	for _, c := range raw {
		if c < 0x21 || c > 0x7e {
			return ""
		}
	}
	return raw
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b[:])
}
