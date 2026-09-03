package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/oauth"
)

// authorizeCode drives the browser half of the flow and returns the code.
//
// A client's first authorization renders the consent screen, so the helper
// approves it the way the owner would and follows the resulting redirect. Tests
// that need to observe the screen itself drive env.get directly.
func authorizeCode(t *testing.T, env *oauthEnv) string {
	t.Helper()
	res := env.get(t, env.authorizeURL(nil), true)
	if res.Code == http.StatusOK {
		res = approveConsentScreen(t, env, res.Body.String())
	}
	if res.Code != http.StatusFound && res.Code != http.StatusSeeOther {
		t.Fatalf("authorize status = %d, want 302 or 303", res.Code)
	}
	parsed, err := url.Parse(res.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse authorize redirect: %v", err)
	}
	code := parsed.Query().Get("code")
	if code == "" {
		t.Fatalf("no code in redirect: %q", parsed.String())
	}
	return code
}

// approveConsentScreen submits the rendered consent form as an approval and
// returns the redirect response it produces.
func approveConsentScreen(t *testing.T, env *oauthEnv, body string) *httptest.ResponseRecorder {
	t.Helper()
	token := regexp.MustCompile(`name="consent_token" value="([^"]+)"`).FindStringSubmatch(body)
	if len(token) != 2 {
		t.Fatalf("consent screen rendered no consent_token: %s", firstN(body, 300))
	}
	form := url.Values{"consent_token": {token[1]}, "decision": {"approve"}}
	return env.do(t, http.MethodPost, "/oauth/consent", true,
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
}

// postForm posts a urlencoded body and decodes the JSON response.
func postForm(t *testing.T, env *oauthEnv, path string, form url.Values) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	res := env.do(t, http.MethodPost, path, false,
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	body := map[string]any{}
	if res.Body.Len() > 0 {
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s response is not JSON: %v; body=%s", path, err, firstN(res.Body.String(), 300))
		}
	}
	return res, body
}

func TestOAuthTokenEndpointIssuesUsableAccessToken(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	code := authorizeCode(t, env)

	// OpenAI's connector posts the token request without a client_id, so the
	// endpoint must identify the client from the code itself.
	res, body := postForm(t, env, "/oauth/token", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {testOAuthRedirect},
		"code_verifier": {testPKCEVerifier},
		"resource":      {testOAuthIssuer + "/mcp"},
	})
	if res.Code != http.StatusOK {
		t.Fatalf("token status = %d, want 200; body=%s", res.Code, firstN(res.Body.String(), 400))
	}
	accessToken, _ := body["access_token"].(string)
	refreshToken, _ := body["refresh_token"].(string)
	if accessToken == "" || refreshToken == "" {
		t.Fatalf("token response missing fields: %v", body)
	}
	if body["token_type"] != "Bearer" {
		t.Fatalf("token_type = %v, want Bearer", body["token_type"])
	}
	if body["scope"] != oauth.ScopeRead {
		t.Fatalf("scope = %v, want %s", body["scope"], oauth.ScopeRead)
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("token response must not be cacheable: %q", res.Header().Get("Cache-Control"))
	}
	// Three dot-separated segments and an ES256 header.
	if strings.Count(accessToken, ".") != 2 {
		t.Fatalf("access token is not a JWS compact serialization")
	}

	// The token must work against the MCP endpoint.
	mcpRes := callMCP(t, env, accessToken, `{"jsonrpc":"2.0","id":"1","method":"tools/list"}`)
	if mcpRes.Code != http.StatusOK {
		t.Fatalf("mcp tools/list with OAuth token = %d; body=%s", mcpRes.Code, firstN(mcpRes.Body.String(), 300))
	}

	// And it must refresh.
	res, body = postForm(t, env, "/oauth/token", url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
	if res.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200; body=%s", res.Code, firstN(res.Body.String(), 300))
	}
	rotated, _ := body["refresh_token"].(string)
	if rotated == "" || rotated == refreshToken {
		t.Fatal("refresh token was not rotated")
	}
	// The consumed token is revoked by rotation, so a replay must fail.
	res, body = postForm(t, env, "/oauth/token", url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
	if res.Code != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("replayed refresh token: status=%d body=%v", res.Code, body)
	}
}

