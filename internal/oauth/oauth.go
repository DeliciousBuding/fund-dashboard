// Package oauth implements the OAuth 2.1 authorization server that fronts the
// fund-dashboard MCP resource server.
//
// Remote MCP clients (ChatGPT custom connectors, Claude, Cursor, …) discover
// this server through RFC 9728 protected-resource metadata and RFC 8414
// authorization-server metadata, register themselves through RFC 7591 dynamic
// client registration (or OpenAI's client-id metadata document), and exchange an
// authorization code + PKCE challenge for a short-lived ES256 access token whose
// audience is pinned to the MCP resource URL.
//
// The resource server side (internal/httpapi.MCPAuth) verifies those tokens
// locally — no database round-trip on the MCP hot path — and maps the granted
// scope onto the existing agenttools role matrix. Static MCP_API_KEY /
// PUBLIC_MCP_KEY bearer auth keeps working unchanged so existing operators
// consumers) are unaffected.
//
// Single-tenant by design: the resource owner is whoever can log in to the
// dashboard, so the authorize endpoint reuses the existing web session cookie
// instead of a user directory.
package oauth

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Scopes advertised to MCP clients. Each maps onto one agenttools role; the
// mapping lives in RoleForScopes so the resource server and the authorization
// server can never disagree about what a token is allowed to do.
const (
	// ScopeRead grants read + external_context tools (agenttools.RoleAnalyst).
	ScopeRead = "fund.read"
	// ScopeWrite additionally grants write + maintenance tools
	// (agenttools.RoleOperator). It is only advertised when explicitly enabled
	// via FUND_OAUTH_ALLOW_WRITE_SCOPE, so a connector cannot silently escalate.
	ScopeWrite = "fund.write"
)

// Subject is the fixed resource-owner identifier embedded in every access
// token. The dashboard is single-tenant, so there is exactly one subject.
const Subject = "fund-owner"

// DefaultResourcePath is the MCP endpoint the access token audience is pinned to.
const DefaultResourcePath = "/mcp"

// OAuth 2.1 / RFC 6749 error codes. Handlers translate these into either a
// redirect back to the client (authorization endpoint) or a JSON body (token
// endpoint).
var (
	ErrInvalidRequest          = errors.New("invalid_request")
	ErrInvalidClient           = errors.New("invalid_client")
	ErrUnauthorizedClient      = errors.New("unauthorized_client")
	ErrAccessDenied            = errors.New("access_denied")
	ErrUnsupportedResponseType = errors.New("unsupported_response_type")
	ErrUnsupportedGrantType    = errors.New("unsupported_grant_type")
	ErrInvalidScope            = errors.New("invalid_scope")
	ErrInvalidRedirectURI      = errors.New("invalid_redirect_uri")
	ErrInvalidGrant            = errors.New("invalid_grant")
	ErrServerError             = errors.New("server_error")
	ErrTemporarilyUnavailable  = errors.New("temporarily_unavailable")
	// ErrLoginRequired is internal: the authorize endpoint turns it into a
	// redirect to the dashboard login page with a return path.
	ErrLoginRequired = errors.New("login_required")
)

// Failure is a structured OAuth error. Redirectable reports whether the error
// may be sent back to the client's redirect_uri (only safe once the redirect
// target itself has been validated).
type Failure struct {
	Code         error
	Description  string
	Status       int
	Redirectable bool
}

func (f *Failure) Error() string {
	if f.Description == "" {
		return f.Code.Error()
	}
	return f.Code.Error() + ": " + f.Description
}

func (f *Failure) Unwrap() error { return f.Code }

// fail builds a non-redirectable failure with the given HTTP status.
func fail(code error, status int, format string, args ...any) *Failure {
	return &Failure{Code: code, Status: status, Description: fmt.Sprintf(format, args...)}
}

