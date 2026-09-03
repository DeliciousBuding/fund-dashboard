package oauth

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAuthorizeRequiresLogin(t *testing.T) {
	svc := newTestService(t, nil)
	clientID := registerTestClient(t, svc, "https://chatgpt.com/cb")
	decision := svc.Authorize(context.Background(), testIssuer, AuthorizeRequest{
		ClientID: clientID, RedirectURI: "https://chatgpt.com/cb", ResponseType: "code",
		Scope: ScopeRead, CodeChallenge: s256Challenge(testVerifier),
		CodeChallengeMethod: "S256", State: "xyz", Authenticated: false,
	})
	if decision.Kind != DecisionLogin {
		t.Fatalf("kind = %s, want login", decision.Kind)
	}
	if !strings.HasPrefix(decision.ReturnPath, "/oauth/authorize?") {
		t.Fatalf("return path is not an authorize URL: %q", decision.ReturnPath)
	}
	if !strings.Contains(decision.ReturnPath, "client_id="+clientID) {
		t.Fatalf("return path lost client_id: %q", decision.ReturnPath)
	}
	// The PKCE challenge must survive the login round-trip or the token exchange
	// can never succeed after the owner logs in.
	if !strings.Contains(decision.ReturnPath, "code_challenge=") {
		t.Fatalf("return path lost the PKCE challenge: %q", decision.ReturnPath)
	}
	if !strings.Contains(decision.ReturnPath, "state=xyz") {
		t.Fatalf("return path lost state: %q", decision.ReturnPath)
	}
}

// authorizeReadOnlyForOwner issues an already-authenticated read-scope authorize
// request, the shape a connector produces after the owner has logged in.
func authorizeReadOnlyForOwner(t *testing.T, svc *Service, clientID, state string) AuthorizeDecision {
	t.Helper()
	return svc.Authorize(context.Background(), testIssuer, AuthorizeRequest{
		ClientID: clientID, RedirectURI: "https://chatgpt.com/cb", ResponseType: "code",
		Scope: ScopeRead, CodeChallenge: s256Challenge(testVerifier),
		CodeChallengeMethod: "S256", State: state,
		Resource: svc.Resource(testIssuer), Authenticated: true, SessionID: "sess-1",
	})
}

func TestAuthorizeShowsConsentOnFirstUse(t *testing.T) {
	svc := newTestService(t, nil)
	clientID := registerTestClient(t, svc, "https://chatgpt.com/cb")
	decision := authorizeReadOnlyForOwner(t, svc, clientID, "xyz")
	// Registration is open, so an unknown client_id proves nothing. The consent
	// screen is the only place the owner can notice an unfamiliar connector before
	// it can read the portfolio.
	if decision.Kind != DecisionConsent {
		t.Fatalf("kind = %s, want consent on first use", decision.Kind)
	}
	if decision.Client == nil || decision.Client.ID != clientID {
		t.Fatalf("consent decision lost the client: %+v", decision.Client)
	}
}

func TestAuthorizeIsSilentAfterTheOwnerApprovedOnce(t *testing.T) {
	svc := newTestService(t, nil)
	clientID := registerTestClient(t, svc, "https://chatgpt.com/cb")

	first := authorizeReadOnlyForOwner(t, svc, clientID, "xyz")
	if first.Kind != DecisionConsent {
		t.Fatalf("first kind = %s, want consent", first.Kind)
	}
	approveConsentForTest(t, svc, first.Grant, "xyz")

	// Every later authorization for the same client and scopes is silent, which is
	// the "log in and authorization succeeds" behaviour the owner experiences.
	second := authorizeReadOnlyForOwner(t, svc, clientID, "xyz")
	if second.Kind != DecisionRedirect {
		t.Fatalf("second kind = %s, want redirect (already approved)", second.Kind)
	}
	if !strings.HasPrefix(second.RedirectURI, "https://chatgpt.com/cb?") {
		t.Fatalf("redirect target wrong: %q", second.RedirectURI)
	}
	if queryParam(t, second.RedirectURI, "state") != "xyz" {
		t.Fatalf("state not echoed: %q", second.RedirectURI)
	}
	if queryParam(t, second.RedirectURI, "code") == "" {
		t.Fatalf("code missing: %q", second.RedirectURI)
	}
	// The code must never leak an access token through the front channel.
	if strings.Contains(second.RedirectURI, "access_token") {
		t.Fatalf("front channel leaked a token: %q", second.RedirectURI)
	}
}

