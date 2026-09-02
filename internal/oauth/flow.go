package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// DecisionKind classifies what the authorize handler must do next.
type DecisionKind string

const (
	// DecisionRedirect means authorization succeeded: redirect to RedirectURI,
	// which already carries code and state.
	DecisionRedirect DecisionKind = "redirect"
	// DecisionLogin means the caller has no dashboard session: send them to the
	// login page with a return path back to this same authorize URL.
	DecisionLogin DecisionKind = "login"
	// DecisionConsent means the grant needs an explicit click (write scopes, or
	// auto-approve disabled): render the consent page.
	DecisionConsent DecisionKind = "consent"
	// DecisionErrorPage means the request cannot be safely redirected (unknown
	// client or unregistered redirect_uri), so it must be rendered locally.
	DecisionErrorPage DecisionKind = "error_page"
)

// AuthorizeRequest is the parsed GET /oauth/authorize query.
type AuthorizeRequest struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resource            string
	// Authenticated reports whether the caller holds a valid dashboard session.
	Authenticated bool
	SessionID     string
}

// AuthorizeDecision is the outcome the handler acts on.
type AuthorizeDecision struct {
	Kind        DecisionKind
	RedirectURI string
	Client      *Client
	Scopes      []string
	Grant       AuthorizationGrant
	Resource    string
	Error       *Failure
	// ReturnPath is the authorize URL to come back to after login.
	ReturnPath string
}

// Authorize validates an authorization request. It never issues a code when the
// client or redirect target is unverified — in that case it returns
// DecisionErrorPage so the handler cannot be turned into an open redirect.
func (s *Service) Authorize(ctx context.Context, issuer string, req AuthorizeRequest) AuthorizeDecision {
	resource, err := s.validateResource(issuer, req.Resource)
	if err != nil {
		return s.unsafeFailure(issuer, req, err)
	}
	if strings.TrimSpace(req.ResponseType) == "" {
		return s.unsafeFailure(issuer, req, fail(ErrInvalidRequest, http.StatusBadRequest, "response_type is required"))
	}
	if req.ResponseType != "code" {
		return s.unsafeFailure(issuer, req, fail(ErrUnsupportedResponseType, http.StatusBadRequest,
			"response_type %q is not supported; only \"code\" is", req.ResponseType))
	}
	if err := ValidateCodeChallenge(req.CodeChallenge, req.CodeChallengeMethod); err != nil {
		return s.unsafeFailure(issuer, req, fail(ErrInvalidRequest, http.StatusBadRequest, "%v", err))
	}
	client, err := s.ResolveClient(ctx, req.ClientID)
	if err != nil {
		return s.unsafeFailure(issuer, req, err)
	}
	redirectURI, err := ValidateRedirectURI(client, req.RedirectURI)
	if err != nil {
		// The redirect target is not verified: never redirect, render locally.
		return AuthorizeDecision{
			Kind:   DecisionErrorPage,
			Client: client,
			Error:  fail(ErrInvalidRedirectURI, http.StatusBadRequest, "%v", err),
		}
	}
	method := strings.TrimSpace(req.CodeChallengeMethod)
	if method == "" {
		method = CodeChallengeMethodS256
	}
	scopes := s.NegotiateScopes(req.Scope)
	grant := AuthorizationGrant{
		ClientID:            client.ID,
		RedirectURI:         redirectURI,
		Scopes:              scopes,
		Resource:            resource,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: method,
		SessionID:           req.SessionID,
		AuthorizedAt:        s.opts.Now().Unix(),
	}

	if !req.Authenticated {
		return AuthorizeDecision{
			Kind:       DecisionLogin,
			Client:     client,
			Scopes:     scopes,
			Grant:      grant,
			Resource:   resource,
			ReturnPath: s.authorizeReturnPath(req),
		}
	}

	if !s.consentRequired(scopes) {
		target, err := s.issueCode(redirectURI, req.State, grant)
		if err != nil {
			return s.unsafeFailure(issuer, req, err)
		}
		return AuthorizeDecision{Kind: DecisionRedirect, RedirectURI: target, Client: client, Scopes: scopes, Grant: grant, Resource: resource}
	}
	return AuthorizeDecision{Kind: DecisionConsent, Client: client, Scopes: scopes, Grant: grant, Resource: resource}
}

// ApproveGrant issues the authorization code for a consent-screen approval.
func (s *Service) ApproveGrant(grant AuthorizationGrant, state string) (string, error) {
	return s.issueCode(grant.RedirectURI, state, grant)
}

// consentRequired reports whether an explicit click is needed. Read-only grants
// for an already-authenticated owner are auto-approved when configured, which is
// what makes "log in and authorization succeeds" true. Anything that could
// mutate portfolio data always asks.
func (s *Service) consentRequired(scopes []string) bool {
	if !s.opts.AutoApprove {
		return true
	}
	for _, scope := range scopes {
		if scope != ScopeRead {
			return true
		}
	}
	return false
}

