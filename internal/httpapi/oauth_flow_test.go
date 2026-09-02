package httpapi

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/oauth"
)

// ── authorize endpoint ──────────────────────────────────────────────────────

func TestOAuthAuthorizeRedirectsToLoginWhenUnauthenticated(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	res := env.get(t, env.authorizeURL(nil), false)
	if res.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body=%s", res.Code, firstN(res.Body.String(), 200))
	}
	location := res.Header().Get("Location")
	if !strings.HasPrefix(location, "/login?next=") {
		t.Fatalf("location = %q, want a /login?next= redirect", location)
	}
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse location: %v", err)
	}
	next := parsed.Query().Get("next")
	// The return path must be a relative authorize URL that still carries the
	// PKCE challenge, or the token exchange can never complete after login.
	if !strings.HasPrefix(next, "/oauth/authorize?") {
		t.Fatalf("next = %q, want an authorize path", next)
	}
	for _, required := range []string{"client_id=", "code_challenge=", "state=state-1", "response_type=code"} {
		if !strings.Contains(next, required) {
			t.Fatalf("next lost %q: %s", required, next)
		}
	}
	if strings.Contains(next, "fund_session") || strings.Contains(location, env.session) {
		t.Fatal("login redirect leaked the session token")
	}
}

func TestOAuthAuthorizeAutoApprovesForAuthenticatedOwner(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	res := env.get(t, env.authorizeURL(nil), true)
	if res.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body=%s", res.Code, firstN(res.Body.String(), 200))
	}
	location := res.Header().Get("Location")
	if !strings.HasPrefix(location, testOAuthRedirect) {
		t.Fatalf("location = %q, want the registered redirect_uri", location)
	}
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse location: %v", err)
	}
	if parsed.Query().Get("code") == "" {
		t.Fatalf("no code in the redirect: %q", location)
	}
	if parsed.Query().Get("state") != "state-1" {
		t.Fatalf("state not echoed: %q", location)
	}
	if strings.Contains(location, "access_token") {
		t.Fatalf("front channel leaked a token: %q", location)
	}
}

func TestOAuthAuthorizeUnknownClientRendersErrorPageWithoutRedirecting(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	target := env.authorizeURL(url.Values{"client_id": {"not-registered"}})
	res := env.get(t, target, true)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", res.Code, firstN(res.Body.String(), 200))
	}
	if res.Header().Get("Location") != "" {
		t.Fatalf("unverified client produced a redirect to %q (open redirect)", res.Header().Get("Location"))
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content type = %q, want text/html", got)
	}
	if !strings.Contains(res.Body.String(), "invalid_client") {
		t.Fatalf("error page did not name the error: %s", firstN(res.Body.String(), 300))
	}
	if strings.Contains(res.Body.String(), "<script") {
		t.Fatal("consent/error page embedded script, which the app CSP forbids")
	}
}

func TestOAuthAuthorizeUnregisteredRedirectURIDoesNotRedirect(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	target := env.authorizeURL(url.Values{"redirect_uri": {"https://evil.example/steal"}})
	res := env.get(t, target, true)
	if res.Header().Get("Location") != "" {
		t.Fatalf("redirected to an unregistered target: %q", res.Header().Get("Location"))
	}
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.Code)
	}
	if !strings.Contains(res.Body.String(), "invalid_redirect_uri") {
		t.Fatalf("error page did not report invalid_redirect_uri: %s", firstN(res.Body.String(), 300))
	}
}

func TestOAuthAuthorizeRejectsImplicitAndPlainPKCE(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	cases := map[string]url.Values{
		"implicit":     {"response_type": {"token"}},
		"plain pkce":   {"code_challenge_method": {"plain"}, "code_challenge": {testPKCEVerifier}},
		"no pkce":      {"code_challenge": {""}, "code_challenge_method": {""}},
		"bad resource": {"resource": {"https://other.example/mcp"}},
	}
	for name, extra := range cases {
		res := env.get(t, env.authorizeURL(extra), true)
		if res.Header().Get("Location") != "" {
			t.Fatalf("%s: redirected to %q", name, res.Header().Get("Location"))
		}
		if res.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", name, res.Code)
		}
	}
}

