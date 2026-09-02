package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CodeChallengeMethodS256 is the only PKCE method accepted. OAuth 2.1 forbids
// "plain" for public clients, so it is rejected outright rather than negotiated.
const CodeChallengeMethodS256 = "S256"

// ValidateCodeChallenge checks the challenge shape at the authorize endpoint so
// a malformed challenge fails before a session is spent on it.
func ValidateCodeChallenge(challenge, method string) error {
	if challenge == "" {
		return errors.New("code_challenge is required")
	}
	if method == "" {
		// OAuth 2.1: S256 is the default when the method is omitted.
		method = CodeChallengeMethodS256
	}
	if method != CodeChallengeMethodS256 {
		return fmt.Errorf("unsupported code_challenge_method %q", method)
	}
	// RFC 7636 §4.1: 43..128 chars from the unreserved set. base64url of a
	// 32-byte SHA-256 digest is exactly 43 characters.
	if len(challenge) < 43 || len(challenge) > 128 {
		return errors.New("code_challenge length out of range")
	}
	for i := 0; i < len(challenge); i++ {
		c := challenge[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '-' || c == '.' || c == '_' || c == '~':
		default:
			return errors.New("code_challenge contains invalid characters")
		}
	}
	return nil
}

// VerifyCodeVerifier recomputes S256(verifier) and compares it with the stored
// challenge in constant time.
func VerifyCodeVerifier(verifier, challenge string) bool {
	if verifier == "" || challenge == "" {
		return false
	}
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtleEqual(computed, challenge)
}

func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// AuthorizationGrant is the validated state carried by an authorization code.
// Everything the token endpoint needs is bound here, so the token request cannot
// silently swap client, audience or scope.
type AuthorizationGrant struct {
	ClientID            string
	RedirectURI         string
	Scopes              []string
	Resource            string
	CodeChallenge       string
	CodeChallengeMethod string
	SessionID           string
	AuthorizedAt        int64
}

// codeEntry is one pending authorization code.
type codeEntry struct {
	grant     AuthorizationGrant
	expiresAt time.Time
}

// codeStore holds pending authorization codes in memory. Codes live 60 seconds
// and are single-use, so process-local storage is sufficient: a restart at worst
// forces the client to re-run the (already authenticated, one-click) authorize
// step. Refresh tokens — which do need durability — live in the database.
type codeStore struct {
	mu      sync.Mutex
	entries map[string]codeEntry
	ttl     time.Duration
	now     func() time.Time
}

func newCodeStore(ttl time.Duration, now func() time.Time) *codeStore {
	return &codeStore{entries: make(map[string]codeEntry), ttl: ttl, now: now}
}

// Issue stores a grant under a fresh random code.
func (c *codeStore) Issue(grant AuthorizationGrant) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate authorization code: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(buf)
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sweepExpiredLocked(now)
	c.entries[code] = codeEntry{grant: grant, expiresAt: now.Add(c.ttl)}
	return code, nil
}

// Redeem atomically consumes a code. Deleting before returning makes replay
// impossible even under concurrent token requests: exactly one caller wins.
func (c *codeStore) Redeem(code string) (AuthorizationGrant, bool) {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[code]
	if !ok {
		return AuthorizationGrant{}, false
	}
	delete(c.entries, code)
	if !now.Before(entry.expiresAt) {
		return AuthorizationGrant{}, false
	}
	return entry.grant, true
}

// Len reports the number of pending codes (diagnostics/tests).
func (c *codeStore) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func (c *codeStore) sweepExpiredLocked(now time.Time) {
	for code, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, code)
		}
	}
}