func (s *Service) issueCode(redirectURI, state string, grant AuthorizationGrant) (string, error) {
	code, err := s.codes.Issue(grant)
	if err != nil {
		return "", fail(ErrServerError, http.StatusInternalServerError, "%v", err)
	}
	target, err := url.Parse(redirectURI)
	if err != nil {
		return "", fail(ErrServerError, http.StatusInternalServerError, "registered redirect_uri is unparsable")
	}
	query := target.Query()
	query.Set("code", code)
	if state != "" {
		query.Set("state", state)
	}
	target.RawQuery = query.Encode()
	return target.String(), nil
}

// unsafeFailure renders an error locally. It is used only for failures that
// happen before the redirect target is verified, so it never redirects.
func (s *Service) unsafeFailure(_ string, req AuthorizeRequest, err error) AuthorizeDecision {
	failure, ok := err.(*Failure)
	if !ok {
		failure = fail(ErrServerError, http.StatusInternalServerError, "%v", err)
	}
	failure.Redirectable = false
	return AuthorizeDecision{
		Kind:       DecisionErrorPage,
		Error:      failure,
		ReturnPath: s.authorizeReturnPath(req),
	}
}

// authorizeReturnPath rebuilds the authorize URL so the login page can send the
// owner straight back. Only the OAuth parameters are echoed — never a cookie or
// session value.
func (s *Service) authorizeReturnPath(req AuthorizeRequest) string {
	query := url.Values{}
	setIfNotEmpty(query, "client_id", req.ClientID)
	setIfNotEmpty(query, "redirect_uri", req.RedirectURI)
	setIfNotEmpty(query, "response_type", req.ResponseType)
	setIfNotEmpty(query, "scope", req.Scope)
	setIfNotEmpty(query, "state", req.State)
	setIfNotEmpty(query, "code_challenge", req.CodeChallenge)
	setIfNotEmpty(query, "code_challenge_method", req.CodeChallengeMethod)
	setIfNotEmpty(query, "resource", req.Resource)
	return "/oauth/authorize?" + query.Encode()
}

func setIfNotEmpty(values url.Values, key, value string) {
	if strings.TrimSpace(value) != "" {
		values.Set(key, value)
	}
}

// validateResource enforces RFC 8707 resource indicators: the requested resource
// must be exactly this MCP endpoint. An absent resource defaults to it, so a
// client that does not implement RFC 8707 still gets a correctly-bound token.
func (s *Service) validateResource(issuer, requested string) (string, error) {
	canonical := s.Resource(issuer)
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return canonical, nil
	}
	if requested != canonical && strings.TrimRight(requested, "/") != canonical {
		return "", fail(ErrInvalidRequest, http.StatusBadRequest,
			"resource %q is not this server; expected %q", requested, canonical)
	}
	return canonical, nil
}

// ── token endpoint ──────────────────────────────────────────────────────────

// TokenRequest is the parsed POST /oauth/token body.
type TokenRequest struct {
	GrantType    string
	Code         string
	RedirectURI  string
	ClientID     string
	CodeVerifier string
	RefreshToken string
	Scope        string
	Resource     string
}

// TokenResponse is the RFC 6749 token response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
}

// IssueToken dispatches on grant_type.
func (s *Service) IssueToken(ctx context.Context, issuer string, req TokenRequest) (*TokenResponse, error) {
	switch strings.TrimSpace(req.GrantType) {
	case "authorization_code":
		return s.exchangeCode(ctx, issuer, req)
	case "refresh_token":
		return s.refresh(ctx, issuer, req)
	case "":
		return nil, fail(ErrInvalidRequest, http.StatusBadRequest, "grant_type is required")
	default:
		return nil, fail(ErrUnsupportedGrantType, http.StatusBadRequest,
			"grant_type %q is not supported", req.GrantType)
	}
}

func (s *Service) exchangeCode(ctx context.Context, issuer string, req TokenRequest) (*TokenResponse, error) {
	if strings.TrimSpace(req.Code) == "" {
		return nil, fail(ErrInvalidRequest, http.StatusBadRequest, "code is required")
	}
	grant, ok := s.codes.Redeem(req.Code)
	if !ok {
		return nil, fail(ErrInvalidGrant, http.StatusBadRequest, "authorization code is invalid, expired, or already used")
	}
	// A token request may legitimately omit client_id (OpenAI's connector does),
	// but when present it must match the grant — otherwise a stolen code could be
	// laundered through a different client registration.
	if provided := strings.TrimSpace(req.ClientID); provided != "" && provided != grant.ClientID {
		return nil, fail(ErrInvalidGrant, http.StatusBadRequest, "client_id does not match the authorization grant")
	}
	if provided := strings.TrimSpace(req.RedirectURI); provided != "" && provided != grant.RedirectURI {
		return nil, fail(ErrInvalidGrant, http.StatusBadRequest, "redirect_uri does not match the authorization grant")
	}
	if !VerifyCodeVerifier(req.CodeVerifier, grant.CodeChallenge) {
		return nil, fail(ErrInvalidGrant, http.StatusBadRequest, "code_verifier does not match the code_challenge")
	}
	if _, err := s.validateResource(issuer, req.Resource); err != nil {
		return nil, err
	}
	return s.issueTokens(ctx, issuer, grant.ClientID, grant.Scopes, grant.Resource)
}

