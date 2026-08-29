package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/auth"
	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

const testAuthPassword = "test-password-1234"

func newTestAuthService(t *testing.T, db *sql.DB) *auth.Service {
	t.Helper()
	store := auth.NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure auth schema: %v", err)
	}
	return auth.NewService(store, auth.Options{})
}

// loginTestUser initializes the credential (idempotent) and returns a fresh
// session token.
func loginTestUser(t *testing.T, svc *auth.Service) string {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.Setup(ctx, testAuthPassword, "127.0.0.1", "test-agent"); err != nil &&
		!errors.Is(err, auth.ErrAlreadyInitialized) {
		t.Fatalf("setup test user: %v", err)
	}
	token, err := svc.Login(ctx, testAuthPassword, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("login test user: %v", err)
	}
	return token
}

// injectSession wraps a handler so every request carries the session cookie
// and (for unsafe methods) the CSRF header — mirroring the SPA fetch client.
func injectSession(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		if isUnsafeMethod(r.Method) {
			r.Header.Set(csrfHeader, csrfHeaderValue)
		}
		next.ServeHTTP(w, r)
	})
}

// newAuthedRouter builds a router with DB + session auth wired and a valid
// session injected into every request (read-path tests for the gated API).
func newAuthedRouter(t *testing.T, cfg config.Config, db *sql.DB, opts ...RouterOption) http.Handler {
	t.Helper()
	svc := newTestAuthService(t, db)
	token := loginTestUser(t, svc)
	all := append([]RouterOption{WithDB(db), WithAuth(svc)}, opts...)
	return injectSession(NewRouter(cfg, all...), token)
}
