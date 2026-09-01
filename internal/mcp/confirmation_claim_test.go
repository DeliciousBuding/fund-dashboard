package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/confirmations"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/agentstate"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

// TestConcurrentToolsCallSameConfirmationClaimsOnce ensures claim-before-execute:
// concurrent tools/call with the same confirmation_id+token produce exactly one
// side-effect; losers fail with invalid_confirmation before write.
func TestConcurrentToolsCallSameConfirmationClaimsOnce(t *testing.T) {
	ctx := context.Background()
	db := openMCPFixture(t)
	defer db.Close()

	confirmationRepo := agentstate.NewConfirmationRepository(db)
	if err := confirmationRepo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure confirmation schema: %v", err)
	}
	auditRepo := agentstate.NewAuditEventRepository(db)
	if err := auditRepo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure audit schema: %v", err)
	}

	now := time.Date(2026, 7, 19, 3, 10, 0, 0, time.UTC)
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	manager, err := confirmations.NewManager([]byte("test-secret"), confirmations.WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	ops := agentops.NewService(agentops.ServiceDeps{
		Registry:         registry,
		Confirmations:    manager,
		ConfirmationRepo: confirmationRepo,
		AuditRepo:        auditRepo,
		Clock:            func() time.Time { return now },
	})

	payload := map[string]any{
		"order_id":       "MCP-CLAIM-RACE",
		"fund_code":      "aapl",
		"trade_time":     "2026-06-03T09:00:00Z",
		"confirm_date":   "2026-06-05",
		"trade_type":     "用户买入",
		"direction":      "buy",
		"confirm_amount": 198.25,
		"confirm_share":  1.0,
		"fee":            0.0,
	}
	prepared, err := ops.PrepareConfirmation(ctx, agentops.PrepareConfirmationInput{
		Tool:            "add_transaction",
		Role:            agenttools.RoleOperator,
		Caller:          "mcp-race",
		RequestID:       "req-mcp-race",
		Payload:         payload,
		EnforceReviewed: true,
	})
	if err != nil {
		t.Fatalf("PrepareConfirmation: %v", err)
	}

	portfolio := portfoliosvc.NewService(db)
	admin := adminsvc.NewServiceWithDriver(db, "sqlite")
	server, err := NewServer(ServerDeps{
		Registry:  registry,
		Portfolio: &portfolio,
		Admin:     &admin,
		AgentOps:  ops,
		Role:      agenttools.RoleOperator,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	paramsObj := map[string]any{
		"name": "add_transaction",
		"arguments": map[string]any{
			"confirmation_id":    prepared.ConfirmationID,
			"confirmation_token": prepared.Token,
			"order_id":           payload["order_id"],
			"fund_code":          payload["fund_code"],
			"trade_time":         payload["trade_time"],
			"confirm_date":       payload["confirm_date"],
			"trade_type":         payload["trade_type"],
			"direction":          payload["direction"],
			"confirm_amount":     payload["confirm_amount"],
			"confirm_share":      payload["confirm_share"],
			"fee":                payload["fee"],
		},
	}
	rawParams, err := json.Marshal(paramsObj)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	const n = 8
	var okCount atomic.Int64
	var denyCount atomic.Int64
	errs := make(chan string, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			resp := server.Handle(ctx, Request{
				JSONRPC: "2.0",
				ID:      json.RawMessage(fmt.Sprintf(`"%d"`, i)),
				Method:  "tools/call",
				Params:  rawParams,
			})
			if resp.Error == nil {
				okCount.Add(1)
				return
			}
			if strings.Contains(resp.Error.Message, "invalid_confirmation") {
				denyCount.Add(1)
				return
			}
			errs <- resp.Error.Message
		}()
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Fatalf("unexpected tools/call error: %s", msg)
	}

	if okCount.Load() != 1 {
		t.Fatalf("successful side-effects = %d, want exactly 1", okCount.Load())
	}
	if denyCount.Load() != int64(n-1) {
		t.Fatalf("invalid_confirmation denials = %d, want %d", denyCount.Load(), n-1)
	}

	var txCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM transactions WHERE order_id = 'MCP-CLAIM-RACE'`).Scan(&txCount); err != nil {
		t.Fatalf("count race tx: %v", err)
	}
	if txCount != 1 {
		t.Fatalf("transactions for order_id = %d, want exactly 1 side-effect", txCount)
	}

	record, err := confirmationRepo.Get(ctx, prepared.ConfirmationID)
	if err != nil {
		t.Fatalf("Get confirmation: %v", err)
	}
	if record == nil || record.UsedAt == nil {
		t.Fatalf("confirmation %#v, want used_at set", record)
	}
}

func TestBareConfirmedTrueStillRejected(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	server := newMCPServerWithRole(t, db, agenttools.RoleOperator)
	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"bare"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"add_transaction","arguments":{"confirmed":true,"fund_code":"AAPL","confirm_amount":1}}`),
	})
	if resp.Error == nil {
		t.Fatalf("expected denial for bare confirmed=true")
	}
	if !strings.Contains(resp.Error.Message, "confirmation") {
		t.Fatalf("error = %#v, want confirmation gate", resp.Error)
	}
}

type claimCountingConsumer struct {
	claims atomic.Int64
	allow  bool
}

func (c *claimCountingConsumer) ClaimConfirmation(_ context.Context, input agentops.ConsumeConfirmationInput) (agentops.ConsumedConfirmation, error) {
	n := c.claims.Add(1)
	if !c.allow {
		return agentops.ConsumedConfirmation{}, errors.New("denied")
	}
	if n > 1 {
		return agentops.ConsumedConfirmation{}, confirmations.ErrAlreadyUsed
	}
	return agentops.ConsumedConfirmation{ConfirmationID: input.ConfirmationID, Tool: input.Tool}, nil
}

func TestClaimFailureDoesNotExecuteWrite(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	portfolio := portfoliosvc.NewService(db)
	admin := adminsvc.NewServiceWithDriver(db, "sqlite")
	consumer := &claimCountingConsumer{allow: false}
	server, err := NewServer(ServerDeps{Portfolio: &portfolio, Admin: &admin, AgentOps: consumer, Role: agenttools.RoleOperator})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	before := 0
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM transactions`).Scan(&before); err != nil {
		t.Fatalf("count before: %v", err)
	}
	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"deny"`),
		Method:  "tools/call",
		Params: json.RawMessage(`{"name":"add_transaction","arguments":{
			"confirmation_id":1,
			"confirmation_token":"x",
			"order_id":"SHOULD-NOT-INSERT",
			"fund_code":"aapl",
			"trade_time":"2026-06-03T09:00:00Z",
			"confirm_date":"2026-06-05",
			"trade_type":"用户买入",
			"direction":"buy",
			"confirm_amount":1,
			"confirm_share":1,
			"fee":0
		}}`),
	})
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "invalid_confirmation") {
		t.Fatalf("error = %#v, want invalid_confirmation", resp.Error)
	}
	after := 0
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM transactions`).Scan(&after); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if after != before {
		t.Fatalf("tx count changed %d -> %d on failed claim", before, after)
	}
	if consumer.claims.Load() != 1 {
		t.Fatalf("claims = %d, want 1", consumer.claims.Load())
	}
}