func (s *Service) refresh(ctx context.Context, issuer string, req TokenRequest) (*TokenResponse, error) {
	raw := strings.TrimSpace(req.RefreshToken)
	if raw == "" {
		return nil, fail(ErrInvalidRequest, http.StatusBadRequest, "refresh_token is required")
	}
	if s.store == nil {
		return nil, fail(ErrServerError, http.StatusInternalServerError, "refresh tokens are not persisted in this deployment")
	}
	stored, err := s.store.RefreshTokenByID(ctx, tokenID(raw), s.opts.Now().Unix())
	if err != nil {
		return nil, fail(ErrServerError, http.StatusInternalServerError, "load refresh token: %v", err)
	}
	if stored == nil {
		return nil, fail(ErrInvalidGrant, http.StatusBadRequest, "refresh_token is invalid, expired, or revoked")
	}
	if provided := strings.TrimSpace(req.ClientID); provided != "" && provided != stored.ClientID {
		return nil, fail(ErrInvalidGrant, http.StatusBadRequest, "client_id does not match the refresh token")
	}
	if _, err := s.validateResource(issuer, req.Resource); err != nil {
		return nil, err
	}
	// Rotation: the presented token is revoked before the replacement is issued,
	// so a replayed refresh token is rejected instead of minting a second session.
	if err := s.store.RevokeRefreshToken(ctx, stored.ID, s.opts.Now().Unix()); err != nil {
		return nil, fail(ErrServerError, http.StatusInternalServerError, "rotate refresh token: %v", err)
	}
	return s.issueTokens(ctx, issuer, stored.ClientID, splitScopes(stored.Scope), stored.Resource)
}

func (s *Service) issueTokens(ctx context.Context, issuer, clientID string, scopes []string, resource string) (*TokenResponse, error) {
	if len(scopes) == 0 {
		scopes = []string{ScopeRead}
	}
	now := s.opts.Now()
	jti, err := randomID(16)
	if err != nil {
		return nil, fail(ErrServerError, http.StatusInternalServerError, "%v", err)
	}
	access, err := s.SignAccessToken(accessTokenClaims{
		Issuer:    issuer,
		Subject:   Subject,
		Audience:  resource,
		ClientID:  clientID,
		Scope:     joinScopes(scopes),
		Expiry:    now.Add(s.opts.AccessTTL).Unix(),
		IssuedAt:  now.Unix(),
		JWTID:     jti,
		TokenType: "at+jwt",
	})
	if err != nil {
		return nil, fail(ErrServerError, http.StatusInternalServerError, "sign access token: %v", err)
	}
	response := &TokenResponse{
		AccessToken: access,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.opts.AccessTTL.Seconds()),
		Scope:       joinScopes(scopes),
	}
	if s.store != nil {
		raw, id, err := NewRefreshToken()
		if err != nil {
			return nil, fail(ErrServerError, http.StatusInternalServerError, "%v", err)
		}
		err = s.store.InsertRefreshToken(ctx, StoredRefreshToken{
			ID:        id,
			ClientID:  clientID,
			Scope:     joinScopes(scopes),
			Resource:  resource,
			CreatedAt: now.Unix(),
			ExpiresAt: now.Add(s.opts.RefreshTTL).Unix(),
		})
		if err != nil {
			// The access token is already minted; failing the whole exchange would
			// leave the client with nothing. Degrade to no refresh token so the
			// client re-authorizes later instead of erroring now — but say so
			// loudly, because a silent degrade is indistinguishable from "the
			// client never asked for one" when debugging a connector that keeps
			// falling back to the login screen.
			slog.Warn("oauth refresh token persistence failed; issuing access token only",
				"client_id", clientID, "error", err.Error())
			return response, nil
		}
		response.RefreshToken = raw
	}
	return response, nil
}

// RevokeRefreshToken implements a minimal RFC 7009 revocation for refresh tokens.
func (s *Service) RevokeRefreshToken(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" || s.store == nil {
		return nil
	}
	return s.store.RevokeRefreshToken(ctx, tokenID(token), s.opts.Now().Unix())
}

// RevokeClient revokes every refresh token issued to a client.
func (s *Service) RevokeClient(ctx context.Context, clientID string) error {
	if s.store == nil {
		return errors.New("oauth: persistence unavailable")
	}
	return s.store.RevokeClientTokens(ctx, clientID, s.opts.Now().Unix())
}

func randomID(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// buildErrorRedirect appends an OAuth error to an already-verified redirect URI.
// Callers must only pass a redirect target that ValidateRedirectURI accepted,
// otherwise this becomes an open redirect.
func (s *Service) buildErrorRedirect(redirectURI, code, description, state string) (string, error) {
	target, err := url.Parse(redirectURI)
	if err != nil {
		return "", fail(ErrServerError, http.StatusInternalServerError, "registered redirect_uri is unparsable")
	}
	query := target.Query()
	query.Set("error", code)
	if description != "" {
		query.Set("error_description", description)
	}
	if state != "" {
		query.Set("state", state)
	}
	target.RawQuery = query.Encode()
	return target.String(), nil
}