func TestAuthorizeAutoApproveCanBeDisabled(t *testing.T) {
	svc := newTestService(t, func(o *Options) { o.AutoApprove = false })
	clientID := registerTestClient(t, svc, "https://chatgpt.com/cb")
	decision := svc.Authorize(context.Background(), testIssuer, AuthorizeRequest{
		ClientID: clientID, RedirectURI: "https://chatgpt.com/cb", ResponseType: "code",
		Scope: ScopeRead, CodeChallenge: s256Challenge(testVerifier),
		CodeChallengeMethod: "S256", Authenticated: true,
	})
	if decision.Kind != DecisionConsent {
		t.Fatalf("kind = %s, want consent when auto-approve is off", decision.Kind)
	}
}

func TestAuthorizeNeverRedirectsForUnverifiedClient(t *testing.T) {
	svc := newTestService(t, func(o *Options) { o.AllowWriteScope = true })
	clientID := registerTestClient(t, svc, "https://chatgpt.com/cb")
	cases := map[string]AuthorizeRequest{
		"unknown client": {
			ClientID: "not-registered", RedirectURI: "https://evil.example/cb",
			ResponseType: "code", CodeChallenge: s256Challenge(testVerifier),
			CodeChallengeMethod: "S256", Authenticated: true,
		},
		"unregistered redirect": {
			ClientID: clientID, RedirectURI: "https://evil.example/cb",
			ResponseType: "code", CodeChallenge: s256Challenge(testVerifier),
			CodeChallengeMethod: "S256", Authenticated: true,
		},
		"missing response type": {
			ClientID: clientID, RedirectURI: "https://chatgpt.com/cb",
			CodeChallenge: s256Challenge(testVerifier), CodeChallengeMethod: "S256",
			Authenticated: true,
		},
		"implicit response type": {
			ClientID: clientID, RedirectURI: "https://chatgpt.com/cb", ResponseType: "token",
			CodeChallenge: s256Challenge(testVerifier), CodeChallengeMethod: "S256",
			Authenticated: true,
		},
		"missing pkce": {
			ClientID: clientID, RedirectURI: "https://chatgpt.com/cb",
			ResponseType: "code", Authenticated: true,
		},
		"plain pkce": {
			ClientID: clientID, RedirectURI: "https://chatgpt.com/cb", ResponseType: "code",
			CodeChallenge: testVerifier, CodeChallengeMethod: "plain", Authenticated: true,
		},
		"foreign resource": {
			ClientID: clientID, RedirectURI: "https://chatgpt.com/cb", ResponseType: "code",
			CodeChallenge: s256Challenge(testVerifier), CodeChallengeMethod: "S256",
			Resource: "https://other.example/mcp", Authenticated: true,
		},
	}
	for name, req := range cases {
		decision := svc.Authorize(context.Background(), testIssuer, req)
		if decision.Kind != DecisionErrorPage {
			t.Fatalf("%s: kind = %s, want error_page", name, decision.Kind)
		}
		if decision.Error == nil {
			t.Fatalf("%s: no error reported", name)
		}
		if decision.Error.Redirectable {
			t.Fatalf("%s: error was marked redirectable", name)
		}
		if decision.RedirectURI != "" {
			t.Fatalf("%s: an unverified request produced a redirect target %q", name, decision.RedirectURI)
		}
	}
}