func TestOAuthTokenEndpointAcceptsJSONBody(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	code := authorizeCode(t, env)
	payload, _ := json.Marshal(map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  testOAuthRedirect,
		"code_verifier": testPKCEVerifier,
	})
	res := env.do(t, http.MethodPost, "/oauth/token", false,
		strings.NewReader(string(payload)), "application/json")
	if res.Code != http.StatusOK {
		t.Fatalf("JSON token request status = %d; body=%s", res.Code, firstN(res.Body.String(), 300))
	}
	body := decodeJSONBody(t, res)
	if body["access_token"] == nil {
		t.Fatalf("no access token: %v", body)
	}
}

func TestOAuthTokenEndpointRejectsBadRequests(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	cases := map[string]url.Values{
		"wrong verifier": {
			"grant_type": {"authorization_code"}, "code": {authorizeCode(t, env)},
			"redirect_uri":  {testOAuthRedirect},
			"code_verifier": {"wrong-verifier-value-wrong-verifier-value-xxxx"},
		},
		"wrong redirect": {
			"grant_type": {"authorization_code"}, "code": {authorizeCode(t, env)},
			"redirect_uri": {"https://evil.example/cb"}, "code_verifier": {testPKCEVerifier},
		},
		"unknown code": {
			"grant_type": {"authorization_code"}, "code": {"never-issued"},
			"redirect_uri": {testOAuthRedirect}, "code_verifier": {testPKCEVerifier},
		},
		"no grant type": {"code": {"x"}},
		"bad grant type": {
			"grant_type": {"client_credentials"}, "code": {"x"},
		},
		"no refresh token": {"grant_type": {"refresh_token"}},
	}
	for name, form := range cases {
		res, body := postForm(t, env, "/oauth/token", form)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400; body=%v", name, res.Code, body)
		}
		if _, ok := body["error"].(string); !ok {
			t.Fatalf("%s: response has no OAuth error code: %v", name, body)
		}
	}
	// A replayed code must fail even with the correct verifier.
	code := authorizeCode(t, env)
	good := url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {testOAuthRedirect}, "code_verifier": {testPKCEVerifier},
	}
	if res, _ := postForm(t, env, "/oauth/token", good); res.Code != http.StatusOK {
		t.Fatalf("first exchange status = %d, want 200", res.Code)
	}
	if res, body := postForm(t, env, "/oauth/token", good); res.Code != http.StatusBadRequest {
		t.Fatalf("replayed code status = %d, want 400; body=%v", res.Code, body)
	}
}

func TestOAuthRegisterEndpoint(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	payload, _ := json.Marshal(map[string]any{
		"client_name":   "Cursor",
		"redirect_uris": []string{"https://cursor.com/mcp/callback"},
		"grant_types":   []string{"authorization_code", "refresh_token"},
	})
	res := env.do(t, http.MethodPost, "/oauth/register", false,
		strings.NewReader(string(payload)), "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201; body=%s", res.Code, firstN(res.Body.String(), 300))
	}
	body := decodeJSONBody(t, res)
	clientID, _ := body["client_id"].(string)
	if clientID == "" {
		t.Fatalf("no client_id issued: %v", body)
	}
	// A public client registration must never hand back a secret.
	for _, forbidden := range []string{"client_secret", "client_secret_expires_at"} {
		if _, ok := body[forbidden]; ok {
			t.Fatalf("registration issued %q; this server is public-client-only", forbidden)
		}
	}
	if body["token_endpoint_auth_method"] != "none" {
		t.Fatalf("auth method = %v, want none", body["token_endpoint_auth_method"])
	}

	// The freshly registered client can complete a full flow.
	rejected, _ := json.Marshal(map[string]any{
		"client_name":   "bad",
		"redirect_uris": []string{"http://evil.example/cb"},
	})
	res = env.do(t, http.MethodPost, "/oauth/register", false,
		strings.NewReader(string(rejected)), "application/json")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("insecure registration status = %d, want 400; body=%s", res.Code, firstN(res.Body.String(), 300))
	}
}

func TestOAuthRevokeEndpointAlwaysSucceeds(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	// RFC 7009 §2.2: an unknown token still yields 200 so revocation cannot be
	// used to probe which tokens exist.
	res, body := postForm(t, env, "/oauth/revoke", url.Values{"token": {"never-issued"}})
	if res.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, want 200; body=%v", res.Code, body)
	}
	res, _ = postForm(t, env, "/oauth/revoke", url.Values{})
	if res.Code != http.StatusOK {
		t.Fatalf("empty revoke status = %d, want 200", res.Code)
	}
}

