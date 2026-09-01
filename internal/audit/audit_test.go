package audit

import (
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
)

func TestRedactMappingRedactsNestedSensitiveValuesWithoutMutatingInput(t *testing.T) {
	input := map[string]any{
		"fund_code": "AAPL",
		"token":     "secret-token",
		"nested": map[string]any{
			"authorization_header": "Bearer secret",
			"safe_note":            "keep me",
		},
		"items": []any{
			map[string]any{"webhook_url": "https://example.test/hook?token=secret"},
			"plain",
		},
	}

	redacted := RedactMapping(input, []string{"token", "authorization", "webhook"})

	if redacted["token"] != RedactedValue {
		t.Fatalf("token = %#v, want redacted", redacted["token"])
	}
	nested := redacted["nested"].(map[string]any)
	if nested["authorization_header"] != RedactedValue || nested["safe_note"] != "keep me" {
		t.Fatalf("nested = %#v, want authorization redacted and safe note preserved", nested)
	}
	item := redacted["items"].([]any)[0].(map[string]any)
	if item["webhook_url"] != RedactedValue {
		t.Fatalf("webhook_url = %#v, want redacted", item["webhook_url"])
	}
	if input["token"] != "secret-token" {
		t.Fatalf("input mutated: %#v", input)
	}
}

func TestNewAttemptEventCopiesToolPolicyAndRedactsArgs(t *testing.T) {
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry returned error: %v", err)
	}
	tool, ok := registry.Lookup("add_transaction")
	if !ok {
		t.Fatalf("add_transaction not found")
	}

	event := NewAttemptEvent(EventInput{
		RequestID: "req-1",
		Caller:    "hermes",
		Tool:      tool,
		Args: map[string]any{
			"fund_code": "AAPL",
			"api_key":   "secret",
		},
	})

	if event.Status != StatusAttempt || event.Tool != "add_transaction" {
		t.Fatalf("event = %#v, want attempt for add_transaction", event)
	}
	if event.Permission != string(agenttools.PermissionRequiresConfirmation) ||
		event.Scope != string(agenttools.ScopeWrite) ||
		event.RiskLevel != string(agenttools.RiskHigh) {
		t.Fatalf("event policy = %#v, want write confirmation high risk", event)
	}
	if event.RedactedArgs["api_key"] != RedactedValue || event.RedactedArgs["fund_code"] != "AAPL" {
		t.Fatalf("RedactedArgs = %#v, want api_key redacted and fund_code preserved", event.RedactedArgs)
	}
	if event.CreatedAt == "" {
		t.Fatalf("CreatedAt is empty")
	}
}

func TestNewResultEventRedactsResultSummary(t *testing.T) {
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry returned error: %v", err)
	}
	tool, ok := registry.Lookup("check_alerts")
	if !ok {
		t.Fatalf("check_alerts not found")
	}

	event := NewResultEvent(EventInput{
		RequestID: "req-2",
		Caller:    "hermes",
		Tool:      tool,
		Result: map[string]any{
			"status":  "sent",
			"webhook": "https://example.test/hook",
		},
	})

	if event.Status != StatusResult {
		t.Fatalf("Status = %q, want result", event.Status)
	}
	if event.ResultSummary["webhook"] != RedactedValue || event.ResultSummary["status"] != "sent" {
		t.Fatalf("ResultSummary = %#v, want webhook redacted and status preserved", event.ResultSummary)
	}
}

func TestNewAttemptEventUsesInjectedClock(t *testing.T) {
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry returned error: %v", err)
	}
	tool, ok := registry.Lookup("add_transaction")
	if !ok {
		t.Fatalf("add_transaction not found")
	}

	// Fixed non-UTC instant: the event must carry the UTC rendering of this
	// exact time, not the wall clock observed by the test.
	fixed := time.Date(2026, 8, 2, 3, 4, 5, 123456789, time.FixedZone("CST", 8*3600))
	event := NewAttemptEvent(EventInput{
		RequestID: "req-clock",
		Caller:    "hermes",
		Tool:      tool,
		Now:       func() time.Time { return fixed },
	})

	want := fixed.UTC().Format(time.RFC3339Nano)
	if event.CreatedAt != want {
		t.Fatalf("CreatedAt = %q, want %q", event.CreatedAt, want)
	}
}

func TestNewResultEventDefaultsToWallClock(t *testing.T) {
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry returned error: %v", err)
	}
	tool, ok := registry.Lookup("check_alerts")
	if !ok {
		t.Fatalf("check_alerts not found")
	}

	before := time.Now()
	event := NewResultEvent(EventInput{RequestID: "req-wall", Tool: tool})
	after := time.Now()

	created, err := time.Parse(time.RFC3339Nano, event.CreatedAt)
	if err != nil {
		t.Fatalf("CreatedAt %q is not RFC3339Nano: %v", event.CreatedAt, err)
	}
	if created.Before(before.Add(-time.Second)) || created.After(after.Add(time.Second)) {
		t.Fatalf("CreatedAt = %q, want within wall-clock window [%s, %s]", event.CreatedAt, before, after)
	}
}
