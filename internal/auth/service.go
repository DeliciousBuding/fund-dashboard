package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Sentinel errors — handlers map these to stable client codes.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAlreadyInitialized = errors.New("already initialized")
	ErrNotInitialized     = errors.New("auth not initialized")
	ErrEnvManaged         = errors.New("credentials managed by FUND_AUTH_PASSWORD_HASH")
	ErrWeakPassword       = errors.New("password too weak")
	ErrSessionNotFound    = errors.New("session not found")
	ErrRateLimited        = errors.New("rate limited")
)

// Options configures a Service.
type Options struct {
	// EnvHash, when set (FUND_AUTH_PASSWORD_HASH), is the authoritative
	// credential: DB credentials are ignored and password change is disabled.
	EnvHash string
	// TTL is the sliding session window (default 720h = 30d). It is clamped
	// to MaxAge: a sliding window longer than the absolute cap is a config error
	// and must not let sessions outlive created+MaxAge.
	TTL time.Duration
	// MaxAge is the absolute session cap from creation (default 2160h = 90d).
	MaxAge time.Duration
	// Now is injectable for tests.
	Now func() time.Time
}

// Service is the single-tenant auth engine: setup, login, session
// authentication (with sliding renewal), password change, and sweeps.
type Service struct {
	store   *Store
	envHash string
	ttl     time.Duration
	maxAge  time.Duration
	now     func() time.Time

	// Limiter guards setup/login/password attempts.
	Limiter *Limiter
}

func NewService(store *Store, opts Options) *Service {
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = 720 * time.Hour
	}
	maxAge := opts.MaxAge
	if maxAge <= 0 {
		maxAge = 2160 * time.Hour
	}
	// The sliding window must never outlive the absolute cap: a TTL longer than
	// MaxAge would otherwise let a freshly created (or idle) session stay valid
	// past created+MaxAge, defeating the configured absolute limit. Clamp TTL
	// down so renewal, initial expiry and the session cookie stay consistent.
	if ttl > maxAge {
		ttl = maxAge
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		store:   store,
		envHash: strings.TrimSpace(opts.EnvHash),
		ttl:     ttl,
		maxAge:  maxAge,
		now:     now,
		Limiter: NewLimiter(now),
	}
}

// SessionTTL exposes the configured sliding window for cookie Max-Age.
func (s *Service) SessionTTL() time.Duration { return s.ttl }

// Status is the unauthenticated discovery payload for the SPA.
type Status struct {
	Initialized bool `json:"initialized"`
	EnvManaged  bool `json:"env_managed"`
}

// GetStatus reports whether the instance is initialized (env hash or DB credential).
func (s *Service) GetStatus(ctx context.Context) (Status, error) {
	if s.envHash != "" {
		return Status{Initialized: true, EnvManaged: true}, nil
	}
	hash, err := s.store.CredentialHash(ctx)
	if err != nil {
		return Status{}, err
	}
	return Status{Initialized: hash != ""}, nil
}

// Setup performs first-run initialization: stores the password hash and opens
// a session. Fails closed when env-managed or already initialized; the insert
// is atomic so concurrent setups yield exactly one winner.
func (s *Service) Setup(ctx context.Context, password, ip, userAgent string) (string, error) {
	if s.envHash != "" {
		return "", ErrEnvManaged
	}
	if err := validatePassword(password); err != nil {
		return "", err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return "", err
	}
	inserted, err := s.store.InsertCredentialIfAbsent(ctx, hash, s.now())
	if err != nil {
		return "", err
	}
	if !inserted {
		return "", ErrAlreadyInitialized
	}
	return s.newSession(ctx, ip, userAgent)
}

// Login verifies the password and opens a session. Failures are indistinguishable
// (no user enumeration — there is exactly one user anyway) and rate-limited.
func (s *Service) Login(ctx context.Context, password, ip, userAgent string) (string, error) {
	hash, err := s.activeHash(ctx)
	if err != nil {
		return "", err
	}
	if hash == "" {
		return "", ErrNotInitialized
	}
	ok, err := VerifyPassword(password, hash)
	if err != nil {
		// Malformed stored hash: fail closed, but this is a server-side fault.
		return "", fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return "", ErrInvalidCredentials
	}
	return s.newSession(ctx, ip, userAgent)
}

// Authenticate validates a session token and applies sliding renewal:
// past half the TTL the session is touched forward, capped at created+MaxAge.
// Returns (nil, nil) for unknown/expired tokens — callers map to 401.
func (s *Service) Authenticate(ctx context.Context, token string) (*Session, error) {
	id := tokenID(token)
	sess, err := s.store.SessionByID(ctx, id)
	if err != nil || sess == nil {
		return nil, err
	}
	now := s.now().Unix()
	if sess.ExpiresAt <= now {
		return nil, nil
	}
	halfLife := sess.LastSeenAt + int64(s.ttl.Seconds())/2
	if now > halfLife {
		newExpiry := now + int64(s.ttl.Seconds())
		if cap := sess.CreatedAt + int64(s.maxAge.Seconds()); newExpiry > cap {
			newExpiry = cap
		}
		// now > halfLife already implies now > sess.LastSeenAt, so the only
		// decision left is whether expiry actually moves. Once the renewal is
		// pinned at the absolute cap (newExpiry == ExpiresAt) a touch would be a
		// no-op DB write on every request, so skip it.
		if newExpiry > sess.ExpiresAt {
			if err := s.store.TouchSession(ctx, id, now, newExpiry); err != nil {
				return nil, err
			}
			sess.LastSeenAt = now
			sess.ExpiresAt = newExpiry
		}
	}
	return sess, nil
}

// Revoke deletes the session behind token (logout / fixation defense).
func (s *Service) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, tokenID(token))
}

