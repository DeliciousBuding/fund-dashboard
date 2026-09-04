package mcp

import (
	"encoding/json"
	"log/slog"
	"math"
	"strings"
	"time"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

func staleNAVSecurities(items []adminsvc.StaleSecurity) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"code":     item.Code,
			"last_nav": item.LastNAV,
		})
	}
	return out
}

func mcpHoldings(signals []portfoliosvc.HarnessHoldingSignal) []map[string]any {
	holdings := make([]map[string]any, 0, len(signals))
	for _, signal := range signals {
		holdings = append(holdings, map[string]any{
			"code":          signal.Code,
			"name":          signal.Name,
			"shares":        signal.HeldShares,
			"security_type": signal.SecurityType,
			"market":        signal.Market,
			"value":         signal.CurrentValue,
			"pnl_pct":       signal.DeviationPct,
			"nav":           signal.LatestNAV,
		})
	}
	return holdings
}

func mcpSourceEvents(events []portfoliosvc.SourceEvent) []map[string]any {
	out := make([]map[string]any, 0, len(events))
	for _, event := range events {
		out = append(out, map[string]any{
			"id":                    event.ID,
			"title":                 event.Title,
			"url":                   event.URL,
			"source":                event.Source,
			"snippet":               event.Snippet,
			"query":                 event.Query,
			"related_security_code": event.RelatedSecurityCode,
			"related_security_name": event.RelatedSecurityName,
			"is_read":               event.IsRead != 0,
			"is_useful":             event.IsUseful != 0,
			"fetched_at":            event.FetchedAt,
			"created_at":            event.CreatedAt,
		})
	}
	return out
}

// maxToolResultBytes soft-caps MCP tool JSON text payload (#240).
const maxToolResultBytes = 1 << 20 // 1 MiB

func textJSONResult(payload any) (map[string]any, *Error) {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, jsonrpcError(-32000, "encode_tool_result_failed")
	}
	if len(encoded) > maxToolResultBytes {
		slog.Error("mcp tool result too large",
			"bytes", len(encoded),
			"max", maxToolResultBytes,
		)
		return nil, jsonrpcError(-32000, "tool_result_too_large")
	}
	result := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(encoded)},
		},
		"isError": false,
	}
	// structuredContent carries the same value as a JSON object so a client can
	// bind it to a schema instead of re-parsing the text part. Remote MCP clients
	// (ChatGPT connectors, deep research) read it; it is purely additive, so
	// consumers that only look at content are unaffected.
	result["structuredContent"] = asJSONObject(encoded)
	return result, nil
}

// asJSONObject decodes an encoded payload into a JSON object, nesting anything
// that is not already an object under "result". The MCP spec requires
// structuredContent to be an object, and a null there is indistinguishable from
// "no result", so this never returns nil.
func asJSONObject(encoded []byte) map[string]any {
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err == nil && object != nil {
		return object
	}
	return map[string]any{"result": json.RawMessage(encoded)}
}

func intArg(args map[string]any, key string, fallback int) int {
	value, ok := args[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case float64:
		// Guard the float64->int conversion: out-of-range values have an
		// implementation-defined result and could collapse to a bogus id.
		if typed > 0 && typed <= math.MaxInt {
			return int(typed)
		}
	case json.Number:
		if parsed, err := typed.Int64(); err == nil && parsed > 0 {
			return int(parsed)
		}
	}
	return fallback
}

// intArgMax clamps positive int args to max (#214).
func intArgMax(args map[string]any, key string, fallback, max int) int {
	v := intArg(args, key, fallback)
	if v <= 0 {
		return fallback
	}
	if max > 0 && v > max {
		return max
	}
	return v
}

func floatArg(args map[string]any, key string, fallback float64) float64 {
	value, ok := args[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return typed
		}
	case int:
		if typed > 0 {
			return float64(typed)
		}
	case json.Number:
		if parsed, err := typed.Float64(); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

// floatArgMax clamps positive float args to max (#215).
func floatArgMax(args map[string]any, key string, fallback, max float64) float64 {
	v := floatArg(args, key, fallback)
	if v <= 0 {
		return fallback
	}
	if max > 0 && v > max {
		return max
	}
	return v
}

func stringArg(args map[string]any, key string) string {
	value, ok := args[key]
	if !ok {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func boolArg(args map[string]any, key string, fallback bool) bool {
	value, ok := args[key]
	if !ok {
		return fallback
	}
	if typed, ok := value.(bool); ok {
		return typed
	}
	return fallback
}

func stringSliceArg(args map[string]any, key string) []string {
	value, ok := args[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
}

// integerFlag parses an integer argument where 0 is a meaningful value (for
// example a 0/1 toggle). It differs from intArg, which treats any non-positive
// value as absent. It reports false for non-integer or out-of-range values.
func integerFlag(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		if typed >= math.MinInt && typed <= math.MaxInt {
			return int(typed), true
		}
	case float64:
		if typed >= math.MinInt && typed <= math.MaxInt && typed == math.Trunc(typed) {
			return int(typed), true
		}
	case json.Number:
		if parsed, err := typed.Int64(); err == nil && parsed >= math.MinInt && parsed <= math.MaxInt {
			return int(parsed), true
		}
	}
	return 0, false
}

func dateOnlyStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := *value
	if len(trimmed) > 10 {
		trimmed = trimmed[:10]
	}
	return &trimmed
}

func jsonrpcError(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

// internalToolError converts a service error into a JSON-RPC tool error.
// The full error is logged server-side; the caller only receives a short,
// sanitized message. PUBLIC_MCP_KEY holders (Analyst) are authenticated but
// must not receive SQL/driver/network internals, so technical errors degrade
// to a stable "internal_error" (parity with httpapi.writeSafeError).
func internalToolError(err error) *Error {
	raw := err.Error()
	msg := sanitizedToolErrorMessage(raw)
	if msg != raw {
		slog.Error("mcp tool internal error", "error", raw)
	}
	return jsonrpcError(-32000, "tool_error: "+msg)
}

// sanitizedToolErrorMessage passes short, agent-actionable validation
// messages through and redacts anything that looks like storage/driver or
// network internals.
func sanitizedToolErrorMessage(msg string) string {
	m := strings.TrimSpace(msg)
	if m == "" || len(m) > 120 {
		return "internal_error"
	}
	low := strings.ToLower(m)
	for _, marker := range []string{
		"sql:", "pq:", "sqlite", "postgres", "pgx", "sqlx", "driver:", "stack", "panic",
		"no such table", "no such column", "no such file", "no such host",
		"does not exist", "sqlstate", "syntax error", "constraint", "database",
		"connection refused", "connection reset", "dial ", "tls:", "connect:",
		"unmarshal", "certificate", "open ", ":\\", "://",
	} {
		if strings.Contains(low, marker) {
			return "internal_error"
		}
	}
	if strings.ContainsAny(m, "{}[]") {
		return "internal_error"
	}
	return m
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

// RecommendedRefreshCodes is the MCP-facing alias for the shared held stale +
// missing NAV code list used by agent crawl_nav stale_only (#252) and by admin
// HTTP crawl-nav parity (#253). The selection rule itself lives in
// adminsvc.RecommendedRefreshCodes so the MCP tool, the REST endpoint and the
// scheduled job can no longer disagree about which securities need
// maintenance.
func RecommendedRefreshCodes(report adminsvc.FreshnessReport) []string {
	return adminsvc.RecommendedRefreshCodes(report)
}
