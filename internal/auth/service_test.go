package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

func newTestService(t *testing.T, opts Options) *Service {
	t.Helper()
	db := testutil.OpenTempDB(t)
	t.Cleanup(func() { db.Close() })
	store := NewStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return NewService(store, opts)
}

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword("correct horse battery", hash)
	if err != nil || !ok {
		t.Fatalf("verify correct = %v, %v", ok, err)
	}
	ok, err = VerifyPassword("wrong", hash)
	if err != nil || ok {
		t.Fatalf("verify wrong = %v, %v", ok, err)
	}
	if _, err := VerifyPassword("x", "not-a-phc"); !errors.Is(err, ErrMalformedPHC) {
		t.Fatalf("malformed = %v, want ErrMalformedPHC", err)
	}
}

func TestVerifyPasswordRejectsOversizedPHCParams(t *testing.T) {
	// Attacker-controlled PHC params must be rejected before argon2 allocates
	// memory / burns CPU on the public login path.
	cases := []string{
		"=19=1073741824,t=3,p=2", // m=1<<30 > 1 GiB cap
		"=19=65536,t=999,p=2",    // t=999 > 10 cap
		"=19=65536,t=3,p=64",     // p=64 > 8 cap
	}
	for _, phc := range cases {
		if _, err := VerifyPassword("x", phc); !errors.Is(err, ErrMalformedPHC) {
			t.Fatalf("oversized PHC %q = %v, want ErrMalformedPHC", phc, err)
		}
	}
}
func TestSetupLoginAuthenticateFlow(t *testing.T) {
	svc := newTestService(t, Options{})
	ctx := context.Background()

	status, err := svc.GetStatus(ctx)
	if err != nil || status.Initialized || status.EnvManaged {
		t.Fatalf("initial status = %#v, %v", status, err)
	}

	token, err := svc.Setup(ctx, "long-enough-password1", "10.0.0.1", "agent/1")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if token == "" {
		t.Fatal("Setup returned empty token")
	}

	if _, err := svc.Setup(ctx, "another-password-1", "", ""); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("second Setup = %v, want ErrAlreadyInitialized", err)
	}

	sess, err := svc.Authenticate(ctx, token)
	if err != nil || sess == nil {
		t.Fatalf("Authenticate = %#v, %v", sess, err)
	}
	if sess.IP != "10.0.0.1" || sess.UserAgent != "agent/1" {
		t.Fatalf("session metadata = %#v", sess)
	}

	if sess, err := svc.Authenticate(ctx, "bogus-token"); err != nil || sess != nil {
		t.Fatalf("Authenticate bogus = %#v, %v", sess, err)
	}

	if _, err := svc.Login(ctx, "wrong-password-x", "", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong login = %v, want ErrInvalidCredentials", err)
	}
	if _, err := svc.Login(ctx, "long-enough-password1", "", ""); err != nil {
		t.Fatalf("good login: %v", err)
	}
}

