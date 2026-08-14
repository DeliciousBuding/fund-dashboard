package agentstate

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/DeliciousBuding/fund-dashboard/internal/audit"
)

type AuditEventRepository struct {
	db *sql.DB
}

func NewAuditEventRepository(db *sql.DB) AuditEventRepository {
	return AuditEventRepository{db: db}
}

func (r AuditEventRepository) EnsureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS agent_audit_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			caller TEXT NOT NULL,
			tool TEXT NOT NULL,
			event_type TEXT NOT NULL,
			status TEXT NOT NULL,
			scope TEXT NOT NULL,
			permission TEXT NOT NULL,
			risk_level TEXT NOT NULL,
			redacted_args_json TEXT NOT NULL,
			result_summary_json TEXT NOT NULL,
			created_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("ensure agent_audit_events schema: %w", err)
	}
	return nil
}

func (r AuditEventRepository) Save(ctx context.Context, event audit.Event) (int64, error) {
	redactedArgs, err := encodeAuditMap(event.RedactedArgs)
	if err != nil {
		return 0, fmt.Errorf("encode audit redacted args: %w", err)
	}
	resultSummary, err := encodeAuditMap(event.ResultSummary)
	if err != nil {
		return 0, fmt.Errorf("encode audit result summary: %w", err)
	}

	var id int64
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO agent_audit_events (
			request_id,
			caller,
			tool,
			event_type,
			status,
			scope,
			permission,
			risk_level,
			redacted_args_json,
			result_summary_json,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`,
		event.RequestID,
		event.Caller,
		event.Tool,
		event.EventType,
		string(event.Status),
		event.Scope,
		event.Permission,
		event.RiskLevel,
		redactedArgs,
		resultSummary,
		event.CreatedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("save agent audit event: %w", err)
	}
	return id, nil
}

func (r AuditEventRepository) Get(ctx context.Context, id int64) (*audit.Event, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			request_id,
			caller,
			tool,
			event_type,
			status,
			scope,
			permission,
			risk_level,
			redacted_args_json,
			result_summary_json,
			created_at
		FROM agent_audit_events
		WHERE id = ?
	`, id)

	var event audit.Event
	var status string
	var redactedArgsJSON, resultSummaryJSON string
	if err := row.Scan(
		&event.RequestID,
		&event.Caller,
		&event.Tool,
		&event.EventType,
		&status,
		&event.Scope,
		&event.Permission,
		&event.RiskLevel,
		&redactedArgsJSON,
		&resultSummaryJSON,
		&event.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent audit event: %w", err)
	}

	var err error
	event.RedactedArgs, err = decodeAuditMap(redactedArgsJSON)
	if err != nil {
		return nil, fmt.Errorf("decode audit redacted args: %w", err)
	}
	event.ResultSummary, err = decodeAuditMap(resultSummaryJSON)
	if err != nil {
		return nil, fmt.Errorf("decode audit result summary: %w", err)
	}
	event.Status = audit.Status(status)
	return &event, nil
}

func encodeAuditMap(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	// Cap persisted JSON so MaxBytes bodies cannot bloat audit rows (#229).
	const maxAuditJSON = 64 * 1024
	if len(encoded) > maxAuditJSON {
		return `{"_truncated":true,"_bytes":` + fmt.Sprintf("%d", len(encoded)) + `}`, nil
	}
	return string(encoded), nil
}

func decodeAuditMap(value string) (map[string]any, error) {
	if value == "" {
		return map[string]any{}, nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return nil, err
	}
	if decoded == nil {
		return map[string]any{}, nil
	}
	return decoded, nil
}
