package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Session is one server-side login session. ID is sha256 hex of the bearer
// token (the raw token never touches the database). Times are unix epoch
// seconds — dialect-trivial and directly comparable in SQL.
type Session struct {
	ID         string
	CreatedAt  int64
	ExpiresAt  int64
	LastSeenAt int64
	IP         string
	UserAgent  string
}

// Store persists credentials and sessions. SQL uses `?` placeholders; the pg
// driver layer rebinds them ($N), so one statement serves both dialects.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// EnsureSchema creates the auth tables on SQLite. On PostgreSQL the tables are
// created by db.EnsurePGSchema instead (see internal/repository/db/schema_pg.go).
func (s *Store) EnsureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS auth_credentials (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			password_hash TEXT NOT NULL,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS auth_sessions (
			id TEXT PRIMARY KEY,
			created_at BIGINT NOT NULL,
			expires_at BIGINT NOT NULL,
			last_seen_at BIGINT NOT NULL,
			ip TEXT,
			user_agent TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires ON auth_sessions(expires_at)`,
		`CREATE TABLE IF NOT EXISTS auth_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts BIGINT NOT NULL,
			event TEXT NOT NULL,
			ip TEXT,
			user_agent TEXT,
			detail TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_events_ts ON auth_events(ts)`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure auth schema: %w", err)
		}
	}
	return nil
}

// CredentialHash returns the stored password hash, or ("", nil) when unset.
func (s *Store) CredentialHash(ctx context.Context) (string, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM auth_credentials WHERE id = 1`).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read auth credential: %w", err)
	}
	return hash, nil
}

// InsertCredentialIfAbsent atomically creates the single credential row.
// Returns false when the row already exists (setup race loser).
func (s *Store) InsertCredentialIfAbsent(ctx context.Context, hash string, now time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_credentials (id, password_hash, created_at, updated_at)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`, hash, now.Unix(), now.Unix())
	if err != nil {
		return false, fmt.Errorf("insert auth credential: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("insert auth credential rows: %w", err)
	}
	return affected == 1, nil
}

// UpdateCredentialHash replaces the stored hash (password change).
func (s *Store) UpdateCredentialHash(ctx context.Context, hash string, now time.Time) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE auth_credentials SET password_hash = ?, updated_at = ? WHERE id = 1
	`, hash, now.Unix()); err != nil {
		return fmt.Errorf("update auth credential: %w", err)
	}
	return nil
}

func (s *Store) CreateSession(ctx context.Context, sess Session) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_sessions (id, created_at, expires_at, last_seen_at, ip, user_agent)
		VALUES (?, ?, ?, ?, ?, ?)
	`, sess.ID, sess.CreatedAt, sess.ExpiresAt, sess.LastSeenAt, sess.IP, sess.UserAgent)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionByID returns the session, or (nil, nil) when unknown.
func (s *Store) SessionByID(ctx context.Context, id string) (*Session, error) {
	var sess Session
	err := s.db.QueryRowContext(ctx, `
		SELECT id, created_at, expires_at, last_seen_at, COALESCE(ip, ''), COALESCE(user_agent, '')
		FROM auth_sessions WHERE id = ?
	`, id).Scan(&sess.ID, &sess.CreatedAt, &sess.ExpiresAt, &sess.LastSeenAt, &sess.IP, &sess.UserAgent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}
	return &sess, nil
}

// TouchSession slides the session forward (last_seen + new expiry).
func (s *Store) TouchSession(ctx context.Context, id string, lastSeenAt, expiresAt int64) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE auth_sessions SET last_seen_at = ?, expires_at = ? WHERE id = ?
	`, lastSeenAt, expiresAt, id); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteSessionByPrefix removes every session whose ID starts with prefix and
// returns the number of rows removed. It is a direct prefix query, so revocation
// is not bounded by the ListSessions LIMIT 200 soft ceiling and still finds
// sessions beyond the first page.
func (s *Store) DeleteSessionByPrefix(ctx context.Context, prefix string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE id LIKE ?`, prefix+"%")
	if err != nil {
		return 0, fmt.Errorf("delete sessions by prefix: %w", err)
	}
	return res.RowsAffected()
}

// DeleteOtherSessions revokes every session except keepID (password change).
func (s *Store) DeleteOtherSessions(ctx context.Context, keepID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE id != ?`, keepID); err != nil {
		return fmt.Errorf("delete other sessions: %w", err)
	}
	return nil
}

