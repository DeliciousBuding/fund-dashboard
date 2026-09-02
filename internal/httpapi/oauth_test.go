package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DeliciousBuding/fund-dashboard/internal/auth"
	"github.com/DeliciousBuding/fund-dashboard/internal/config"
	"github.com/DeliciousBuding/fund-dashboard/internal/oauth"
)

const testOAuthIssuer = "https://fund.example.test"
const testOAuthRedirect = "https://chatgpt.com/oauth/mcp/callback"
const testPKCEVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

// oauthEnv is a router with the OAuth authorization server, the web session
// service and the MCP endpoint all wired against one temp database.
type oauthEnv struct {
	router   http.Handler
	oauthSvc *oauth.Service
	authSvc  *auth.Service
	session  string
	clientID string
	db       *sql.DB
}

func newOAuthEnv(t *testing.T, cfg config.Config, mutate func(*oauth.Options)) *oauthEnv {
	t.Helper()
	db := openPortfolioHTTPFixture(t)
	authSvc := newTestAuthService(t, db)
	session := loginTestUser(t, authSvc)

	store := oauth.NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure oauth schema: %v", err)
	}
	opts := oauth.Options{PublicBaseURL: testOAuthIssuer, AutoApprove: cfg.OAuthAutoApprove}
	if mutate != nil {
		mutate(&opts)
	}
	oauthSvc := oauth.NewService(store, opts)
	if err := oauthSvc.EnsureSigningKey(context.Background()); err != nil {
		t.Fatalf("ensure signing key: %v", err)
	}
	client, _, err := oauthSvc.RegisterClient(context.Background(), oauth.RegisterClientRequest{
		ClientName:   "ChatGPT connector",
		RedirectURIs: []string{testOAuthRedirect},
	})
	if err != nil {
		t.Fatalf("register client: %v", err)
	}

	// The SPA fallback must be mounted, exactly as in production: it is the thing
	// that would otherwise swallow /.well-known/* and answer 200 + index.html.
	staticFS := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>spa</title>")}}
	router := NewRouter(cfg, WithDB(db), WithAuth(authSvc), WithOAuth(oauthSvc), WithStaticFS(staticFS))
	t.Cleanup(func() { _ = db.Close() })
	return &oauthEnv{
		router: router, oauthSvc: oauthSvc, authSvc: authSvc,
		session: session, clientID: client.ID, db: db,
	}
}

// do issues a request against the router, optionally carrying the session cookie.
func (e *oauthEnv) do(t *testing.T, method, target string, authed bool, body *strings.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != nil {
		reader = body
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, reader)
	req.Host = strings.TrimPrefix(strings.TrimPrefix(testOAuthIssuer, "https://"), "http://")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if authed {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: e.session})
	}
	res := httptest.NewRecorder()
	e.router.ServeHTTP(res, req)
	return res
}

func (e *oauthEnv) get(t *testing.T, target string, authed bool) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodGet, target, authed, nil, "")
}

func decodeJSONBody(t *testing.T, res *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not JSON (%s): %v; body=%s",
			res.Header().Get("Content-Type"), err, res.Body.String())
	}
	return out
}

func assertJSONContentType(t *testing.T, res *httptest.ResponseRecorder) {
	t.Helper()
	contentType := res.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("content type = %q, want application/json; body=%s", contentType, firstN(res.Body.String(), 160))
	}
}

func firstN(value string, n int) string {
	if len(value) <= n {
		return value
	}
	return value[:n] + "…"
}

// authorizeURL builds a well-formed authorize request for the registered client.
func (e *oauthEnv) authorizeURL(extra url.Values) string {
	query := url.Values{
		"client_id":             {e.clientID},
		"redirect_uri":          {testOAuthRedirect},
		"response_type":         {"code"},
		"scope":                 {oauth.ScopeRead},
		"state":                 {"state-1"},
		"code_challenge":        {s256(testPKCEVerifier)},
		"code_challenge_method": {"S256"},
		"resource":              {testOAuthIssuer + "/mcp"},
	}
	for key, values := range extra {
		for _, value := range values {
			query.Set(key, value)
		}
	}
	return "/oauth/authorize?" + query.Encode()
}

