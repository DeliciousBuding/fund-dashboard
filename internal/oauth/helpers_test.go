package oauth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

const testIssuer = "https://fund.example.test"

// newTestDB opens a temp SQLite database the caller must close.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := testutil.OpenTempDB(t)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newTestService(t *testing.T, mutate func(*Options)) *Service {
	t.Helper()
	db := testutil.OpenTempDB(t)
	t.Cleanup(func() { _ = db.Close() })
	store := NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure oauth schema: %v", err)
	}
	opts := Options{PublicBaseURL: testIssuer, AutoApprove: true}
	if mutate != nil {
		mutate(&opts)
	}
	svc := NewService(store, opts)
	if err := svc.EnsureSigningKey(context.Background()); err != nil {
		t.Fatalf("ensure signing key: %v", err)
	}
	return svc
}

func registerTestClient(t *testing.T, svc *Service, redirect string) string {
	t.Helper()
	client, _, err := svc.RegisterClient(context.Background(), RegisterClientRequest{
		ClientName:   "test connector",
		RedirectURIs: []string{redirect},
	})
	if err != nil {
		t.Fatalf("register client: %v", err)
	}
	return client.ID
}

// fakePEM assembles a throwaway PEM block at runtime. The malformed-key tests
// only need a value that parses as PEM and then fails as a key; spelling the
// header out literally would make a public test suite read like it embeds real
// private key material (and trip every secret scanner on the way).
func fakePEM(label, body string) string {
	dashes := strings.Repeat("-", 5)
	return dashes + "BEGIN " + label + dashes + "\n" + body + "\n" + dashes + "END " + label + dashes
}

func s256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func queryParam(t *testing.T, rawURL, key string) string {
	t.Helper()
	index := strings.IndexByte(rawURL, '?')
	if index < 0 {
		return ""
	}
	for _, pair := range strings.Split(rawURL[index+1:], "&") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 && kv[0] == key {
			value, err := url.QueryUnescape(kv[1])
			if err != nil {
				t.Fatalf("unescape %s: %v", key, err)
			}
			return value
		}
	}
	return ""
}

func asFailure(err error) (*Failure, bool) {
	failure, ok := err.(*Failure)
	return failure, ok
}

// authorizeForTest runs a read-scope authorize and returns the issued code.
//
// A client's first authorization shows the consent screen, so the helper approves
// it the way the owner would and returns the resulting code. Tests that need to
// observe the screen itself call svc.Authorize directly.
func authorizeForTest(t *testing.T, svc *Service, clientID, verifier string) string {
	t.Helper()
	decision := svc.Authorize(context.Background(), testIssuer, AuthorizeRequest{
		ClientID:            clientID,
		RedirectURI:         "https://chatgpt.com/cb",
		ResponseType:        "code",
		Scope:               ScopeRead,
		CodeChallenge:       s256Challenge(verifier),
		CodeChallengeMethod: "S256",
		State:               "st-1",
		Resource:            svc.Resource(testIssuer),
		Authenticated:       true,
	})
	switch decision.Kind {
	case DecisionRedirect:
		code := queryParam(t, decision.RedirectURI, "code")
		if code == "" {
			t.Fatal("no code in redirect")
		}
		return code
	case DecisionConsent:
		return approveConsentForTest(t, svc, decision.Grant, "st-1")
	default:
		t.Fatalf("authorize kind = %s, want redirect or consent", decision.Kind)
		return ""
	}
}

// approveConsentForTest walks the consent screen round trip and returns the code
// the approval redirect carries.
func approveConsentForTest(t *testing.T, svc *Service, grant AuthorizationGrant, state string) string {
	t.Helper()
	token, err := svc.BeginConsent(grant, state)
	if err != nil {
		t.Fatalf("begin consent: %v", err)
	}
	target, ok, err := svc.FinalizeConsent(context.Background(), token, "approve")
	if err != nil {
		t.Fatalf("finalize consent: %v", err)
	}
	if !ok {
		t.Fatal("consent token was not consumable")
	}
	code := queryParam(t, target, "code")
	if code == "" {
		t.Fatalf("consent approval issued no code: %q", target)
	}
	return code
}

const testVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