// sessionListLimit caps the settings session view. The ceiling stays in the
// store as payload/DB protection; ListSessions returns the full-row Total and
// a Truncated signal so upper layers surface the cut instead of silently
// dropping older sessions.
const sessionListLimit = 200

// SessionPage is one capped page of sessions plus the total row count.
type SessionPage struct {
	Sessions  []Session
	Total     int
	Truncated bool
}

// ListSessions returns the most recently seen sessions, capped at
// sessionListLimit, together with the full table count. Total comes from a
// separate COUNT(*) pass: the count/rows pair can race a concurrent
// create/delete, which is benign for the single-tenant settings view.
//
// The ORDER BY carries an id tiebreaker because last_seen_at is unix seconds,
// so two logins in the same second tie, and neither SQLite nor PostgreSQL
// defines the relative order of tied rows: it follows whatever the query plan
// produces. Without a total order the page order is not reproducible across
// engines, plans or runs, which is how the committed golden wire sample for
// GET /api/auth/sessions came to flip between two CI runs of the same tree.
// id is the TEXT primary key on both dialects, so it is the portable choice.
func (s *Store) ListSessions(ctx context.Context) (SessionPage, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_sessions`).Scan(&total); err != nil {
		return SessionPage{}, fmt.Errorf("count sessions: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, created_at, expires_at, last_seen_at, COALESCE(ip, ''), COALESCE(user_agent, '')
		FROM auth_sessions ORDER BY last_seen_at DESC, id DESC LIMIT ?
	`, sessionListLimit)
	if err != nil {
		return SessionPage{}, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.CreatedAt, &sess.ExpiresAt, &sess.LastSeenAt, &sess.IP, &sess.UserAgent); err != nil {
			return SessionPage{}, fmt.Errorf("scan session: %w", err)
		}
		out = append(out, sess)
	}
	if err := rows.Err(); err != nil {
		return SessionPage{}, fmt.Errorf("list sessions rows: %w", err)
	}
	return SessionPage{Sessions: out, Total: total, Truncated: total > len(out)}, nil
}

// DeleteExpiredSessions removes sessions past expiry; returns rows deleted.
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE expires_at < ?`, now.Unix())
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return res.RowsAffected()
}

// AuthEvent is one auth audit row. ts is unix epoch seconds; detail never
// carries passwords/tokens (design 06 §2.2).
type AuthEvent struct {
	TS        int64  `json:"ts"`
	Event     string `json:"event"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// InsertAuthEvent appends an auth audit event. ts comes from the caller so the
// service can use its injected clock (Now) — same pattern as CreateSession.
func (s *Store) InsertAuthEvent(ctx context.Context, event, ip, userAgent, detail string, ts int64) error {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_events (ts, event, ip, user_agent, detail)
		VALUES (?, ?, ?, ?, ?)
	`, ts, event, ip, userAgent, detail); err != nil {
		return fmt.Errorf("insert auth event: %w", err)
	}
	return nil
}

// ListAuthEvents returns the newest limit events, newest first. limit is
// clamped to 500 (design §2.2).
func (s *Store) ListAuthEvents(ctx context.Context, limit int) ([]AuthEvent, error) {
	if limit <= 0 {
		return nil, nil
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts, event, COALESCE(ip, ''), COALESCE(user_agent, ''), COALESCE(detail, '')
		FROM auth_events ORDER BY ts DESC, id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list auth events: %w", err)
	}
	defer rows.Close()
	var out []AuthEvent
	for rows.Next() {
		var ev AuthEvent
		if err := rows.Scan(&ev.TS, &ev.Event, &ev.IP, &ev.UserAgent, &ev.Detail); err != nil {
			return nil, fmt.Errorf("scan auth event: %w", err)
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// SweepAuthEvents deletes audit rows older than cutoffEpoch (unix seconds);
// returns rows deleted. Called daily from the scheduler.
func (s *Store) SweepAuthEvents(ctx context.Context, cutoffEpoch int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM auth_events WHERE ts < ?`, cutoffEpoch)
	if err != nil {
		return 0, fmt.Errorf("sweep auth events: %w", err)
	}
	return res.RowsAffected()
}
