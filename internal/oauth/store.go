package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
)

// Client is a registered OAuth client. Public clients only: this server never
// issues or accepts a client secret, so Token is always empty and
// TokenEndpointAuthMethod is "none".
type Client struct {
	ID                      string
	Name                    string
	RedirectURIs            []string
	GrantTypes              []string
	TokenEndpointAuthMethod string
	Scope                   string
	ClientURI               string
	RegistrationAccessToken string
	CreatedAt               int64
	// MetadataDocument records that the client was resolved from an OpenAI
	// client-id metadata document rather than a stored registration.
	MetadataDocument bool
}

// StoredSigningKey is the persisted ES256 keypair row.
type StoredSigningKey struct {
	Kid           string
	Algorithm     string
	PrivateKeyPEM string
	PublicJWKJSON string
	CreatedAt     int64
}

// StoredRefreshToken is one issued refresh token. The raw token never reaches
// the database: ID is its sha256 hex, mirroring auth_sessions.
type StoredRefreshToken struct {
	ID        string
	ClientID  string
	Scope     string
	Resource  string
	CreatedAt int64
	ExpiresAt int64
	RevokedAt int64
}

// Store persists OAuth clients, refresh tokens and the signing key.
// SQL uses `?` placeholders; the pg driver layer rebinds them ($N), so one
// statement serves both dialects (same convention as internal/auth).
type Store struct {
	db *sql.DB
}

// NewStore builds a Store. A nil db is allowed: the service then runs with
// in-memory-only semantics (no client or refresh-token persistence), which keeps
// unit tests and DB-less boots working.
func NewStore(db *sql.DB) *Store {
	if db == nil {
		return nil
	}
	return &Store{db: db}
}

