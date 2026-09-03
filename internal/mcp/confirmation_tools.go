package mcp

import (
	"context"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
)

// callPrepareConfirmation implements the prepare_confirmation MCP tool: it mints
// a one-time confirmation credential for a confirmation-gated write tool. The
// returned confirmation_id and token must be passed back on the final write call,
// where claimWriteConfirmation verifies and burns them before any side effect.
//
// This mirrors the HTTP /api/agent/confirmations/prepare surface so MCP-only
// clients (ChatGPT connectors) can walk the same two-step write boundary that
// static-key operators use.
func (s *Server) callPrepareConfirmation(ctx context.Context, args map[string]any) (map[string]any, *Error) {
	if s.confirmationPrep == nil {
		return nil, jsonrpcError(-32001, "tool_denied: confirmation_service_unavailable")
	}
	tool := stringArg(args, "tool")
	if tool == "" {
		return nil, jsonrpcError(-32602, "invalid_params: tool is required")
	}
	payload, _ := args["payload"].(map[string]any)
	prepared, err := s.confirmationPrep.PrepareConfirmation(ctx, agentops.PrepareConfirmationInput{
		Tool:            tool,
		Role:            agenttools.RoleOperator,
		Caller:          firstNonEmpty(stringArg(args, "caller"), "mcp"),
		RequestID:       stringArg(args, "request_id"),
		Payload:         payload,
		EnforceReviewed: boolArg(args, "enforce_reviewed", false),
	})
	if err != nil {
		// Do not leak the underlying reason (unknown tool, disabled scope, secret
		// misconfiguration) to the caller; the write tool returns the same stable
		// code when the credential is invalid.
		return nil, jsonrpcError(-32001, "tool_denied: invalid_confirmation")
	}
	return textJSONResult(map[string]any{
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
