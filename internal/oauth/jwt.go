package oauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

// AlgES256 is the only signing algorithm this server issues. Pinning a single
// algorithm means verification can reject "alg: none" and RS/HS confusion by
// construction: a token whose header is not exactly ES256 never reaches the
// signature check.
const AlgES256 = "ES256"

// accessTokenClaims is the RFC 9068-shaped JWT payload. "aud" is pinned to the
// MCP resource URL so a token minted for this resource server cannot be replayed
// against another service that happens to trust the same issuer.
type accessTokenClaims struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	Audience  string `json:"aud"`
	ClientID  string `json:"client_id,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Expiry    int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
	JWTID     string `json:"jti,omitempty"`
	TokenType string `json:"token_type,omitempty"`
}

// AccessToken is the verified view of a bearer token consumed by the resource
// server.
type AccessToken struct {
	Issuer   string
	Audience string
	ClientID string
	Subject  string
	Scopes   []string
	Expiry   time.Time
	JWTID    string
}

// HasScope reports whether the token carries scope.
func (t *AccessToken) HasScope(scope string) bool {
	for _, s := range t.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// signingKey is one ES256 keypair plus its stable key id.
type signingKey struct {
	kid     string
	private *ecdsa.PrivateKey
	jwk     map[string]any
}

// keySet guards the active signing key. It is written once at boot
// (EnsureSigningKey) and read on every token issue/verify, so a RWMutex keeps
// the MCP hot path lock-free in the common case.
type keySet struct {
	mu     sync.RWMutex
	active *signingKey
}

func (k *keySet) get() *signingKey {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.active
}

func (k *keySet) set(key *signingKey) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.active = key
}

// EnsureSigningKey resolves the active ES256 key. Order: explicit PEM from
// FUND_OAUTH_SIGNING_KEY, then the persisted database key, then a freshly
// generated key persisted for the next boot. Generating on first use means a
// deployment needs no key ceremony while still surviving restarts.
func (s *Service) EnsureSigningKey(ctx context.Context) error {
	if s.keys == nil {
		s.keys = &keySet{}
	}
	if pemKey := strings.TrimSpace(s.opts.SigningKeyPEM); pemKey != "" {
		key, err := parsePrivateKeyPEM(pemKey)
		if err != nil {
			return fmt.Errorf("oauth: parse FUND_OAUTH_SIGNING_KEY: %w", err)
		}
		s.keys.set(key)
		return nil
	}
	if s.store == nil {
		// No persistence available: fall back to an ephemeral key so development
		// and tests still work. Tokens die with the process.
		key, err := generateSigningKey()
		if err != nil {
			return err
		}
		s.keys.set(key)
		return nil
	}
	stored, err := s.store.LoadSigningKey(ctx)
	if err != nil {
		return fmt.Errorf("oauth: load signing key: %w", err)
	}
	if stored != nil {
		key, err := parsePrivateKeyPEM(stored.PrivateKeyPEM)
		if err != nil {
			return fmt.Errorf("oauth: stored signing key is unusable: %w", err)
		}
		s.keys.set(key)
		return nil
	}
	key, err := generateSigningKey()
	if err != nil {
		return err
	}
	// Insert-if-absent: two racing boots converge on one persisted key, and the
	// loser adopts the winner's key instead of diverging.
	inserted, err := s.store.InsertSigningKeyIfAbsent(ctx, StoredSigningKey{
		Kid:           key.kid,
		Algorithm:     AlgES256,
		PrivateKeyPEM: encodePrivateKeyPEM(key.private),
		PublicJWKJSON: mustJSON(key.jwk),
		CreatedAt:     s.opts.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("oauth: persist signing key: %w", err)
	}
	if !inserted {
		winner, err := s.store.LoadSigningKey(ctx)
		if err != nil {
			return fmt.Errorf("oauth: reload signing key: %w", err)
		}
		if winner == nil {
			return errors.New("oauth: signing key vanished during generation")
		}
		adopted, err := parsePrivateKeyPEM(winner.PrivateKeyPEM)
		if err != nil {
			return fmt.Errorf("oauth: persisted signing key is unusable: %w", err)
		}
		s.keys.set(adopted)
		return nil
	}
	s.keys.set(key)
	return nil
}

// JWKS returns the public key set advertised at /oauth/jwks so third parties can
// verify tokens out of band.
func (s *Service) JWKS() (map[string]any, error) {
	key := s.activeKey()
	if key == nil {
		return nil, errors.New("oauth: signing key not initialized")
	}
	return map[string]any{"keys": []any{key.jwk}}, nil
}

func (s *Service) activeKey() *signingKey {
	if s.keys == nil {
		return nil
	}
	return s.keys.get()
}

// SignAccessToken mints a compact-serialized ES256 JWT.
func (s *Service) SignAccessToken(claims accessTokenClaims) (string, error) {
	key := s.activeKey()
	if key == nil {
		return "", errors.New("oauth: signing key not initialized")
	}
	header := map[string]any{"alg": AlgES256, "typ": "JWT", "kid": key.kid}
	signingInput, err := encodeSegmentPair(header, claims)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(signingInput))
	asn1Sig, err := ecdsa.SignASN1(rand.Reader, key.private, digest[:])
	if err != nil {
		return "", fmt.Errorf("oauth: sign access token: %w", err)
	}
	raw, err := asn1ToRawES256(asn1Sig)
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(raw), nil
}

// VerifyAccessToken validates signature, algorithm, issuer, audience and expiry.
// Every check fails closed: an unknown kid, a non-ES256 header, a mismatched
// audience or a clock-expired token all yield the same generic invalid_token.
func (s *Service) VerifyAccessToken(token, expectedIssuer, expectedAudience string) (*AccessToken, error) {
	key := s.activeKey()
	if key == nil {
		return nil, errors.New("signing key not initialized")
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed token")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("malformed token header")
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, errors.New("malformed token header")
	}
	if header.Alg != AlgES256 {
		return nil, fmt.Errorf("unsupported alg %q", header.Alg)
	}
	if header.Kid != key.kid {
		return nil, errors.New("unknown signing key id")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("malformed token signature")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r, ss, err := rawToASN1ES256(signature)
	if err != nil {
		return nil, err
	}
	if !ecdsa.VerifyASN1(&key.private.PublicKey, digest[:], mustASN1(r, ss)) {
		return nil, errors.New("signature mismatch")
	}
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("malformed token claims")
	}
	var claims accessTokenClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, errors.New("malformed token claims")
	}
	if claims.Issuer != expectedIssuer {
		return nil, fmt.Errorf("issuer mismatch: %q", claims.Issuer)
	}
	if claims.Audience != expectedAudience {
		return nil, fmt.Errorf("audience mismatch: %q", claims.Audience)
	}
	now := s.opts.Now()
	if claims.Expiry != 0 && now.Unix() >= claims.Expiry {
		return nil, errors.New("token expired")
	}
	return &AccessToken{
		Issuer:   claims.Issuer,
		Audience: claims.Audience,
		ClientID: claims.ClientID,
		Subject:  claims.Subject,
		Scopes:   splitScopes(claims.Scope),
		Expiry:   time.Unix(claims.Expiry, 0),
		JWTID:    claims.JWTID,
	}, nil
}

func mustASN1(r, ss *big.Int) []byte {
	out, err := asn1.Marshal(struct{ R, S *big.Int }{r, ss})
	if err != nil {
		return nil
	}
	return out
}

// asn1ToRawES256 converts a DER ECDSA signature into the fixed 64-byte r||s
// JOSE encoding (SEC1 lengths, left-padded).
func asn1ToRawES256(der []byte) ([]byte, error) {
	var parsed struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(der, &parsed); err != nil {
		return nil, fmt.Errorf("oauth: decode ECDSA signature: %w", err)
	}
	raw := make([]byte, 64)
	copy(raw[32-len(parsed.R.Bytes()):32], parsed.R.Bytes())
	copy(raw[64-len(parsed.S.Bytes()):64], parsed.S.Bytes())
	return raw, nil
}

func rawToASN1ES256(raw []byte) (*big.Int, *big.Int, error) {
	if len(raw) != 64 {
		return nil, nil, errors.New("malformed ES256 signature length")
	}
	return new(big.Int).SetBytes(raw[:32]), new(big.Int).SetBytes(raw[32:]), nil
}

func generateSigningKey() (*signingKey, error) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("oauth: generate signing key: %w", err)
	}
	return newSigningKey(private), nil
}

func newSigningKey(private *ecdsa.PrivateKey) *signingKey {
	pub := private.PublicKey
	x := base64.RawURLEncoding.EncodeToString(padded(pub.X, 32))
	y := base64.RawURLEncoding.EncodeToString(padded(pub.Y, 32))
	jwk := map[string]any{
		"kty": "EC",
		"crv": "P-256",
		"x":   x,
		"y":   y,
		"alg": AlgES256,
		"use": "sig",
	}
	kid := jwkThumbprint(jwk)
	jwk["kid"] = kid
	return &signingKey{kid: kid, private: private, jwk: jwk}
}

// jwkThumbprint is RFC 7638 over the required EC members in lexicographic order,
// giving a key id that is stable across restarts and deployments.
func jwkThumbprint(jwk map[string]any) string {
	canonical := fmt.Sprintf(`{"crv":"%s","kty":"%s","x":"%s","y":"%s"}`,
		jwk["crv"], jwk["kty"], jwk["x"], jwk["y"])
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

func padded(value *big.Int, size int) []byte {
	raw := value.Bytes()
	if len(raw) >= size {
		return raw[len(raw)-size:]
	}
	out := make([]byte, size)
	copy(out[size-len(raw):], raw)
	return out
}

func parsePrivateKeyPEM(raw string) (*signingKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("not a PEM block")
	}
	var (
		key any
		err error
	)
	switch block.Type {
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported PEM type %q (want PRIVATE KEY or EC PRIVATE KEY)", block.Type)
	}
	if err != nil {
		return nil, err
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not an ECDSA private key")
	}
	if ecKey.Curve != elliptic.P256() {
		return nil, errors.New("key must use the P-256 curve (ES256)")
	}
	return newSigningKey(ecKey), nil
}

func encodePrivateKeyPEM(private *ecdsa.PrivateKey) string {
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return ""
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func encodeSegmentPair(header, claims any) (string, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("oauth: encode token header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("oauth: encode token claims: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON), nil
}

func mustJSON(value any) string {
	out, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(out)
}

func splitScopes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Fields(raw)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func joinScopes(scopes []string) string {
	return strings.Join(scopes, " ")
}
