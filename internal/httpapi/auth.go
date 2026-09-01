package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/auth"
	"github.com/go-chi/chi/v5"
)

// sessionCookieName carries the session token. HttpOnly + SameSite=Lax + Secure
// (configurable for plain-HTTP LAN deployments via FUND_AUTH_SECURE_COOKIE=false).
const sessionCookieName = "fund_session"

// csrfHeader / csrfHeaderValue: browsers cannot attach custom headers from
// cross-site forms, so requiring this header on session-authenticated mutations
// is a CSRF tripwire on top of SameSite=Lax and the Origin allowlist.
const (
	csrfHeader      = "X-Fund-Request"
	csrfHeaderValue = "fetch"
)

const maxAuthBodyBytes = 4 << 10 // 4 KiB — credentials only

// registerAuthRoutes mounts /api/auth/*. Status/setup/login are public
// (rate-limited); logout/password/sessions/events require a valid session.
func registerAuthRoutes(r chi.Router, svc *auth.Service, secureCookie bool, origins []string, trusted []*net.IPNet) {
	r.Route("/api/auth", func(a chi.Router) {
		a.Get("/status", handleAuthStatus(svc))
		a.Post("/setup", handleAuthSetup(svc, secureCookie, trusted))
		a.Post("/login", handleAuthLogin(svc, secureCookie, trusted))
		a.With(SessionAuth(svc, origins)).Post("/logout", handleAuthLogout(svc, trusted))
		a.With(SessionAuth(svc, origins)).Post("/password", handleAuthPassword(svc, trusted))
		a.With(SessionAuth(svc, origins)).Get("/sessions", handleAuthSessions(svc))
		a.With(SessionAuth(svc, origins)).Post("/sessions/{id}/revoke", handleAuthSessionRevoke(svc, trusted))
		a.With(SessionAuth(svc, origins)).Get("/events", handleAuthEvents(svc))
	})
}

type authCredentialsRequest struct {
	Password string `json:"password"`
}

func handleAuthStatus(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := svc.GetStatus(r.Context())
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		body := map[string]any{
			"initialized":   status.Initialized,
			"env_managed":   status.EnvManaged,
			"authenticated": false,
		}
		if sess := sessionFromRequest(r, svc); sess != nil {
			body["authenticated"] = true
			body["session_expires_at"] = sess.ExpiresAt
		}
		WriteJSON(w, http.StatusOK, body)
	}
}

func handleAuthSetup(svc *auth.Service, secureCookie bool, trusted []*net.IPNet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, trusted)
		// No limiter bucket here: setup has no guessable credential (one-shot
		// first-run password init); failures are 409/403, not brute-force
		// material. The old "setup:" Allow-only bucket could never lock.
		var req authCredentialsRequest
		if !decodeAuthBody(w, r, &req) {
			return
		}
		// Session fixation: never carry a pre-existing session into a fresh login.
		revokeRequestSession(r, svc)
		token, err := svc.Setup(r.Context(), req.Password, ip, truncatedUserAgent(r))
		switch {
		case errors.Is(err, auth.ErrEnvManaged):
			writeError(w, http.StatusForbidden, "auth_env_managed")
			return
		case errors.Is(err, auth.ErrAlreadyInitialized):
			writeError(w, http.StatusConflict, "already_initialized")
			return
		case errors.Is(err, auth.ErrWeakPassword):
			writeError(w, http.StatusBadRequest, "weak_password")
			return
		case err != nil:
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		svc.RecordAuthEvent(r.Context(), "setup", ip, truncatedUserAgent(r), "")
		setSessionCookie(w, token, svc.SessionTTL(), secureCookie)
		WriteJSON(w, http.StatusCreated, map[string]any{"ok": true})
	}
}

