// Package confirmations manages short-lived confirmation tokens for agent write operations.
// Tokens carry a tool name, payload hash, and TTL. Raw tokens are never stored; only HMAC hashes
// are persisted.
package confirmations

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
)

var (
	ErrEmptySecret                 = errors.New("confirmation secret is empty")
	ErrConfirmationNotRequired     = errors.New("tool does not require confirmation")
	ErrPayloadHash                 = errors.New("payload hash failed")
	ErrToolMismatch                = errors.New("confirmation tool mismatch")
	ErrPayloadMismatch             = errors.New("confirmation payload mismatch")
	ErrTokenMismatch               = errors.New("confirmation token mismatch")
	ErrAlreadyUsed                 = errors.New("confirmation already used")
	ErrExpired                     = errors.New("confirmation expired")
	ErrInvalidConfirmationLifetime = errors.New("confirmation ttl must be positive")
)

type Manager struct {
	secret []byte
	clock  func() time.Time
}

type Option func(*Manager)

type IssueInput struct {
	Tool    agenttools.ToolDefinition
	Payload map[string]any
	TTL     time.Duration
}

type VerifyInput struct {
	Record  Record
	Token   string
	Tool    agenttools.ToolDefinition
	Payload map[string]any
}

type IssuedConfirmation struct {
	Token  string
	Record Record
}

type Record struct {
	Tool        string     `json:"tool"`
	TokenHash   string     `json:"token_hash"`
	PayloadHash string     `json:"payload_hash"`
	ExpiresAt   time.Time  `json:"expires_at"`
	UsedAt      *time.Time `json:"used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func NewManager(secret []byte, options ...Option) (*Manager, error) {
	if len(secret) == 0 {
		return nil, ErrEmptySecret
	}
	manager := &Manager{
		secret: append([]byte(nil), secret...),
		clock:  time.Now,
	}
	for _, option := range options {
		option(manager)
	}
	if manager.clock == nil {
		manager.clock = time.Now
	}
	return manager, nil
}

func WithClock(clock func() time.Time) Option {
	return func(manager *Manager) {
		manager.clock = clock
	}
}

func (m *Manager) Issue(input IssueInput) (IssuedConfirmation, error) {
	if !requiresConfirmation(input.Tool) {
		return IssuedConfirmation{}, ErrConfirmationNotRequired
	}
	ttl := input.TTL
	if ttl == 0 && input.Tool.Confirmation.TokenTTLSeconds != nil {
		ttl = time.Duration(*input.Tool.Confirmation.TokenTTLSeconds) * time.Second
	}
	if ttl <= 0 {
		return IssuedConfirmation{}, ErrInvalidConfirmationLifetime
	}
	payloadHash, err := m.PayloadHash(input.Payload)
	if err != nil {
		return IssuedConfirmation{}, err
	}
	token, err := generateToken()
	if err != nil {
		return IssuedConfirmation{}, err
	}
	now := m.clock().UTC()
	return IssuedConfirmation{
		Token: token,
		Record: Record{
			Tool:        input.Tool.Name,
			TokenHash:   m.TokenHash(token),
			PayloadHash: payloadHash,
			CreatedAt:   now,
			ExpiresAt:   now.Add(ttl),
		},
	}, nil
}

func (m *Manager) Verify(input VerifyInput) error {
	if input.Record.Tool != input.Tool.Name {
		return ErrToolMismatch
	}
	payloadHash, err := m.PayloadHash(input.Payload)
	if err != nil {
		return err
	}
	if input.Record.PayloadHash != payloadHash {
		return ErrPayloadMismatch
	}
	if subtle.ConstantTimeCompare([]byte(input.Record.TokenHash), []byte(m.TokenHash(input.Token))) != 1 {
		return ErrTokenMismatch
	}
	if input.Record.UsedAt != nil {
		return ErrAlreadyUsed
	}
	if !m.clock().UTC().Before(input.Record.ExpiresAt) {
		return ErrExpired
	}
	return nil
}

func (m *Manager) PayloadHash(payload map[string]any) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPayloadHash, err)
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

// MustPayloadHash is a test-only helper (panic on error).
// Production callers must use PayloadHash.
func (m *Manager) MustPayloadHash(payload map[string]any) string {
	hash, err := m.PayloadHash(payload)
	if err != nil {
		panic(err)
	}
	return hash
}

func (m *Manager) TokenHash(token string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func requiresConfirmation(tool agenttools.ToolDefinition) bool {
	return tool.Confirmation.Required && tool.Capability.Permission == agenttools.PermissionRequiresConfirmation
}

func generateToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate confirmation token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
