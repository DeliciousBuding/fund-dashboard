package agentops

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/audit"
	"github.com/DeliciousBuding/fund-dashboard/internal/confirmations"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/agentstate"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/sqlitedb"
)

func TestPrepareConfirmationPersistsHashOnlyTokenAndAuditAttempt(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	service := newAgentOpsFixture(t, db, func() time.Time {
		return time.Date(2026, 7, 7, 5, 10, 0, 0, time.UTC)
	})
	payload := map[string]any{
		"fund_code": "AAPL",
		"amount":    100.5,
		"api_key":   "secret",
	}

	prepared, err := service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool:            "add_transaction",
		Role:            agenttools.RoleOperator,
		Caller:          "hermes",
		RequestID:       "req-prepare-1",
		Payload:         payload,
		EnforceReviewed: true,
	})
	if err != nil {
		t.Fatalf("PrepareConfirmation returned error: %v", err)
	}

	if prepared.Token == "" || prepared.ConfirmationID <= 0 || prepared.AuditEventID <= 0 {
		t.Fatalf("prepared = %#v, want token, confirmation id, and audit id", prepared)
	}
	if prepared.Tool != "add_transaction" || prepared.PayloadHash == "" {
		t.Fatalf("prepared = %#v, want add_transaction payload hash", prepared)
	}
	if !prepared.ExpiresAt.Equal(time.Date(2026, 7, 7, 5, 25, 0, 0, time.UTC)) {
		t.Fatalf("ExpiresAt = %s, want registry TTL", prepared.ExpiresAt)
	}

	record, err := service.confirmationRepo.Get(ctx, prepared.ConfirmationID)
	if err != nil {
		t.Fatalf("confirmation Get returned error: %v", err)
	}
	if record == nil {
		t.Fatalf("confirmation record is nil")
	}
	if record.TokenHash == prepared.Token {
		t.Fatalf("stored raw token in confirmation record")
	}
	if record.PayloadHash != prepared.PayloadHash || record.Tool != "add_transaction" {
		t.Fatalf("record = %#v, want persisted tool and payload hash", record)
	}

	event, err := service.auditRepo.Get(ctx, prepared.AuditEventID)
	if err != nil {
		t.Fatalf("audit Get returned error: %v", err)
	}
	if event == nil || event.Caller != "hermes" || event.RequestID != "req-prepare-1" {
		t.Fatalf("audit event = %#v, want caller and request id", event)
	}
	if event.Status != audit.StatusAttempt || event.Tool != "add_transaction" {
		t.Fatalf("audit event = %#v, want attempt for add_transaction", event)
	}
	if event.RedactedArgs["api_key"] != audit.RedactedValue || event.RedactedArgs["fund_code"] != "AAPL" {
		t.Fatalf("RedactedArgs = %#v, want api_key redacted and fund_code preserved", event.RedactedArgs)
	}
}