func handleAuthLogin(svc *auth.Service, secureCookie bool, trusted []*net.IPNet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, trusted)
		if retryAfter, ok := svc.Limiter.Allow("login:" + ip); !ok {
			// Lockout audit happens once, when the lock trips (see
			// recordLoginFailure) — not on every rejected retry, so an ongoing
			// brute force cannot flood auth_events with one row per 429.
			rateLimited(w, retryAfter)
			return
		}
		var req authCredentialsRequest
		if !decodeAuthBody(w, r, &req) {
			return
		}
		token, err := svc.Login(r.Context(), req.Password, ip, truncatedUserAgent(r))
		switch {
		case errors.Is(err, auth.ErrNotInitialized):
			// Same response as bad credentials: no extra oracle beyond /status.
			recordLoginFailure(r, svc, ip, "not_initialized")
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		case errors.Is(err, auth.ErrInvalidCredentials):
			recordLoginFailure(r, svc, ip, "invalid_credentials")
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		case err != nil:
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		svc.Limiter.Success("login:" + ip)
		svc.RecordAuthEvent(r.Context(), "login_ok", ip, truncatedUserAgent(r), "")
		revokeRequestSession(r, svc)
		setSessionCookie(w, token, svc.SessionTTL(), secureCookie)
		WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func handleAuthLogout(svc *auth.Service, trusted []*net.IPNet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		revokeRequestSession(r, svc)
		svc.RecordAuthEvent(r.Context(), "logout", clientIP(r, trusted), truncatedUserAgent(r), "")
		clearSessionCookie(w)
		WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

type authPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func handleAuthPassword(svc *auth.Service, trusted []*net.IPNet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r, trusted)
		if retryAfter, ok := svc.Limiter.Allow("password:" + ip); !ok {
			rateLimited(w, retryAfter)
			return
		}
		var req authPasswordRequest
		if !decodeAuthBody(w, r, &req) {
			return
		}
		token := ""
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			token = cookie.Value
		}
		err := svc.ChangePassword(r.Context(), token, req.CurrentPassword, req.NewPassword)
		switch {
		case errors.Is(err, auth.ErrEnvManaged):
			writeError(w, http.StatusForbidden, "auth_env_managed")
			return
		case errors.Is(err, auth.ErrInvalidCredentials):
			if tripped, lock := svc.Limiter.Failure("password:" + ip); tripped {
				svc.RecordAuthEvent(r.Context(), "lockout", ip, truncatedUserAgent(r),
					fmt.Sprintf("retry_after=%ds", int(lock.Seconds())+1))
			}
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		case errors.Is(err, auth.ErrWeakPassword):
			writeError(w, http.StatusBadRequest, "weak_password")
			return
		case errors.Is(err, auth.ErrNotInitialized):
			writeError(w, http.StatusConflict, "not_initialized")
			return
		case err != nil:
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		svc.Limiter.Success("password:" + ip)
		svc.RecordAuthEvent(r.Context(), "password_change", ip, truncatedUserAgent(r), "")
		WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "other_sessions_revoked": true})
	}
}

func handleAuthSessions(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		current := ""
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			current = cookie.Value
		}
		sessions, err := svc.ListSessions(r.Context(), current)
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		if sessions == nil {
			sessions = []auth.SessionInfo{}
		}
		WriteJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
	}
}

func handleAuthSessionRevoke(svc *auth.Service, trusted []*net.IPNet) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefix := chi.URLParam(r, "id")
		// Bound the path parameter before it can reach the audit detail column.
		if len(prefix) > 64 {
			writeError(w, http.StatusBadRequest, "bad_request")
			return
		}
		err := svc.RevokeByIDPrefix(r.Context(), prefix)
		switch {
		case errors.Is(err, auth.ErrSessionNotFound):
			writeError(w, http.StatusNotFound, "not_found")
			return
		case err != nil:
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		// detail carries only the redacted session ID prefix — never the token.
		svc.RecordAuthEvent(r.Context(), "session_revoke", clientIP(r, trusted), truncatedUserAgent(r), prefix)
		WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// handleAuthEvents exposes the auth audit timeline (design 06 §2.2): newest
// first, newest auth/events up to 500; default limit 100.
func handleAuthEvents(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, ok := intQueryOpt(w, r, "limit", 100)
		if !ok {
			return
		}
		if limit <= 0 {
			limit = 100
		}
		if limit > 500 {
			limit = 500
		}
		events, err := svc.ListAuthEvents(r.Context(), limit)
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		if events == nil {
			events = []auth.AuthEvent{}
		}
		WriteJSON(w, http.StatusOK, map[string]any{"events": events})
	}
}

