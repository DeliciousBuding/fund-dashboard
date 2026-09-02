package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

func TestAccessTokenSignVerifyRoundTrip(t *testing.T) {
	svc := newTestService(t, nil)
	token, err := svc.SignAccessToken(accessTokenClaims{
		Issuer:   testIssuer,
		Subject:  Subject,
		Audience: svc.Resource(testIssuer),
		ClientID: "client-1",
		Scope:    ScopeRead,
		Expiry:   svc.opts.Now().Add(time.Hour).Unix(),
		IssuedAt: svc.opts.Now().Unix(),
		JWTID:    "jti-1",
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("token is not three segments: %q", token)
	}
	verified, err := svc.VerifyAccessToken(token, testIssuer, svc.Resource(testIssuer))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if verified.ClientID != "client-1" || !verified.HasScope(ScopeRead) || verified.JWTID != "jti-1" {
		t.Fatalf("claims lost: %+v", verified)
	}
	if verified.Subject != Subject {
		t.Fatalf("subject = %q, want %q", verified.Subject, Subject)
	}
}

func TestAccessTokenVerificationFailsClosed(t *testing.T) {
	svc := newTestService(t, nil)
	audience := svc.Resource(testIssuer)
	mint := func(issuer, aud string, expiry time.Time, scope string) string {
		token, err := svc.SignAccessToken(accessTokenClaims{
			Issuer: issuer, Subject: Subject, Audience: aud, Scope: scope,
			Expiry: expiry.Unix(), IssuedAt: svc.opts.Now().Unix(),
		})
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return token
	}
	good := mint(testIssuer, audience, svc.opts.Now().Add(time.Hour), ScopeRead)

	if _, err := svc.VerifyAccessToken(mint("https://evil.example", audience, svc.opts.Now().Add(time.Hour), ScopeRead), testIssuer, audience); err == nil {
		t.Fatal("token with a foreign issuer was accepted")
	}
	if _, err := svc.VerifyAccessToken(mint(testIssuer, "https://other.example/mcp", svc.opts.Now().Add(time.Hour), ScopeRead), testIssuer, audience); err == nil {
		t.Fatal("token with a foreign audience was accepted")
	}
	if _, err := svc.VerifyAccessToken(mint(testIssuer, audience, svc.opts.Now().Add(-time.Hour), ScopeRead), testIssuer, audience); err == nil {
		t.Fatal("expired token was accepted")
	}

	// Tampered payload: keep the real signature, swap in a write-scope claim.
	parts := strings.Split(good, ".")
	forged, err := json.Marshal(accessTokenClaims{
		Issuer: testIssuer, Subject: Subject, Audience: audience,
		Scope: ScopeWrite, Expiry: svc.opts.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("marshal forged claims: %v", err)
	}
	parts[1] = base64.RawURLEncoding.EncodeToString(forged)
	if _, err := svc.VerifyAccessToken(strings.Join(parts, "."), testIssuer, audience); err == nil {
		t.Fatal("forged claims segment was accepted")
	}

	// alg=none downgrade must never reach the signature check.
	headerNone := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	if _, err := svc.VerifyAccessToken(headerNone+"."+parts[1]+".", testIssuer, audience); err == nil {
		t.Fatal("alg=none token was accepted")
	}
	// Algorithm confusion: claim HS256 with the same bytes.
	headerHS := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	if _, err := svc.VerifyAccessToken(headerHS+"."+parts[1]+"."+parts[2], testIssuer, audience); err == nil {
		t.Fatal("HS256-confused token was accepted")
	}
	// Unknown kid must be rejected even with a well-formed signature.
	headerBadKid := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","typ":"JWT","kid":"attacker"}`))
	if _, err := svc.VerifyAccessToken(headerBadKid+"."+parts[1]+"."+parts[2], testIssuer, audience); err == nil {
		t.Fatal("token claiming an unknown kid was accepted")
	}
	for _, bad := range []string{"", "abc", "a.b", "a.b.c.d", "!!!.###.$$$", good[:len(good)-4]} {
		if _, err := svc.VerifyAccessToken(bad, testIssuer, audience); err == nil {
			t.Fatalf("malformed token %q was accepted", bad)
		}
	}
}

func TestSigningKeyPersistsAcrossInstances(t *testing.T) {
	db := testutil.OpenTempDB(t)
	defer db.Close()
	store := NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	ctx := context.Background()

	first := NewService(store, Options{PublicBaseURL: testIssuer})
	if err := first.EnsureSigningKey(ctx); err != nil {
		t.Fatalf("first key: %v", err)
	}
	token, err := first.SignAccessToken(accessTokenClaims{
		Issuer: testIssuer, Subject: Subject, Audience: first.Resource(testIssuer),
		Scope: ScopeRead, Expiry: time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// A restart must reuse the persisted key, otherwise every issued token dies
	// with the process and connectors would need re-authorizing after each deploy.
	second := NewService(store, Options{PublicBaseURL: testIssuer})
	if err := second.EnsureSigningKey(ctx); err != nil {
		t.Fatalf("second key: %v", err)
	}
	if first.activeKey().kid != second.activeKey().kid {
		t.Fatalf("kid changed across restart: %s != %s", first.activeKey().kid, second.activeKey().kid)
	}
	if _, err := second.VerifyAccessToken(token, testIssuer, second.Resource(testIssuer)); err != nil {
		t.Fatalf("token from the previous instance failed to verify: %v", err)
	}

	stored, err := store.LoadSigningKey(ctx)
	if err != nil || stored == nil {
		t.Fatalf("signing key was not persisted: %v", err)
	}
	if stored.Algorithm != AlgES256 {
		t.Fatalf("stored algorithm = %q", stored.Algorithm)
	}
	if !strings.Contains(stored.PrivateKeyPEM, "BEGIN PRIVATE KEY") {
		t.Fatal("stored key is not PKCS#8 PEM")
	}
}

func TestExplicitSigningKeyPEMIsHonoured(t *testing.T) {
	generated, err := generateSigningKey()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	svc := newTestService(t, func(o *Options) { o.SigningKeyPEM = encodePrivateKeyPEM(generated.private) })
	if svc.activeKey().kid != generated.kid {
		t.Fatalf("explicit PEM ignored: kid %s != %s", svc.activeKey().kid, generated.kid)
	}
}

func TestEnsureSigningKeyRejectsBadPEM(t *testing.T) {
	db := testutil.OpenTempDB(t)
	defer db.Close()
	store := NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	rsaLike := "-----BEGIN RSA PRIVATE KEY-----\nZm9v\n-----END RSA PRIVATE KEY-----"
	for _, bad := range []string{"not-a-pem", rsaLike, "-----BEGIN PRIVATE KEY-----\nZm9v\n-----END PRIVATE KEY-----"} {
		svc := NewService(store, Options{PublicBaseURL: testIssuer, SigningKeyPEM: bad})
		if err := svc.EnsureSigningKey(context.Background()); err == nil {
			t.Fatalf("bad signing key %q accepted", bad)
		}
	}
}

func TestSigningAndVerificationRequireAnInitializedKey(t *testing.T) {
	svc := NewService(nil, Options{PublicBaseURL: testIssuer})
	if _, err := svc.SignAccessToken(accessTokenClaims{}); err == nil {
		t.Fatal("signing without a key succeeded")
	}
	if _, err := svc.VerifyAccessToken("a.b.c", testIssuer, testIssuer+"/mcp"); err == nil {
		t.Fatal("verification without a key succeeded")
	}
	if _, err := svc.JWKS(); err == nil {
		t.Fatal("jwks without a key succeeded")
	}
}

func TestJWKSThumbprintIsStableAndLeaksNothing(t *testing.T) {
	first, err := generateSigningKey()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// The same key material must always produce the same kid (RFC 7638).
	again := newSigningKey(first.private)
	if again.kid != first.kid {
		t.Fatalf("kid is not deterministic: %s != %s", again.kid, first.kid)
	}
	encoded := mustJSON(first.jwk)
	for _, forbidden := range []string{`"d"`, "private", first.private.D.String()} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("public JWK leaked private material (%q)", forbidden)
		}
	}
}
