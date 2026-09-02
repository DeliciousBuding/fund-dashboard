package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/DeliciousBuding/fund-dashboard/internal/auth"
	"github.com/DeliciousBuding/fund-dashboard/internal/oauth"
	"github.com/go-chi/chi/v5"
)

// maxOAuthBodyBytes caps token/registration request bodies.
const maxOAuthBodyBytes = 16 << 10

// registerOAuthRoutes mounts the OAuth 2.1 authorization server that fronts the
// MCP resource endpoint, plus the RFC 9728 / RFC 8414 discovery documents.
//
// Route order matters: this runs before the SPA fallback, because the fallback
// answers every unknown path with index.html. Without these routes registered a
// client asking for /.well-known/oauth-protected-resource receives HTTP 200 and
// an HTML shell, which fails discovery in a way that looks like "server has no
// auth" rather than "server is broken".
func registerOAuthRoutes(r chi.Router, svc *oauth.Service, authSvc *auth.Service, limiter *RateLimiter, ipKey func(*http.Request) string) {
	// Discovery is unauthenticated by definition and cheap; keep it outside the
	// limiter so a client's metadata probe can never be starved by a scan.
	for _, path := range oauth.WellKnownPathProtectedResource(svc.Options().ResourcePath) {
		r.Get(path, handleProtectedResourceMetadata(svc))
	}
	for _, path := range oauth.WellKnownPathAuthorizationServer(svc.Options().ResourcePath) {
		r.Get(path, handleAuthorizationServerMetadata(svc))
	}

	r.Group(func(g chi.Router) {
		g.Use(RateLimit(limiter, ipKey))
		// Browser-facing: the authorize endpoint inspects the dashboard session
		// itself so an unauthenticated owner is redirected to login rather than
		// handed a JSON 401.
		g.Get("/oauth/authorize", handleOAuthAuthorize(svc, authSvc))
		g.Post("/oauth/consent", handleOAuthConsent(svc, authSvc))
		g.Get("/oauth/assets/consent.css", handleOAuthConsentCSS())
		// Machine-facing.
		g.Post("/oauth/token", handleOAuthToken(svc))
		g.Post("/oauth/register", handleOAuthRegister(svc))
		g.Post("/oauth/revoke", handleOAuthRevoke(svc))
		g.Get("/oauth/jwks", handleOAuthJWKS(svc))
		g.Get("/oauth/about", handleOAuthAbout(svc))
	})
}

// resolveOAuthIssuer picks the issuer for this request. An explicitly configured
// FUND_PUBLIC_BASE_URL always wins; only when it is unset do we fall back to the
// request-derived origin (development convenience — behind a proxy the Host
// header is attacker-influenced, so production must configure it).
func resolveOAuthIssuer(r *http.Request, svc *oauth.Service) string {
	if configured := strings.TrimSpace(svc.Options().PublicBaseURL); configured != "" {
		return strings.TrimRight(configured, "/")
	}
	scheme := "http"
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		if first := strings.Split(proto, ",")[0]; strings.TrimSpace(first) != "" {
			scheme = strings.TrimSpace(first)
		}
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host
}

func writeNoStoreJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	WriteJSON(w, status, body)
}

func handleProtectedResourceMetadata(svc *oauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issuer := resolveOAuthIssuer(r, svc)
		w.Header().Set("Cache-Control", "no-store")
		WriteJSON(w, http.StatusOK, svc.ProtectedResourceMetadata(issuer))
	}
}

func handleAuthorizationServerMetadata(svc *oauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issuer := resolveOAuthIssuer(r, svc)
		w.Header().Set("Cache-Control", "no-store")
		WriteJSON(w, http.StatusOK, svc.AuthorizationServerMetadata(issuer))
	}
}

func handleOAuthJWKS(svc *oauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jwks, err := svc.JWKS()
		if err != nil {
			writeOAuthServerError(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		WriteJSON(w, http.StatusOK, jwks)
	}
}

func handleOAuthAbout(svc *oauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, svc.AboutDocument(resolveOAuthIssuer(r, svc)))
	}
}

// ── authorize ───────────────────────────────────────────────────────────────

