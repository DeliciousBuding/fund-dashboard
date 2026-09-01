package contracts

import (
	"strings"
	"testing"
)

// The JSON-Schema files under docs/go-backend-rewrite/ were removed with the
// rewrite docs (fbeafd9); the Go validators below are the live contract.

func TestValidateAgentContextPackJSONRejectsMissingDisabledBackupBoundary(t *testing.T) {
	payload := []byte(`{
		"schema_version":"agent-context-pack-v1",
		"generated_at":"2026-07-07T03:35:00Z",
		"decision_boundary":"facts_only",
		"identity":{"portfolio_id":1,"base_currency":"CNY","data_version":"nav:2026-07-06"},
		"portfolio":{"summary":{},"allocation":{},"risk_flags":[]},
		"holdings":[],
		"data_quality":{"overall_score":100,"level":"good","stale_price_count":0,"missing_cost_basis_count":0,"missing_change_pct_count":0,"holdings_coverage_pct":100,"limitations":[]},
		"source_context":{"queries":[],"targets":[],"stored_events_summary":{"total":0,"unread":0,"useful":0,"ignored":0},"recent_events":[]},
		"permissions":{"decision_boundary":"facts_only","read_scope":[],"write_scope":[],"requires_confirmation":[],"disabled_operations":[]},
		"capabilities":[],
		"maintenance":{"recommended_actions":[]},
		"agent_brief":"facts only"
	}`)

	err := ValidateAgentContextPackJSON(payload)
	if err == nil {
		t.Fatalf("ValidateAgentContextPackJSON returned nil, want missing backup_producer error")
	}
	if !strings.Contains(err.Error(), "backup_producer") {
		t.Fatalf("error = %q, want backup_producer", err.Error())
	}
}

func TestValidateToolRegistryJSONRejectsConfirmationWithoutTTL(t *testing.T) {
	// Use a full 44-tool registry so validation reaches the tool rules instead of
	// failing on the minimum-tool-count guard first.
	payload := buildToolRegistryJSON(t, 44, func(_ int, tools []map[string]any) {
		tools[0]["capability"].(map[string]any)["permission"] = "requires_confirmation"
		tools[0]["capability"].(map[string]any)["risk_level"] = "high"
		tools[0]["confirmation"].(map[string]any)["required"] = true
		tools[0]["confirmation"].(map[string]any)["reason"] = "confirm write"
		tools[0]["confirmation"].(map[string]any)["token_ttl_seconds"] = nil
	})

	err := ValidateToolRegistryJSON(payload)
	if err == nil {
		t.Fatalf("ValidateToolRegistryJSON returned nil, want confirmation TTL error")
	}
	if !strings.Contains(err.Error(), "token_ttl_seconds") {
		t.Fatalf("error = %q, want token_ttl_seconds", err.Error())
	}
}

func TestValidateAgentContextPackJSONRejectsTrailingJSON(t *testing.T) {
	payload := []byte(`{
		"schema_version":"agent-context-pack-v1",
		"generated_at":"2026-07-07T03:35:00Z",
		"decision_boundary":"facts_only",
		"identity":{"portfolio_id":1,"base_currency":"CNY","data_version":"nav:2026-07-06"},
		"portfolio":{"summary":{},"allocation":{},"risk_flags":[]},
		"holdings":[],
		"data_quality":{"overall_score":100,"level":"good","stale_price_count":0,"missing_cost_basis_count":0,"missing_change_pct_count":0,"holdings_coverage_pct":100,"limitations":[]},
		"source_context":{"queries":[],"targets":[],"stored_events_summary":{"total":0,"unread":0,"useful":0,"ignored":0},"recent_events":[]},
		"permissions":{"decision_boundary":"facts_only","read_scope":[],"write_scope":[],"requires_confirmation":[],"disabled_operations":["backup_producer"]},
		"capabilities":[],
		"maintenance":{"recommended_actions":[]},
		"agent_brief":"facts only"
	} {}`)

	if err := ValidateAgentContextPackJSON(payload); err == nil {
		t.Fatalf("ValidateAgentContextPackJSON returned nil, want trailing JSON error")
	}
}