func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// ── discovery ───────────────────────────────────────────────────────────────

func TestOAuthDiscoveryServesJSONOnEveryWellKnownPath(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)

	prmPaths := oauth.WellKnownPathProtectedResource("/mcp")
	for _, path := range prmPaths {
		res := env.get(t, path, false)
		if res.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200; body=%s", path, res.Code, firstN(res.Body.String(), 200))
		}
		assertJSONContentType(t, res)
		body := decodeJSONBody(t, res)
		if body["resource"] != testOAuthIssuer+"/mcp" {
			t.Fatalf("%s: resource = %v", path, body["resource"])
		}
		servers, ok := body["authorization_servers"].([]any)
		if !ok || len(servers) != 1 || servers[0] != testOAuthIssuer {
			t.Fatalf("%s: authorization_servers = %v", path, body["authorization_servers"])
		}
	}

	asmPaths := oauth.WellKnownPathAuthorizationServer("/mcp")
	for _, path := range asmPaths {
		res := env.get(t, path, false)
		if res.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200; body=%s", path, res.Code, firstN(res.Body.String(), 200))
		}
		assertJSONContentType(t, res)
		body := decodeJSONBody(t, res)
		if body["issuer"] != testOAuthIssuer {
			t.Fatalf("%s: issuer = %v", path, body["issuer"])
		}
		if body["token_endpoint"] != testOAuthIssuer+"/oauth/token" {
			t.Fatalf("%s: token_endpoint = %v", path, body["token_endpoint"])
		}
		if body["client_id_metadata_document_supported"] != true {
			t.Fatalf("%s: CIMD not advertised", path)
		}
	}
}

// TestUnknownOAuthPathsAreJSON404 pins the trap this whole feature has to avoid:
// the SPA fallback answers any unmatched path with index.html and HTTP 200, so
// an unregistered discovery path would look like a successful response carrying
// garbage. A connector then concludes the server has no auth at all.
func TestUnknownOAuthPathsAreJSON404NotTheSPA(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	for _, path := range []string{
		"/.well-known/oauth-protected-resource/wrong",
		"/.well-known/oauth-authorization-server/wrong",
		"/.well-known/not-a-real-thing",
		"/oauth/does-not-exist",
		"/oauth",
	} {
		res := env.get(t, path, false)
		if res.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404; body=%s", path, res.Code, firstN(res.Body.String(), 200))
		}
		if strings.Contains(res.Body.String(), "<!doctype html>") {
			t.Fatalf("GET %s fell through to the SPA HTML shell", path)
		}
		assertJSONContentType(t, res)
	}
}

func TestOAuthAboutAndJWKSEndpoints(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)

	res := env.get(t, "/oauth/about", false)
	if res.Code != http.StatusOK {
		t.Fatalf("/oauth/about = %d", res.Code)
	}
	assertJSONContentType(t, res)
	about := decodeJSONBody(t, res)
	if about["mcp_endpoint"] != testOAuthIssuer+"/mcp" {
		t.Fatalf("about mcp_endpoint = %v", about["mcp_endpoint"])
	}

	res = env.get(t, "/oauth/jwks", false)
	if res.Code != http.StatusOK {
		t.Fatalf("/oauth/jwks = %d", res.Code)
	}
	assertJSONContentType(t, res)
	jwks := decodeJSONBody(t, res)
	keys, ok := jwks["keys"].([]any)
	if !ok || len(keys) != 1 {
		t.Fatalf("jwks keys = %v", jwks["keys"])
	}
	if strings.Contains(res.Body.String(), `"d":`) {
		t.Fatal("jwks leaked the private exponent")
	}
}

func TestOAuthConsentCSSIsSameOriginStylesheet(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	res := env.get(t, "/oauth/assets/consent.css", false)
	if res.Code != http.StatusOK {
		t.Fatalf("consent css = %d", res.Code)
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Fatalf("content type = %q, want text/css", got)
	}
	// The app CSP is style-src 'self', so the consent page must not rely on
	// inline styles; verify the stylesheet actually carries the rules.
	if !strings.Contains(res.Body.String(), ".card") {
		t.Fatal("consent stylesheet looks empty")
	}
}
