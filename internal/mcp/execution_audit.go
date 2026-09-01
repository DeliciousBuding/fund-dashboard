package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/audit"
)

// ExecutionAuditSink persists sanitized tool execution outcomes. It is an
// explicitly optional server dependency: when nil, execution audit is skipped.
//
// Implementations receive only audit.ExecutionEvent values whose status and
// error category belong to fixed closed sets, so raw error text, SQL state
// codes, file paths, and URLs cannot reach the audit store through this
// channel.
type ExecutionAuditSink interface {
	RecordExecution(ctx context.Context, event audit.ExecutionEvent) error
}

// recordExecution is a best-effort side channel. Sink errors are logged and
// swallowed, and sink panics are contained, so audit failures can never change
// the tool result or abort a tools/call response.
func (s *Server) recordExecution(ctx context.Context, tool string, status audit.ExecutionStatus, category audit.ExecutionErrorCategory, started time.Time) {
	sink := s.executionAudit
	if sink == nil {
		return
	}
	duration := time.Duration(0)
	if !started.IsZero() {
		duration = time.Since(started)
	}
	event := audit.NewExecutionEvent(audit.ExecutionEventInput{
		Tool:          tool,
		Status:        status,
		ErrorCategory: category,
		Duration:      duration,
	})
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("mcp execution audit panic", "tool", tool, "panic", fmt.Sprint(recovered))
			}
		}()
		if err := sink.RecordExecution(ctx, event); err != nil {
			slog.Warn("mcp execution audit failed", "tool", tool, "error", err.Error())
		}
	}()
}

// executionErrorCategory maps a JSON-RPC tool error to a closed, sanitizer-backed
// category. The message itself is discarded: the existing sanitizer is consulted
// only to decide whether an error is a short agent-actionable validation
// passthrough or must collapse to internal, so SQL details, file paths, and
// URLs never reach the audit trail.
func executionErrorCategory(err *Error) audit.ExecutionErrorCategory {
	if err == nil {
		return ""
	}
	switch err.Code {
	case -32601:
		return audit.ExecutionCategoryNotImplemented
	case -32001:
		return audit.ExecutionCategoryDenied
	case -32602:
		// invalid_params messages are server-generated static gates. The
		// sanitizer still runs: if a future gate ever echoes storage/network
		// detail, it collapses to internal instead of validation.
		if sanitizedToolErrorMessage(err.Message) == err.Message {
			return audit.ExecutionCategoryValidation
		}
		return audit.ExecutionCategoryInternal
	case -32000:
		return categoryForToolError(err.Message)
	default:
		return audit.ExecutionCategoryInternal
	}
}

// categoryForToolError classifies a -32000 "tool_error: ..." outcome. Only
// payloads that survive the existing sanitizer unchanged and are not one of the
// fixed server-side outcomes count as validation; everything else collapses to
// internal. The raw payload is never stored.
func categoryForToolError(message string) audit.ExecutionErrorCategory {
	payload := strings.TrimPrefix(message, "tool_error: ")
	if payload == message || payload == "" {
		return audit.ExecutionCategoryInternal
	}
	if sanitizedToolErrorMessage(payload) != payload {
		return audit.ExecutionCategoryInternal
	}
	if fixedInternalToolMessage(payload) {
		return audit.ExecutionCategoryInternal
	}
	return audit.ExecutionCategoryValidation
}

// fixedInternalToolMessage lists the static -32000 payloads that describe
// server-side configuration, cancellation, or sanitization outcomes rather
// than input validation.
func fixedInternalToolMessage(message string) bool {
	switch message {
	case "internal_error",
		"cancelled",
		"admin service is required",
		"admin freshness service is required",
		"nav crawler is not configured",
		"snapshot recalculator is not configured",
		"holdings crawler is not configured":
		return true
	default:
		return false
	}
}
