package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/confirmations"
	"github.com/go-chi/chi/v5"
)

func registerAgentConfirmationRoutes(r chi.Router, service *agentops.Service) {
	r.Post("/api/agent/confirmations/prepare", handlePrepareAgentConfirmation(service))
	r.Post("/api/agent/confirmations/{id}/consume", handleConsumeAgentConfirmation(service))
}

func handlePrepareAgentConfirmation(service *agentops.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var input agentConfirmationRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if err := clampAgentConfirmationIdentity(&input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		prepared, err := service.PrepareConfirmation(r.Context(), agentops.PrepareConfirmationInput{
			Tool:            input.Tool,
			Role:            agenttools.RoleOperator,
			Caller:          input.Caller,
			RequestID:       input.RequestID,
			Payload:         input.Payload,
			EnforceReviewed: input.EnforceReviewed,
		})
		if err != nil {
			writeAgentConfirmationError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusCreated, map[string]any{
			"decision_boundary": "confirmation_only",
			"confirmation_id":   prepared.ConfirmationID,
			"audit_event_id":    prepared.AuditEventID,
			"token":             prepared.Token,
			"tool":              prepared.Tool,
			"payload_hash":      prepared.PayloadHash,
			"created_at":        prepared.CreatedAt,
			"expires_at":        prepared.ExpiresAt,
		})
	}
}

func handleConsumeAgentConfirmation(service *agentops.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid confirmation id")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var input agentConfirmationRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if err := clampAgentConfirmationIdentity(&input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		consumed, err := service.ConsumeConfirmation(r.Context(), agentops.ConsumeConfirmationInput{
			Tool:            input.Tool,
			Role:            agenttools.RoleOperator,
			Caller:          input.Caller,
			RequestID:       input.RequestID,
			ConfirmationID:  id,
			Token:           input.Token,
			Payload:         input.Payload,
			EnforceReviewed: input.EnforceReviewed,
			ResultSummary:   input.ResultSummary,
		})
		if err != nil {
			writeAgentConfirmationError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"ok":                true,
			"decision_boundary": "confirmation_only",
			"confirmation_id":   consumed.ConfirmationID,
			"audit_event_id":    consumed.AuditEventID,
			"tool":              consumed.Tool,
			"payload_hash":      consumed.PayloadHash,
			"created_at":        consumed.CreatedAt,
			"expires_at":        consumed.ExpiresAt,
			"used_at":           consumed.UsedAt,
		})
	}
}

type agentConfirmationRequest struct {
	Tool            string         `json:"tool"`
	Role            string         `json:"role"`
	Caller          string         `json:"caller"`
	RequestID       string         `json:"request_id"`
	Token           string         `json:"token"`
	Payload         map[string]any `json:"payload"`
	ResultSummary   map[string]any `json:"result_summary"`
	EnforceReviewed bool           `json:"enforce_reviewed"`
}

func statusForAgentConfirmationError(err error) int {
	switch {
	case errors.Is(err, agentops.ErrUnknownTool), errors.Is(err, agentops.ErrUnknownConfirmation):
		return http.StatusNotFound
	case errors.Is(err, agentops.ErrToolDisabled), errors.Is(err, agentops.ErrScopeNotAllowed):
		return http.StatusForbidden
	case errors.Is(err, agentops.ErrReviewRequired), errors.Is(err, confirmations.ErrAlreadyUsed):
		return http.StatusConflict
	case errors.Is(err, agentops.ErrConfirmationNotRequired),
		errors.Is(err, confirmations.ErrPayloadMismatch),
		errors.Is(err, confirmations.ErrTokenMismatch),
		errors.Is(err, confirmations.ErrToolMismatch),
		errors.Is(err, confirmations.ErrExpired):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func writeAgentConfirmationError(w http.ResponseWriter, r *http.Request, err error) {
	status := statusForAgentConfirmationError(err)
	if status >= 500 {
		writeSafeError(w, r, status, err)
		return
	}
	msg := err.Error()
	if !looksSafeClientMessage(msg, strings.ToLower(msg)) {
		writeSafeError(w, r, status, err)
		return
	}
	writeError(w, status, msg)
}

// clampAgentConfirmationIdentity bounds request_id/caller before audit persist (#229).
// Returns stable client codes (#266).
func clampAgentConfirmationIdentity(input *agentConfirmationRequest) error {
	const maxID = 128
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.Caller = strings.TrimSpace(input.Caller)
	if len(input.RequestID) > maxID {
		return errors.New("request_id_too_long")
	}
	if len(input.Caller) > maxID {
		return errors.New("caller_too_long")
	}
	return nil
}