func TestAuthorizeConsentRequiredForWriteScope(t *testing.T) {
	svc := newTestService(t, func(o *Options) { o.AllowWriteScope = true })
	clientID := registerTestClient(t, svc, "https://chatgpt.com/cb")
	decision := svc.Authorize(context.Background(), testIssuer, AuthorizeRequest{
		ClientID: clientID, RedirectURI: "https://chatgpt.com/cb", ResponseType: "code",
		Scope: ScopeRead + " " + ScopeWrite, CodeChallenge: s256Challenge(testVerifier),
		CodeChallengeMethod: "S256", State: "st", Authenticated: true,
	})
	if decision.Kind != DecisionConsent {
		t.Fatalf("kind = %s, want consent (write scope must never auto-approve)", decision.Kind)
	}
	token, err := svc.BeginConsent(decision.Grant, "st")
	if err != nil {
		t.Fatalf("begin consent: %v", err)
	}
	target, ok, err := svc.FinalizeConsent(context.Background(), token, "approve")
	if err != nil || !ok {
		t.Fatalf("approve: ok=%v err=%v", ok, err)
	}
	if queryParam(t, target, "code") == "" {
		t.Fatalf("consent approval issued no code: %q", target)
	}
	if !strings.HasPrefix(target, "https://chatgpt.com/cb?") {
		t.Fatalf("approval redirected somewhere unexpected: %q", target)
	}

	denyToken, err := svc.BeginConsent(decision.Grant, "st")
	if err != nil {
		t.Fatalf("begin deny consent: %v", err)
	}
	denied, ok, err := svc.FinalizeConsent(context.Background(), denyToken, "deny")
	if err != nil || !ok {
		t.Fatalf("deny: ok=%v err=%v", ok, err)
	}
	if queryParam(t, denied, "error") != "access_denied" {
		t.Fatalf("denial did not produce access_denied: %q", denied)
	}
	if !strings.HasPrefix(denied, "https://chatgpt.com/cb?") {
		t.Fatalf("denial redirected somewhere unexpected: %q", denied)
	}
}

func TestFullAuthorizationCodeFlow(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()
	clientID := registerTestClient(t, svc, "https://chatgpt.com/cb")
	resource := svc.Resource(testIssuer)
	code := authorizeForTest(t, svc, clientID, testVerifier)

	// The token request omits client_id, exactly as OpenAI's connector does.
	tokens, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "authorization_code", Code: code,
		RedirectURI: "https://chatgpt.com/cb", CodeVerifier: testVerifier, Resource: resource,
	})
	if err != nil {
		t.Fatalf("token exchange: %v", err)
	}
	if tokens.TokenType != "Bearer" || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("unexpected token response: %+v", tokens)
	}
	if tokens.Scope != ScopeRead {
		t.Fatalf("scope = %q, want %q", tokens.Scope, ScopeRead)
	}
	if tokens.ExpiresIn != int64((time.Hour).Seconds()) {
		t.Fatalf("expires_in = %d, want 3600", tokens.ExpiresIn)
	}

	verified, err := svc.VerifyAccessToken(tokens.AccessToken, testIssuer, resource)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	if verified.ClientID != clientID || !verified.HasScope(ScopeRead) {
		t.Fatalf("verified claims wrong: %+v", verified)
	}
	if RoleForScopes(verified.Scopes) != "analyst" {
		t.Fatalf("role = %s, want analyst", RoleForScopes(verified.Scopes))
	}

	// The raw refresh token must never be stored: only its sha256 id is.
	stored, err := svc.store.RefreshTokenByID(ctx, tokenID(tokens.RefreshToken), svc.opts.Now().Unix())
	if err != nil || stored == nil {
		t.Fatalf("refresh token was not persisted: %v", err)
	}
	if stored.ClientID != clientID || stored.Resource != resource {
		t.Fatalf("refresh token bound to the wrong client/resource: %+v", stored)
	}

	// Code replay must fail.
	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "authorization_code", Code: code,
		RedirectURI: "https://chatgpt.com/cb", CodeVerifier: testVerifier,
	}); err == nil {
		t.Fatal("authorization code was replayable")
	}

	// Wrong verifier must fail, and the code must be consumed by the attempt.
	code2 := authorizeForTest(t, svc, clientID, testVerifier)
	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "authorization_code", Code: code2,
		RedirectURI: "https://chatgpt.com/cb", CodeVerifier: "wrong-verifier-value-wrong-verifier-value-xxx",
	}); err == nil {
		t.Fatal("wrong code_verifier accepted")
	}
}

