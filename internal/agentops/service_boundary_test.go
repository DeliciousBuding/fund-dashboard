package agentops

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/confirmations"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/agentstate"
)

func TestPrepareConfirmationRejectsOversizedIdentity(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	service := newAgentOpsFixture(t, db, time.Now)

	oversized := strings.Repeat("x", maxAgentIdentityLength+1)
	for name, input := range map[string]PrepareConfirmationInput{
		"caller": {
			Tool: "add_transaction", Role: agenttools.RoleOperator,
			Caller: oversized, Payload: map[string]any{"fund_code": "AAPL"},
		},
		"request_id": {
			Tool: "add_transaction", Role: agenttools.RoleOperator,
			RequestID: oversized, Payload: map[string]any{"fund_code": "AAPL"},
		},
	} {
		if _, err := service.PrepareConfirmation(ctx, input); !errors.Is(err, ErrIdentityTooLong) {
			t.Fatalf("%s: prepare error = %v, want ErrIdentityTooLong", name, err)
		}
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_confirmations`).Scan(&count); err != nil {
		t.Fatalf("count confirmations: %v", err)
	}
	if count != 0 {
		t.Fatalf("confirmation rows = %d, want none after rejected identity", count)
	}
}

func TestClaimConfirmationRejectsOversizedIdentityWithoutMarkingUsed(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	service := newAgentOpsFixture(t, db, time.Now)

	payload := map[string]any{"fund_code": "AAPL"}
	prepared, err := service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool: "add_transaction", Role: agenttools.RoleOperator,
		Caller: "hermes", Payload: payload,
	})
	if err != nil {
		t.Fatalf("PrepareConfirmation: %v", err)
	}

	_, err = service.ClaimConfirmation(ctx, ConsumeConfirmationInput{
		Tool: "add_transaction", Role: agenttools.RoleOperator,
		Caller:         strings.Repeat("x", maxAgentIdentityLength+1),
		ConfirmationID: prepared.ConfirmationID, Token: prepared.Token,
		Payload: payload,
	})
	if !errors.Is(err, ErrIdentityTooLong) {
		t.Fatalf("claim error = %v, want ErrIdentityTooLong", err)
	}

	record, err := service.confirmationRepo.Get(ctx, prepared.ConfirmationID)
	if err != nil {
		t.Fatalf("Get confirmation: %v", err)
	}
	if record == nil || record.UsedAt != nil {
		t.Fatalf("record = %#v, want unused after rejected claim", record)
	}
}

func TestClaimConfirmationRejectsExpiredTokenWithoutMarkingUsed(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	now := time.Date(2026, 7, 7, 5, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	service := newAgentOpsFixture(t, db, clock)

	payload := map[string]any{"fund_code": "AAPL"}
	prepared, err := service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool: "add_transaction", Role: agenttools.RoleOperator,
		Caller: "hermes", Payload: payload, TTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("PrepareConfirmation: %v", err)
	}

	now = now.Add(2 * time.Minute)
	_, err = service.ClaimConfirmation(ctx, ConsumeConfirmationInput{
		Tool: "add_transaction", Role: agenttools.RoleOperator,
		Caller: "hermes", ConfirmationID: prepared.ConfirmationID,
		Token: prepared.Token, Payload: payload,
	})
	if !errors.Is(err, confirmations.ErrExpired) {
		t.Fatalf("claim error = %v, want ErrExpired", err)
	}

	record, err := service.confirmationRepo.Get(ctx, prepared.ConfirmationID)
	if err != nil {
		t.Fatalf("Get confirmation: %v", err)
	}
	if record == nil || record.UsedAt != nil {
		t.Fatalf("record = %#v, want unused after expired claim", record)
	}
}

func TestPrepareConfirmationMissingAuditStoreLeavesNoRows(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()

	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	manager, err := confirmations.NewManager([]byte("test-secret"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	service := NewService(ServiceDeps{
		Registry:         registry,
		Confirmations:    manager,
		ConfirmationRepo: agentstate.NewConfirmationRepository(db),
	})

	_, err = service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool: "add_transaction", Role: agenttools.RoleOperator,
		Caller: "hermes", Payload: map[string]any{"fund_code": "AAPL"},
	})
	if !errors.Is(err, ErrMissingAuditStore) {
		t.Fatalf("prepare error = %v, want ErrMissingAuditStore", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_confirmations`).Scan(&count); err != nil {
		t.Fatalf("count confirmations: %v", err)
	}
	if count != 0 {
		t.Fatalf("confirmation rows = %d, want none after failed prepare", count)
	}
}

// TestGuardOrderIsPreservedPerEntryPoint pins the deliberate asymmetry between
// the two confirmation entry points after their guards were factored into
// shared helpers. Prepare validates the caller-supplied identity before the
// persistence wiring; claim validates the wiring first. A request that is both
// mis-attributed and hitting a mis-wired service therefore gets a different
// sentinel - and a different HTTP status - on each path. That is existing
// behaviour, not an accident to "tidy": reordering either prelude would change
// the status code a doubly-invalid request sees.
func TestGuardOrderIsPreservedPerEntryPoint(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()

	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	manager, err := confirmations.NewManager([]byte("test-secret"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	// AuditRepo deliberately left unwired.
	service := NewService(ServiceDeps{
		Registry:         registry,
		Confirmations:    manager,
		ConfirmationRepo: agentstate.NewConfirmationRepository(db),
	})
	oversized := strings.Repeat("x", maxAgentIdentityLength+1)

	_, err = service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool: "add_transaction", Role: agenttools.RoleOperator,
		Caller: oversized, Payload: map[string]any{"fund_code": "AAPL"},
	})
	if !errors.Is(err, ErrIdentityTooLong) {
		t.Fatalf("prepare error = %v, want ErrIdentityTooLong (identity is checked before the stores)", err)
	}

	_, err = service.ClaimConfirmation(ctx, ConsumeConfirmationInput{
		Tool: "add_transaction", Role: agenttools.RoleOperator,
		Caller: oversized, ConfirmationID: 1, Token: "token",
		Payload: map[string]any{"fund_code": "AAPL"},
	})
	if !errors.Is(err, ErrMissingAuditStore) {
		t.Fatalf("claim error = %v, want ErrMissingAuditStore (stores are checked before the identity)", err)
	}
}