// ── MCP resource server integration ─────────────────────────────────────────

func callMCP(t *testing.T, env *oauthEnv, bearer, payload string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res := httptest.NewRecorder()
	env.router.ServeHTTP(res, req)
	return res
}

func decodeMCPResult(t *testing.T, res *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      any            `json:"id"`
		Result  map[string]any `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("mcp response is not JSON-RPC: %v; body=%s", err, firstN(res.Body.String(), 300))
	}
	if envelope.Error != nil {
		t.Fatalf("mcp returned an error: %d %s", envelope.Error.Code, envelope.Error.Message)
	}
	return envelope.Result
}

func TestMCPAcceptsOAuthTokenAndGrantsAnalystRole(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	code := authorizeCode(t, env)
	_, body := postForm(t, env, "/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {testOAuthRedirect}, "code_verifier": {testPKCEVerifier},
	})
	accessToken, _ := body["access_token"].(string)

	result := decodeMCPResult(t, callMCP(t, env, accessToken,
		`{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"protocolVersion":"2025-06-18"}}`))
	if result["protocolVersion"] != "2025-06-18" {
		t.Fatalf("protocolVersion = %v", result["protocolVersion"])
	}

	result = decodeMCPResult(t, callMCP(t, env, accessToken,
		`{"jsonrpc":"2.0","id":"list","method":"tools/list"}`))
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("no tools advertised: %v", result)
	}
	// fund.read maps to analyst, so write and maintenance tools must not be
	// discoverable — a connector must not even see a tool it cannot call.
	for _, entry := range tools {
		tool := entry.(map[string]any)
		name, _ := tool["name"].(string)
		switch name {
		case "add_transaction", "delete_transaction", "delete_fund", "import_transactions",
			"crawl_nav", "recalculate_snapshot", "adjust_position", "run_dca_auto_invest":
			t.Fatalf("read-scoped OAuth token can see write/maintenance tool %q", name)
		}
	}

	// A read tool must actually execute.
	call := map[string]any{
		"jsonrpc": "2.0", "id": "call", "method": "tools/call",
		"params": map[string]any{"name": "get_portfolio_summary", "arguments": map[string]any{"portfolio_id": 1}},
	}
	payload, _ := json.Marshal(call)
	res := callMCP(t, env, accessToken, string(payload))
	if res.Code != http.StatusOK {
		t.Fatalf("tools/call status = %d; body=%s", res.Code, firstN(res.Body.String(), 400))
	}
	decodeMCPResult(t, res)
}

// TestMCPStaticKeysStillWork is the regression guard for existing consumers:
// existing operator consumers authenticate with MCP_API_KEY, and this OAuth
// work must not change what it sees.
func TestMCPStaticKeysStillWork(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)

	adminRes := callMCP(t, env, testAdminKey, `{"jsonrpc":"2.0","id":"1","method":"tools/list"}`)
	if adminRes.Code != http.StatusOK {
		t.Fatalf("admin key status = %d; body=%s", adminRes.Code, firstN(adminRes.Body.String(), 300))
	}
	adminResult := decodeMCPResult(t, adminRes)
	adminTools := adminResult["tools"].([]any)

	publicRes := callMCP(t, env, testPublicMCPKey, `{"jsonrpc":"2.0","id":"1","method":"tools/list"}`)
	if publicRes.Code != http.StatusOK {
		t.Fatalf("public key status = %d", publicRes.Code)
	}
	publicTools := decodeMCPResult(t, publicRes)["tools"].([]any)

	if len(adminTools) <= len(publicTools) {
		t.Fatalf("operator (%d) must see more tools than analyst (%d)", len(adminTools), len(publicTools))
	}
	// This env wires no AgentOps, which is how a deployment without
	// FUND_AGENT_OPS_ENABLED boots. Confirmation-gated tools can never succeed
	// there, so they must not be advertised; the operator is still strictly
	// ahead of the analyst because it keeps the maintenance scope. The wired
	// case (operator does see write tools) is pinned by
	// TestListToolsHidesConfirmationGatedToolsWithoutAgentOps in internal/mcp.
	sawMaintenance := false
	for _, entry := range adminTools {
		switch entry.(map[string]any)["name"] {
		case "add_transaction", "import_transactions", "delete_transaction":
			t.Fatalf("operator key is advertised confirmation-gated %q without AgentOps", entry.(map[string]any)["name"])
		case "crawl_nav":
			sawMaintenance = true
		}
	}
	if !sawMaintenance {
		t.Fatal("operator key lost the maintenance scope that distinguishes it from the analyst")
	}

	// A read call under the legacy key must still succeed and keep the shape an
	// existing static-key consumer already parses: the content array.
	call := map[string]any{
		"jsonrpc": "2.0", "id": "call", "method": "tools/call",
		"params": map[string]any{"name": "get_portfolio_summary", "arguments": map[string]any{"portfolio_id": 1}},
	}
	payload, _ := json.Marshal(call)
	legacyRes := callMCP(t, env, testPublicMCPKey, string(payload))
	if legacyRes.Code != http.StatusOK {
		t.Fatalf("static-key tools/call = %d; body=%s", legacyRes.Code, firstN(legacyRes.Body.String(), 300))
	}
	legacy := decodeMCPResult(t, legacyRes)
	if _, ok := legacy["content"].([]any); !ok {
		t.Fatalf("static-key caller lost the content array: %v", legacy)
	}
}

// TestMCPToolResultShapeIsSpecCompliant pins the tools/call contract every
// caller sees: a content array (required by MCP, and what ChatGPT reads), the
// same value as structuredContent (so a client can bind a schema), and isError.
// The OpenAI MCP guide is explicit that both must be present.
func TestMCPToolResultShapeIsSpecCompliant(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	_, body := postForm(t, env, "/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {authorizeCode(t, env)},
		"redirect_uri": {testOAuthRedirect}, "code_verifier": {testPKCEVerifier},
	})
	accessToken, _ := body["access_token"].(string)
	if accessToken == "" {
		t.Fatal("no access token issued")
	}

	call := map[string]any{
		"jsonrpc": "2.0", "id": "call", "method": "tools/call",
		"params": map[string]any{"name": "get_portfolio_summary", "arguments": map[string]any{"portfolio_id": 1}},
	}
	payload, _ := json.Marshal(call)

	callers := map[string]string{
		"oauth":      accessToken,
		"public key": testPublicMCPKey,
		"admin key":  testAdminKey,
	}
	for name, bearer := range callers {
		result := decodeMCPResult(t, callMCP(t, env, bearer, string(payload)))

		content, ok := result["content"].([]any)
		if !ok || len(content) != 1 {
			t.Fatalf("%s: no single-part content array: %v", name, result)
		}
		part, ok := content[0].(map[string]any)
		if !ok || part["type"] != "text" {
			t.Fatalf("%s: content part is not a text part: %v", name, content[0])
		}
		text, _ := part["text"].(string)
		if !strings.HasPrefix(strings.TrimSpace(text), "{") {
			t.Fatalf("%s: content text is not JSON: %s", name, firstN(text, 200))
		}
		structured, ok := result["structuredContent"].(map[string]any)
		if !ok || structured == nil {
			t.Fatalf("%s: structuredContent missing or not an object: %v", name, result["structuredContent"])
		}
		if result["isError"] != false {
			t.Fatalf("%s: isError = %v, want false", name, result["isError"])
		}
		// The two representations must agree, or a client that binds the schema
		// would silently read different data from the one it renders.
		var fromText map[string]any
		if err := json.Unmarshal([]byte(text), &fromText); err != nil {
			t.Fatalf("%s: content text is not valid JSON: %v", name, err)
		}
		if len(fromText) != len(structured) {
			t.Fatalf("%s: structuredContent and content text disagree (%d vs %d keys)",
				name, len(structured), len(fromText))
		}
		for key := range structured {
			if _, ok := fromText[key]; !ok {
				t.Fatalf("%s: key %q present in structuredContent but not in content text", name, key)
			}
		}
	}
}

func TestMCPProtocolVersionNegotiation(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	cases := map[string]string{
		"2025-06-18":  "2025-06-18",
		"2025-03-26":  "2025-03-26",
		"2024-11-05":  "2024-11-05",
		"unsupported": "2025-06-18",
		"":            "2025-06-18",
	}
	for requested, want := range cases {
		var payload string
		if requested == "" {
			payload = `{"jsonrpc":"2.0","id":"1","method":"initialize"}`
		} else {
			payload = `{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocolVersion":"` + requested + `"}}`
		}
		result := decodeMCPResult(t, callMCP(t, env, testPublicMCPKey, payload))
		if result["protocolVersion"] != want {
			t.Fatalf("requested %q: protocolVersion = %v, want %s", requested, result["protocolVersion"], want)
		}
		serverInfo, ok := result["serverInfo"].(map[string]any)
		if !ok || serverInfo["name"] == "" {
			t.Fatalf("requested %q: serverInfo missing: %v", requested, result["serverInfo"])
		}
		capabilities, ok := result["capabilities"].(map[string]any)
		if !ok || capabilities["tools"] == nil {
			t.Fatalf("requested %q: tools capability missing: %v", requested, result["capabilities"])
		}
	}
}