// ChangePassword verifies the old password, stores the new hash, and revokes
// every other session (the current session survives).
func (s *Service) ChangePassword(ctx context.Context, currentToken, oldPassword, newPassword string) error {
	if s.envHash != "" {
		return ErrEnvManaged
	}
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	hash, err := s.store.CredentialHash(ctx)
	if err != nil {
		return err
	}
	if hash == "" {
		return ErrNotInitialized
	}
	ok, err := VerifyPassword(oldPassword, hash)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return ErrInvalidCredentials
	}
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.store.UpdateCredentialHash(ctx, newHash, s.now()); err != nil {
		return err
	}
	return s.store.DeleteOtherSessions(ctx, tokenID(currentToken))
}

// SessionInfo is the redacted session view for the settings UI.
type SessionInfo struct {
	IDPrefix   string `json:"id_prefix"`
	CreatedAt  int64  `json:"created_at"`
	ExpiresAt  int64  `json:"expires_at"`
	LastSeenAt int64  `json:"last_seen_at"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	Current    bool   `json:"current"`
}

func (s *Service) ListSessions(ctx context.Context, currentToken string) ([]SessionInfo, error) {
	sessions, err := s.store.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	currentID := tokenID(currentToken)
	out := make([]SessionInfo, 0, len(sessions))
	for _, sess := range sessions {
		prefix := sess.ID
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}
		ua := sess.UserAgent
		if len(ua) > 120 {
			ua = ua[:120]
		}
		out = append(out, SessionInfo{
			IDPrefix:   prefix,
			CreatedAt:  sess.CreatedAt,
			ExpiresAt:  sess.ExpiresAt,
			LastSeenAt: sess.LastSeenAt,
			IP:         sess.IP,
			UserAgent:  ua,
			Current:    sess.ID == currentID,
		})
	}
	return out, nil
}

// RevokeByIDPrefix deletes the session whose ID starts with prefix (min 8
// chars). Revoking the current session is allowed (acts as logout).
func (s *Service) RevokeByIDPrefix(ctx context.Context, prefix string) error {
	if len(prefix) < 8 {
		return ErrSessionNotFound
	}
	sessions, err := s.store.ListSessions(ctx)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		if strings.HasPrefix(sess.ID, prefix) {
			return s.store.DeleteSession(ctx, sess.ID)
		}
	}
	return ErrSessionNotFound
}

// SweepExpired deletes expired sessions and returns how many rows were
// removed. Wired into the daily 03:00 scheduler window via
// jobs.AuthSessionSweeper (internal/app wires it); the bare-SQL path in
// internal/jobs remains only as a legacy fallback.
func (s *Service) SweepExpired(ctx context.Context) (int64, error) {
	return s.store.DeleteExpiredSessions(ctx, s.now())
}

// RecordAuthEvent appends an auth audit row (setup|login_ok|login_fail|lockout|
// logout|password_change|session_revoke). Best-effort: an audit write failure
// is logged and swallowed — auditing must never break the auth flow.
// detail never carries passwords or tokens (design 06 §2.2).
func (s *Service) RecordAuthEvent(ctx context.Context, event, ip, userAgent, detail string) {
	if err := s.store.InsertAuthEvent(ctx, event, ip, userAgent, detail, s.now().Unix()); err != nil {
		slog.Warn("auth event record failed",
			"event", event,
			"ip", ip,
			"error", err.Error(),
		)
	}
}

// ListAuthEvents exposes the recent auth audit timeline (newest first,
// limit ≤ 500) for the settings/system audit UI.
func (s *Service) ListAuthEvents(ctx context.Context, limit int) ([]AuthEvent, error) {
	return s.store.ListAuthEvents(ctx, limit)
}

// SweepAuthEvents deletes audit rows older than cutoff (default 180d from the
// scheduler's 03:00 window). Implements jobs.AuthEventSweeper.
func (s *Service) SweepAuthEvents(ctx context.Context, cutoffEpoch int64) (int64, error) {
	return s.store.SweepAuthEvents(ctx, cutoffEpoch)
}

func (s *Service) activeHash(ctx context.Context) (string, error) {
	if s.envHash != "" {
		return s.envHash, nil
	}
	return s.store.CredentialHash(ctx)
}

func (s *Service) newSession(ctx context.Context, ip, userAgent string) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	now := s.now().Unix()
	sess := Session{
		ID:         tokenID(token),
		CreatedAt:  now,
		ExpiresAt:  now + int64(s.ttl.Seconds()),
		LastSeenAt: now,
		IP:         ip,
		UserAgent:  userAgent,
	}
	if err := s.store.CreateSession(ctx, sess); err != nil {
		return "", err
	}
	return token, nil
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// tokenID is the storage key: sha256 hex of the raw token (raw never stored).
func tokenID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func validatePassword(password string) error {
	if len(password) < MinPasswordLen {
		return fmt.Errorf("%w: min %d characters", ErrWeakPassword, MinPasswordLen)
	}
	if len(password) > MaxPasswordLen {
		return fmt.Errorf("%w: max %d characters", ErrWeakPassword, MaxPasswordLen)
	}
	if !passwordHasLetterAndDigit(password) {
		// Design 06 §2.2: require at least one ASCII letter + one ASCII digit so
		// dictionary-line passwords ("correct horse battery") stay rejected.
		return fmt.Errorf("%w: must contain at least one ASCII letter and one ASCII digit", ErrWeakPassword)
	}
	return nil
}

func passwordHasLetterAndDigit(password string) bool {
	hasLetter, hasDigit := false, false
	for i := 0; i < len(password); i++ {
		c := password[i]
		switch {
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			hasLetter = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
		if hasLetter && hasDigit {
			return true
		}
	}
	return false
}
