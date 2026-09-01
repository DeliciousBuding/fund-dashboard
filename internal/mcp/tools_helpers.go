package mcp

import (
	"encoding/json"
	"log/slog"
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
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(encoded)},
		},
	}, nil
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
		if typed > 0 {
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

// internalToolError preserves the underlying error for operator debuggability
// while bounding message length to avoid dumping oversized internals.
func internalToolError(err error) *Error {
	msg := err.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	return jsonrpcError(-32000, "tool_error: "+msg)
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

// RecommendedRefreshCodes merges held stale + missing NAV codes for agent crawl_nav stale_only (#252).
// Exported for admin HTTP crawl-nav parity (#253).
func RecommendedRefreshCodes(report adminsvc.FreshnessReport) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(report.StaleSecurities)+len(report.MissingNAVSecurities))
	for _, item := range report.StaleSecurities {
		code := adminsvc.NormalizeSecurityCode(item.Code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	for _, item := range report.MissingNAVSecurities {
		code := adminsvc.NormalizeSecurityCode(item.Code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}