func TestTokenExchangeRejectsMismatchedClientAndRedirect(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()
	clientID := registerTestClient(t, svc, "https://chatgpt.com/cb")
	otherID := registerTestClient(t, svc, "https://chatgpt.com/cb")

	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "authorization_code", Code: authorizeForTest(t, svc, clientID, testVerifier),
		ClientID: otherID, RedirectURI: "https://chatgpt.com/cb", CodeVerifier: testVerifier,
	}); err == nil {
		t.Fatal("code was laundered through a different client_id")
	}
	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "authorization_code", Code: authorizeForTest(t, svc, clientID, testVerifier),
		ClientID: clientID, RedirectURI: "https://evil.example/cb", CodeVerifier: testVerifier,
	}); err == nil {
		t.Fatal("mismatched redirect_uri accepted at the token endpoint")
	}
	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "authorization_code", Code: "never-issued", CodeVerifier: testVerifier,
	}); err == nil {
		t.Fatal("unknown code accepted")
	}
	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "authorization_code", CodeVerifier: testVerifier,
	}); err == nil {
		t.Fatal("missing code accepted")
	}
	for _, grantType := range []string{"client_credentials", "password", "implicit", "urn:ietf:params:oauth:grant-type:jwt-bearer"} {
		if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{GrantType: grantType}); err == nil {
			t.Fatalf("grant_type %q accepted", grantType)
		} else {
			failure, ok := asFailure(err)
			if !ok || failure.Code != ErrUnsupportedGrantType {
				t.Fatalf("grant_type %q: error = %v, want unsupported_grant_type", grantType, err)
			}
		}
	}
	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{}); err == nil {
		t.Fatal("missing grant_type accepted")
	}
	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{GrantType: "refresh_token"}); err == nil {
		t.Fatal("refresh grant without a token accepted")
	}
}

func TestRefreshTokenRotation(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()
	clientID := registerTestClient(t, svc, "https://chatgpt.com/cb")
	code := authorizeForTest(t, svc, clientID, testVerifier)
	first, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "authorization_code", Code: code,
		RedirectURI: "https://chatgpt.com/cb", CodeVerifier: testVerifier,
	})
	if err != nil {
		t.Fatalf("initial exchange: %v", err)
	}

	rotated, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "refresh_token", RefreshToken: first.RefreshToken, ClientID: clientID,
	})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if rotated.RefreshToken == first.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	if rotated.AccessToken == first.AccessToken {
		t.Fatal("access token was not reissued on refresh")
	}
	// The presented token is revoked before the replacement is issued, so a
	// replayed refresh token cannot mint a second live session.
	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "refresh_token", RefreshToken: first.RefreshToken,
	}); err == nil {
		t.Fatal("rotated refresh token was replayable")
	}
	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "refresh_token", RefreshToken: "never-issued",
	}); err == nil {
		t.Fatal("unknown refresh token accepted")
	}
	otherID := registerTestClient(t, svc, "https://chatgpt.com/cb")
	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "refresh_token", RefreshToken: rotated.RefreshToken, ClientID: otherID,
	}); err == nil {
		t.Fatal("refresh token accepted for a different client_id")
	}
	if err := svc.RevokeRefreshToken(ctx, rotated.RefreshToken); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := svc.IssueToken(ctx, testIssuer, TokenRequest{
		GrantType: "refresh_token", RefreshToken: rotated.RefreshToken,
	}); err == nil {
		t.Fatal("revoked refresh token still usable")
	}
	// Revoking an unknown token is a no-op, not an error (RFC 7009 §2.2).
	if err := svc.RevokeRefreshToken(ctx, "never-issued"); err != nil {
		t.Fatalf("revoking an unknown token errored: %v", err)
	}
	if err := svc.RevokeClient(ctx, clientID); err != nil {
		t.Fatalf("revoke client: %v", err)
	}
}

