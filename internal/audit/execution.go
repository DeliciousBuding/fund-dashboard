package audit

import "time"

// ExecutionStatus classifies the outcome of one tool execution for the audit
// trail. It is a closed set: raw error text must never appear here or in any
// other execution event field.
type ExecutionStatus string

const (
	ExecutionOK             ExecutionStatus = "ok"
	ExecutionErrored        ExecutionStatus = "errored"
	ExecutionPanicRecovered ExecutionStatus = "panic_recovered"
)

// IsValid reports whether the status belongs to the closed set.
func (s ExecutionStatus) IsValid() bool {
	switch s {
	case ExecutionOK, ExecutionErrored, ExecutionPanicRecovered:
		return true
	default:
		return false
	}
}

// ExecutionErrorCategory is the closed set of sanitized error classes that may
// accompany an errored execution event. It deliberately carries no error text,
// so SQL state codes, file paths, URLs, and driver details cannot reach the
// audit store through this field.
type ExecutionErrorCategory string

const (
	ExecutionCategoryValidation     ExecutionErrorCategory = "validation"
	ExecutionCategoryDenied         ExecutionErrorCategory = "denied"
	ExecutionCategoryNotImplemented ExecutionErrorCategory = "not_implemented"
	ExecutionCategoryInternal       ExecutionErrorCategory = "internal"
)

// IsValid reports whether the category belongs to the closed set.
func (c ExecutionErrorCategory) IsValid() bool {
	switch c {
	case ExecutionCategoryValidation, ExecutionCategoryDenied,
		ExecutionCategoryNotImplemented, ExecutionCategoryInternal:
		return true
	default:
		return false
	}
}

// ExecutionEvent is the persisted-ready execution outcome envelope. Fields are
// intentionally bounded: Tool is the registry tool name, Status and
// ErrorCategory are closed sets, and DurationMs is the only measurement kept.
type ExecutionEvent struct {
	Tool          string                 `json:"tool"`
	RequestID     string                 `json:"request_id,omitempty"`
	Caller        string                 `json:"caller,omitempty"`
	Status        ExecutionStatus        `json:"status"`
	ErrorCategory ExecutionErrorCategory `json:"error_category,omitempty"`
	DurationMs    int64                  `json:"duration_ms"`
	RecordedAt    string                 `json:"recorded_at"`
}

// ExecutionEventInput feeds NewExecutionEvent.
type ExecutionEventInput struct {
	Tool          string
	RequestID     string
	Caller        string
	Status        ExecutionStatus
	ErrorCategory ExecutionErrorCategory
	Duration      time.Duration
	// Now overrides the timestamp source for tests. When nil time.Now is used.
	Now func() time.Time
}

// NewExecutionEvent normalizes caller input into a safe, closed-set event:
// unknown statuses collapse to errored, unknown categories collapse to
// internal (or are dropped entirely for ok outcomes), and negative durations
// clamp to zero. No free-form error text exists on the event, so sanitization
// cannot be bypassed by a caller mistake.
func NewExecutionEvent(input ExecutionEventInput) ExecutionEvent {
	status := input.Status
	if !status.IsValid() {
		status = ExecutionErrored
	}
	category := input.ErrorCategory
	if status == ExecutionOK {
		category = ""
	} else if !category.IsValid() {
		category = ExecutionCategoryInternal
	}
	durationMs := input.Duration.Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	now := input.Now
	if now == nil {
		now = time.Now
	}
	return ExecutionEvent{
		Tool:          input.Tool,
		RequestID:     input.RequestID,
		Caller:        input.Caller,
		Status:        status,
		ErrorCategory: category,
		DurationMs:    durationMs,
		RecordedAt:    now().UTC().Format(time.RFC3339Nano),
	}
}
