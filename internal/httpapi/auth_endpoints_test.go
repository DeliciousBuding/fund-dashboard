package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/auth"
)

// End-to-end matrix for /api/auth/* + session gating (design doc 04 §W1
// acceptance). Uses the full router with a temp DB.

func newAuthTestRouter(t *testing.T) (http.Handler, *auth.Service) {
	t.Helper()
	db := openPortfolioHTTPFixture(t)
	t.Cleanup(func() { db.Close() })
	svc := newTestAuthService(t, db)
	return NewRouter(testCfg(), WithDB(db), WithAuth(svc)), svc
}

func postAuth(t *testing.T, router http.Handler, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	// Session-gated mutations require the CSRF header; harmless on public ones.
	req.Header.Set(csrfHeader, csrfHeaderValue)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func sessionCookieFrom(t *testing.T, res *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range res.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	t.Fatalf("no session cookie in response; set-cookie=%v", res.Header().Values("Set-Cookie"))
	return nil
}

func TestAuthStatusSetupLoginLogoutFlow(t *testing.T) {
	router, _ := newAuthTestRouter(t)

	// Uninitialized: status reports initialized=false, reads are gated.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &status); err != nil {
		t.Fatalf("status json: %v", err)
	}
	if status["initialized"] != false || status["authenticated"] != false {
		t.Fatalf("fresh status = %#v", status)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/portfolio/", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("ungated read without session = %d, want 401", res.Code)
	}

	// Setup.
	res = postAuth(t, router, "/api/auth/setup", map[string]string{"password": testAuthPassword})
	if res.Code != http.StatusCreated {
		t.Fatalf("setup status=%d body=%s", res.Code, res.Body.String())
	}
	cookie := sessionCookieFrom(t, res)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
		t.Fatalf("cookie flags = %#v", cookie)
	}

	// Second setup → 409.
	res = postAuth(t, router, "/api/auth/setup", map[string]string{"password": "another-password-1"})
	if res.Code != http.StatusConflict {
		t.Fatalf("second setup=%d want 409 body=%s", res.Code, res.Body.String())
	}

	// Authenticated read passes with the cookie.
	req = httptest.NewRequest(http.MethodGet, "/api/portfolio/", nil)
	req.AddCookie(cookie)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("authed read = %d body=%s", res.Code, res.Body.String())
	}

	// Wrong password → 401.
	res = postAuth(t, router, "/api/auth/login", map[string]string{"password": "wrong-password-99"})
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("bad login=%d want 401", res.Code)
	}

	// Logout revokes; cookie must die server-side.
	res = postAuth(t, router, "/api/auth/logout", nil, cookie)
	if res.Code != http.StatusOK {
		t.Fatalf("logout=%d body=%s", res.Code, res.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/portfolio/", nil)
	req.AddCookie(cookie)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("read after logout = %d, want 401", res.Code)
	}

	// Fresh login works.
	res = postAuth(t, router, "/api/auth/login", map[string]string{"password": testAuthPassword})
	if res.Code != http.StatusOK {
		t.Fatalf("login=%d body=%s", res.Code, res.Body.String())
	}
}

func TestAuthLoginRejectsTrailingJSONDocuments(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	res := postAuth(t, router, "/api/auth/setup", map[string]string{"password": testAuthPassword})
	if res.Code != http.StatusCreated {
		t.Fatalf("setup status=%d body=%s", res.Code, res.Body.String())
	}

	// A credential body must be exactly one JSON object; a chained second document
	// must be rejected instead of being silently ignored.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader("{\"password\":\""+testAuthPassword+"\"} {\"password\":\"x\"}"))
	req.Header.Set("Content-Type", "application/json")
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("chained body status = %d, want 400; body=%s", res.Code, res.Body.String())
	}
}

