package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// maxCIMDBytes caps a client-id metadata document. These are tiny JSON objects;
// the cap is abuse insurance, not a functional limit.
const maxCIMDBytes = 64 << 10

// cimdCacheTTL bounds how long a resolved metadata document is trusted.
const cimdCacheTTL = 10 * time.Minute

// RegisterClientRequest is an RFC 7591 dynamic client registration body.
type RegisterClientRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope"`
	ClientURI               string   `json:"client_uri"`
	LogoURI                 string   `json:"logo_uri"`
	PolicyURI               string   `json:"policy_uri"`
	TosURI                  string   `json:"tos_uri"`
	SoftwareID              string   `json:"software_id"`
	SoftwareVersion         string   `json:"software_version"`
}

// RegisterClientResponse is the RFC 7591 registration response. No secret is
// ever issued: MCP clients connecting to ChatGPT are public clients using PKCE.
type RegisterClientResponse struct {
	ClientID                string   `json:"client_id"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope,omitempty"`
	ClientURI               string   `json:"client_uri,omitempty"`
}

// RegisterClient validates and persists a dynamic registration.
func (s *Service) RegisterClient(ctx context.Context, req RegisterClientRequest) (*Client, *RegisterClientResponse, error) {
	if len(req.RedirectURIs) == 0 {
		return nil, nil, fail(ErrInvalidClient, http.StatusBadRequest, "redirect_uris is required")
	}
	cleaned := make([]string, 0, len(req.RedirectURIs))
	seen := make(map[string]struct{}, len(req.RedirectURIs))
	for _, raw := range req.RedirectURIs {
		uri, err := normalizeRedirectURI(raw)
		if err != nil {
			return nil, nil, fail(ErrInvalidRedirectURI, http.StatusBadRequest, "%v", err)
		}
		if _, dup := seen[uri]; dup {
			continue
		}
		seen[uri] = struct{}{}
		cleaned = append(cleaned, uri)
	}

	grants := normalizeGrantTypes(req.GrantTypes)
	authMethod := strings.TrimSpace(req.TokenEndpointAuthMethod)
	if authMethod == "" {
		authMethod = "none"
	}
	if authMethod != "none" {
		// This server is public-client-only. Accepting a confidential method we
		// cannot enforce would be a downgrade, so reject rather than silently
		// coerce.
		return nil, nil, fail(ErrInvalidClient, http.StatusBadRequest,
			"token_endpoint_auth_method %q is not supported; only public clients (none) are accepted", authMethod)
	}

	clientID, err := newClientID()
	if err != nil {
		return nil, nil, fail(ErrServerError, http.StatusInternalServerError, "generate client_id: %v", err)
	}
	name := strings.TrimSpace(req.ClientName)
	if name == "" {
		name = "MCP client " + clientID[:8]
	}
	client := Client{
		ID:                      clientID,
		Name:                    truncate(name, 120),
		RedirectURIs:            cleaned,
		GrantTypes:              grants,
		TokenEndpointAuthMethod: authMethod,
		Scope:                   strings.TrimSpace(req.Scope),
		ClientURI:               strings.TrimSpace(req.ClientURI),
		CreatedAt:               s.opts.Now().Unix(),
	}
	if s.store != nil {
		if err := s.store.InsertClient(ctx, client); err != nil {
			return nil, nil, fail(ErrServerError, http.StatusInternalServerError, "persist client: %v", err)
		}
	}
	return &client, &RegisterClientResponse{
		ClientID:                client.ID,
		ClientIDIssuedAt:        client.CreatedAt,
		ClientName:              client.Name,
		RedirectURIs:            client.RedirectURIs,
		GrantTypes:              client.GrantTypes,
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: client.TokenEndpointAuthMethod,
		Scope:                   client.Scope,
		ClientURI:               client.ClientURI,
	}, nil
}

// ResolveClient looks up a client by id. Unknown ids that look like an OpenAI
// client-id metadata document URL are resolved through CIMD (host-allowlisted);
// everything else is invalid_client.
func (s *Service) ResolveClient(ctx context.Context, clientID string) (*Client, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil, fail(ErrInvalidClient, http.StatusBadRequest, "client_id is required")
	}
	if s.store != nil {
		stored, err := s.store.ClientByID(ctx, clientID)
		if err != nil {
			return nil, fail(ErrServerError, http.StatusInternalServerError, "load client: %v", err)
		}
		if stored != nil {
			return stored, nil
		}
	}
	if isCIMDCandidate(clientID) {
		return s.cimd.Resolve(ctx, clientID)
	}
	return nil, fail(ErrInvalidClient, http.StatusBadRequest, "unknown client_id")
}

