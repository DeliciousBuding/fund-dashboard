package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
)

// mcpAuthScope is stored on the request context after successful MCP auth.
type mcpAuthScope struct {
	Role agenttools.Role
	Key  string // "admin" or "public"
}

type mcpAuthContextKey struct{}

func withMCPAuth(ctx context.Context, scope mcpAuthScope) context.Context {
	return context.WithValue(ctx, mcpAuthContextKey{}, scope)
}

func mcpAuthFromContext(ctx context.Context) (mcpAuthScope, bool) {
	scope, ok := ctx.Value(mcpAuthContextKey{}).(mcpAuthScope)
	return scope, ok
}

// MCPAuth protects /mcp with Bearer tokens.
//
// Policy (fail-closed):
//   - If neither MCP_API_KEY nor PUBLIC_MCP_KEY is configured → 401 for all requests.
//   - MCP_API_KEY → RoleOperator (write tools still require confirmation via registry).
//   - PUBLIC_MCP_KEY → RoleAnalyst (read + external_context only).
//   - When both are set, either key is accepted; admin key is preferred if both match.
func MCPAuth(adminKey, publicKey string) func(http.Handler) http.Handler {
	adminKey = strings.TrimSpace(adminKey)
	publicKey = strings.TrimSpace(publicKey)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if adminKey == "" && publicKey == "" {
				WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}

			token := bearerToken(r.Header.Get("Authorization"))
			if token == "" {
				WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}

			scope, ok := matchMCPKey(token, adminKey, publicKey)
			if !ok {
				WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}

			next.ServeHTTP(w, r.WithContext(withMCPAuth(r.Context(), scope)))
		})
	}
}

func bearerToken(authHeader string) string {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(authHeader) >= len(prefix) && strings.EqualFold(authHeader[:len(prefix)], prefix) {
		return strings.TrimSpace(authHeader[len(prefix):])
	}
	// Accept raw token without scheme for clients that already strip it.
	if !strings.Contains(authHeader, " ") {
		return authHeader
	}
	return ""
}

func matchMCPKey(token, adminKey, publicKey string) (mcpAuthScope, bool) {
	// subtle.ConstantTimeCompare panics on length mismatch — guard first.
	if adminKey != "" && len(token) == len(adminKey) &&
		subtle.ConstantTimeCompare([]byte(token), []byte(adminKey)) == 1 {
		return mcpAuthScope{Role: agenttools.RoleOperator, Key: "admin"}, true
	}
	if publicKey != "" && len(token) == len(publicKey) &&
		subtle.ConstantTimeCompare([]byte(token), []byte(publicKey)) == 1 {
		return mcpAuthScope{Role: agenttools.RoleAnalyst, Key: "public"}, true
	}
	return mcpAuthScope{}, false
}