func handleOAuthAuthorize(svc *oauth.Service, authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		issuer := resolveOAuthIssuer(r, svc)
		query := r.URL.Query()
		session := sessionFromRequest(r, authSvc)
		request := oauth.AuthorizeRequest{
			ClientID:            query.Get("client_id"),
			RedirectURI:         query.Get("redirect_uri"),
			ResponseType:        query.Get("response_type"),
			Scope:               query.Get("scope"),
			State:               query.Get("state"),
			CodeChallenge:       query.Get("code_challenge"),
			CodeChallengeMethod: query.Get("code_challenge_method"),
			Resource:            query.Get("resource"),
			Authenticated:       session != nil,
		}
		if session != nil {
			request.SessionID = session.ID
		}
		decision := svc.Authorize(r.Context(), issuer, request)
		switch decision.Kind {
		case oauth.DecisionRedirect:
			http.Redirect(w, r, decision.RedirectURI, http.StatusFound)
		case oauth.DecisionLogin:
			// "Jump to the site, log in, and authorization succeeds": the login
			// page is told where to come back to, so after one password submit the
			// owner lands on this same authorize URL with a session cookie and is
			// redirected straight to the client.
			http.Redirect(w, r, loginReturnURL(decision.ReturnPath), http.StatusFound)
		case oauth.DecisionConsent:
			token, err := svc.BeginConsent(decision.Grant, request.State)
			if err != nil {
				writeOAuthServerError(w, r, err)
				return
			}
			renderOAuthConsent(w, r, consentView{
				ClientName:   decision.Client.Name,
				Issuer:       issuer,
				Scopes:       describeScopes(decision.Scopes),
				ConsentToken: token,
				State:        request.State,
				AutoApproved: false,
			})
		case oauth.DecisionErrorPage:
			renderOAuthError(w, r, issuer, decision.Error)
		default:
			renderOAuthError(w, r, issuer, &oauth.Failure{
				Code:        oauth.ErrServerError,
				Description: "unexpected authorization decision",
				Status:      http.StatusInternalServerError,
			})
		}
	}
}

// loginReturnURL builds /login?next=<authorize-url>. Only a same-origin path
// starting with /oauth/authorize is ever accepted as a return target (see
// safeOAuthReturn), so this cannot be turned into an open redirect.
func loginReturnURL(returnPath string) string {
	next := safeOAuthReturn(returnPath)
	if next == "" {
		return "/login"
	}
	return "/login?next=" + url.QueryEscape(next)
}

// maxOAuthReturnLength caps a return path. A multi-kilobyte "next" is never
// legitimate and would be reflected into a redirect, so bound it.
const maxOAuthReturnLength = 2048

// safeOAuthReturn validates a post-login return path. It must be a relative path
// under /oauth/ (never a scheme-relative //host, never absolute), which keeps the
// login page from being usable as a redirector to an attacker-controlled origin.
func safeOAuthReturn(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > maxOAuthReturnLength {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/oauth/") {
		return ""
	}
	if strings.HasPrefix(trimmed, "//") {
		return ""
	}
	if strings.ContainsAny(trimmed, "\r\n\t") {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() || parsed.Host != "" || parsed.Scheme != "" {
		return ""
	}
	// Resolve dot segments BEFORE re-checking the prefix. A browser normalizes
	// "/oauth/../api/admin" to "/api/admin" on navigation, so validating the raw
	// string only would let the login page be used as a redirector to any path on
	// this origin.
	cleaned := path.Clean("/" + parsed.Path)
	if !strings.HasPrefix(cleaned, "/oauth/") {
		return ""
	}
	return cleaned + optionalQuery(parsed)
}

func optionalQuery(parsed *url.URL) string {
	if parsed.RawQuery == "" {
		return ""
	}
	return "?" + parsed.RawQuery
}

func handleOAuthConsent(svc *oauth.Service, authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxOAuthBodyBytes)
		if err := r.ParseForm(); err != nil {
			writeOAuthError(w, http.StatusBadRequest, oauth.ErrInvalidRequest, "malformed consent form")
			return
		}
		session := sessionFromRequest(r, authSvc)
		if session == nil {
			http.Redirect(w, r, loginReturnURL("/oauth/authorize"), http.StatusFound)
			return
		}
		token := strings.TrimSpace(r.PostFormValue("consent_token"))
		grant, state, ok := svc.ConsumeConsent(token)
		if !ok {
			writeOAuthError(w, http.StatusBadRequest, oauth.ErrInvalidRequest,
				"consent session expired or was already used; restart the authorization request")
			return
		}
		if strings.TrimSpace(r.PostFormValue("decision")) == "deny" {
			target, err := svc.DenyRedirect(grant, state)
			if err != nil {
				writeOAuthServerError(w, r, err)
				return
			}
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		target, err := svc.ApproveGrant(grant, state)
		if err != nil {
			writeOAuthServerError(w, r, err)
			return
		}
		http.Redirect(w, r, target, http.StatusFound)
	}
}

// ── token ───────────────────────────────────────────────────────────────────

func handleOAuthToken(svc *oauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request, ok := parseTokenRequest(w, r)
		if !ok {
			return
		}
		// HTTP Basic may carry the client_id. This server is public-client-only,
		// so a presented secret is ignored rather than validated: accepting a
		// secret we never issued would only create a false sense of confidentiality.
		if basicID, _, found := r.BasicAuth(); found && request.ClientID == "" {
			request.ClientID = basicID
		}
		issuer := resolveOAuthIssuer(r, svc)
		response, err := svc.IssueToken(r.Context(), issuer, request)
		if err != nil {
			var failure *oauth.Failure
			if errors.As(err, &failure) {
				writeOAuthFailure(w, failure)
				return
			}
			writeOAuthServerError(w, r, err)
			return
		}
		writeNoStoreJSON(w, http.StatusOK, response)
	}
}

