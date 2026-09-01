// Package agentops provides agent-facing confirmation prepare/consume workflows.
// It coordinates the tool registry, confirmation manager, persistence, and audit recording.
package agentops

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/audit"
	"github.com/DeliciousBuding/fund-dashboard/internal/confirmations"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/agentstate"
)

var (
	ErrUnknownTool              = errors.New("unknown tool")
	ErrToolDisabled             = errors.New("tool is disabled")
	ErrReviewRequired           = errors.New("tool requires policy review")
	ErrScopeNotAllowed          = errors.New("role scope not allowed")
	ErrConfirmationNotRequired  = errors.New("tool does not require confirmation")
	ErrUnknownConfirmation      = errors.New("unknown confirmation")
	ErrMissingRegistry          = errors.New("agentops registry is nil")
	ErrMissingConfirmationStore = errors.New("agentops confirmation repository is missing")
	ErrMissingAuditStore        = errors.New("agentops audit repository is missing")
	ErrIdentityTooLong          = errors.New("agent identity field exceeds maximum length")
)

// maxAgentIdentityLength bounds caller/request_id before audit persistence.
// The HTTP surface clamps at the same boundary; this is defense in depth for
// direct/MCP callers so oversized identity strings cannot bloat audit rows.
const maxAgentIdentityLength = 128

type Service struct {
	registry         *agenttools.Registry
	confirmations    *confirmations.Manager
	confirmationRepo agentstate.ConfirmationRepository
	auditRepo        agentstate.AuditEventRepository
	clock            func() time.Time
}

type ServiceDeps struct {
	Registry         *agenttools.Registry
	Confirmations    *confirmations.Manager
	ConfirmationRepo agentstate.ConfirmationRepository
	AuditRepo        agentstate.AuditEventRepository
	Clock            func() time.Time
}

type PrepareConfirmationInput struct {
	Tool            string
	Role            agenttools.Role
	Caller          string
	RequestID       string
	Payload         map[string]any
	EnforceReviewed bool
	TTL             time.Duration
}

type PreparedConfirmation struct {
	ConfirmationID int64     `json:"confirmation_id"`
	AuditEventID   int64     `json:"audit_event_id"`
	Token          string    `json:"token"`
	Tool           string    `json:"tool"`
	PayloadHash    string    `json:"payload_hash"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type ConsumeConfirmationInput struct {
	Tool            string
	Role            agenttools.Role
	Caller          string
	RequestID       string
	ConfirmationID  int64
	Token           string
	Payload         map[string]any
	EnforceReviewed bool
	ResultSummary   map[string]any
}

type ConsumedConfirmation struct {
	ConfirmationID int64     `json:"confirmation_id"`
	AuditEventID   int64     `json:"audit_event_id"`
	Tool           string    `json:"tool"`
	PayloadHash    string    `json:"payload_hash"`
	UsedAt         time.Time `json:"used_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
}

func NewService(deps ServiceDeps) *Service {
	clock := deps.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Service{
		registry:         deps.Registry,
		confirmations:    deps.Confirmations,
		confirmationRepo: deps.ConfirmationRepo,
		auditRepo:        deps.AuditRepo,
		clock:            clock,
	}
}

func (s *Service) PrepareConfirmation(ctx context.Context, input PrepareConfirmationInput) (PreparedConfirmation, error) {
	if s.registry == nil {
		return PreparedConfirmation{}, ErrMissingRegistry
	}
	if s.confirmations == nil {
		return PreparedConfirmation{}, confirmations.ErrEmptySecret
	}
	if len(input.Caller) > maxAgentIdentityLength || len(input.RequestID) > maxAgentIdentityLength {
		return PreparedConfirmation{}, ErrIdentityTooLong
	}
	// Fail fast on missing stores before issuing a token or persisting a row:
	// a later audit failure must not leave an orphan confirmation behind.
	if s.confirmationRepo == (agentstate.ConfirmationRepository{}) {
		return PreparedConfirmation{}, ErrMissingConfirmationStore
	}
	if s.auditRepo == (agentstate.AuditEventRepository{}) {
		return PreparedConfirmation{}, ErrMissingAuditStore
	}

	tool, ok := s.registry.Lookup(input.Tool)
	if !ok {
		return PreparedConfirmation{}, ErrUnknownTool
	}
	decision := s.registry.Authorize(agenttools.AuthorizeRequest{
		Tool:            input.Tool,
		Role:            input.Role,
		Confirmed:       true,
		EnforceReviewed: input.EnforceReviewed,
	})
	if !decision.Allowed {
		return PreparedConfirmation{}, mapDenyReason(decision.Reason)
	}
	if tool.Capability.Permission != agenttools.PermissionRequiresConfirmation {
		return PreparedConfirmation{}, ErrConfirmationNotRequired
	}

	issued, err := s.confirmations.Issue(confirmations.IssueInput{
		Tool:    tool,
		Payload: input.Payload,
		TTL:     input.TTL,
	})
	if err != nil {
		return PreparedConfirmation{}, err
	}
	confirmationID, err := s.confirmationRepo.Save(ctx, issued.Record)
	if err != nil {
		return PreparedConfirmation{}, fmt.Errorf("persist prepared confirmation: %w", err)
	}

	auditID, err := s.auditRepo.Save(ctx, audit.NewAttemptEvent(audit.EventInput{
		RequestID: input.RequestID,
		Caller:    input.Caller,
		Tool:      tool,
		Args:      input.Payload,
	}))
	if err != nil {
		return PreparedConfirmation{}, fmt.Errorf("persist confirmation audit attempt: %w", err)
	}

	return PreparedConfirmation{
		ConfirmationID: confirmationID,
		AuditEventID:   auditID,
		Token:          issued.Token,
		Tool:           issued.Record.Tool,
		PayloadHash:    issued.Record.PayloadHash,
		CreatedAt:      issued.Record.CreatedAt,
		ExpiresAt:      issued.Record.ExpiresAt,
	}, nil
}

