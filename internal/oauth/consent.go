package oauth

import (
	"sync"
	"time"
)

// consentTTL bounds how long a rendered consent screen stays actionable.
const consentTTL = 10 * time.Minute

// consentEntry binds a one-time consent token to the grant it approves.
type consentEntry struct {
	grant     AuthorizationGrant
	state     string
	expiresAt time.Time
}

// consentStore issues and consumes single-use consent tokens.
//
// The consent form is a same-origin POST and the session cookie is SameSite=Lax,
// so cross-site forgery is already blocked twice over. The token is the third
// layer: it proves the POST belongs to a consent screen this server actually
// rendered, which also stops an attacker from replaying an authorize URL into a
// forged approval form.
type consentStore struct {
	mu      sync.Mutex
	entries map[string]consentEntry
	now     func() time.Time
}

func newConsentStore(now func() time.Time) *consentStore {
	return &consentStore{entries: make(map[string]consentEntry), now: now}
}

// Begin stores a pending consent screen and returns its one-time token.
func (c *consentStore) Begin(grant AuthorizationGrant, state string) (string, error) {
	token, err := randomID(24)
	if err != nil {
		return "", err
	}
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
	c.entries[token] = consentEntry{grant: grant, state: state, expiresAt: now.Add(consentTTL)}
	return token, nil
}

// Consume atomically removes and returns a pending consent screen. A token can
// approve exactly one grant, so a double submit cannot issue two codes.
func (c *consentStore) Consume(token string) (AuthorizationGrant, string, bool) {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[token]
	if !ok {
		return AuthorizationGrant{}, "", false
	}
	delete(c.entries, token)
	if !now.Before(entry.expiresAt) {
		return AuthorizationGrant{}, "", false
	}
	return entry.grant, entry.state, true
}

// Len reports pending consent screens (diagnostics/tests).
func (c *consentStore) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// BeginConsent renders a consent screen for a grant and returns the one-time
// token the form must carry back.
func (s *Service) BeginConsent(grant AuthorizationGrant, state string) (string, error) {
	if s.consents == nil {
		s.consents = newConsentStore(s.opts.Now)
	}
	return s.consents.Begin(grant, state)
}

// ConsumeConsent redeems a consent token.
func (s *Service) ConsumeConsent(token string) (AuthorizationGrant, string, bool) {
	if s.consents == nil {
		return AuthorizationGrant{}, "", false
	}
	return s.consents.Consume(token)
}

// DenyRedirect builds the RFC 6749 §4.1.2.1 "access_denied" callback for a
// consent screen the owner rejected. The redirect target is the already-verified
// redirect_uri from the grant, so denying can never become an open redirect.
func (s *Service) DenyRedirect(grant AuthorizationGrant, state string) (string, error) {
	return s.buildErrorRedirect(grant.RedirectURI, "access_denied", "the resource owner denied the request", state)
}

// ErrorRedirect builds a redirect carrying an OAuth error for a grant whose
// redirect target has already been verified.
func (s *Service) ErrorRedirect(grant AuthorizationGrant, code, description, state string) (string, error) {
	return s.buildErrorRedirect(grant.RedirectURI, code, description, state)
}