// parseTokenRequest accepts both form encoding (the OAuth default) and JSON,
// because MCP clients are inconsistent about which they send.
func parseTokenRequest(w http.ResponseWriter, r *http.Request) (oauth.TokenRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxOAuthBodyBytes)
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "application/json") {
		var payload struct {
			GrantType    string `json:"grant_type"`
			Code         string `json:"code"`
			RedirectURI  string `json:"redirect_uri"`
			ClientID     string `json:"client_id"`
			CodeVerifier string `json:"code_verifier"`
			RefreshToken string `json:"refresh_token"`
			Scope        string `json:"scope"`
			Resource     string `json:"resource"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeOAuthError(w, http.StatusBadRequest, oauth.ErrInvalidRequest, "request body is not valid JSON")
			return oauth.TokenRequest{}, false
		}
		return oauth.TokenRequest{
			GrantType:    payload.GrantType,
			Code:         payload.Code,
			RedirectURI:  payload.RedirectURI,
			ClientID:     payload.ClientID,
			CodeVerifier: payload.CodeVerifier,
			RefreshToken: payload.RefreshToken,
			Scope:        payload.Scope,
			Resource:     payload.Resource,
		}, true
	}
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, oauth.ErrInvalidRequest, "malformed form body")
		return oauth.TokenRequest{}, false
	}
	form := r.PostForm
	return oauth.TokenRequest{
		GrantType:    form.Get("grant_type"),
		Code:         form.Get("code"),
		RedirectURI:  form.Get("redirect_uri"),
		ClientID:     form.Get("client_id"),
		CodeVerifier: form.Get("code_verifier"),
		RefreshToken: form.Get("refresh_token"),
		Scope:        form.Get("scope"),
		Resource:     form.Get("resource"),
	}, true
}

// ── dynamic client registration (RFC 7591) ──────────────────────────────────

func handleOAuthRegister(svc *oauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxOAuthBodyBytes)
		var request oauth.RegisterClientRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeOAuthError(w, http.StatusBadRequest, oauth.ErrInvalidClient, "request body is not valid JSON")
			return
		}
		_, response, err := svc.RegisterClient(r.Context(), request)
		if err != nil {
			var failure *oauth.Failure
			if errors.As(err, &failure) {
				writeOAuthFailure(w, failure)
				return
			}
			writeOAuthServerError(w, r, err)
			return
		}
		writeNoStoreJSON(w, http.StatusCreated, response)
	}
}

// ── revocation (RFC 7009) ───────────────────────────────────────────────────

func handleOAuthRevoke(svc *oauth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxOAuthBodyBytes)
		if err := r.ParseForm(); err != nil {
			// RFC 7009: the server responds 200 even for an invalid token, so a
			// revoker cannot probe which tokens exist. A malformed body is the one
			// case that is a genuine client error.
			writeOAuthError(w, http.StatusBadRequest, oauth.ErrInvalidRequest, "malformed form body")
			return
		}
		token := r.PostFormValue("token")
		if strings.TrimSpace(token) != "" {
			if err := svc.RevokeRefreshToken(r.Context(), token); err != nil {
				slog.Warn("oauth revoke failed",
					"request_id", RequestIDFromContext(r.Context()),
					"error", err.Error())
			}
		}
		writeNoStoreJSON(w, http.StatusOK, map[string]any{"revoked": true})
	}
}

// ── error rendering ─────────────────────────────────────────────────────────

func writeOAuthError(w http.ResponseWriter, status int, code error, description string) {
	writeNoStoreJSON(w, status, map[string]any{
		"error":             code.Error(),
		"error_description": description,
	})
}

func writeOAuthFailure(w http.ResponseWriter, failure *oauth.Failure) {
	status := failure.Status
	if status == 0 {
		status = http.StatusBadRequest
	}
	writeOAuthError(w, status, failure.Code, failure.Description)
}

func writeOAuthServerError(w http.ResponseWriter, r *http.Request, err error) {
	// Never echo internal detail to an OAuth client: these endpoints are
	// internet-facing and the message would be a reconnaissance aid.
	slog.Error("oauth internal error",
		"request_id", RequestIDFromContext(r.Context()),
		"path", r.URL.Path,
		"error", err.Error())
	writeOAuthError(w, http.StatusInternalServerError, oauth.ErrServerError, "internal error")
}

// describeScopes turns granted scopes into human-readable consent copy.
func describeScopes(scopes []string) []scopeView {
	out := make([]scopeView, 0, len(scopes))
	for _, scope := range scopes {
		switch scope {
		case oauth.ScopeRead:
			out = append(out, scopeView{
				Name:        scope,
				Title:       "读取持仓与分析数据",
				Description: "组合概览、持仓、净值历史、收益率/回撤、市场与基金检索。",
			})
		case oauth.ScopeWrite:
			out = append(out, scopeView{
				Name:        scope,
				Title:       "写入与运维操作",
				Description: "新增/修改/删除交易、定投计划、基金档案，以及触发抓取与快照重算。每个写操作仍需在对话中二次确认。",
			})
		default:
			out = append(out, scopeView{Name: scope, Title: scope})
		}
	}
	return out
}