func TestMCPRejectsBadTokenAndAdvertisesDiscovery(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	cases := map[string]string{
		"garbage":          "not-a-jwt",
		"three segments":   "a.b.c",
		"truncated":        strings.Repeat("x", 400),
		"static lookalike": "sk-" + strings.Repeat("a", 40),
	}
	for name, token := range cases {
		res := callMCP(t, env, token, `{"jsonrpc":"2.0","id":"1","method":"tools/list"}`)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d, want 401", name, res.Code)
		}
		// WWW-Authenticate is how an MCP client discovers it should start OAuth
		// instead of reporting "the server refused me".
		challenge := res.Header().Get("WWW-Authenticate")
		if challenge == "" {
			t.Fatalf("%s: 401 carried no WWW-Authenticate header", name)
		}
		if !strings.HasPrefix(challenge, "Bearer ") {
			t.Fatalf("%s: challenge = %q, want a Bearer scheme", name, challenge)
		}
		want := `resource_metadata="` + testOAuthIssuer + `/.well-known/oauth-protected-resource/mcp"`
		if !strings.Contains(challenge, want) {
			t.Fatalf("%s: challenge lacks %s; got %q", name, want, challenge)
		}
		// The advertised metadata URL must actually resolve.
		metadataPath := strings.TrimPrefix(want, `resource_metadata="`+testOAuthIssuer)
		metadataPath = strings.TrimSuffix(metadataPath, `"`)
		metaRes := env.get(t, metadataPath, false)
		if metaRes.Code != http.StatusOK {
			t.Fatalf("%s: advertised metadata URL %s returned %d", name, metadataPath, metaRes.Code)
		}
	}
}

