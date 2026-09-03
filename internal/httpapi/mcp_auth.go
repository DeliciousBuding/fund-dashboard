package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/oauth"
)

// mcpAuthScope is stored on the request context after successful MCP auth.
type mcpAuthScope struct {
	Role agenttools.Role
	// Key identifies the credential kind for logs and the per-key rate limiter:
	// "admin" (MCP_API_KEY), "public" (PUBLIC_MCP_KEY) or "oauth".
	Key string
	// ClientID is set for OAuth tokens only, so audit rows can attribute a call
	// to the connector that made it.
	ClientID string
}

type mcpAuthContextKey struct{}

func withMCPAuth(ctx context.Context, scope mcpAuthScope) context.Context {
	return context.WithValue(ctx, mcpAuthContextKey{}, scope)
}

func mcpAuthFromContext(ctx context.Context) (mcpAuthScope, bool) {
	scope, ok := ctx.Value(mcpAuthContextKey{}).(mcpAuthScope)
	return scope, ok
}

// MCPAuth protects /mcp.
//
// Two credential kinds are accepted, and both fail closed:
//
//   - Static bearer keys. MCP_API_KEY → RoleOperator, PUBLIC_MCP_KEY →
//     RoleAnalyst. This is the pre-existing contract and must keep working
//     unchanged for current operators.
//   - OAuth 2.1 access tokens (ES256 JWT, audience-bound to this MCP resource
//     URL) issued by the built-in authorization server. The granted scope maps
//     onto the same role matrix: fund.read → analyst, fund.write → operator.
//
// When neither static key is configured and OAuth is off, every request is
// rejected. When a request is rejected the response carries WWW-Authenticate
// with the RFC 9728 protected-resource metadata URL — that header is how an MCP
// client discovers it should start an OAuth flow instead of reporting a plain
// "server refused me" failure.
func MCPAuth(adminKey, publicKey string, oauthSvc *oauth.Service) func(http.Handler) http.Handler {
	adminKey = strings.TrimSpace(adminKey)
	publicKey = strings.TrimSpace(publicKey)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r.Header.Get("Authorization"))
			if token == "" {
				rejectMCP(w, r, oauthSvc, "missing_token", "authorization header is required")
				return
			}

			if scope, ok := matchMCPKey(token, adminKey, publicKey); ok {
				next.ServeHTTP(w, r.WithContext(withMCPAuth(r.Context(), scope)))
				return
			}

			if oauthSvc != nil {
				scope, err := verifyMCPOAuthToken(r, oauthSvc, token)
				if err == nil {
					next.ServeHTTP(w, r.WithContext(withMCPAuth(r.Context(), scope)))
					return
				}
				rejectMCP(w, r, oauthSvc, "invalid_token", err.Error())
				return
			}

			if adminKey == "" && publicKey == "" {
				rejectMCP(w, r, nil, "unauthorized", "no MCP credentials are configured")
				return
			}
			rejectMCP(w, r, oauthSvc, "invalid_token", "token is not valid")
		})
	}
}

// verifyMCPOAuthToken validates an access token against this resource server.
// Issuer and audience are both checked, so a token minted for a different
// service that happens to share a key cannot be replayed here.
func verifyMCPOAuthToken(r *http.Request, svc *oauth.Service, token string) (mcpAuthScope, error) {
	issuer := resolveOAuthIssuer(r, svc)
	resource := svc.Resource(issuer)
	verified, err := svc.VerifyAccessToken(token, issuer, resource)
	if err != nil {
		return mcpAuthScope{}, err
	}
	if len(verified.Scopes) == 0 {
		return mcpAuthScope{}, errors.New("token carries no scope")
	}
	role, ok := mapOAuthRole(oauth.RoleForScopes(verified.Scopes))
	if !ok {
		return mcpAuthScope{}, errors.New("token scope does not map to a known role")
	}
	// A write-scoped token is only honoured when this deployment advertises
	// fund.write at all. This is belt-and-braces: NegotiateScopes already refuses
	// to grant it, so a token can only reach here if the flag was turned off
	// after the token was issued.
	if role == agenttools.RoleOperator && !svc.Options().AllowWriteScope {
		role = agenttools.RoleAnalyst
	}
	return mcpAuthScope{Role: role, Key: "oauth", ClientID: verified.ClientID}, nil
}

// mapOAuthRole translates the OAuth role name into the agenttools role. Keeping
// the translation here means internal/oauth never imports agenttools.
func mapOAuthRole(name string) (agenttools.Role, bool) {
	switch name {
	case "analyst":
		return agenttools.RoleAnalyst, true
	case "operator":
		return agenttools.RoleOperator, true
	default:
		return "", false
	}
}

// rejectMCP writes the 401 body plus the discovery hint an MCP client needs.
func rejectMCP(w http.ResponseWriter, r *http.Request, svc *oauth.Service, code, description string) {
	if svc != nil {
		issuer := resolveOAuthIssuer(r, svc)
		// MCP's authorization spec uses RFC 8615 path-aware well-known URLs, so
		// the advertised metadata location carries the resource path suffix.
		paths := oauth.WellKnownPathProtectedResource(svc.Options().ResourcePath)
		metadataURL := issuer + paths[len(paths)-1]
		challenge := `Bearer resource_metadata="` + metadataURL + `", error="` + code + `", error_description="` + sanitizeChallengeDescription(description) + `"`
		// The challenge scope tells the MCP client which scopes it must request
		// during authorization. Without it ChatGPT requests only the first
		// supported scope (fund.read), which strands write-capable connectors on
		// the analyst role. Advertising the full supported set lets a connector
		// ask for read+write in one pass; a read-only deployment advertises only
		// fund.read.
		if scopes := svc.Options().ScopesSupported(); len(scopes) > 0 {
			challenge += `, scope="` + strings.Join(scopes, " ") + `"`
		}
		w.Header().Set("WWW-Authenticate", challenge)
	}
	WriteJSON(w, http.StatusUnauthorized, map[string]any{
		"error":             "unauthorized",
		"error_description": description,
	})
}

// sanitizeChallengeDescription keeps the header injectable-free: a challenge
// parameter is a quoted-string, so quotes, backslashes and CR/LF must not survive.
func sanitizeChallengeDescription(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unauthorized"
	}
	replacer := strings.NewReplacer(
		`"`, `'`,
		`\`, `/`,
		"\r", " ",
		"\n", " ",
	)
	value = replacer.Replace(value)
	if len(value) > 160 {
		value = value[:160]
	}
	return value
}

func bearerToken(authHeader string) string {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return ""
	}
	for _, prefix := range []string{"Bearer ", "N_Bearer "} {
		if len(authHeader) > len(prefix) && strings.EqualFold(authHeader[:len(prefix)], prefix) {
			return strings.TrimSpace(authHeader[len(prefix):])
		}
		// A bare scheme with nothing after it is not a credential. Without the
		// length check this falls through to the raw-token branch and returns the
		// literal word "Bearer" as a token.
		if strings.EqualFold(authHeader, strings.TrimSpace(prefix)) {
			return ""
		}
	}
	// Accept a raw token without any scheme for clients that already strip it.
	// N_Bearer is MCP's experimental token type and is handled above.
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