func TestEnvManagedMode(t *testing.T) {
	hash, err := HashPassword("env-managed-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	svc := newTestService(t, Options{EnvHash: hash})
	ctx := context.Background()

	status, err := svc.GetStatus(ctx)
	if err != nil || !status.Initialized || !status.EnvManaged {
		t.Fatalf("env status = %#v, %v", status, err)
	}
	if _, err := svc.Setup(ctx, "long-enough-password1", "", ""); !errors.Is(err, ErrEnvManaged) {
		t.Fatalf("setup in env mode = %v, want ErrEnvManaged", err)
	}
	token, err := svc.Login(ctx, "env-managed-password", "", "")
	if err != nil || token == "" {
		t.Fatalf("env login = %v, %v", token, err)
	}
	err = svc.ChangePassword(ctx, token, "env-managed-password", "brand-new-password")
	if !errors.Is(err, ErrEnvManaged) {
		t.Fatalf("change password in env mode = %v, want ErrEnvManaged", err)
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	svc := newTestService(t, Options{})
	ctx := context.Background()
	if _, err := svc.Setup(ctx, "initial-password-1", "", ""); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	current, err := svc.Login(ctx, "initial-password-1", "", "")
	if err != nil {
		t.Fatalf("login 1: %v", err)
	}
	other, err := svc.Login(ctx, "initial-password-1", "", "")
	if err != nil {
		t.Fatalf("login 2: %v", err)
	}

	if err := svc.ChangePassword(ctx, current, "initial-password-1", "rotated-password-2"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if sess, _ := svc.Authenticate(ctx, current); sess == nil {
		t.Fatal("current session must survive password change")
	}
	if sess, _ := svc.Authenticate(ctx, other); sess != nil {
		t.Fatal("other session must be revoked by password change")
	}
	if _, err := svc.Login(ctx, "rotated-password-2", "", ""); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
}

func TestSlidingRenewalAndAbsoluteCap(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	svc := newTestService(t, Options{
		TTL:    24 * time.Hour,
		MaxAge: 48 * time.Hour,
		Now:    func() time.Time { return now },
	})
	ctx := context.Background()
	token, err := svc.Setup(ctx, "sliding-window-pw1", "", "")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	// Before half the TTL: no renewal.
	sess, _ := svc.Authenticate(ctx, token)
	if sess.ExpiresAt != now.Unix()+86400 {
		t.Fatalf("initial expiry = %d, want %d", sess.ExpiresAt, now.Unix()+86400)
	}

	// Past half TTL: slides forward by TTL.
	now = now.Add(13 * time.Hour) // t0+46800 > halfLife t0+43200
	sess, _ = svc.Authenticate(ctx, token)
	if sess.ExpiresAt != now.Unix()+86400 {
		t.Fatalf("slid expiry = %d, want %d", sess.ExpiresAt, now.Unix()+86400)
	}

	// Near the absolute cap (but still within the current expiry): renewal is
	// capped at created+MaxAge.
	now = now.Add(14 * time.Hour) // t0+97200 < current expiry t0+133200
	sess, _ = svc.Authenticate(ctx, token)
	if want := int64(1_800_000_000 + 172800); sess.ExpiresAt != want {
		t.Fatalf("capped expiry = %d, want %d (created+maxAge)", sess.ExpiresAt, want)
	}

	// Past the cap: session dead.
	now = now.Add(48 * time.Hour)
	if sess, _ := svc.Authenticate(ctx, token); sess != nil {
		t.Fatal("session past absolute cap must be rejected")
	}
}

func TestSweepExpired(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	svc := newTestService(t, Options{TTL: time.Hour, Now: func() time.Time { return now }})
	ctx := context.Background()
	token, err := svc.Setup(ctx, "sweep-me-password1", "", "")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	now = now.Add(2 * time.Hour) // session now expired
	deleted, err := svc.SweepExpired(ctx)
	if err != nil || deleted != 1 {
		t.Fatalf("SweepExpired = %d, %v; want 1", deleted, err)
	}
	if sess, _ := svc.Authenticate(ctx, token); sess != nil {
		t.Fatal("swept session must be gone")
	}
}

func TestRevokeByIDPrefix(t *testing.T) {
	svc := newTestService(t, Options{})
	ctx := context.Background()
	token, err := svc.Setup(ctx, "prefix-revoke-pw1", "", "")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	sessions, err := svc.ListSessions(ctx, token)
	if err != nil || len(sessions) != 1 || !sessions[0].Current {
		t.Fatalf("ListSessions = %#v, %v", sessions, err)
	}
	if err := svc.RevokeByIDPrefix(ctx, "short"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("short prefix = %v, want ErrSessionNotFound", err)
	}
	if err := svc.RevokeByIDPrefix(ctx, sessions[0].IDPrefix); err != nil {
		t.Fatalf("RevokeByIDPrefix: %v", err)
	}
	if sess, _ := svc.Authenticate(ctx, token); sess != nil {
		t.Fatal("revoked session must be gone")
	}
}

func TestLimiterLockoutAndReset(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	limiter := NewLimiter(func() time.Time { return now })

	for i := 0; i < 4; i++ {
		if _, ok := limiter.Allow("ip:1"); !ok {
			t.Fatalf("attempt %d should be allowed", i)
		}
		limiter.Failure("ip:1")
	}
	if _, ok := limiter.Allow("ip:1"); !ok {
		t.Fatal("5th attempt must be allowed before lockout")
	}
	limiter.Failure("ip:1") // 5th failure trips the lockout

	retryAfter, ok := limiter.Allow("ip:1")
	if ok || retryAfter <= 0 {
		t.Fatalf("locked = %v retryAfter=%v, want locked with positive retry", ok, retryAfter)
	}

	// A different key is unaffected.
	if _, ok := limiter.Allow("ip:2"); !ok {
		t.Fatal("other key must not be locked")
	}

	// Lock expires after the lockout window.
	now = now.Add(16 * time.Minute)
	if _, ok := limiter.Allow("ip:1"); !ok {
		t.Fatal("lock must expire after lockout window")
	}

	// Success resets failure state.
	limiter.Failure("ip:3")
	limiter.Success("ip:3")
	if _, ok := limiter.Allow("ip:3"); !ok {
		t.Fatal("success must reset failures")
	}
}

func TestLimiterGlobalWindow(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	limiter := NewLimiter(func() time.Time { return now })
	limiter.GlobalPerHour = 3
	for i := 0; i < 3; i++ {
		limiter.Failure("ip:x")
	}
	if _, ok := limiter.Allow("ip:y"); ok {
		t.Fatal("global hourly budget exhausted → must reject")
	}
	now = now.Add(61 * time.Minute)
	if _, ok := limiter.Allow("ip:y"); !ok {
		t.Fatal("global window must slide after an hour")
	}
}

func TestWeakPasswordRejected(t *testing.T) {
	svc := newTestService(t, Options{})
	if _, err := svc.Setup(context.Background(), "short", "", ""); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("short password = %v, want ErrWeakPassword", err)
	}
}