func TestAuthPasswordChangeResetsFailureState(t *testing.T) {
	router, svc := newAuthTestRouter(t)
	res := postAuth(t, router, "/api/auth/setup", map[string]string{"password": testAuthPassword})
	if res.Code != http.StatusCreated {
		t.Fatalf("setup status=%d body=%s", res.Code, res.Body.String())
	}
	cookie := sessionCookieFrom(t, res)

	wrong := map[string]string{"current_password": "wrong-old-password", "new_password": "new-password-1234"}
	// Fail MaxFails-1 times without tripping the lockout.
	for i := 0; i < svc.Limiter.MaxFails-1; i++ {
		res = postAuth(t, router, "/api/auth/password", wrong, cookie)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("wrong password attempt %d = %d, want 401; body=%s", i+1, res.Code, res.Body.String())
		}
	}

	// A successful change must clear the per-IP failure state (login parity).
	good := map[string]string{"current_password": testAuthPassword, "new_password": "new-password-1234"}
	res = postAuth(t, router, "/api/auth/password", good, cookie)
	if res.Code != http.StatusOK {
		t.Fatalf("password change = %d, want 200; body=%s", res.Code, res.Body.String())
	}

	// If state had survived, the very first post-change failure would be the
	// MaxFails-th and immediately lock the key, so the second attempt would 429.
	for i := 0; i < svc.Limiter.MaxFails-1; i++ {
		res = postAuth(t, router, "/api/auth/password", map[string]string{
			"current_password": "wrong-old-password",
			"new_password":     "another-password-9",
		}, cookie)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("post-change wrong attempt %d = %d, want 401 (state must be reset); body=%s", i+1, res.Code, res.Body.String())
		}
	}
}

func TestAuthSetupRejectsWeakPassword(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	res := postAuth(t, router, "/api/auth/setup", map[string]string{"password": "short"})
	if res.Code != http.StatusBadRequest {
		t.Fatalf("weak setup=%d want 400 body=%s", res.Code, res.Body.String())
	}
}

func TestAuthLoginRateLimitLocksOut(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	postAuth(t, router, "/api/auth/setup", map[string]string{"password": testAuthPassword})

	var last *httptest.ResponseRecorder
	for i := 0; i < 5; i++ {
		last = postAuth(t, router, "/api/auth/login", map[string]string{"password": "wrong-password-99"})
		if last.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status=%d want 401", i, last.Code)
		}
	}
	// 6th attempt: locked.
	last = postAuth(t, router, "/api/auth/login", map[string]string{"password": "wrong-password-99"})
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("locked attempt=%d want 429 body=%s", last.Code, last.Body.String())
	}
	if last.Header().Get("Retry-After") == "" {
		t.Fatal("429 must carry Retry-After")
	}
}