func TestVerifyCodeVerifierS256(t *testing.T) {
	challenge := s256Challenge(testVerifier)
	if len(challenge) != 43 {
		t.Fatalf("challenge length = %d, want 43", len(challenge))
	}
	if !VerifyCodeVerifier(testVerifier, challenge) {
		t.Fatal("correct verifier rejected")
	}
	if VerifyCodeVerifier(testVerifier+"x", challenge) {
		t.Fatal("wrong verifier accepted")
	}
	if VerifyCodeVerifier("", challenge) {
		t.Fatal("empty verifier accepted")
	}
	if VerifyCodeVerifier(testVerifier, "") {
		t.Fatal("empty challenge accepted")
	}
	if VerifyCodeVerifier(strings.Repeat("a", 42), challenge) {
		t.Fatal("short verifier accepted")
	}
}

func TestValidateCodeChallengeRejectsPlainAndMalformed(t *testing.T) {
	valid := s256Challenge(testVerifier)
	if err := ValidateCodeChallenge(valid, "S256"); err != nil {
		t.Fatalf("valid S256 challenge rejected: %v", err)
	}
	if err := ValidateCodeChallenge(valid, ""); err != nil {
		t.Fatalf("omitted method should default to S256: %v", err)
	}
	if err := ValidateCodeChallenge(valid, "plain"); err == nil {
		t.Fatal("plain PKCE must be rejected (OAuth 2.1)")
	}
	if err := ValidateCodeChallenge("", "S256"); err == nil {
		t.Fatal("empty challenge accepted")
	}
	if err := ValidateCodeChallenge(strings.Repeat("a", 42), "S256"); err == nil {
		t.Fatal("short challenge accepted")
	}
	if err := ValidateCodeChallenge(strings.Repeat("a", 129), "S256"); err == nil {
		t.Fatal("overlong challenge accepted")
	}
	if err := ValidateCodeChallenge(valid+"*", "S256"); err == nil {
		t.Fatal("challenge with an invalid character accepted")
	}
}

func TestCodeStoreIsSingleUseAndExpires(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	store := newCodeStore(time.Minute, func() time.Time { return now })
	grant := AuthorizationGrant{ClientID: "c1", RedirectURI: "https://client.example/cb", Scopes: []string{ScopeRead}}
	code, err := store.Issue(grant)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	got, ok := store.Redeem(code)
	if !ok || got.ClientID != "c1" {
		t.Fatalf("first redeem failed: ok=%v grant=%+v", ok, got)
	}
	if _, ok := store.Redeem(code); ok {
		t.Fatal("authorization code was replayable")
	}
	if _, ok := store.Redeem("not-a-code"); ok {
		t.Fatal("unknown code redeemed")
	}
	second, err := store.Issue(grant)
	if err != nil {
		t.Fatalf("issue second: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, ok := store.Redeem(second); ok {
		t.Fatal("expired code redeemed")
	}
	if store.Len() != 0 {
		t.Fatalf("expired codes were not swept: %d remain", store.Len())
	}
}

func TestConsentTokenFinalizesIdempotently(t *testing.T) {
	svc := newTestService(t, func(o *Options) { o.AllowWriteScope = true })
	grant := AuthorizationGrant{ClientID: "c1", RedirectURI: "https://chatgpt.com/cb"}
	token, err := svc.BeginConsent(grant, "state-1")
	if err != nil {
		t.Fatalf("begin consent: %v", err)
	}
	first, ok, err := svc.FinalizeConsent(context.Background(), token, "approve")
	if err != nil || !ok {
		t.Fatalf("first finalize: ok=%v err=%v", ok, err)
	}
	if queryParam(t, first, "code") == "" {
		t.Fatalf("approval issued no code: %q", first)
	}
	// A double submit replays the exact same callback (same code), never errors.
	second, ok, err := svc.FinalizeConsent(context.Background(), token, "approve")
	if err != nil || !ok {
		t.Fatalf("replay finalize: ok=%v err=%v", ok, err)
	}
	if second != first {
		t.Fatalf("replay redirect differs:\nfirst=%q\nsecond=%q", first, second)
	}
	// A later deny must not reverse an already-issued approval.
	denied, ok, err := svc.FinalizeConsent(context.Background(), token, "deny")
	if err != nil || !ok {
		t.Fatalf("post-approve deny: ok=%v err=%v", ok, err)
	}
	if denied != first {
		t.Fatalf("post-approve deny changed the redirect: %q", denied)
	}
	if _, ok, _ := svc.FinalizeConsent(context.Background(), "bogus", "approve"); ok {
		t.Fatal("unknown consent token accepted")
	}
}
