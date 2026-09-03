package oauth

import (
	"context"
	"sync"
	"time"
)

// consentTTL bounds how long a rendered consent screen stays actionable.
const consentTTL = 10 * time.Minute

// consentEntry binds a one-time consent token to the grant it approves. Once the
// owner submits a decision the entry remembers the resulting redirect, so a
// double submit replays the same target instead of stranding the owner on an
// error page after the code was already issued.
type consentEntry struct {
	grant     AuthorizationGrant
	state     string
	expiresAt time.Time
	// redirect is empty until the entry is finalized by the first decision.
	redirect string
}

// consentStore issues and finalizes single-use consent tokens.
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

// Finalize atomically maps a consent token to its final redirect exactly once.
// build is invoked only by the first submitter (approve or deny); every later
// submitter receives the already-issued redirect. A token can therefore never
// approve two grants, but a browser that double-submits the form is sent to the
// same callback with the same code rather than to an error page.
//
// The returned ok is false only when the token is unknown or expired. When build
// fails (for example the approval could not be persisted) the entry is left
// pending so the owner can retry, and the error is returned unchanged.
func (c *consentStore) Finalize(token string, build func(AuthorizationGrant, string) (string, error)) (redirect string, ok bool, err error) {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
	entry, exists := c.entries[token]
	if !exists {
		return "", false, nil
	}
	if entry.redirect != "" {
		return entry.redirect, true, nil
	}
	redirect, err = build(entry.grant, entry.state)
	if err != nil {
		return "", false, err
	}
	entry.redirect = redirect
	c.entries[token] = entry
	return redirect, true, nil
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

// FinalizeConsent redeems a consent token against a decision and returns the
// final redirect. Replaying the same token returns the same redirect.
func (s *Service) FinalizeConsent(ctx context.Context, token, decision string) (redirect string, ok bool, err error) {
	if s.consents == nil {
		s.consents = newConsentStore(s.opts.Now)
	}
	return s.consents.Finalize(token, func(grant AuthorizationGrant, state string) (string, error) {
		if decision == "deny" {
			return s.DenyRedirect(grant, state)
		}
		return s.ApproveGrant(ctx, grant, state)
	})
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
