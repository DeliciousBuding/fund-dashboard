// Package agentstate provides SQLite-backed persistence for agent confirmation records
// and audit events. It owns the schema migrations for agent_confirmations and agent_audit_events.
package agentstate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/confirmations"
)

type ConfirmationRepository struct {
	db *sql.DB
}

func NewConfirmationRepository(db *sql.DB) ConfirmationRepository {
	return ConfirmationRepository{db: db}
}

func (r ConfirmationRepository) EnsureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS agent_confirmations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tool TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			payload_hash TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			used_at TEXT,
			created_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("ensure agent_confirmations schema: %w", err)
	}
	// SQLite parity with the PG index set (schema_pg.go). Best-effort so a
	// legacy table with a different shape can never block boot.
	if _, err := r.db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_agent_confirmations_tool ON agent_confirmations(tool)
	`); err != nil {
		slog.Warn("agent_confirmations tool index skipped", "error", err)
	}
	return nil
}

func (r ConfirmationRepository) Save(ctx context.Context, record confirmations.Record) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO agent_confirmations (
			tool,
			token_hash,
			payload_hash,
			expires_at,
			used_at,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`,
		record.Tool,
		record.TokenHash,
		record.PayloadHash,
		formatTime(record.ExpiresAt),
		formatOptionalTime(record.UsedAt),
		formatTime(record.CreatedAt),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("save agent confirmation: %w", err)
	}
	return id, nil
}

func (r ConfirmationRepository) Get(ctx context.Context, id int64) (*confirmations.Record, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			tool,
			token_hash,
			payload_hash,
			expires_at,
			used_at,
			created_at
		FROM agent_confirmations
		WHERE id = ?
	`, id)

	var record confirmations.Record
	var expiresAt, usedAt, createdAt sql.NullString
	if err := row.Scan(
		&record.Tool,
		&record.TokenHash,
		&record.PayloadHash,
		&expiresAt,
		&usedAt,
		&createdAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent confirmation: %w", err)
	}
	var err error
	record.ExpiresAt, err = parseRequiredTime(expiresAt, "expires_at")
	if err != nil {
		return nil, err
	}
	record.CreatedAt, err = parseRequiredTime(createdAt, "created_at")
	if err != nil {
		return nil, err
	}
	record.UsedAt, err = parseOptionalTime(usedAt)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r ConfirmationRepository) MarkUsed(ctx context.Context, id int64, usedAt time.Time) error {
	// Atomic single-use: only the first updater with used_at IS NULL wins (PG TOCTOU-safe).
	result, err := r.db.ExecContext(ctx, `
		UPDATE agent_confirmations
		SET used_at = ?
		WHERE id = ? AND used_at IS NULL
	`, formatTime(usedAt), id)
	if err != nil {
		return fmt.Errorf("mark agent confirmation used: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("agent confirmation rows affected: %w", err)
	}
	if affected == 0 {
		// Either unknown id or already consumed — caller treats as already-used / invalid.
		return confirmations.ErrAlreadyUsed
	}
	return nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func parseRequiredTime(value sql.NullString, field string) (time.Time, error) {
	if !value.Valid || value.String == "" {
		return time.Time{}, fmt.Errorf("agent confirmation %s is empty", field)
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse agent confirmation %s: %w", field, err)
	}
	return parsed, nil
}

func parseOptionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || value.String == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, fmt.Errorf("parse agent confirmation used_at: %w", err)
	}
	return &parsed, nil
}