func TestExpiredRefreshTokenIsRejected(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	ctx := context.Background()
	now := time.Now().Unix()
	raw, id, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := store.InsertRefreshToken(ctx, StoredRefreshToken{
		ID: id, ClientID: "c1", Scope: ScopeRead, Resource: testIssuer + "/mcp",
		CreatedAt: now - 7200, ExpiresAt: now - 3600,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := store.RefreshTokenByID(ctx, tokenID(raw), now)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got != nil {
		t.Fatalf("expired refresh token was returned: %+v", got)
	}
	if id == raw {
		t.Fatal("refresh token was stored in clear")
	}
}

func TestResourceValidation(t *testing.T) {
	svc := newTestService(t, nil)
	canonical := svc.Resource(testIssuer)
	if got, err := svc.validateResource(testIssuer, ""); err != nil || got != canonical {
		t.Fatalf("absent resource should default to %q: got=%q err=%v", canonical, got, err)
	}
	if got, err := svc.validateResource(testIssuer, canonical); err != nil || got != canonical {
		t.Fatalf("exact resource rejected: got=%q err=%v", got, err)
	}
	if _, err := svc.validateResource(testIssuer, "https://other.example/mcp"); err == nil {
		t.Fatal("foreign resource accepted")
	}
}

func TestIssuerFallsBackOnlyWhenUnconfigured(t *testing.T) {
	configured := newTestService(t, nil)
	if got := configured.Issuer("https://attacker.example"); got != testIssuer {
		t.Fatalf("configured issuer was overridden by a request-derived value: %q", got)
	}
	unconfigured := newTestService(t, func(o *Options) { o.PublicBaseURL = "" })
	if got := unconfigured.Issuer("http://localhost:9999"); got != "http://localhost:9999" {
		t.Fatalf("fallback issuer = %q", got)
	}
	if got := unconfigured.Issuer("https://host.example/"); got != "https://host.example" {
		t.Fatalf("fallback issuer kept a trailing slash: %q", got)
	}
}

func TestNilStoreDegradesWithoutPersistence(t *testing.T) {
	svc := NewService(nil, Options{PublicBaseURL: testIssuer})
	if err := svc.EnsureSigningKey(context.Background()); err != nil {
		t.Fatalf("ephemeral key: %v", err)
	}
	if _, _, err := svc.RegisterClient(context.Background(), RegisterClientRequest{
		RedirectURIs: []string{"https://chatgpt.com/cb"},
	}); err != nil {
		t.Fatalf("register without store: %v", err)
	}
	if _, err := svc.IssueToken(context.Background(), testIssuer, TokenRequest{
		GrantType: "refresh_token", RefreshToken: "x",
	}); err == nil {
		t.Fatal("refresh should be unavailable without persistence")
	}
	if err := svc.RevokeRefreshToken(context.Background(), "x"); err != nil {
		t.Fatalf("revoke without store should be a no-op: %v", err)
	}
}

func TestFailureWrappingAndStatus(t *testing.T) {
	failure := fail(ErrInvalidGrant, http.StatusBadRequest, "code %s", "abc")
	if failure.Error() != "invalid_grant: code abc" {
		t.Fatalf("error string = %q", failure.Error())
	}
	if failure.Unwrap() != ErrInvalidGrant {
		t.Fatal("Unwrap did not return the OAuth code")
	}
	if fail(ErrServerError, 0, "x").Error() != "server_error: x" {
		t.Fatal("unexpected formatting")
	}
	if fail(ErrServerError, 0, "plain").Error() != "server_error: plain" {
		t.Fatal("no-arg formatting changed the message")
	}
}