// EnsureSchema creates the OAuth tables on SQLite. On PostgreSQL they are
// created by db.EnsurePGSchema instead (internal/repository/db/schema_pg.go).
func (s *Store) EnsureSchema(ctx context.Context) error {
	if s == nil {
		return nil
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS oauth_clients (
			client_id TEXT PRIMARY KEY,
			client_name TEXT,
			redirect_uris TEXT NOT NULL,
			grant_types TEXT NOT NULL,
			token_endpoint_auth_method TEXT NOT NULL,
			scope TEXT,
			client_uri TEXT,
			registration_access_token TEXT,
			created_at BIGINT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
			id TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			scope TEXT NOT NULL,
			resource TEXT NOT NULL,
			created_at BIGINT NOT NULL,
			expires_at BIGINT NOT NULL,
			revoked_at BIGINT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_refresh_expires ON oauth_refresh_tokens(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_oauth_refresh_client ON oauth_refresh_tokens(client_id)`,
		`CREATE TABLE IF NOT EXISTS oauth_signing_key (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			kid TEXT NOT NULL,
			algorithm TEXT NOT NULL,
			private_key_pem TEXT NOT NULL,
			public_jwk_json TEXT NOT NULL,
			created_at BIGINT NOT NULL
		)`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure oauth schema: %w", err)
		}
	}
	return nil
}

// ── signing key ─────────────────────────────────────────────────────────────

// LoadSigningKey returns the persisted key, or (nil, nil) when absent.
func (s *Store) LoadSigningKey(ctx context.Context) (*StoredSigningKey, error) {
	if s == nil {
		return nil, nil
	}
	var key StoredSigningKey
	err := s.db.QueryRowContext(ctx, `
		SELECT kid, algorithm, private_key_pem, public_jwk_json, created_at
		FROM oauth_signing_key WHERE id = 1
	`).Scan(&key.Kid, &key.Algorithm, &key.PrivateKeyPEM, &key.PublicJWKJSON, &key.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read oauth signing key: %w", err)
	}
	return &key, nil
}

// InsertSigningKeyIfAbsent atomically creates the singleton key row. Returns
// false when a key already exists (generation race loser).
func (s *Store) InsertSigningKeyIfAbsent(ctx context.Context, key StoredSigningKey) (bool, error) {
	if s == nil {
		return false, nil
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_signing_key (id, kid, algorithm, private_key_pem, public_jwk_json, created_at)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`, key.Kid, key.Algorithm, key.PrivateKeyPEM, key.PublicJWKJSON, key.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("insert oauth signing key: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("insert oauth signing key rows: %w", err)
	}
	return affected == 1, nil
}

// ── clients ─────────────────────────────────────────────────────────────────

// InsertClient persists a dynamically registered client.
func (s *Store) InsertClient(ctx context.Context, client Client) error {
	if s == nil {
		return errors.New("oauth: client persistence unavailable")
	}
	redirects, err := json.Marshal(client.RedirectURIs)
	if err != nil {
		return fmt.Errorf("encode redirect_uris: %w", err)
	}
	grants, err := json.Marshal(client.GrantTypes)
	if err != nil {
		return fmt.Errorf("encode grant_types: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_clients (
			client_id, client_name, redirect_uris, grant_types,
			token_endpoint_auth_method, scope, client_uri,
			registration_access_token, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(client_id) DO UPDATE SET
			client_name = excluded.client_name,
			redirect_uris = excluded.redirect_uris,
			grant_types = excluded.grant_types,
			token_endpoint_auth_method = excluded.token_endpoint_auth_method,
			scope = excluded.scope,
			client_uri = excluded.client_uri,
			registration_access_token = excluded.registration_access_token
	`, client.ID, client.Name, string(redirects), string(grants),
		client.TokenEndpointAuthMethod, client.Scope, client.ClientURI,
		client.RegistrationAccessToken, client.CreatedAt); err != nil {
		return fmt.Errorf("insert oauth client: %w", err)
	}
	return nil
}

// ClientByID loads a registered client, or (nil, nil) when unknown.
func (s *Store) ClientByID(ctx context.Context, clientID string) (*Client, error) {
	if s == nil {
		return nil, nil
	}
	var (
		client    Client
		redirects string
		grants    string
		name      sql.NullString
		scope     sql.NullString
		uri       sql.NullString
		rat       sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT client_id, client_name, redirect_uris, grant_types,
		       token_endpoint_auth_method, scope, client_uri,
		       registration_access_token, created_at
		FROM oauth_clients WHERE client_id = ?
	`, clientID).Scan(&client.ID, &name, &redirects, &grants,
		&client.TokenEndpointAuthMethod, &scope, &uri, &rat, &client.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read oauth client: %w", err)
	}
	client.Name = name.String
	client.Scope = scope.String
	client.ClientURI = uri.String
	client.RegistrationAccessToken = rat.String
	if err := json.Unmarshal([]byte(redirects), &client.RedirectURIs); err != nil {
		return nil, fmt.Errorf("decode redirect_uris for %s: %w", clientID, err)
	}
	if err := json.Unmarshal([]byte(grants), &client.GrantTypes); err != nil {
		return nil, fmt.Errorf("decode grant_types for %s: %w", clientID, err)
	}
	return &client, nil
}

// DeleteClient removes a registration (used by tests and admin cleanup).
func (s *Store) DeleteClient(ctx context.Context, clientID string) error {
	if s == nil {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM oauth_clients WHERE client_id = ?`, clientID); err != nil {
		return fmt.Errorf("delete oauth client: %w", err)
	}
	return nil
}

// ── refresh tokens ──────────────────────────────────────────────────────────

// NewRefreshToken generates an opaque refresh token and returns it together with
// its storage id (sha256 hex). The raw value is shown to the client exactly once.
func NewRefreshToken() (raw, id string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, tokenID(raw), nil
}

// tokenID is the storage key for an opaque token: sha256 hex (raw never stored).
func tokenID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// InsertRefreshToken persists a newly issued refresh token. Expired rows are
// swept opportunistically on the same call so no scheduler wiring is needed.
func (s *Store) InsertRefreshToken(ctx context.Context, token StoredRefreshToken) error {
	if s == nil {
		return errors.New("oauth: refresh token persistence unavailable")
	}
	// revoked_at must be NULL (not 0) while the token is live: the revocation
	// predicates use "revoked_at IS NULL", and the expiry sweep would otherwise
	// see 0 as a revocation timestamp far in the past and delete a token it just
	// wrote. Storing 0 here silently destroyed every refresh token on issue.
	var revoked any
	if token.RevokedAt != 0 {
		revoked = token.RevokedAt
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_refresh_tokens (id, client_id, scope, resource, created_at, expires_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, token.ID, token.ClientID, token.Scope, token.Resource, token.CreatedAt, token.ExpiresAt, revoked); err != nil {
		return fmt.Errorf("insert oauth refresh token: %w", err)
	}
	s.sweepExpiredRefreshTokens(ctx, token.CreatedAt)
	return nil
}

// RefreshTokenByID loads a non-revoked, unexpired refresh token.
func (s *Store) RefreshTokenByID(ctx context.Context, id string, now int64) (*StoredRefreshToken, error) {
	if s == nil {
		return nil, nil
	}
	var token StoredRefreshToken
	var revoked sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT id, client_id, scope, resource, created_at, expires_at, revoked_at
		FROM oauth_refresh_tokens WHERE id = ?
	`, id).Scan(&token.ID, &token.ClientID, &token.Scope, &token.Resource,
		&token.CreatedAt, &token.ExpiresAt, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read oauth refresh token: %w", err)
	}
	token.RevokedAt = revoked.Int64
	if token.RevokedAt != 0 || token.ExpiresAt <= now {
		return nil, nil
	}
	return &token, nil
}

// RevokeRefreshToken marks a token used/revoked. Refresh token rotation calls
// this before issuing the replacement, so a replayed token is rejected.
func (s *Store) RevokeRefreshToken(ctx context.Context, id string, now int64) error {
	if s == nil {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE oauth_refresh_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL
	`, now, id); err != nil {
		return fmt.Errorf("revoke oauth refresh token: %w", err)
	}
	return nil
}

// RevokeClientTokens revokes every refresh token for a client (admin kill switch).
func (s *Store) RevokeClientTokens(ctx context.Context, clientID string, now int64) error {
	if s == nil {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE oauth_refresh_tokens SET revoked_at = ? WHERE client_id = ? AND revoked_at IS NULL
	`, now, clientID); err != nil {
		return fmt.Errorf("revoke oauth client tokens: %w", err)
	}
	return nil
}

// refreshTokenRetention is how long an expired or revoked refresh token row is
// kept before the opportunistic sweep drops it.
const refreshTokenRetention int64 = 30 * 86400

// sweepExpiredRefreshTokens is best-effort hygiene; a failure never blocks issue.
// Both predicates are strictly "older than the retention window", so a live token
// can never be swept regardless of clock skew.
func (s *Store) sweepExpiredRefreshTokens(ctx context.Context, now int64) {
	cutoff := now - refreshTokenRetention
	if cutoff <= 0 {
		return
	}
	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM oauth_refresh_tokens
		WHERE expires_at <= ? OR (revoked_at IS NOT NULL AND revoked_at > 0 AND revoked_at <= ?)
	`, cutoff, cutoff); err != nil {
		slog.Warn("oauth refresh token sweep failed", "error", err.Error())
	}
}

// CountClients reports how many clients are registered (admin/status surface).
func (s *Store) CountClients(ctx context.Context) (int, error) {
	if s == nil {
		return 0, nil
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM oauth_clients`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count oauth clients: %w", err)
	}
	return count, nil
}