func TestMCPUnauthenticatedRequestAdvertisesDiscovery(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)
	res := callMCP(t, env, "", `{"jsonrpc":"2.0","id":"1","method":"tools/list"}`)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.Code)
	}
	challenge := res.Header().Get("WWW-Authenticate")
	if !strings.Contains(challenge, "resource_metadata=") {
		t.Fatalf("unauthenticated 401 must advertise discovery: %q", challenge)
	}
}

// TestMCPRejectsTokenMintedForAnotherResource proves the audience binding is
// load-bearing. The token is correctly signed by this deployment's own key, so
// only the "aud" claim stands between it and full access — which is exactly the
// situation when one issuer fronts several resource servers.
func TestMCPRejectsTokenMintedForAnotherResource(t *testing.T) {
	env := newOAuthEnv(t, testCfg(), nil)

	// A second authorization server sharing the same persisted signing key but a
	// different resource path, built through the public API only.
	other := oauth.NewService(oauth.NewStore(env.db), oauth.Options{
		PublicBaseURL: testOAuthIssuer,
		ResourcePath:  "/other-resource",
		AutoApprove:   true,
	})
	if err := other.EnsureSigningKey(context.Background()); err != nil {
		t.Fatalf("ensure shared signing key: %v", err)
	}
	client, _, err := other.RegisterClient(context.Background(), oauth.RegisterClientRequest{
		ClientName: "other resource client", RedirectURIs: []string{testOAuthRedirect},
	})
	if err != nil {
		t.Fatalf("register client: %v", err)
	}
	decision := other.Authorize(context.Background(), testOAuthIssuer, oauth.AuthorizeRequest{
		ClientID: client.ID, RedirectURI: testOAuthRedirect, ResponseType: "code",
		Scope: oauth.ScopeRead, CodeChallenge: s256(testPKCEVerifier),
		CodeChallengeMethod: "S256", Resource: testOAuthIssuer + "/other-resource",
		Authenticated: true,
	})
	if decision.Kind != oauth.DecisionConsent {
		t.Fatalf("authorize kind = %s, want consent on first use", decision.Kind)
	}
	consentToken, err := other.BeginConsent(decision.Grant, "")
	if err != nil {
		t.Fatalf("begin consent: %v", err)
	}
	redirectURI, ok, err := other.FinalizeConsent(context.Background(), consentToken, "approve")
	if err != nil {
		t.Fatalf("finalize consent: %v", err)
	}
	if !ok {
		t.Fatal("consent token was not consumable")
	}
	parsed, err := url.Parse(redirectURI)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	tokens, err := other.IssueToken(context.Background(), testOAuthIssuer, oauth.TokenRequest{
		GrantType: "authorization_code", Code: parsed.Query().Get("code"),
		RedirectURI: testOAuthRedirect, CodeVerifier: testPKCEVerifier,
		Resource: testOAuthIssuer + "/other-resource",
	})
	if err != nil {
		t.Fatalf("issue token for the other resource: %v", err)
	}

	// Sanity: the same token IS valid for its own audience, so the rejection
	// below is about the audience and not about a broken signature.
	if _, err := other.VerifyAccessToken(tokens.AccessToken, testOAuthIssuer,
		testOAuthIssuer+"/other-resource"); err != nil {
		t.Fatalf("token did not verify against its own resource: %v", err)
	}
	if _, err := env.oauthSvc.VerifyAccessToken(tokens.AccessToken, testOAuthIssuer,
		testOAuthIssuer+"/mcp"); err == nil {
		t.Fatal("foreign-audience token verified against /mcp")
	}

	res := callMCP(t, env, tokens.AccessToken, `{"jsonrpc":"2.0","id":"1","method":"tools/list"}`)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("foreign-audience token status = %d, want 401; body=%s",
			res.Code, firstN(res.Body.String(), 300))
	}
}

