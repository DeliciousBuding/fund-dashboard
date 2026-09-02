package oauth

import (
	"strings"
)

// WellKnownPathProtectedResource returns every path that must serve RFC 9728
// protected-resource metadata for the given resource path.
//
// MCP's authorization spec follows RFC 8615 path-aware well-known construction,
// so a resource at "/mcp" is described at
// "/.well-known/oauth-protected-resource/mcp". Real clients are inconsistent
// about whether they append the path suffix, so both forms are served — serving
// only one is the single most common cause of a connector silently failing
// discovery and falling back to "no auth".
func WellKnownPathProtectedResource(resourcePath string) []string {
	base := "/.well-known/oauth-protected-resource"
	if suffix := strings.TrimPrefix(resourcePath, "/"); suffix != "" {
		return []string{base, base + "/" + suffix}
	}
	return []string{base}
}

// WellKnownPathAuthorizationServer returns every path that must serve RFC 8414
// authorization-server metadata. "/.well-known/openid-configuration" is included
// because a number of MCP clients probe the OpenID Connect name first.
func WellKnownPathAuthorizationServer(resourcePath string) []string {
	bases := []string{"/.well-known/oauth-authorization-server", "/.well-known/openid-configuration"}
	if suffix := strings.TrimPrefix(resourcePath, "/"); suffix != "" {
		out := make([]string, 0, len(bases)*2)
		for _, base := range bases {
			out = append(out, base, base+"/"+suffix)
		}
		return out
	}
	return bases
}

// ProtectedResourceMetadata is the RFC 9728 document describing the MCP
// resource server. "resource" is the canonical audience a client must request
// and that every access token is bound to.
func (s *Service) ProtectedResourceMetadata(issuer string) map[string]any {
	return map[string]any{
		"resource":                                   s.Resource(issuer),
		"authorization_servers":                      []string{issuer},
		"scopes_supported":                           s.opts.ScopesSupported(),
		"bearer_methods_supported":                   []string{"header"},
		"resource_signing_alg_values_supported":      []string{},
		"tls_client_certificate_bound_access_tokens": false,
		"documentation":                              issuer + "/oauth/about",
	}
}

// AuthorizationServerMetadata is the RFC 8414 document. Every endpoint is
// absolute and derived from the configured public base URL so a client behind a
// proxy never has to guess.
func (s *Service) AuthorizationServerMetadata(issuer string) map[string]any {
	return map[string]any{
		"issuer":                 issuer,
		"authorization_endpoint": issuer + "/oauth/authorize",
		"token_endpoint":         issuer + "/oauth/token",
		"registration_endpoint":  issuer + "/oauth/register",
		"jwks_uri":               issuer + "/oauth/jwks",
		"scopes_supported":       s.opts.ScopesSupported(),
		// OAuth 2.1: authorization code only, PKCE mandatory.
		"response_types_supported":         []string{"code"},
		"response_modes_supported":         []string{"query"},
		"grant_types_supported":            []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported": []string{CodeChallengeMethodS256},
		// Public clients only. Advertising "none" tells a connector not to send a
		// client secret; this server never accepts one.
		"token_endpoint_auth_methods_supported":            []string{"none"},
		"token_endpoint_auth_signing_alg_values_supported": []string{},
		// OpenAI's connector identifies itself with a client_id that is a URL
		// pointing at its metadata document. Advertising support lets it skip
		// dynamic registration.
		"client_id_metadata_document_supported":          true,
		"service_documentation":                          issuer + "/oauth/about",
		"access_token_formats":                           []string{"jwt"},
		"revocation_endpoint":                            issuer + "/oauth/revoke",
		"revocation_endpoint_auth_methods_supported":     []string{"none"},
		"authorization_response_iss_parameter_supported": false,
	}
}

// AboutDocument is a small human/agent-readable description served at
// /oauth/about. It is not part of any RFC, but it gives an operator (or another
// agent) a single URL that explains what this authorization server is for.
func (s *Service) AboutDocument(issuer string) map[string]any {
	return map[string]any{
		"service":      "fund-dashboard",
		"issuer":       issuer,
		"resource":     s.Resource(issuer),
		"mcp_endpoint": s.Resource(issuer),
		"profile":      "OAuth 2.1 authorization code + PKCE (S256), public clients only",
		"scopes":       s.scopesDescription(),
		"discovery": map[string]any{
			"protected_resource":   WellKnownPathProtectedResource(s.opts.ResourcePath),
			"authorization_server": WellKnownPathAuthorizationServer(s.opts.ResourcePath),
		},
		"endpoints": map[string]string{
			"authorize": issuer + "/oauth/authorize",
			"token":     issuer + "/oauth/token",
			"register":  issuer + "/oauth/register",
			"jwks":      issuer + "/oauth/jwks",
			"revoke":    issuer + "/oauth/revoke",
		},
		"notes": "Single-tenant dashboard: the resource owner is whoever can log in. Access tokens are ES256 JWTs audience-bound to the MCP resource URL.",
	}
}

func (s *Service) scopesDescription() []map[string]string {
	out := []map[string]string{{
		"scope":       ScopeRead,
		"role":        "analyst",
		"description": "Read portfolio, holdings, NAV history, analytics and market context.",
	}}
	if s.opts.AllowWriteScope {
		out = append(out, map[string]string{
			"scope":       ScopeWrite,
			"role":        "operator",
			"description": "Additionally perform write and maintenance tools (each still confirmation-gated).",
		})
	}
	return out
}

// NegotiateScopes intersects the requested scopes with what this deployment
// supports. Unknown scopes are dropped rather than rejected so a client asking
// for "openid profile email" alongside fund.read still gets a usable token; an
// empty intersection falls back to read so the connector is never handed a token
// that authorizes nothing.
func (s *Service) NegotiateScopes(requested string) []string {
	supported := s.opts.ScopesSupported()
	allowed := make(map[string]struct{}, len(supported))
	for _, scope := range supported {
		allowed[scope] = struct{}{}
	}
	var granted []string
	for _, scope := range splitScopes(requested) {
		if _, ok := allowed[scope]; ok {
			granted = append(granted, scope)
		}
	}
	granted = dedupeStrings(granted)
	if len(granted) == 0 {
		return []string{ScopeRead}
	}
	return granted
}

// RoleForScopes maps granted scopes onto the agenttools role used by the MCP
// server. This is the single place where OAuth scope becomes an authorization
// decision, so the resource server and this server cannot drift apart.
func RoleForScopes(scopes []string) string {
	for _, scope := range scopes {
		if scope == ScopeWrite {
			return "operator"
		}
	}
	return "analyst"
}