func TestPrepareConfirmationRejectsDisabledAndReviewRequiredTools(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	service := newAgentOpsFixture(t, db, time.Now)

	_, err := service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool:    "backup_producer",
		Role:    agenttools.RoleOperator,
		Caller:  "hermes",
		Payload: map[string]any{"target": "backup"},
	})
	if !errors.Is(err, ErrToolDisabled) {
		t.Fatalf("disabled prepare error = %v, want ErrToolDisabled", err)
	}

	_, err = service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool:            "add_security",
		Role:            agenttools.RoleOperator,
		Caller:          "hermes",
		Payload:         map[string]any{"code": "AAPL"},
		EnforceReviewed: true,
	})
	if !errors.Is(err, ErrReviewRequired) {
		t.Fatalf("review prepare error = %v, want ErrReviewRequired", err)
	}

	var confirmationsCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_confirmations`).Scan(&confirmationsCount); err != nil {
		t.Fatalf("count confirmations: %v", err)
	}
	if confirmationsCount != 0 {
		t.Fatalf("confirmation rows = %d, want none after rejected prepares", confirmationsCount)
	}
}

func TestPrepareConfirmationRejectsReadToolsAndInsufficientRole(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	service := newAgentOpsFixture(t, db, time.Now)

	_, err := service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool:    "get_portfolio_summary",
		Role:    agenttools.RoleViewer,
		Caller:  "hermes",
		Payload: map[string]any{"portfolio_id": 1},
	})
	if !errors.Is(err, ErrConfirmationNotRequired) {
		t.Fatalf("read prepare error = %v, want ErrConfirmationNotRequired", err)
	}

	_, err = service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool:    "add_transaction",
		Role:    agenttools.RoleViewer,
		Caller:  "hermes",
		Payload: map[string]any{"fund_code": "AAPL"},
	})
	if !errors.Is(err, ErrScopeNotAllowed) {
		t.Fatalf("scope prepare error = %v, want ErrScopeNotAllowed", err)
	}
}

func TestConsumeConfirmationVerifiesMarksUsedAndRecordsResult(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	now := time.Date(2026, 7, 7, 5, 30, 0, 0, time.UTC)
	service := newAgentOpsFixture(t, db, func() time.Time {
		return now
	})
	payload := map[string]any{
		"fund_code": "AAPL",
		"amount":    100.5,
		"api_key":   "secret",
	}

	prepared, err := service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool:            "add_transaction",
		Role:            agenttools.RoleOperator,
		Caller:          "hermes",
		RequestID:       "req-consume-1",
		Payload:         payload,
		EnforceReviewed: true,
	})
	if err != nil {
		t.Fatalf("PrepareConfirmation returned error: %v", err)
	}

	consumed, err := service.ConsumeConfirmation(ctx, ConsumeConfirmationInput{
		Tool:            "add_transaction",
		Role:            agenttools.RoleOperator,
		Caller:          "hermes",
		RequestID:       "req-consume-1",
		ConfirmationID:  prepared.ConfirmationID,
		Token:           prepared.Token,
		Payload:         payload,
		EnforceReviewed: true,
		ResultSummary: map[string]any{
			"status":  "verified",
			"api_key": "secret",
		},
	})
	if err != nil {
		t.Fatalf("ConsumeConfirmation returned error: %v", err)
	}
	if consumed.ConfirmationID != prepared.ConfirmationID || consumed.AuditEventID <= prepared.AuditEventID {
		t.Fatalf("consumed = %#v, want same confirmation and later audit event", consumed)
	}
	if consumed.Tool != "add_transaction" || consumed.PayloadHash != prepared.PayloadHash {
		t.Fatalf("consumed = %#v, want tool and payload hash", consumed)
	}

	record, err := service.confirmationRepo.Get(ctx, prepared.ConfirmationID)
	if err != nil {
		t.Fatalf("confirmation Get returned error: %v", err)
	}
	if record == nil || record.UsedAt == nil {
		t.Fatalf("record = %#v, want used_at set", record)
	}
	if !record.UsedAt.Equal(now) || !consumed.UsedAt.Equal(now) {
		t.Fatalf("used_at record=%v consumed=%v, want %v", record.UsedAt, consumed.UsedAt, now)
	}

	event, err := service.auditRepo.Get(ctx, consumed.AuditEventID)
	if err != nil {
		t.Fatalf("audit Get returned error: %v", err)
	}
	if event == nil || event.Status != audit.StatusResult || event.Tool != "add_transaction" {
		t.Fatalf("audit event = %#v, want result event", event)
	}
	if event.ResultSummary["api_key"] != audit.RedactedValue || event.ResultSummary["status"] != "verified" {
		t.Fatalf("ResultSummary = %#v, want redacted result summary", event.ResultSummary)
	}
}

func TestConsumeConfirmationRejectsReplayAndPayloadMismatchWithoutMarkingUsed(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	now := time.Date(2026, 7, 7, 5, 40, 0, 0, time.UTC)
	service := newAgentOpsFixture(t, db, func() time.Time {
		return now
	})
	payload := map[string]any{"fund_code": "AAPL", "amount": 100.5}
	prepared, err := service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool:            "add_transaction",
		Role:            agenttools.RoleOperator,
		Caller:          "hermes",
		RequestID:       "req-consume-2",
		Payload:         payload,
		EnforceReviewed: true,
	})
	if err != nil {
		t.Fatalf("PrepareConfirmation returned error: %v", err)
	}

	_, err = service.ConsumeConfirmation(ctx, ConsumeConfirmationInput{
		Tool:            "add_transaction",
		Role:            agenttools.RoleOperator,
		Caller:          "hermes",
		RequestID:       "req-consume-2",
		ConfirmationID:  prepared.ConfirmationID,
		Token:           prepared.Token,
		Payload:         map[string]any{"fund_code": "AAPL", "amount": 101},
		EnforceReviewed: true,
	})
	if !errors.Is(err, confirmations.ErrPayloadMismatch) {
		t.Fatalf("payload mismatch error = %v, want ErrPayloadMismatch", err)
	}
	record, err := service.confirmationRepo.Get(ctx, prepared.ConfirmationID)
	if err != nil {
		t.Fatalf("confirmation Get returned error: %v", err)
	}
	if record == nil || record.UsedAt != nil {
		t.Fatalf("record = %#v, want mismatch to leave confirmation unused", record)
	}

	if _, err := service.ConsumeConfirmation(ctx, ConsumeConfirmationInput{
		Tool:            "add_transaction",
		Role:            agenttools.RoleOperator,
		Caller:          "hermes",
		RequestID:       "req-consume-2",
		ConfirmationID:  prepared.ConfirmationID,
		Token:           prepared.Token,
		Payload:         payload,
		EnforceReviewed: true,
	}); err != nil {
		t.Fatalf("first valid consume returned error: %v", err)
	}
	record, err = service.confirmationRepo.Get(ctx, prepared.ConfirmationID)
	if err != nil {
		t.Fatalf("confirmation Get after consume returned error: %v", err)
	}
	if record == nil || record.UsedAt == nil || !record.UsedAt.Equal(now) {
		t.Fatalf("record after consume = %#v, want used_at %v", record, now)
	}
	_, err = service.ConsumeConfirmation(ctx, ConsumeConfirmationInput{
		Tool:            "add_transaction",
		Role:            agenttools.RoleOperator,
		Caller:          "hermes",
		RequestID:       "req-consume-2",
		ConfirmationID:  prepared.ConfirmationID,
		Token:           prepared.Token,
		Payload:         payload,
		EnforceReviewed: true,
	})
	if !errors.Is(err, confirmations.ErrAlreadyUsed) {
		t.Fatalf("replay error = %v, want ErrAlreadyUsed", err)
	}
}

func openAgentOpsFixture(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
		Path: filepath.Join(t.TempDir(), "fund.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite fixture: %v", err)
	}
	confirmationRepo := agentstate.NewConfirmationRepository(db)
	if err := confirmationRepo.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure confirmation schema: %v", err)
	}
	auditRepo := agentstate.NewAuditEventRepository(db)
	if err := auditRepo.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure audit schema: %v", err)
	}
	return db
}

func newAgentOpsFixture(t *testing.T, db *sql.DB, clock func() time.Time) *Service {
	t.Helper()
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry returned error: %v", err)
	}
	manager, err := confirmations.NewManager([]byte("test-secret"), confirmations.WithClock(clock))
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	return NewService(ServiceDeps{
		Registry:         registry,
		Confirmations:    manager,
		ConfirmationRepo: agentstate.NewConfirmationRepository(db),
		AuditRepo:        agentstate.NewAuditEventRepository(db),
		Clock:            clock,
	})
}


func TestClaimConfirmationConcurrentSingleWinner(t *testing.T) {
	ctx := context.Background()
	db := openAgentOpsFixture(t)
	defer db.Close()
	now := time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC)
	service := newAgentOpsFixture(t, db, func() time.Time { return now })
	payload := map[string]any{"fund_code": "AAPL", "amount": 50.0}
	prepared, err := service.PrepareConfirmation(ctx, PrepareConfirmationInput{
		Tool:            "add_transaction",
		Role:            agenttools.RoleOperator,
		Caller:          "concurrent-test",
		RequestID:       "req-claim-race",
		Payload:         payload,
		EnforceReviewed: true,
	})
	if err != nil {
		t.Fatalf("PrepareConfirmation: %v", err)
	}

	const n = 16
	type outcome struct {
		err error
		id  int64
	}
	results := make(chan outcome, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			consumed, err := service.ClaimConfirmation(ctx, ConsumeConfirmationInput{
				Tool:            "add_transaction",
				Role:            agenttools.RoleOperator,
				Caller:          "concurrent-test",
				RequestID:       "req-claim-race",
				ConfirmationID:  prepared.ConfirmationID,
				Token:           prepared.Token,
				Payload:         payload,
				EnforceReviewed: true,
				ResultSummary:   map[string]any{"authorization": "claimed"},
			})
			results <- outcome{err: err, id: consumed.ConfirmationID}
		}()
	}
	wg.Wait()
	close(results)

	var wins, alreadyUsed, other int
	for r := range results {
		switch {
		case r.err == nil:
			wins++
			if r.id != prepared.ConfirmationID {
				t.Fatalf("winner confirmation id = %d, want %d", r.id, prepared.ConfirmationID)
			}
		case errors.Is(r.err, confirmations.ErrAlreadyUsed):
			alreadyUsed++
		default:
			other++
			t.Errorf("unexpected claim error: %v", r.err)
		}
	}
	if wins != 1 {
		t.Fatalf("wins = %d, want exactly 1", wins)
	}
	if alreadyUsed != n-1 {
		t.Fatalf("alreadyUsed = %d, want %d", alreadyUsed, n-1)
	}

	record, err := service.confirmationRepo.Get(ctx, prepared.ConfirmationID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if record == nil || record.UsedAt == nil {
		t.Fatalf("record = %#v, want used_at set after claim race", record)
	}
}