// Options configures a Service.
type Options struct {
	// PublicBaseURL is the externally visible origin, e.g.
	// "https://fund.example.com". Every advertised endpoint and the token
	// "iss"/"aud" claims are derived from it. When empty the service falls back
	// to request-derived origins (development convenience only).
	PublicBaseURL string
	// ResourcePath is the MCP endpoint path (default "/mcp"). The access token
	// audience is PublicBaseURL + ResourcePath.
	ResourcePath string
	// SigningKeyPEM optionally pins the ES256 private key (PKCS#8 PEM). When
	// empty the service loads or generates a persistent key in the database.
	SigningKeyPEM string
	// AccessTTL is the access token lifetime (default 1h).
	AccessTTL time.Duration
	// RefreshTTL is the refresh token lifetime (default 720h = 30d).
	RefreshTTL time.Duration
	// CodeTTL is the authorization code lifetime (default 60s).
	CodeTTL time.Duration
	// AutoApprove skips the consent screen for read-only grants when the caller
	// already holds a dashboard session AND has approved that client before —
	// "log in and authorization succeeds" for every authorization after the first.
	// A client's first authorization always shows the consent screen: registration
	// is open, so a silent first grant would be a phishing shortcut to read access.
	AutoApprove bool
	// AllowWriteScope advertises and honours fund.write. Off by default so a
	// connector can never obtain operator powers unless explicitly enabled.
	AllowWriteScope bool
	// CIMDHosts allowlists the hosts permitted for OpenAI client-id metadata
	// documents (default ["chatgpt.com"]). This is an SSRF guard: a client_id
	// that is not allowlisted is never fetched.
	CIMDHosts []string
	// Now is injectable for tests.
	Now func() time.Time
}

func (o Options) withDefaults() Options {
	if strings.TrimSpace(o.ResourcePath) == "" {
		o.ResourcePath = DefaultResourcePath
	}
	if !strings.HasPrefix(o.ResourcePath, "/") {
		o.ResourcePath = "/" + o.ResourcePath
	}
	if o.AccessTTL <= 0 {
		o.AccessTTL = time.Hour
	}
	if o.RefreshTTL <= 0 {
		o.RefreshTTL = 720 * time.Hour
	}
	if o.CodeTTL <= 0 {
		o.CodeTTL = 60 * time.Second
	}
	if len(o.CIMDHosts) == 0 {
		o.CIMDHosts = []string{"chatgpt.com"}
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	o.PublicBaseURL = strings.TrimRight(strings.TrimSpace(o.PublicBaseURL), "/")
	return o
}

// ScopesSupported returns the scopes this deployment advertises.
func (o Options) ScopesSupported() []string {
	if o.AllowWriteScope {
		return []string{ScopeRead, ScopeWrite}
	}
	return []string{ScopeRead}
}

// Service is the authorization server.
type Service struct {
	store *Store
	opts  Options

	keys     *keySet
	codes    *codeStore
	consents *consentStore

	cimd *cimdResolver
}

// NewService builds an authorization server. The signing key is resolved lazily
// by EnsureSigningKey so construction never touches the database.
func NewService(store *Store, opts Options) *Service {
	opts = opts.withDefaults()
	return &Service{
		store: store,
		opts:  opts,
		codes: newCodeStore(opts.CodeTTL, opts.Now),
		cimd:  newCIMDResolver(opts.CIMDHosts),
	}
}

// Options exposes the effective (defaulted) configuration.
func (s *Service) Options() Options { return s.opts }

// Issuer is the authorization server identifier. When PublicBaseURL is
// configured it is authoritative; otherwise fallback (request-derived origin)
// is used so local development still produces self-consistent metadata.
func (s *Service) Issuer(fallback string) string {
	if s.opts.PublicBaseURL != "" {
		return s.opts.PublicBaseURL
	}
	return strings.TrimRight(strings.TrimSpace(fallback), "/")
}

// Resource is the MCP resource URL that access tokens are audience-bound to.
func (s *Service) Resource(issuer string) string {
	return strings.TrimRight(issuer, "/") + s.opts.ResourcePath
}

// Enabled reports whether OAuth is switched on. A disabled service must not
// advertise metadata or accept tokens.
func (s *Service) Enabled() bool { return s != nil }