func TestAuthSessionsListAndRevoke(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	res := postAuth(t, router, "/api/auth/setup", map[string]string{"password": testAuthPassword})
	first := sessionCookieFrom(t, res)
	res = postAuth(t, router, "/api/auth/login", map[string]string{"password": testAuthPassword})
	second := sessionCookieFrom(t, res)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sessions", nil)
	req.AddCookie(first)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("sessions=%d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		Sessions  []auth.SessionInfo `json:"sessions"`
		Total     int                `json:"total"`
		Truncated bool               `json:"truncated"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("sessions json: %v", err)
	}
	if len(body.Sessions) != 2 {
		t.Fatalf("sessions=%d want 2: %#v", len(body.Sessions), body.Sessions)
	}
	if body.Total != 2 || body.Truncated {
		t.Fatalf("sessions signals = total %d truncated %v; want 2/false", body.Total, body.Truncated)
	}
	var current int
	var otherPrefix string
	for _, s := range body.Sessions {
		if s.Current {
			current++
		} else {
			otherPrefix = s.IDPrefix
		}
		if len(s.IDPrefix) != 8 || strings.Contains(s.IDPrefix, "$") {
			t.Fatalf("bad id prefix %#v", s.IDPrefix)
		}
	}
	if current != 1 || otherPrefix == "" {
		t.Fatalf("current marking wrong: %#v", body.Sessions)
	}

	// Revoke the other session; it must die.
	res = postAuth(t, router, "/api/auth/sessions/"+otherPrefix+"/revoke", nil, first)
	if res.Code != http.StatusOK {
		t.Fatalf("revoke=%d body=%s", res.Code, res.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/portfolio/", nil)
	req.AddCookie(second)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session read = %d, want 401", res.Code)
	}
}

func TestAuthPasswordChangeRevokesOthers(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	res := postAuth(t, router, "/api/auth/setup", map[string]string{"password": testAuthPassword})
	first := sessionCookieFrom(t, res)
	res = postAuth(t, router, "/api/auth/login", map[string]string{"password": testAuthPassword})
	second := sessionCookieFrom(t, res)

	res = postAuth(t, router, "/api/auth/password", map[string]string{
		"current_password": testAuthPassword,
		"new_password":     "rotated-password-2",
	}, first)
	if res.Code != http.StatusOK {
		t.Fatalf("password change=%d body=%s", res.Code, res.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/portfolio/", nil)
	req.AddCookie(second)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("other session after rotation = %d, want 401", res.Code)
	}

	res = postAuth(t, router, "/api/auth/login", map[string]string{"password": "rotated-password-2"})
	if res.Code != http.StatusOK {
		t.Fatalf("login with rotated password=%d body=%s", res.Code, res.Body.String())
	}
}

func TestAuthSessionFixationDefense(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	res := postAuth(t, router, "/api/auth/setup", map[string]string{"password": testAuthPassword})
	first := sessionCookieFrom(t, res)

	// Re-login while carrying an existing session → old session revoked.
	res = postAuth(t, router, "/api/auth/login", map[string]string{"password": testAuthPassword}, first)
	if res.Code != http.StatusOK {
		t.Fatalf("re-login=%d body=%s", res.Code, res.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/api/portfolio/", nil)
	req.AddCookie(first)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("pre-login session must be revoked, got %d", res.Code)
	}
}

func TestAuthSessionWriteRequiresCSRFHeader(t *testing.T) {
	router, _ := newAuthTestRouter(t)
	res := postAuth(t, router, "/api/auth/setup", map[string]string{"password": testAuthPassword})
	cookie := sessionCookieFrom(t, res)

	// Session-authed POST without the CSRF header → 403 csrf_header_required.
	payload, _ := json.Marshal(map[string]any{"title": "x", "source": "test"})
	req := httptest.NewRequest(http.MethodPost, "/api/portfolio/source-events", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("session write without CSRF header=%d want 403 body=%s", res.Code, res.Body.String())
	}

	// With the header → passes the guard (201).
	req = httptest.NewRequest(http.MethodPost, "/api/portfolio/source-events", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeader, csrfHeaderValue)
	req.AddCookie(cookie)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("session write with CSRF header=%d want 201 body=%s", res.Code, res.Body.String())
	}
}

func TestAuthEnvManagedMode(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	t.Cleanup(func() { db.Close() })
	hash, err := auth.HashPassword("env-managed-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	svc := newTestAuthService(t, db)
	// Rebuild service in env mode.
	svc = auth.NewService(auth.NewStore(db), auth.Options{EnvHash: hash})
	router := NewRouter(testCfg(), WithDB(db), WithAuth(svc))

	res := postAuth(t, router, "/api/auth/setup", map[string]string{"password": "some-long-password"})
	if res.Code != http.StatusForbidden {
		t.Fatalf("setup in env mode=%d want 403 body=%s", res.Code, res.Body.String())
	}
	res = postAuth(t, router, "/api/auth/login", map[string]string{"password": "env-managed-password"})
	if res.Code != http.StatusOK {
		t.Fatalf("env login=%d body=%s", res.Code, res.Body.String())
	}
	cookie := sessionCookieFrom(t, res)
	res = postAuth(t, router, "/api/auth/password", map[string]string{
		"current_password": "env-managed-password",
		"new_password":     "another-password-1",
	}, cookie)
	if res.Code != http.StatusForbidden {
		t.Fatalf("password change in env mode=%d want 403 body=%s", res.Code, res.Body.String())
	}
}

// The settings sessions endpoint must surface the store soft ceiling: 200
// newest sessions plus total/truncated, so the SPA can tell the user rows were
// cut instead of silently showing an incomplete list.
func TestAuthSessionsTruncationSignals(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	t.Cleanup(func() { db.Close() })
	store := auth.NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure auth schema: %v", err)
	}
	base := int64(1_700_000_000)
	for i := 1; i <= 201; i++ {
		id := fmt.Sprintf("%08x%056x", i, 0)
		if err := store.CreateSession(context.Background(), auth.Session{
			ID:         id,
			CreatedAt:  base,
			ExpiresAt:  base + 3600,
			LastSeenAt: base + int64(i),
			IP:         "192.0.2.10",
			UserAgent:  "httpapi-test-agent",
		}); err != nil {
			t.Fatalf("CreateSession %d: %v", i, err)
		}
	}

	// newAuthedRouter adds setup+login sessions on top of the seeded rows.
	router := newAuthedRouter(t, testCfg(), db)
	var wantTotal int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM auth_sessions`).Scan(&wantTotal); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if wantTotal <= 200 {
		t.Fatalf("fixture under ceiling: %d sessions", wantTotal)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/sessions", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("sessions=%d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		Sessions  []auth.SessionInfo `json:"sessions"`
		Total     int                `json:"total"`
		Truncated bool               `json:"truncated"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("sessions json: %v", err)
	}
	if len(body.Sessions) != 200 || body.Total != wantTotal || !body.Truncated {
		t.Fatalf("sessions truncation = %d/%d/%v; want 200/%d/true", len(body.Sessions), body.Total, body.Truncated, wantTotal)
	}
}