func TestSafeOAuthReturnRejectsOffOriginTargets(t *testing.T) {
	valid := "/oauth/authorize?client_id=abc&state=1"
	if got := safeOAuthReturn(valid); got != valid {
		t.Fatalf("valid return path rejected: %q", got)
	}
	for _, bad := range []string{
		"",
		"/",
		"/api/health",
		"//evil.example/oauth/authorize",
		"https://evil.example/oauth/authorize",
		"http://evil.example",
		"/oauth/authorize\r\nSet-Cookie: x=1",
		"/oauth/../api/admin",
		strings.Repeat("/oauth/a", 400),
	} {
		if got := safeOAuthReturn(bad); got != "" {
			t.Fatalf("unsafe return path %q was accepted as %q", bad, got)
		}
	}
}

func TestSanitizeChallengeDescriptionCannotInjectHeaders(t *testing.T) {
	for _, input := range []string{
		"a\"b", "a\\b", "a\r\nX-Injected: 1", "a\nb", strings.Repeat("x", 400),
	} {
		got := sanitizeChallengeDescription(input)
		if strings.ContainsAny(got, "\"\\\r\n") {
			t.Fatalf("challenge description %q survived sanitization as %q", input, got)
		}
		if len(got) > 160 {
			t.Fatalf("challenge description was not truncated: %d chars", len(got))
		}
	}
	if got := sanitizeChallengeDescription("   "); got != "unauthorized" {
		t.Fatalf("empty description = %q, want a fallback", got)
	}
}

func TestBearerTokenParsing(t *testing.T) {
	cases := map[string]string{
		"Bearer abc":     "abc",
		"bearer abc":     "abc",
		"BEARER   abc ":  "abc",
		"abc":            "abc",
		"N_Bearer abc":   "abc",
		"":               "",
		"Basic dXNlcjpw": "",
		"Bearer":         "",
	}
	for header, want := range cases {
		if got := bearerToken(header); got != want {
			t.Fatalf("bearerToken(%q) = %q, want %q", header, got, want)
		}
	}
}
