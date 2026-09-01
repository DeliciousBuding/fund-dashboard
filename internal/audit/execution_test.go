package audit

import (
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/chinatime"
)

func TestExecutionStatusIsClosedSet(t *testing.T) {
	for _, status := range []ExecutionStatus{ExecutionOK, ExecutionErrored, ExecutionPanicRecovered} {
		if !status.IsValid() {
			t.Fatalf("status %q should be valid", status)
		}
	}
	for _, status := range []ExecutionStatus{"", "ok!", "panic", "error", "SQLSTATE 42P01"} {
		if status.IsValid() {
			t.Fatalf("status %q should be rejected", status)
		}
	}
}

func TestExecutionErrorCategoryIsClosedSet(t *testing.T) {
	for _, category := range []ExecutionErrorCategory{
		ExecutionCategoryValidation,
		ExecutionCategoryDenied,
		ExecutionCategoryNotImplemented,
		ExecutionCategoryInternal,
	} {
		if !category.IsValid() {
			t.Fatalf("category %q should be valid", category)
		}
	}
	for _, category := range []ExecutionErrorCategory{
		"",
		"sql_error",
		"ERROR: relation transactions does not exist",
		"file:///tmp/x",
	} {
		if category.IsValid() {
			t.Fatalf("category %q should be rejected", category)
		}
	}
}

func TestNewExecutionEventDropsCategoryForOK(t *testing.T) {
	event := NewExecutionEvent(ExecutionEventInput{
		Tool:          "crawl_nav",
		Status:        ExecutionOK,
		ErrorCategory: ExecutionCategoryValidation,
		Duration:      2 * time.Millisecond,
		Now: func() time.Time {
			return time.Date(2026, 8, 2, 3, 4, 5, 123456789, chinatime.Loc)
		},
	})
	if event.Tool != "crawl_nav" || event.Status != ExecutionOK {
		t.Fatalf("event = %#v, want ok for crawl_nav", event)
	}
	if event.ErrorCategory != "" {
		t.Fatalf("ErrorCategory = %q, want empty for ok outcome", event.ErrorCategory)
	}
	if event.DurationMs != 2 {
		t.Fatalf("DurationMs = %d, want 2", event.DurationMs)
	}
	want := time.Date(2026, 8, 2, 3, 4, 5, 123456789, chinatime.Loc).UTC().Format(time.RFC3339Nano)
	if event.RecordedAt != want {
		t.Fatalf("RecordedAt = %q, want %q", event.RecordedAt, want)
	}
}

func TestNewExecutionEventNormalizesUnsafeInput(t *testing.T) {
	// Unknown status collapses to errored and unknown category collapses to
	// internal: free-form text can never reach the audit store.
	event := NewExecutionEvent(ExecutionEventInput{
		Tool:          "add_transaction",
		Status:        ExecutionStatus("exploded"),
		ErrorCategory: ExecutionErrorCategory("ERROR: relation x does not exist (SQLSTATE 42P01)"),
		Duration:      -5 * time.Millisecond,
	})
	if event.Status != ExecutionErrored {
		t.Fatalf("Status = %q, want errored", event.Status)
	}
	if event.ErrorCategory != ExecutionCategoryInternal {
		t.Fatalf("ErrorCategory = %q, want internal", event.ErrorCategory)
	}
	if event.DurationMs != 0 {
		t.Fatalf("DurationMs = %d, want clamped 0", event.DurationMs)
	}

	// panic_recovered with no category defaults to internal.
	panicEvent := NewExecutionEvent(ExecutionEventInput{
		Tool:   "crawl_nav",
		Status: ExecutionPanicRecovered,
	})
	if panicEvent.Status != ExecutionPanicRecovered || panicEvent.ErrorCategory != ExecutionCategoryInternal {
		t.Fatalf("panic event = %#v, want panic_recovered/internal", panicEvent)
	}

	// Errored events keep a valid category and record a non-negative duration.
	errored := NewExecutionEvent(ExecutionEventInput{
		Tool:          "crawl_nav",
		Status:        ExecutionErrored,
		ErrorCategory: ExecutionCategoryValidation,
		Duration:      700 * time.Microsecond,
	})
	if errored.Status != ExecutionErrored || errored.ErrorCategory != ExecutionCategoryValidation {
		t.Fatalf("errored event = %#v, want errored/validation", errored)
	}
	if errored.DurationMs < 0 {
		t.Fatalf("DurationMs = %d, want >= 0", errored.DurationMs)
	}
}

func TestNewExecutionEventCarriesRequestIDAndCaller(t *testing.T) {
	event := NewExecutionEvent(ExecutionEventInput{
		Tool:      "crawl_nav",
		RequestID: "req-abc",
		Caller:    "operator",
		Status:    ExecutionOK,
	})
	if event.RequestID != "req-abc" || event.Caller != "operator" {
		t.Fatalf("event = %#v, want attribution carried through", event)
	}
}
