package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/config"
	"github.com/DeliciousBuding/fund-dashboard/internal/confirmations"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/agentstate"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/sqlitedb"
)

func TestAgentConfirmationRoutesAreOptIn(t *testing.T) {
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test"})

	req := httptest.NewRequest(http.MethodPost, "/api/agent/confirmations/prepare", strings.NewReader(`{}`))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when agentops is not wired", res.Code)
	}
}

func TestAgentConfirmationRoutesPrepareAndConsume(t *testing.T) {
	db := openAgentConfirmationHTTPFixture(t)
	defer db.Close()
	router := NewRouter(
		testCfg(),
		WithAgentOps(newAgentConfirmationHTTPService(t, db, func() time.Time {
			return time.Date(2026, 7, 7, 8, 20, 0, 0, time.UTC)
		})),
	)
	payload := map[string]any{"fund_code": "AAPL", "amount": 100.5, "api_key": "secret"}

	prepared := doJSONRequest(t, router, http.MethodPost, "/api/agent/confirmations/prepare", map[string]any{
		"tool":             "add_transaction",
		"role":             "operator",
		"caller":           "hermes",
		"request_id":       "req-http-confirm-1",
		"payload":          payload,
		"enforce_reviewed": true,
	}, http.StatusCreated)
	if prepared["token"] == "" || prepared["confirmation_id"].(float64) <= 0 {
		t.Fatalf("prepared = %#v, want token and confirmation id", prepared)
	}
	if prepared["decision_boundary"] != "confirmation_only" {
		t.Fatalf("prepared decision_boundary = %v, want confirmation_only", prepared["decision_boundary"])
	}
	if strings.Contains(toJSONString(t, prepared), "secret") {
		t.Fatalf("prepare response leaked secret payload: %s", toJSONString(t, prepared))
	}

	confirmationID := int(prepared["confirmation_id"].(float64))
	consumed := doJSONRequest(t, router, http.MethodPost, "/api/agent/confirmations/"+itoa(confirmationID)+"/consume", map[string]any{
		"tool":             "add_transaction",
		"role":             "operator",
		"caller":           "hermes",
		"request_id":       "req-http-confirm-1",
		"token":            prepared["token"],
		"payload":          payload,
		"enforce_reviewed": true,
		"result_summary": map[string]any{
			"status":  "verified",
			"api_key": "secret",
		},
	}, http.StatusOK)
	if consumed["ok"] != true || consumed["confirmation_id"].(float64) != float64(confirmationID) {
		t.Fatalf("consumed = %#v, want ok response for same confirmation id", consumed)
	}
	if strings.Contains(toJSONString(t, consumed), "secret") {
		t.Fatalf("consume response leaked secret payload: %s", toJSONString(t, consumed))
	}

	replay := doJSONRequest(t, router, http.MethodPost, "/api/agent/confirmations/"+itoa(confirmationID)+"/consume", map[string]any{
		"tool":             "add_transaction",
		"role":             "operator",
		"caller":           "hermes",
		"request_id":       "req-http-confirm-1",
		"token":            prepared["token"],
		"payload":          payload,
		"enforce_reviewed": true,
	}, http.StatusConflict)
	if replay["error"] != confirmations.ErrAlreadyUsed.Error() {
		t.Fatalf("replay = %#v, want already used error", replay)
	}
}

func TestAgentConfirmationRoutesRejectDisabledPrepare(t *testing.T) {
	db := openAgentConfirmationHTTPFixture(t)
	defer db.Close()
	router := NewRouter(
		testCfg(),
		WithAgentOps(newAgentConfirmationHTTPService(t, db, time.Now)),
	)

	rejected := doJSONRequest(t, router, http.MethodPost, "/api/agent/confirmations/prepare", map[string]any{
		"tool":    "backup_producer",
		"role":    "operator",
		"caller":  "hermes",
		"payload": map[string]any{"target": "backup"},
	}, http.StatusForbidden)
	if rejected["error"] != agentops.ErrToolDisabled.Error() {
		t.Fatalf("rejected = %#v, want disabled error", rejected)
	}
}

func openAgentConfirmationHTTPFixture(t *testing.T) *sql.DB {
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

func newAgentConfirmationHTTPService(t *testing.T, db *sql.DB, clock func() time.Time) *agentops.Service {
	t.Helper()
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry returned error: %v", err)
	}
	manager, err := confirmations.NewManager([]byte("test-secret"), confirmations.WithClock(clock))
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	return agentops.NewService(agentops.ServiceDeps{
		Registry:         registry,
		Confirmations:    manager,
		ConfirmationRepo: agentstate.NewConfirmationRepository(db),
		AuditRepo:        agentstate.NewAuditEventRepository(db),
		Clock:            clock,
	})
}

func TestAgentConfirmationErrorStatusMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{agentops.ErrToolDisabled, http.StatusForbidden},
		{agentops.ErrReviewRequired, http.StatusConflict},
		{agentops.ErrScopeNotAllowed, http.StatusForbidden},
		{agentops.ErrConfirmationNotRequired, http.StatusBadRequest},
		{agentops.ErrUnknownConfirmation, http.StatusNotFound},
		{confirmations.ErrAlreadyUsed, http.StatusConflict},
		{confirmations.ErrPayloadMismatch, http.StatusBadRequest},
		{errors.New("other"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		if got := statusForAgentConfirmationError(tc.err); got != tc.want {
			t.Fatalf("statusForAgentConfirmationError(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}

func TestClampAgentConfirmationIdentity(t *testing.T) {
	in := agentConfirmationRequest{RequestID: strings.Repeat("r", 129), Caller: "ok"}
	if err := clampAgentConfirmationIdentity(&in); err == nil || err.Error() != "request_id_too_long" {
		t.Fatalf("got %v, want request_id_too_long", err)
	}
	in = agentConfirmationRequest{RequestID: "ok", Caller: strings.Repeat("c", 129)}
	if err := clampAgentConfirmationIdentity(&in); err == nil || err.Error() != "caller_too_long" {
		t.Fatalf("got %v, want caller_too_long", err)
	}
	in = agentConfirmationRequest{RequestID: "  id  ", Caller: "  hermes  "}
	if err := clampAgentConfirmationIdentity(&in); err != nil {
		t.Fatal(err)
	}
	if in.RequestID != "id" || in.Caller != "hermes" {
		t.Fatalf("%+v", in)
	}
}

