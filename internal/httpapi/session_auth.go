package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/DeliciousBuding/fund-dashboard/internal/auth"
)

type sessionContextKey struct{}

// SessionFromContext returns the authenticated session attached by
// SessionAuth / BrowserWriteAuth, or nil.
func SessionFromContext(ctx context.Context) *auth.Session {
	sess, _ := ctx.Value(sessionContextKey{}).(*auth.Session)
	return sess
}

// SessionAuth requires a valid session cookie. Fail-closed: a nil service
// (no DB) rejects everything. Unsafe methods additionally pass the Origin
// allowlist and must carry the CSRF header (SameSite=Lax is necessary but not
// sufficient against all cross-site vectors).
func SessionAuth(svc *auth.Service, origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess := sessionFromRequest(r, svc)
			if sess == nil {
				WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			if isUnsafeMethod(r.Method) {
				if !browserMutationAllowed(r, origins) {
					WriteJSON(w, http.StatusForbidden, map[string]any{"error": "origin_not_allowed"})
					return
				}
				if r.Header.Get(csrfHeader) != csrfHeaderValue {
					WriteJSON(w, http.StatusForbidden, map[string]any{"error": "csrf_header_required"})
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, sess)))
		})
	}
}

// BrowserWriteAuth guards browser mutation routes. Authentication: valid
// session cookie (preferred), or the legacy edge-injected EdgeKey while
// edgeAuthEnabled. On unsafe methods it additionally enforces the
// Origin/Sec-Fetch-Site allowlist, and session-authenticated requests must
// carry the CSRF header.
func BrowserWriteAuth(svc *auth.Service, edgeKey string, edgeAuthEnabled bool, origins []string) func(http.Handler) http.Handler {
	edgeKey = strings.TrimSpace(edgeKey)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			viaSession := false
			ctx := r.Context()
			if sess := sessionFromRequest(r, svc); sess != nil {
				viaSession = true
				ctx = context.WithValue(ctx, sessionContextKey{}, sess)
			} else {
				provided := strings.TrimSpace(r.Header.Get(edgeKeyHeader))
				if !edgeAuthEnabled || edgeKey == "" || len(provided) != len(edgeKey) ||
					subtle.ConstantTimeCompare([]byte(provided), []byte(edgeKey)) != 1 {
					WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
					return
				}
			}
			if isUnsafeMethod(r.Method) {
				if !browserMutationAllowed(r, origins) {
					WriteJSON(w, http.StatusForbidden, map[string]any{"error": "origin_not_allowed"})
					return
				}
				if viaSession && r.Header.Get(csrfHeader) != csrfHeaderValue {
					WriteJSON(w, http.StatusForbidden, map[string]any{"error": "csrf_header_required"})
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