func TestOAuthConsentPageFlowForWriteScope(t *testing.T) {
	cfg := testCfg()
	cfg.OAuthAllowWriteScope = true
	env := newOAuthEnv(t, cfg, func(o *oauth.Options) { o.AllowWriteScope = true })

	res := env.get(t, env.authorizeURL(url.Values{"scope": {oauth.ScopeRead + " " + oauth.ScopeWrite}}), true)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (consent screen); body=%s", res.Code, firstN(res.Body.String(), 300))
	}
	body := res.Body.String()
	if !strings.Contains(body, "授权") {
		t.Fatalf("consent page has no approve control: %s", firstN(body, 400))
	}
	if !strings.Contains(body, oauth.ScopeWrite) {
		t.Fatalf("consent page did not disclose the write scope: %s", firstN(body, 400))
	}
	// CSP is style-src 'self', so inline styles would be dropped and the page
	// would render unstyled.
	if strings.Contains(body, "style=\"") || strings.Contains(body, "<style") {
		t.Fatal("consent page used inline styles, which the app CSP blocks")
	}
	if !strings.Contains(body, "/oauth/assets/consent.css") {
		t.Fatal("consent page did not link the same-origin stylesheet")
	}

	token := regexp.MustCompile(`name="consent_token" value="([^"]+)"`).FindStringSubmatch(body)
	if len(token) != 2 {
		t.Fatalf("no consent_token in the page: %s", firstN(body, 400))
	}

	form := url.Values{"consent_token": {token[1]}, "decision": {"approve"}}
	approve := env.do(t, http.MethodPost, "/oauth/consent", true,
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if approve.Code != http.StatusFound {
		t.Fatalf("approve status = %d, want 302; body=%s", approve.Code, firstN(approve.Body.String(), 200))
	}
	location := approve.Header().Get("Location")
	if !strings.HasPrefix(location, testOAuthRedirect) {
		t.Fatalf("approve redirected to %q", location)
	}
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse approve location: %v", err)
	}
	if parsed.Query().Get("code") == "" {
		t.Fatal("approval issued no code")
	}

	// The consent token is single-use: a second submit must not mint a second code.
	replay := env.do(t, http.MethodPost, "/oauth/consent", true,
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if replay.Code != http.StatusBadRequest {
		t.Fatalf("consent replay status = %d, want 400", replay.Code)
	}
	if replay.Header().Get("Location") != "" {
		t.Fatalf("consent replay redirected to %q", replay.Header().Get("Location"))
	}
}

func TestOAuthConsentRequiresASession(t *testing.T) {
	cfg := testCfg()
	cfg.OAuthAutoApprove = false
	env := newOAuthEnv(t, cfg, func(o *oauth.Options) { o.AutoApprove = false })
	form := url.Values{"consent_token": {"whatever"}, "decision": {"approve"}}
	res := env.do(t, http.MethodPost, "/oauth/consent", false,
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if res.Code != http.StatusFound {
		t.Fatalf("status = %d, want a 302 to login", res.Code)
	}
	if !strings.HasPrefix(res.Header().Get("Location"), "/login") {
		t.Fatalf("location = %q, want /login", res.Header().Get("Location"))
	}
}

func TestOAuthConsentDenialReportsAccessDenied(t *testing.T) {
	cfg := testCfg()
	env := newOAuthEnv(t, cfg, func(o *oauth.Options) { o.AllowWriteScope = true; o.AutoApprove = false })
	cfg.OAuthAllowWriteScope = true

	res := env.get(t, env.authorizeURL(nil), true)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want the consent screen", res.Code)
	}
	token := regexp.MustCompile(`name="consent_token" value="([^"]+)"`).FindStringSubmatch(res.Body.String())
	if len(token) != 2 {
		t.Fatal("no consent_token rendered")
	}
	form := url.Values{"consent_token": {token[1]}, "decision": {"deny"}}
	deny := env.do(t, http.MethodPost, "/oauth/consent", true,
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if deny.Code != http.StatusFound {
		t.Fatalf("deny status = %d, want 302", deny.Code)
	}
	parsed, err := url.Parse(deny.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse deny location: %v", err)
	}
	if parsed.Query().Get("error") != "access_denied" {
		t.Fatalf("denial error = %q, want access_denied", parsed.Query().Get("error"))
	}
}
