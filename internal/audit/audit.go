// Package audit provides agent action audit events with recursive redaction.
// It produces attempt/result envelopes suitable for persistence without leaking
// secrets, tokens, or webhook URLs.
package audit

import (
	"strings"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
)

const RedactedValue = "[REDACTED]"

type Status string

const (
	StatusAttempt Status = "attempt"
	StatusResult  Status = "result"
)

type EventInput struct {
	RequestID string
	Caller    string
	Tool      agenttools.ToolDefinition
	Args      map[string]any
	Result    map[string]any
	// Now overrides the audit timestamp source for tests. When nil the
	// package default (time.Now) is used.
	Now func() time.Time
}

type Event struct {
	RequestID     string         `json:"request_id"`
	Caller        string         `json:"caller"`
	Tool          string         `json:"tool"`
	EventType     string         `json:"event_type"`
	Status        Status         `json:"status"`
	Scope         string         `json:"scope"`
	Permission    string         `json:"permission"`
	RiskLevel     string         `json:"risk_level"`
	RedactedArgs  map[string]any `json:"redacted_args,omitempty"`
	ResultSummary map[string]any `json:"result_summary,omitempty"`
	CreatedAt     string         `json:"created_at"`
}

func NewAttemptEvent(input EventInput) Event {
	event := newEvent(input, StatusAttempt)
	event.RedactedArgs = RedactMapping(input.Args, input.Tool.Audit.RedactArgs)
	return event
}

func NewResultEvent(input EventInput) Event {
	event := newEvent(input, StatusResult)
	event.ResultSummary = RedactMapping(input.Result, input.Tool.Audit.RedactArgs)
	return event
}

func RedactMapping(input map[string]any, keys []string) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		if shouldRedactKey(key, keys) {
			output[key] = RedactedValue
			continue
		}
		output[key] = redactValue(value, keys)
	}
	return output
}

func newEvent(input EventInput, status Status) Event {
	now := input.Now
	if now == nil {
		now = time.Now
	}
	return Event{
		RequestID:  input.RequestID,
		Caller:     input.Caller,
		Tool:       input.Tool.Name,
		EventType:  input.Tool.Audit.EventType,
		Status:     status,
		Scope:      string(input.Tool.Capability.Scope),
		Permission: string(input.Tool.Capability.Permission),
		RiskLevel:  string(input.Tool.Capability.RiskLevel),
		CreatedAt:  now().UTC().Format(time.RFC3339Nano),
	}
}

func redactValue(value any, keys []string) any {
	switch typed := value.(type) {
	case map[string]any:
		return RedactMapping(typed, keys)
	case []any:
		output := make([]any, len(typed))
		for i, item := range typed {
			output[i] = redactValue(item, keys)
		}
		return output
	default:
		return value
	}
}

func shouldRedactKey(key string, keys []string) bool {
	lowerKey := strings.ToLower(key)
	for _, redactKey := range keys {
		redactKey = strings.ToLower(strings.TrimSpace(redactKey))
		if redactKey != "" && strings.Contains(lowerKey, redactKey) {
			return true
		}
	}
	return false
}
