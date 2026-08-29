package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// GET /api/auth/events(design 06 §2.2):需 session;倒序;limit ≤ 500。
// 同时验证 auth_events 写入点:setup/login_ok/login_fail/logout 均落表。

func TestAuthEventsEndpointRequiresSession(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	t.Cleanup(func() { db.Close() })
	svc := newTestAuthService(t, db)
	router := NewRouter(testCfg(), WithDB(db), WithAuth(svc))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/events", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("events without session = %d, want 401; body=%s", res.Code, res.Body.String())
	}
}

func TestAuthEventsEndpointRecordsFlow(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	t.Cleanup(func() { db.Close() })
	svc := newTestAuthService(t, db)
	router := NewRouter(testCfg(), WithDB(db), WithAuth(svc))

	res := postAuth(t, router, "/api/auth/setup", map[string]string{"password": testAuthPassword})
	if res.Code != http.StatusCreated {
		t.Fatalf("setup = %d body=%s", res.Code, res.Body.String())
	}
	session := sessionCookieFrom(t, res)

	// 成功登录 → login_ok
	res = postAuth(t, router, "/api/auth/login", map[string]string{"password": testAuthPassword})
	if res.Code != http.StatusOK {
		t.Fatalf("login = %d body=%s", res.Code, res.Body.String())
	}

	// 失败登录 → login_fail
	res = postAuth(t, router, "/api/auth/login", map[string]string{"password": "wrong-password-99"})
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("bad login = %d body=%s", res.Code, res.Body.String())
	}

	events := getAuthEvents(t, router, session)
	for _, want := range []string{"setup", "login_ok", "login_fail"} {
		if !hasAuthEvent(events, want) {
			t.Fatalf("events missing %q: %s", want, toJSONString(t, events))
		}
	}
	// limit clamp:超大 limit 不报错。
	req := httptest.NewRequest(http.MethodGet, "/api/auth/events?limit=99999", nil)
	req.AddCookie(session)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("events limit clamp = %d body=%s", rr.Code, rr.Body.String())
	}

	// logout(用 session)→ logout 事件。
	res = postAuth(t, router, "/api/auth/logout", nil, session)
	if res.Code != http.StatusOK {
		t.Fatalf("logout = %d body=%s", res.Code, res.Body.String())
	}
	// 重新登录获得新会话(旧会话已撤销),再查 events。
	res = postAuth(t, router, "/api/auth/login", map[string]string{"password": testAuthPassword})
	if res.Code != http.StatusOK {
		t.Fatalf("re-login = %d body=%s", res.Code, res.Body.String())
	}
	events = getAuthEvents(t, router, sessionCookieFrom(t, res))
	if !hasAuthEvent(events, "logout") {
		t.Fatalf("events missing logout: %s", toJSONString(t, events))
	}
}

func getAuthEvents(t *testing.T, router http.Handler, cookie *http.Cookie) []map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/events", nil)
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("GET /api/auth/events = %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode events: %v; body=%s", err, res.Body.String())
	}
	return body.Events
}

func hasAuthEvent(events []map[string]any, name string) bool {
	for _, ev := range events {
		if ev["event"] == name {
			return true
		}
	}
	return false
}