// ClaimConfirmation verifies tool/role/token/payload and atomically marks used_at
// (CAS: used_at IS NULL) BEFORE any write side-effect.
//
// Safe under-commit: if the subsequent tool call fails, the confirmation stays burned.
// Callers must prepare a new confirmation rather than risking double-write under concurrent
// tools/call with the same confirmation_id+token. Prefer under-commit over over-commit.
func (s *Service) ClaimConfirmation(ctx context.Context, input ConsumeConfirmationInput) (ConsumedConfirmation, error) {
	if s.registry == nil {
		return ConsumedConfirmation{}, ErrMissingRegistry
	}
	if s.confirmations == nil {
		return ConsumedConfirmation{}, confirmations.ErrEmptySecret
	}
	if s.confirmationRepo == (agentstate.ConfirmationRepository{}) {
		return ConsumedConfirmation{}, ErrMissingConfirmationStore
	}
	if s.auditRepo == (agentstate.AuditEventRepository{}) {
		return ConsumedConfirmation{}, ErrMissingAuditStore
	}
	if len(input.Caller) > maxAgentIdentityLength || len(input.RequestID) > maxAgentIdentityLength {
		return ConsumedConfirmation{}, ErrIdentityTooLong
	}

	tool, ok := s.registry.Lookup(input.Tool)
	if !ok {
		return ConsumedConfirmation{}, ErrUnknownTool
	}
	decision := s.registry.Authorize(agenttools.AuthorizeRequest{
		Tool:            input.Tool,
		Role:            input.Role,
		Confirmed:       true,
		EnforceReviewed: input.EnforceReviewed,
	})
	if !decision.Allowed {
		return ConsumedConfirmation{}, mapDenyReason(decision.Reason)
	}
	if tool.Capability.Permission != agenttools.PermissionRequiresConfirmation {
		return ConsumedConfirmation{}, ErrConfirmationNotRequired
	}

	record, err := s.confirmationRepo.Get(ctx, input.ConfirmationID)
	if err != nil {
		return ConsumedConfirmation{}, fmt.Errorf("load prepared confirmation: %w", err)
	}
	if record == nil {
		return ConsumedConfirmation{}, ErrUnknownConfirmation
	}
	if err := s.confirmations.Verify(confirmations.VerifyInput{
		Record:  *record,
		Token:   input.Token,
		Tool:    tool,
		Payload: input.Payload,
	}); err != nil {
		return ConsumedConfirmation{}, err
	}

	// Atomic single-use claim first so concurrent claimants cannot both proceed to write.
	// Audit failure after mark still leaves token burned (safe under-commit).
	usedAt := s.clock().UTC()
	if err := s.confirmationRepo.MarkUsed(ctx, input.ConfirmationID, usedAt); err != nil {
		// Race-safe: second concurrent consumer loses MarkUsed and surfaces already-used.
		if errors.Is(err, confirmations.ErrAlreadyUsed) {
			return ConsumedConfirmation{}, confirmations.ErrAlreadyUsed
		}
		return ConsumedConfirmation{}, fmt.Errorf("mark confirmation used: %w", err)
	}
	auditID, err := s.auditRepo.Save(ctx, audit.NewResultEvent(audit.EventInput{
		RequestID: input.RequestID,
		Caller:    input.Caller,
		Tool:      tool,
		Result:    input.ResultSummary,
	}))
	if err != nil {
		// Confirmation already claimed; surface soft error so operators re-check audit trail.
		return ConsumedConfirmation{
			ConfirmationID: input.ConfirmationID,
			Tool:           record.Tool,
			PayloadHash:    record.PayloadHash,
			UsedAt:         usedAt,
			ExpiresAt:      record.ExpiresAt,
			CreatedAt:      record.CreatedAt,
		}, fmt.Errorf("persist confirmation audit result: %w", err)
	}

	return ConsumedConfirmation{
		ConfirmationID: input.ConfirmationID,
		AuditEventID:   auditID,
		Tool:           record.Tool,
		PayloadHash:    record.PayloadHash,
		UsedAt:         usedAt,
		ExpiresAt:      record.ExpiresAt,
		CreatedAt:      record.CreatedAt,
	}, nil
}

// ConsumeConfirmation is the HTTP consume path; it claims (verify + MarkUsed CAS) immediately.
// Prefer ClaimConfirmation at write boundaries so naming matches claim-before-execute.
func (s *Service) ConsumeConfirmation(ctx context.Context, input ConsumeConfirmationInput) (ConsumedConfirmation, error) {
	return s.ClaimConfirmation(ctx, input)
}

func mapDenyReason(reason agenttools.DenyReason) error {
	switch reason {
	case agenttools.DenyUnknownTool:
		return ErrUnknownTool
	case agenttools.DenyDisabled:
		return ErrToolDisabled
	case agenttools.DenyReviewRequired:
		return ErrReviewRequired
	case agenttools.DenyScope:
		return ErrScopeNotAllowed
	default:
		return fmt.Errorf("tool authorization denied: %s", reason)
	}
}
