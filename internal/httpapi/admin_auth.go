package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AdminAuth returns middleware that protects /api/admin/* routes.
// It validates Bearer tokens against the configured MCP_API_KEY.
// If no key is configured, admin routes are disabled (401 for all requests).
func AdminAuth(adminKey string) func(http.Handler) http.Handler {
	adminKey = strings.TrimSpace(adminKey)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if adminKey == "" {
				WriteJSON(w, http.StatusUnauthorized, map[string]any{
					"error": "unauthorized",
				})
				return
			}
			token := bearerToken(r.Header.Get("Authorization"))
			if token == "" || len(token) != len(adminKey) ||
				subtle.ConstantTimeCompare([]byte(token), []byte(adminKey)) != 1 {
				WriteJSON(w, http.StatusUnauthorized, map[string]any{
					"error": "unauthorized",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