// ValidateRedirectURI enforces exact-match redirect validation (OAuth 2.1 §2.1).
// Loopback ports are ignored only for http://127.0.0.1 / http://localhost, which
// RFC 8252 §7.3 permits for native development clients.
func ValidateRedirectURI(client *Client, redirectURI string) (string, error) {
	redirectURI = strings.TrimSpace(redirectURI)
	if redirectURI == "" {
		if len(client.RedirectURIs) == 1 {
			return client.RedirectURIs[0], nil
		}
		return "", errors.New("redirect_uri is required")
	}
	for _, registered := range client.RedirectURIs {
		if registered == redirectURI {
			return redirectURI, nil
		}
		if loopbackEquivalent(registered, redirectURI) {
			return redirectURI, nil
		}
	}
	return "", fmt.Errorf("redirect_uri %q is not registered for this client", redirectURI)
}

func loopbackEquivalent(registered, candidate string) bool {
	a, errA := url.Parse(registered)
	b, errB := url.Parse(candidate)
	if errA != nil || errB != nil {
		return false
	}
	if a.Scheme != "http" || b.Scheme != "http" {
		return false
	}
	if !isLoopbackHost(a.Hostname()) || !isLoopbackHost(b.Hostname()) {
		return false
	}
	return a.Path == b.Path && a.RawQuery == b.RawQuery
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return true
	default:
		return false
	}
}

// normalizeRedirectURI enforces the redirect_uri shape at registration time:
// absolute https URLs, no fragments. Loopback http is allowed so a developer can
// point a local MCP inspector at the server.
func normalizeRedirectURI(raw string) (string, error) {
	uri := strings.TrimSpace(raw)
	if uri == "" {
		return "", errors.New("redirect_uri must not be empty")
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("redirect_uri %q is not a valid URL", uri)
	}
	if !parsed.IsAbs() {
		return "", fmt.Errorf("redirect_uri %q must be absolute", uri)
	}
	if parsed.Fragment != "" {
		return "", fmt.Errorf("redirect_uri %q must not contain a fragment", uri)
	}
	// A wildcard or empty host would let any subdomain (or a later DNS/tenant
	// takeover) receive authorization codes, so hostnames must be literal.
	host := parsed.Hostname()
	if host == "" || strings.ContainsAny(host, "*?[]{}\\") {
		return "", fmt.Errorf("redirect_uri %q must name a literal host", uri)
	}
	// Embedded credentials in a redirect target are a phishing vector (the
	// authority section is easy to misread) and are never needed by a real MCP
	// client, so reject rather than silently strip.
	if parsed.User != nil {
		return "", fmt.Errorf("redirect_uri %q must not contain userinfo", uri)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return uri, nil
	case "http":
		if isLoopbackHost(parsed.Hostname()) {
			return uri, nil
		}
		return "", fmt.Errorf("redirect_uri %q must use https (plain http is allowed only for loopback)", uri)
	default:
		return "", fmt.Errorf("redirect_uri %q has unsupported scheme %q", uri, parsed.Scheme)
	}
}

