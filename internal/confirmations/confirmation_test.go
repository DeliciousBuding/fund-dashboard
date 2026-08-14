package confirmations

import (
	"errors"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
)

func TestIssueCreatesBoundedConfirmationWithoutPersistingRawToken(t *testing.T) {
	now := time.Date(2026, 7, 7, 4, 10, 0, 0, time.UTC)
	manager := newTestManager(t, func() time.Time { return now })
	tool := lookupTool(t, "add_transaction")
	payload := map[string]any{"fund_code": "AAPL", "amount": 100.5}

	issued, err := manager.Issue(IssueInput{
		Tool:    tool,
		Payload: payload,
		TTL:     15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if issued.Token == "" {
		t.Fatalf("Token is empty")
	}
	if issued.Record.TokenHash == "" || issued.Record.TokenHash == issued.Token {
		t.Fatalf("TokenHash = %q, token = %q; want stored hash only", issued.Record.TokenHash, issued.Token)
	}
	if issued.Record.Tool != "add_transaction" {
		t.Fatalf("Record.Tool = %q, want add_transaction", issued.Record.Tool)
	}
	if issued.Record.PayloadHash != manager.MustPayloadHash(payload) {
		t.Fatalf("PayloadHash = %q, want deterministic payload hash", issued.Record.PayloadHash)
	}
	if !issued.Record.CreatedAt.Equal(now) || !issued.Record.ExpiresAt.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("record times = created %s expires %s", issued.Record.CreatedAt, issued.Record.ExpiresAt)
	}
	if issued.Record.UsedAt != nil {
		t.Fatalf("UsedAt = %#v, want nil", issued.Record.UsedAt)
	}
}

func TestVerifyBindsTokenToToolPayloadExpiryAndUseState(t *testing.T) {
	now := time.Date(2026, 7, 7, 4, 20, 0, 0, time.UTC)
	manager := newTestManager(t, func() time.Time { return now })
	tool := lookupTool(t, "add_transaction")
	payload := map[string]any{"fund_code": "AAPL", "amount": 100.5}
	issued, err := manager.Issue(IssueInput{Tool: tool, Payload: payload, TTL: time.Minute})
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if err := manager.Verify(VerifyInput{Record: issued.Record, Token: issued.Token, Tool: tool, Payload: payload}); err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	otherTool := lookupTool(t, "check_alerts")
	if err := manager.Verify(VerifyInput{Record: issued.Record, Token: issued.Token, Tool: otherTool, Payload: payload}); !errors.Is(err, ErrToolMismatch) {
		t.Fatalf("Verify wrong tool error = %v, want ErrToolMismatch", err)
	}
	if err := manager.Verify(VerifyInput{Record: issued.Record, Token: issued.Token, Tool: tool, Payload: map[string]any{"fund_code": "AAPL", "amount": 101}}); !errors.Is(err, ErrPayloadMismatch) {
		t.Fatalf("Verify wrong payload error = %v, want ErrPayloadMismatch", err)
	}
	if err := manager.Verify(VerifyInput{Record: issued.Record, Token: "wrong-token", Tool: tool, Payload: payload}); !errors.Is(err, ErrTokenMismatch) {
		t.Fatalf("Verify wrong token error = %v, want ErrTokenMismatch", err)
	}
	usedAt := now.Add(30 * time.Second)
	used := issued.Record
	used.UsedAt = &usedAt
	if err := manager.Verify(VerifyInput{Record: used, Token: issued.Token, Tool: tool, Payload: payload}); !errors.Is(err, ErrAlreadyUsed) {
		t.Fatalf("Verify used record error = %v, want ErrAlreadyUsed", err)
	}
	now = now.Add(2 * time.Minute)
	if err := manager.Verify(VerifyInput{Record: issued.Record, Token: issued.Token, Tool: tool, Payload: payload}); !errors.Is(err, ErrExpired) {
		t.Fatalf("Verify expired record error = %v, want ErrExpired", err)
	}
}

func TestIssueRejectsToolsThatDoNotRequireConfirmation(t *testing.T) {
	manager := newTestManager(t, time.Now)
	readTool := lookupTool(t, "get_portfolio_summary")

	_, err := manager.Issue(IssueInput{Tool: readTool, Payload: map[string]any{"limit": 1}, TTL: time.Minute})
	if !errors.Is(err, ErrConfirmationNotRequired) {
		t.Fatalf("Issue read-only tool error = %v, want ErrConfirmationNotRequired", err)
	}
}

func newTestManager(t *testing.T, clock func() time.Time) *Manager {
	t.Helper()
	manager, err := NewManager([]byte("test-secret"), WithClock(clock))
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	return manager
}

func lookupTool(t *testing.T, name string) agenttools.ToolDefinition {
	t.Helper()
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry returned error: %v", err)
	}
	tool, ok := registry.Lookup(name)
	if !ok {
		t.Fatalf("%s not found", name)
	}
	return tool
}
