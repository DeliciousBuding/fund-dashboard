// Package contracts provides lightweight Go-native validation for critical agent
// contract invariants: facts-only boundary, disabled backup producer, confirmation TTLs,
// and registry tool metadata.
package contracts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func ValidateAgentContextPackJSON(payload []byte) error {
	var pack agentContextPack
	if err := decodeStrict(payload, &pack); err != nil {
		return fmt.Errorf("decode agent context pack: %w", err)
	}
	if pack.SchemaVersion != "agent-context-pack-v1" {
		return fmt.Errorf("schema_version = %q, want agent-context-pack-v1", pack.SchemaVersion)
	}
	if err := validateDateTime("generated_at", pack.GeneratedAt); err != nil {
		return err
	}
	if pack.DecisionBoundary != "facts_only" {
		return fmt.Errorf("decision_boundary = %q, want facts_only", pack.DecisionBoundary)
	}
	if pack.Identity.PortfolioID < 1 {
		return errors.New("identity.portfolio_id must be >= 1")
	}
	if len(pack.Identity.BaseCurrency) != 3 {
		return fmt.Errorf("identity.base_currency = %q, want 3-letter code", pack.Identity.BaseCurrency)
	}
	if pack.Identity.DataVersion == "" {
		return errors.New("identity.data_version is required")
	}
	if pack.DataQuality.OverallScore < 0 || pack.DataQuality.OverallScore > 100 {
		return fmt.Errorf("data_quality.overall_score = %d, want 0..100", pack.DataQuality.OverallScore)
	}
	if !allowedString(pack.DataQuality.Level, "good", "usable", "limited", "poor") {
		return fmt.Errorf("data_quality.level = %q, want good/usable/limited/poor", pack.DataQuality.Level)
	}
	if pack.DataQuality.HoldingsCoveragePct < 0 || pack.DataQuality.HoldingsCoveragePct > 100 {
		return fmt.Errorf("data_quality.holdings_coverage_pct = %v, want 0..100", pack.DataQuality.HoldingsCoveragePct)
	}
	if pack.SourceContext.StoredEventsSummary.Total < 0 ||
		pack.SourceContext.StoredEventsSummary.Unread < 0 ||
		pack.SourceContext.StoredEventsSummary.Useful < 0 ||
		pack.SourceContext.StoredEventsSummary.Ignored < 0 {
		return errors.New("source_context.stored_events_summary counts must be non-negative")
	}
	if pack.Permissions.DecisionBoundary != "facts_only" {
		return fmt.Errorf("permissions.decision_boundary = %q, want facts_only", pack.Permissions.DecisionBoundary)
	}
	if !contains(pack.Permissions.DisabledOperations, "backup_producer") {
		return errors.New("permissions.disabled_operations must include backup_producer")
	}
	if pack.AgentBrief == "" {
		return errors.New("agent_brief is required")
	}
	return nil
}

func ValidateToolRegistryJSON(payload []byte) error {
	var registry toolRegistry
	if err := decodeStrict(payload, &registry); err != nil {
		return fmt.Errorf("decode tool registry: %w", err)
	}
	if registry.SchemaVersion != "tool-registry-v1" {
		return fmt.Errorf("schema_version = %q, want tool-registry-v1", registry.SchemaVersion)
	}
	if err := validateDateTime("generated_at", registry.GeneratedAt); err != nil {
		return err
	}
	if len(registry.Tools) < 44 {
		return fmt.Errorf("tools length = %d, want at least 44", len(registry.Tools))
	}
	seen := map[string]struct{}{}
	for _, tool := range registry.Tools {
		if err := validateToolDefinition(tool); err != nil {
			return err
		}
		if _, exists := seen[tool.Name]; exists {
			return fmt.Errorf("duplicate tool %q", tool.Name)
		}
		seen[tool.Name] = struct{}{}
	}
	return nil
}

func validateToolDefinition(tool toolDefinition) error {
	if !validSnakeName(tool.Name) {
		return fmt.Errorf("tool name %q is not snake_case", tool.Name)
	}
	if !allowedString(tool.Category, "query", "portfolio", "transactions", "admin", "operations", "securities", "market", "analysis", "report", "dca") {
		return fmt.Errorf("tool %q category = %q", tool.Name, tool.Category)
	}
	if tool.Description == "" {
		return fmt.Errorf("tool %q missing description", tool.Name)
	}
	if tool.OutputEnvelope.Kind != "json" && tool.OutputEnvelope.Kind != "text" && tool.OutputEnvelope.Kind != "stream" {
		return fmt.Errorf("tool %q output_envelope.kind = %q", tool.Name, tool.OutputEnvelope.Kind)
	}
	if tool.Capability.Tool != tool.Name {
		return fmt.Errorf("tool %q capability.tool = %q", tool.Name, tool.Capability.Tool)
	}
	if !allowedString(tool.Capability.Scope, "read", "write", "maintenance", "external_context", "disabled") {
		return fmt.Errorf("tool %q scope = %q", tool.Name, tool.Capability.Scope)
	}
	if !allowedString(tool.Capability.Permission, "allowed", "requires_confirmation", "disabled") {
		return fmt.Errorf("tool %q permission = %q", tool.Name, tool.Capability.Permission)
	}
	if !allowedString(tool.Capability.RiskLevel, "low", "medium", "high") {
		return fmt.Errorf("tool %q risk_level = %q", tool.Name, tool.Capability.RiskLevel)
	}
	if tool.Capability.Permission == "requires_confirmation" && !tool.Confirmation.Required {
		return fmt.Errorf("tool %q requires confirmation but confirmation.required is false", tool.Name)
	}
	if tool.Confirmation.Required {
		if tool.Confirmation.Reason == nil || *tool.Confirmation.Reason == "" {
			return fmt.Errorf("tool %q confirmation reason is required", tool.Name)
		}
		if tool.Confirmation.TokenTTLSeconds == nil || *tool.Confirmation.TokenTTLSeconds < 1 {
			return fmt.Errorf("tool %q confirmation token_ttl_seconds must be >= 1", tool.Name)
		}
	}
	if !allowedString(tool.Audit.EventType, "read", "maintenance", "write", "external_context", "disabled") {
		return fmt.Errorf("tool %q audit.event_type = %q", tool.Name, tool.Audit.EventType)
	}
	if tool.Migration != nil {
		if tool.Migration.SourceFile == "" {
			return fmt.Errorf("tool %q migration.source_file is required", tool.Name)
		}
		if !allowedString(tool.Migration.PermissionSource, "harness", "inferred") {
			return fmt.Errorf("tool %q migration.permission_source = %q", tool.Name, tool.Migration.PermissionSource)
		}
		if tool.Migration.PermissionSource == "inferred" && !tool.Migration.ReviewRequired {
			return fmt.Errorf("tool %q inferred migration must require review", tool.Name)
		}
	}
	return nil
}

func decodeStrict(payload []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("multiple JSON values")
	}
	return nil
}

func validateDateTime(field string, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		return fmt.Errorf("%s = %q is not RFC3339 datetime: %w", field, value, err)
	}
	return nil
}

func allowedString(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validSnakeName(value string) bool {
	if value == "" || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