func normalizeGrantTypes(requested []string) []string {
	allowed := map[string]struct{}{
		"authorization_code": {},
		"refresh_token":      {},
	}
	out := make([]string, 0, 2)
	for _, grant := range requested {
		trimmed := strings.TrimSpace(grant)
		if _, ok := allowed[trimmed]; ok {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		out = []string{"authorization_code", "refresh_token"}
	}
	return dedupeStrings(out)
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func newClientID() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}

// ── OpenAI client-id metadata documents (CIMD) ──────────────────────────────
//
// ChatGPT's connector identifies itself with a client_id that is an https URL
// pointing at a JSON metadata document listing its redirect URIs. Fetching a
// caller-supplied URL is an SSRF vector, so the host must be allowlisted, the
// scheme must be https, redirects are not followed, the body is size-capped, and
// the resolved address is rejected when it is not a public unicast IP.

func isCIMDCandidate(clientID string) bool {
	return strings.HasPrefix(clientID, "https://")
}

type cimdResolver struct {
	hosts   map[string]struct{}
	mu      sync.Mutex
	cache   map[string]cimdCacheEntry
	client  *http.Client
	fetchFn func(ctx context.Context, rawURL string) ([]byte, error)
}

type cimdCacheEntry struct {
	client    *Client
	expiresAt time.Time
}

func newCIMDResolver(hosts []string) *cimdResolver {
	allow := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		if trimmed := strings.ToLower(strings.TrimSpace(host)); trimmed != "" {
			allow[trimmed] = struct{}{}
		}
	}
	transport := &http.Transport{
		// No redirect following: a metadata document must be exactly where the
		// allowlisted client_id says it is.
		DisableKeepAlives: false,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := (&net.Resolver{}).LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if !isPublicUnicast(ip.IP) {
					return nil, fmt.Errorf("resolved address for %s is not a public unicast IP", host)
				}
			}
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
		},
	}
	return &cimdResolver{
		hosts: allow,
		cache: make(map[string]cimdCacheEntry),
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func isPublicUnicast(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	// Reject the RFC 6598 shared address space (carrier-grade NAT) and the IPv4
	// broadcast address explicitly: IsPrivate covers neither.
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1]&0xC0 == 64 {
			return false
		}
		if v4[0] == 255 {
			return false
		}
	}
	return true
}

func (r *cimdResolver) allowed(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, errors.New("client_id is not a valid URL")
	}
	if parsed.Scheme != "https" {
		return nil, errors.New("client_id metadata document must use https")
	}
	host := strings.ToLower(parsed.Hostname())
	if _, ok := r.hosts[host]; !ok {
		return nil, fmt.Errorf("client_id host %q is not allowlisted for metadata documents", host)
	}
	return parsed, nil
}

// Resolve fetches (or reuses a cached) metadata document for clientID.
func (r *cimdResolver) Resolve(ctx context.Context, clientID string) (*Client, error) {
	parsed, err := r.allowed(clientID)
	if err != nil {
		return nil, fail(ErrInvalidClient, http.StatusBadRequest, "%v", err)
	}
	now := time.Now()
	r.mu.Lock()
	if entry, ok := r.cache[clientID]; ok && now.Before(entry.expiresAt) {
		r.mu.Unlock()
		return entry.client, nil
	}
	r.mu.Unlock()

	fetch := r.fetchFn
	if fetch == nil {
		fetch = r.httpFetch
	}
	body, err := fetch(ctx, parsed.String())
	if err != nil {
		return nil, fail(ErrInvalidClient, http.StatusBadRequest,
			"could not fetch client_id metadata document: %v", err)
	}
	var doc struct {
		ClientID     string   `json:"client_id"`
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
		Scope        string   `json:"scope"`
		ClientURI    string   `json:"client_uri"`
		GrantTypes   []string `json:"grant_types"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fail(ErrInvalidClient, http.StatusBadRequest,
			"client_id metadata document is not valid JSON")
	}
	if len(doc.RedirectURIs) == 0 {
		return nil, fail(ErrInvalidClient, http.StatusBadRequest,
			"client_id metadata document declares no redirect_uris")
	}
	cleaned := make([]string, 0, len(doc.RedirectURIs))
	for _, raw := range doc.RedirectURIs {
		uri, err := normalizeRedirectURI(raw)
		if err != nil {
			return nil, fail(ErrInvalidClient, http.StatusBadRequest,
				"client_id metadata document has an invalid redirect_uri: %v", err)
		}
		cleaned = append(cleaned, uri)
	}
	name := strings.TrimSpace(doc.ClientName)
	if name == "" {
		name = parsed.Host
	}
	client := &Client{
		ID:                      clientID,
		Name:                    truncate(name, 120),
		RedirectURIs:            cleaned,
		GrantTypes:              normalizeGrantTypes(doc.GrantTypes),
		TokenEndpointAuthMethod: "none",
		Scope:                   strings.TrimSpace(doc.Scope),
		ClientURI:               strings.TrimSpace(doc.ClientURI),
		MetadataDocument:        true,
	}
	r.mu.Lock()
	r.cache[clientID] = cimdCacheEntry{client: client, expiresAt: now.Add(cimdCacheTTL)}
	r.mu.Unlock()
	return client, nil
}

func (r *cimdResolver) httpFetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fund-dashboard-oauth/1.0")
	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata document returned HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxCIMDBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxCIMDBytes {
		return nil, errors.New("metadata document exceeds 64 KiB")
	}
	return body, nil
}