// ── helpers ─────────────────────────────────────────────────────────────

// recordLoginFailure counts the failed attempt and emits exactly one lockout
// audit event when the failure trips the escalating lock (design 06 §2.2).
// Recording on trip — not on every rejected retry — keeps an ongoing brute
// force from writing one auth_events row per 429.
func recordLoginFailure(r *http.Request, svc *auth.Service, ip, reason string) {
	tripped, lock := svc.Limiter.Failure("login:" + ip)
	if tripped {
		svc.RecordAuthEvent(r.Context(), "lockout", ip, truncatedUserAgent(r),
			fmt.Sprintf("retry_after=%ds", int(lock.Seconds())+1))
	}
	svc.RecordAuthEvent(r.Context(), "login_fail", ip, truncatedUserAgent(r), reason)
}

func decodeAuthBody(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return false
	}
	// A credential body must be exactly one JSON object. Reject trailing values
	// and garbage so chained documents cannot smuggle unexpected input into a
	// handler that read only the first value.
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "bad_request")
		return false
	}
	return true
}

func rateLimited(w http.ResponseWriter, retryAfter time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
	writeError(w, http.StatusTooManyRequests, "rate_limited")
}

func setSessionCookie(w http.ResponseWriter, token string, ttl time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// sessionFromRequest returns the authenticated session for the request cookie,
// or nil. Errors are treated as unauthenticated (fail closed).
func sessionFromRequest(r *http.Request, svc *auth.Service) *auth.Session {
	if svc == nil {
		return nil
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}
	sess, err := svc.Authenticate(r.Context(), cookie.Value)
	if err != nil {
		return nil
	}
	return sess
}

// revokeRequestSession implements session-fixation defense: before issuing a
// fresh token, the session carried by the request cookie is revoked.
func revokeRequestSession(r *http.Request, svc *auth.Service) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return
	}
	if err := svc.Revoke(r.Context(), cookie.Value); err != nil {
		slog.Warn("auth revoke before re-login failed",
			"request_id", RequestIDFromContext(r.Context()),
			"path", r.URL.Path,
			"error", err.Error(),
		)
	}
}

// clientIP resolves the effective client IP for the rate-limit/audit keys.
//
// With a trusted-proxy allowlist (cfg.TrustedProxies from FUND_TRUSTED_PROXIES),
// X-Forwarded-For is honored only when the direct peer (RemoteAddr) is inside
// the allowlist; the chain is then walked right-to-left skipping trusted hops,
// and the first untrusted IP is the client. An untrusted direct peer ignores
// XFF entirely (fail-closed — the advertised client IP cannot be spoofed).
//
// With an empty proxy allowlist we must NOT trust X-Forwarded-For: it is
// entirely client-controlled, so trusting it lets an attacker rotate the header
// to evade the login limiter and forge the audited source IP. Fail closed to the
// direct peer address (degrades to a single bucket — acceptable for single-tenant).
func clientIP(r *http.Request, trusted []*net.IPNet) string {
	remote := remoteHost(r.RemoteAddr)
	if len(trusted) == 0 {
		return remote
	}
	if !ipInNetworks(remote, trusted) {
		return remote // untrusted direct peer → XFF is attacker-controlled, ignore it.
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(parts[i])
			if ip == "" {
				continue
			}
			if !ipInNetworks(ip, trusted) {
				return ip
			}
		}
	}
	return remote
}

// remoteHost strips the port from a RemoteAddr (falling back to the raw value).
func remoteHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func ipInNetworks(ip string, networks []*net.IPNet) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, network := range networks {
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

func truncatedUserAgent(r *http.Request) string {
	ua := strings.TrimSpace(r.UserAgent())
	if len(ua) > 120 {
		ua = ua[:120]
	}
	return ua
}
