package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestContractSchemaFilesParseAsJSON(t *testing.T) {
	for _, name := range []string{"agent-context-pack.schema.json", "tool-registry.schema.json"} {
		payload, err := os.ReadFile(contractSchemaPath(t, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(payload, &parsed); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if parsed["$schema"] == "" || parsed["title"] == "" {
			t.Fatalf("%s missing $schema/title: %#v", name, parsed)
		}
	}
}

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

	if err := ValidateAgentContextPackJSON(payload); err == nil {
		t.Fatalf("ValidateAgentContextPackJSON returned nil, want missing backup_producer error")
	}
}

func TestValidateToolRegistryJSONRejectsConfirmationWithoutTTL(t *testing.T) {
	payload := []byte(`{
		"schema_version":"tool-registry-v1",
		"generated_at":"2026-07-07T03:35:00Z",
		"tools":[{
			"name":"add_transaction",
			"category":"transactions",
			"description":"write transaction",
			"input_schema":{},
			"output_envelope":{"kind":"json","json_root":null},
			"capability":{"tool":"add_transaction","scope":"write","permission":"requires_confirmation","risk_level":"high","use_for":"write transaction"},
			"confirmation":{"required":true,"reason":"confirm write","token_ttl_seconds":null},
			"audit":{"event_type":"write","record_attempt":true,"record_result":true,"redact_args":["token"]},
			"mcp_annotations":{"read_only_hint":false,"destructive_hint":true,"open_world_hint":false}
		}]
	}`)

	if err := ValidateToolRegistryJSON(payload); err == nil {
		t.Fatalf("ValidateToolRegistryJSON returned nil, want confirmation TTL error")
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

func contractSchemaPath(t *testing.T, name string) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", "..", "docs", "go-backend-rewrite", "contracts", name))
}
