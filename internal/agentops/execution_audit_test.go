package agentops

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/audit"
)

func TestRecordExecutionPersistsClosedSetSummary(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	service := newAgentOpsFixture(t, db, func() time.Time {
		return time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	})

	event := audit.NewExecutionEvent(audit.ExecutionEventInput{
		Tool:          "add_transaction",
		RequestID:     "req-exec-1",
		Caller:        "hermes",
		Status:        audit.ExecutionErrored,
		ErrorCategory: audit.ExecutionCategoryDenied,
		Duration:      12 * time.Millisecond,
		Now:           func() time.Time { return time.Date(2026, 9, 2, 1, 2, 3, 0, time.UTC) },
	})
	if err := service.RecordExecution(ctx, event); err != nil {
		t.Fatalf("RecordExecution returned error: %v", err)
	}

	events, err := service.auditRepo.List(ctx, 10)
	if err != nil {
		t.Fatalf("audit List returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	row := events[0]
	if row.EventType != "execution" || row.Status != audit.StatusResult {
		t.Fatalf("row = %#v, want execution/result envelope", row)
	}
	if row.RequestID != "req-exec-1" || row.Caller != "hermes" || row.Tool != "add_transaction" {
		t.Fatalf("row identity = %#v", row)
	}
	// Registry attribution: add_transaction capability fields must be filled.
	if row.Scope == "" || row.Permission == "" || row.RiskLevel == "" {
		t.Fatalf("row capability attribution empty: %#v", row)
	}
	if row.ResultSummary["kind"] != "execution" ||
		row.ResultSummary["execution_status"] != string(audit.ExecutionErrored) ||
		row.ResultSummary["error_category"] != string(audit.ExecutionCategoryDenied) {
		t.Fatalf("result summary = %#v", row.ResultSummary)
	}
	// No raw error text channel exists: summary keys are the closed set.
	for key := range row.ResultSummary {
		switch key {
		case "kind", "execution_status", "error_category", "duration_ms", "recorded_at":
		default:
			t.Fatalf("unexpected summary key %q", key)
		}
	}
}

func TestRecordExecutionOKOutcomeOmitsCategory(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	service := newAgentOpsFixture(t, db, nil)

	event := audit.NewExecutionEvent(audit.ExecutionEventInput{
		Tool:   "add_transaction",
		Status: audit.ExecutionOK,
	})
	if err := service.RecordExecution(ctx, event); err != nil {
		t.Fatalf("RecordExecution returned error: %v", err)
	}
	events, err := service.auditRepo.List(ctx, 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %d err = %v, want 1 row", len(events), err)
	}
	if _, exists := events[0].ResultSummary["error_category"]; exists {
		t.Fatalf("ok outcome must not persist error_category: %#v", events[0].ResultSummary)
	}
}

func TestRecordExecutionUnknownToolStillPersists(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	service := newAgentOpsFixture(t, db, nil)

	event := audit.NewExecutionEvent(audit.ExecutionEventInput{
		Tool:          "not_a_registered_tool",
		Status:        audit.ExecutionErrored,
		ErrorCategory: audit.ExecutionCategoryInternal,
	})
	if err := service.RecordExecution(ctx, event); err != nil {
		t.Fatalf("RecordExecution returned error: %v", err)
	}
	events, err := service.auditRepo.List(ctx, 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %d err = %v, want 1 row", len(events), err)
	}
	if events[0].Scope != "" || events[0].Permission != "" || events[0].RiskLevel != "" {
		t.Fatalf("unknown tool must have empty capability attribution: %#v", events[0])
	}
}

func TestRecordExecutionRejectsOversizedIdentity(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	service := newAgentOpsFixture(t, db, nil)

	event := audit.NewExecutionEvent(audit.ExecutionEventInput{
		Tool:      "add_transaction",
		Caller:    strings.Repeat("c", 129),
		RequestID: "ok",
		Status:    audit.ExecutionOK,
	})
	if err := service.RecordExecution(ctx, event); err != ErrIdentityTooLong {
		t.Fatalf("err = %v, want ErrIdentityTooLong", err)
	}
	events, err := service.auditRepo.List(ctx, 10)
	if err != nil {
		t.Fatalf("audit List returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("oversized identity must not persist, got %d rows", len(events))
	}
}
