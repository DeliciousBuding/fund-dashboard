package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

// /oauth/consent POST 的 Origin 复核：复用 browserMutationAllowed 的既有语义 ——
// 浏览器标记的跨站提交 403，无浏览器信号的 curl 式客户端保持可用（CI
// scripts/smoke-oauth.sh 即为此形态），同站与 localhost 提交照常通过。

// consentOriginTestCfg is testCfg plus an explicit origin allowlist matching the
// consent page's own origin, so the same-origin case can be exercised.
func consentOriginTestCfg() config.Config {
	cfg := testCfg()
	cfg.AllowedOrigins = []string{"https://fund.example.test"}
	return cfg
}

// consentOriginPost submits the consent form with an optional Origin /
// Sec-Fetch-Site pair, always carrying the dashboard session cookie.
func consentOriginPost(t *testing.T, env *oauthEnv, form url.Values, origin, secFetchSite string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/oauth/consent", strings.NewReader(form.Encode()))
	req.Host = strings.TrimPrefix(testOAuthIssuer, "https://")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: env.session})
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if secFetchSite != "" {
		req.Header.Set("Sec-Fetch-Site", secFetchSite)
	}
	res := httptest.NewRecorder()
	env.router.ServeHTTP(res, req)
	return res
}

// beginConsentForTest drives authorize → consent screen and returns the form
// holding the fresh one-shot consent token.
func beginConsentForTest(t *testing.T, env *oauthEnv) url.Values {
	t.Helper()
	res := env.get(t, env.authorizeURL(nil), true)
	if res.Code != http.StatusOK {
		t.Fatalf("authorize status = %d, want the consent screen; body=%s", res.Code, firstN(res.Body.String(), 200))
	}
	token := regexp.MustCompile(`name="consent_token" value="([^"]+)"`).FindStringSubmatch(res.Body.String())
	if len(token) != 2 {
		t.Fatal("no consent_token rendered")
	}
	return url.Values{"consent_token": {token[1]}, "decision": {"approve"}}
}

func assertConsentRedirect(t *testing.T, res *httptest.ResponseRecorder) {
	t.Helper()
	if res.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body=%s", res.Code, firstN(res.Body.String(), 200))
	}
	if location := res.Header().Get("Location"); !strings.HasPrefix(location, testOAuthRedirect) {
		t.Fatalf("location = %q, want the registered redirect_uri", location)
	}
}

func TestOAuthConsentRejectsCrossSiteBrowserPost(t *testing.T) {
	env := newOAuthEnv(t, consentOriginTestCfg(), nil)
	form := beginConsentForTest(t, env)

	res := consentOriginPost(t, env, form, "https://evil.example", "cross-site")
	if res.Code != http.StatusForbidden {
		t.Fatalf("cross-site consent status = %d, want 403; body=%s", res.Code, res.Body.String())
	}
	if res.Body.String() != `{"error":"origin_not_allowed"}`+"\n" {
		t.Fatalf("cross-site consent body = %q, want origin_not_allowed envelope", res.Body.String())
	}
}

func TestOAuthConsentRejectsCrossSiteBySecFetchEvenWithoutOrigin(t *testing.T) {
	env := newOAuthEnv(t, consentOriginTestCfg(), nil)
	form := beginConsentForTest(t, env)

	// A cross-site form POST from a page that strips Origin is still labelled by
	// Sec-Fetch-Site — the browser signal alone must suffice to reject.
	res := consentOriginPost(t, env, form, "", "cross-site")
	if res.Code != http.StatusForbidden {
		t.Fatalf("cross-site (no Origin) consent status = %d, want 403", res.Code)
	}
}

func TestOAuthConsentAllowsCurlStylePostWithoutBrowserSignals(t *testing.T) {
	env := newOAuthEnv(t, consentOriginTestCfg(), nil)
	form := beginConsentForTest(t, env)

	// scripts/smoke-oauth.sh (CI hard gate) POSTs consent via curl: no Origin,
	// no Sec-Fetch-Site. Non-browser clients must keep working.
	res := consentOriginPost(t, env, form, "", "")
	assertConsentRedirect(t, res)
}

func TestOAuthConsentAllowsSameOriginBrowserPost(t *testing.T) {
	env := newOAuthEnv(t, consentOriginTestCfg(), nil)
	form := beginConsentForTest(t, env)

	res := consentOriginPost(t, env, form, "https://fund.example.test", "same-origin")
	assertConsentRedirect(t, res)
}

func TestOAuthConsentAllowsLocalhostOriginBrowserPost(t *testing.T) {
	env := newOAuthEnv(t, consentOriginTestCfg(), nil)
	form := beginConsentForTest(t, env)

	// localhost on any port is always accepted by the origin check (local dev /
	// smoke), exactly as for every other browser write path.
	res := consentOriginPost(t, env, form, "http://localhost:5173", "same-origin")
	assertConsentRedirect(t, res)
}
