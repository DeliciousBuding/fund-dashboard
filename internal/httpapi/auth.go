package httpapi

import (
	"encoding/json"
	"errors"
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
// (rate-limited); logout/password/sessions require a valid session.
func registerAuthRoutes(r chi.Router, svc *auth.Service, secureCookie bool, origins []string) {
	r.Route("/api/auth", func(a chi.Router) {
		a.Get("/status", handleAuthStatus(svc))
		a.Post("/setup", handleAuthSetup(svc, secureCookie))
		a.Post("/login", handleAuthLogin(svc, secureCookie))
		a.With(SessionAuth(svc, origins)).Post("/logout", handleAuthLogout(svc))
		a.With(SessionAuth(svc, origins)).Post("/password", handleAuthPassword(svc))
		a.With(SessionAuth(svc, origins)).Get("/sessions", handleAuthSessions(svc))
		a.With(SessionAuth(svc, origins)).Post("/sessions/{id}/revoke", handleAuthSessionRevoke(svc))
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

func handleAuthSetup(svc *auth.Service, secureCookie bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if retryAfter, ok := svc.Limiter.Allow("setup:" + ip); !ok {
			rateLimited(w, retryAfter)
			return
		}
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
		setSessionCookie(w, token, svc.SessionTTL(), secureCookie)
		WriteJSON(w, http.StatusCreated, map[string]any{"ok": true})
	}
}

func handleAuthLogin(svc *auth.Service, secureCookie bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if retryAfter, ok := svc.Limiter.Allow("login:" + ip); !ok {
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
			svc.Limiter.Failure("login:" + ip)
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		case errors.Is(err, auth.ErrInvalidCredentials):
			svc.Limiter.Failure("login:" + ip)
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		case err != nil:
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		svc.Limiter.Success("login:" + ip)
		revokeRequestSession(r, svc)
		setSessionCookie(w, token, svc.SessionTTL(), secureCookie)
		WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

func handleAuthLogout(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		revokeRequestSession(r, svc)
		clearSessionCookie(w)
		WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

type authPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func handleAuthPassword(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
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
			svc.Limiter.Failure("password:" + ip)
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

func handleAuthSessionRevoke(svc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := svc.RevokeByIDPrefix(r.Context(), chi.URLParam(r, "id"))
		switch {
		case errors.Is(err, auth.ErrSessionNotFound):
			writeError(w, http.StatusNotFound, "not_found")
			return
		case err != nil:
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}

// ── helpers ─────────────────────────────────────────────────────────────

func decodeAuthBody(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(out); err != nil {
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

// clientIP trusts the right-most X-Forwarded-For entry (appended by the
// trusted edge) and falls back to RemoteAddr. Without a proxy, per-IP buckets
// degrade toward a single bucket — acceptable for single-tenant (worst case the
// owner locks themselves out for 15 minutes; data is never at risk).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			if ip := strings.TrimSpace(parts[i]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func truncatedUserAgent(r *http.Request) string {
	ua := strings.TrimSpace(r.UserAgent())
	if len(ua) > 120 {
		ua = ua[:120]
	}
	return ua
}
