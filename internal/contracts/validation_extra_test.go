package contracts

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func validAgentContextPackJSON(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		"schema_version":    "agent-context-pack-v1",
		"generated_at":      "2026-07-07T03:35:00Z",
		"decision_boundary": "facts_only",
		"identity": map[string]any{
			"portfolio_id":  1,
			"base_currency": "CNY",
			"data_version":  "nav:2026-07-06",
		},
		"portfolio": map[string]any{},
		"holdings":  []any{},
		"data_quality": map[string]any{
			"overall_score":            100,
			"level":                    "good",
			"stale_price_count":        0,
			"missing_cost_basis_count": 0,
			"missing_change_pct_count": 0,
			"holdings_coverage_pct":    100,
			"limitations":              []any{},
		},
		"source_context": map[string]any{
			"queries": []any{},
			"targets": []any{},
			"stored_events_summary": map[string]any{
				"total":   0,
				"unread":  0,
				"useful":  0,
				"ignored": 0,
			},
			"recent_events": []any{},
		},
		"permissions": map[string]any{
			"decision_boundary":     "facts_only",
			"read_scope":            []any{},
			"write_scope":           []any{},
			"requires_confirmation": []any{},
			"disabled_operations":   []any{"backup_producer"},
		},
		"capabilities": []any{},
		"maintenance":  map[string]any{},
		"agent_brief":  "facts only",
	}
}

func marshalPack(t *testing.T, pack map[string]any) []byte {
	t.Helper()
	payload, err := json.Marshal(pack)
	if err != nil {
		t.Fatalf("marshal pack: %v", err)
	}
	return payload
}

func TestValidateAgentContextPackJSONValidAndBoundaries(t *testing.T) {
	valid := validAgentContextPackJSON(t)
	if err := ValidateAgentContextPackJSON(marshalPack(t, valid)); err != nil {
		t.Fatalf("valid pack rejected: %v", err)
	}

	// Boundary values accepted at the extremes of every numeric range.
	boundary := validAgentContextPackJSON(t)
	boundary["identity"].(map[string]any)["portfolio_id"] = 1
	boundary["data_quality"].(map[string]any)["overall_score"] = 0
	boundary["data_quality"].(map[string]any)["holdings_coverage_pct"] = 0
	if err := ValidateAgentContextPackJSON(marshalPack(t, boundary)); err != nil {
		t.Fatalf("lower-boundary pack rejected: %v", err)
	}

	boundary = validAgentContextPackJSON(t)
	boundary["data_quality"].(map[string]any)["overall_score"] = 100
	boundary["data_quality"].(map[string]any)["holdings_coverage_pct"] = 100
	if err := ValidateAgentContextPackJSON(marshalPack(t, boundary)); err != nil {
		t.Fatalf("upper-boundary pack rejected: %v", err)
	}
}

func TestValidateAgentContextPackJSONRejectsInvalidBranches(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(map[string]any)
		wantErr string
	}{
		{
			name:    "schema version",
			mutate:  func(p map[string]any) { p["schema_version"] = "v2" },
			wantErr: "schema_version",
		},
		{
			name:    "generated at empty",
			mutate:  func(p map[string]any) { p["generated_at"] = "" },
			wantErr: "generated_at is required",
		},
		{
			name:    "generated at malformed",
			mutate:  func(p map[string]any) { p["generated_at"] = "2026-07-07 03:35:00" },
			wantErr: "not RFC3339",
		},
		{
			name:    "top decision boundary",
			mutate:  func(p map[string]any) { p["decision_boundary"] = "write" },
			wantErr: "facts_only",
		},
		{
			name:    "portfolio id zero",
			mutate:  func(p map[string]any) { p["identity"].(map[string]any)["portfolio_id"] = 0 },
			wantErr: "portfolio_id",
		},
		{
			name:    "portfolio id string type",
			mutate:  func(p map[string]any) { p["identity"].(map[string]any)["portfolio_id"] = "1" },
			wantErr: "identity",
		},
		{
			name:    "base currency too short",
			mutate:  func(p map[string]any) { p["identity"].(map[string]any)["base_currency"] = "CN" },
			wantErr: "base_currency",
		},
		{
			name:    "base currency too long",
			mutate:  func(p map[string]any) { p["identity"].(map[string]any)["base_currency"] = "CNYY" },
			wantErr: "base_currency",
		},
		{
			name:    "base currency number type",
			mutate:  func(p map[string]any) { p["identity"].(map[string]any)["base_currency"] = 156 },
			wantErr: "identity",
		},
		{
			name:    "data version empty",
			mutate:  func(p map[string]any) { p["identity"].(map[string]any)["data_version"] = "" },
			wantErr: "data_version",
		},
		{
			name:    "overall score below",
			mutate:  func(p map[string]any) { p["data_quality"].(map[string]any)["overall_score"] = -1 },
			wantErr: "overall_score",
		},
		{
			name:    "overall score above",
			mutate:  func(p map[string]any) { p["data_quality"].(map[string]any)["overall_score"] = 101 },
			wantErr: "overall_score",
		},
		{
			name:    "overall score float type",
			mutate:  func(p map[string]any) { p["data_quality"].(map[string]any)["overall_score"] = 55.5 },
			wantErr: "data_quality",
		},
		{
			name:    "quality level invalid",
			mutate:  func(p map[string]any) { p["data_quality"].(map[string]any)["level"] = "excellent" },
			wantErr: "level",
		},
		{
			name:    "coverage below",
			mutate:  func(p map[string]any) { p["data_quality"].(map[string]any)["holdings_coverage_pct"] = -0.1 },
			wantErr: "holdings_coverage_pct",
		},
		{
			name:    "coverage above",
			mutate:  func(p map[string]any) { p["data_quality"].(map[string]any)["holdings_coverage_pct"] = 100.1 },
			wantErr: "holdings_coverage_pct",
		},
		{
			name: "stored events negative",
			mutate: func(p map[string]any) {
				p["source_context"].(map[string]any)["stored_events_summary"].(map[string]any)["total"] = -1
			},
			wantErr: "stored_events_summary",
		},
		{
			name: "permissions boundary",
			mutate: func(p map[string]any) {
				p["permissions"].(map[string]any)["decision_boundary"] = "write"
			},
			wantErr: "permissions.decision_boundary",
		},
		{
			name: "disabled operations missing backup",
			mutate: func(p map[string]any) {
				p["permissions"].(map[string]any)["disabled_operations"] = []any{"restore_db"}
			},
			wantErr: "backup_producer",
		},
		{
			name:    "agent brief empty",
			mutate:  func(p map[string]any) { p["agent_brief"] = "" },
			wantErr: "agent_brief",
		},
		{
			name:    "unknown field",
			mutate:  func(p map[string]any) { p["secret_sauce"] = true },
			wantErr: "unknown field",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pack := validAgentContextPackJSON(t)
			tc.mutate(pack)
			err := ValidateAgentContextPackJSON(marshalPack(t, pack))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func validToolMap(t *testing.T, name string) map[string]any {
	t.Helper()
	return map[string]any{
		"name":         name,
		"category":     "query",
		"description":  "query tool",
		"input_schema": map[string]any{},
		"output_envelope": map[string]any{
			"kind":      "json",
			"json_root": nil,
		},
		"capability": map[string]any{
			"tool":       name,
			"scope":      "read",
			"permission": "allowed",
			"risk_level": "low",
			"use_for":    "query",
		},
		"confirmation": map[string]any{
			"required":          false,
			"reason":            nil,
			"token_ttl_seconds": nil,
		},
		"audit": map[string]any{
			"event_type":     "read",
			"record_attempt": true,
			"record_result":  true,
			"redact_args":    []any{},
		},
		"mcp_annotations": map[string]any{
			"read_only_hint":   true,
			"destructive_hint": false,
			"open_world_hint":  false,
		},
	}
}

func buildToolRegistryJSON(t *testing.T, count int, mutate func(i int, tools []map[string]any)) []byte {
	t.Helper()
	tools := make([]map[string]any, count)
	for i := range tools {
		tools[i] = validToolMap(t, toolName(i))
	}
	if mutate != nil {
		mutate(count, tools)
	}
	payload, err := json.Marshal(map[string]any{
		"schema_version": "tool-registry-v1",
		"generated_at":   "2026-07-07T03:35:00Z",
		"tools":          tools,
	})
	if err != nil {
		t.Fatalf("marshal registry: %v", err)
	}
	return payload
}

func toolName(i int) string {
	return fmt.Sprintf("tool_%02d", i)
}

func TestValidateToolRegistryJSONValidRegistryPasses(t *testing.T) {
	// Production LoadJSON enforces the same 44-tool minimum; mirror that contract.
	if err := ValidateToolRegistryJSON(buildToolRegistryJSON(t, 44, nil)); err != nil {
		t.Fatalf("valid 44-tool registry rejected: %v", err)
	}
}

func TestValidateToolRegistryJSONRejectsBranchBugs(t *testing.T) {
	cases := []struct {
		name    string
		payload func(t *testing.T) []byte
		wantErr string
	}{
		{
			name: "wrong schema version",
			payload: func(t *testing.T) []byte {
				payload := buildToolRegistryJSON(t, 44, nil)
				var root map[string]any
				if err := json.Unmarshal(payload, &root); err != nil {
					t.Fatal(err)
				}
				root["schema_version"] = "v9"
				out, err := json.Marshal(root)
				if err != nil {
					t.Fatal(err)
				}
				return out
			},
			wantErr: "schema_version",
		},
		{
			name: "malformed generated at",
			payload: func(t *testing.T) []byte {
				payload := buildToolRegistryJSON(t, 44, nil)
				var root map[string]any
				if err := json.Unmarshal(payload, &root); err != nil {
					t.Fatal(err)
				}
				root["generated_at"] = "yesterday"
				out, err := json.Marshal(root)
				if err != nil {
					t.Fatal(err)
				}
				return out
			},
			wantErr: "not RFC3339",
		},
		{
			name:    "below minimum tool count",
			payload: func(t *testing.T) []byte { return buildToolRegistryJSON(t, 43, nil) },
			wantErr: "at least 44",
		},
		{
			name: "duplicate tool name",
			payload: func(t *testing.T) []byte {
				return buildToolRegistryJSON(t, 44, func(_ int, tools []map[string]any) {
					tools[10]["name"] = "tool_00"
					tools[10]["capability"].(map[string]any)["tool"] = "tool_00"
				})
			},
			wantErr: "duplicate tool",
		},
		{
			name: "invalid tool name",
			payload: func(t *testing.T) []byte {
				return buildToolRegistryJSON(t, 44, func(_ int, tools []map[string]any) {
					tools[0]["name"] = "AddTransaction"
				})
			},
			wantErr: "not snake_case",
		},
		{
			name: "confirmation without TTL among 44 tools",
			payload: func(t *testing.T) []byte {
				return buildToolRegistryJSON(t, 44, func(_ int, tools []map[string]any) {
					tools[7]["capability"].(map[string]any)["permission"] = "requires_confirmation"
					tools[7]["capability"].(map[string]any)["risk_level"] = "high"
					tools[7]["confirmation"].(map[string]any)["required"] = true
					tools[7]["confirmation"].(map[string]any)["reason"] = "confirm write"
					tools[7]["confirmation"].(map[string]any)["token_ttl_seconds"] = nil
				})
			},
			wantErr: "token_ttl_seconds",
		},
		{
			name: "unknown registry field",
			payload: func(t *testing.T) []byte {
				payload := buildToolRegistryJSON(t, 44, nil)
				var root map[string]any
				if err := json.Unmarshal(payload, &root); err != nil {
					t.Fatal(err)
				}
				root["extra"] = true
				out, err := json.Marshal(root)
				if err != nil {
					t.Fatal(err)
				}
				return out
			},
			wantErr: "unknown field",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateToolRegistryJSON(tc.payload(t))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestValidateToolDefinitionBranches(t *testing.T) {
	str := func(s string) *string { return &s }
	ttl := func(n int) *int { return &n }

	valid := func() toolDefinition {
		return toolDefinition{
			Name:           "add_transaction",
			Category:       "transactions",
			Description:    "write transaction",
			InputSchema:    map[string]any{},
			OutputEnvelope: outputEnvelope{Kind: "json"},
			Capability: toolCapability{
				Tool:       "add_transaction",
				Scope:      "write",
				Permission: "allowed",
				RiskLevel:  "high",
				UseFor:     "write transaction",
			},
			Audit:          auditPolicy{EventType: "write"},
			MCPAnnotations: mcpAnnotations{},
		}
	}

	cases := []struct {
		name    string
		mutate  func(*toolDefinition)
		wantErr string
	}{
		{"valid baseline", func(*toolDefinition) {}, ""},
		{"name empty", func(td *toolDefinition) { td.Name = "" }, "not snake_case"},
		{"name leading uppercase", func(td *toolDefinition) { td.Name = "Add" }, "not snake_case"},
		{"name hyphen", func(td *toolDefinition) { td.Name = "add-transaction" }, "not snake_case"},
		{"name digits ok", func(td *toolDefinition) { td.Name = "add_2_transactions"; td.Capability.Tool = td.Name }, ""},
		{"category empty", func(td *toolDefinition) { td.Category = "" }, "category"},
		{"category valid dca", func(td *toolDefinition) { td.Category = "dca" }, ""},
		{"description empty", func(td *toolDefinition) { td.Description = "" }, "description"},
		{"output kind invalid", func(td *toolDefinition) { td.OutputEnvelope.Kind = "xml" }, "output_envelope.kind"},
		{"output kind text ok", func(td *toolDefinition) { td.OutputEnvelope.Kind = "text" }, ""},
		{"output kind stream ok", func(td *toolDefinition) { td.OutputEnvelope.Kind = "stream" }, ""},
		{"capability tool mismatch", func(td *toolDefinition) { td.Capability.Tool = "other_tool" }, "capability.tool"},
		{"scope invalid", func(td *toolDefinition) { td.Capability.Scope = "sudo" }, "scope"},
		{"scope maintenance ok", func(td *toolDefinition) { td.Capability.Scope = "maintenance" }, ""},
		{"scope disabled ok", func(td *toolDefinition) { td.Capability.Scope = "disabled" }, ""},
		{"permission invalid", func(td *toolDefinition) { td.Capability.Permission = "auto" }, "permission"},
		{"risk level invalid", func(td *toolDefinition) { td.Capability.RiskLevel = "extreme" }, "risk_level"},
		{"requires confirmation but not required", func(td *toolDefinition) { td.Capability.Permission = "requires_confirmation" }, "confirmation.required"},
		{"confirmation reason missing", func(td *toolDefinition) {
			td.Capability.Permission = "requires_confirmation"
			td.Confirmation.Required = true
			td.Confirmation.TokenTTLSeconds = ttl(300)
		}, "reason"},
		{"confirmation reason empty", func(td *toolDefinition) {
			td.Capability.Permission = "requires_confirmation"
			td.Confirmation.Required = true
			td.Confirmation.Reason = str("")
			td.Confirmation.TokenTTLSeconds = ttl(300)
		}, "reason"},
		{"confirmation ttl missing", func(td *toolDefinition) {
			td.Capability.Permission = "requires_confirmation"
			td.Confirmation.Required = true
			td.Confirmation.Reason = str("confirm")
		}, "token_ttl_seconds"},
		{"confirmation ttl zero", func(td *toolDefinition) {
			td.Capability.Permission = "requires_confirmation"
			td.Confirmation.Required = true
			td.Confirmation.Reason = str("confirm")
			td.Confirmation.TokenTTLSeconds = ttl(0)
		}, "token_ttl_seconds"},
		{"confirmation ttl negative", func(td *toolDefinition) {
			td.Capability.Permission = "requires_confirmation"
			td.Confirmation.Required = true
			td.Confirmation.Reason = str("confirm")
			td.Confirmation.TokenTTLSeconds = ttl(-1)
		}, "token_ttl_seconds"},
		{"confirmation valid", func(td *toolDefinition) {
			td.Capability.Permission = "requires_confirmation"
			td.Confirmation.Required = true
			td.Confirmation.Reason = str("confirm")
			td.Confirmation.TokenTTLSeconds = ttl(300)
		}, ""},
		{"audit event type invalid", func(td *toolDefinition) { td.Audit.EventType = "hack" }, "audit.event_type"},
		{"audit event type maintenance ok", func(td *toolDefinition) { td.Audit.EventType = "maintenance" }, ""},
		{"migration source file missing", func(td *toolDefinition) {
			td.Migration = &migrationMetadata{PermissionSource: "harness"}
		}, "migration.source_file"},
		{"migration permission source invalid", func(td *toolDefinition) {
			td.Migration = &migrationMetadata{SourceFile: "a.go", PermissionSource: "guess"}
		}, "permission_source"},
		{"inferred migration without review", func(td *toolDefinition) {
			td.Migration = &migrationMetadata{SourceFile: "a.go", PermissionSource: "inferred"}
		}, "require review"},
		{"inferred migration with review ok", func(td *toolDefinition) {
			td.Migration = &migrationMetadata{SourceFile: "a.go", PermissionSource: "inferred", ReviewRequired: true}
		}, ""},
		{"harness migration ok", func(td *toolDefinition) {
			td.Migration = &migrationMetadata{SourceFile: "a.go", PermissionSource: "harness"}
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td := valid()
			tc.mutate(&td)
			err := validateToolDefinition(td)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid tool, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestValidSnakeName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"add_transaction", true},
		{"a", true},
		{"a1_b2", true},
		{"", false},
		{"Add", false},
		{"_private", false},
		{"1tool", false},
		{"add-transaction", false},
		{"add transaction", false},
		{"add.transaction", false},
	}
	for _, tc := range cases {
		if got := validSnakeName(tc.in); got != tc.want {
			t.Fatalf("validSnakeName(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestValidateDateTime(t *testing.T) {
	if err := validateDateTime("generated_at", "2026-07-07T03:35:00Z"); err != nil {
		t.Fatalf("RFC3339 rejected: %v", err)
	}
	if err := validateDateTime("generated_at", "2026-07-07T03:35:00.123456789+08:00"); err != nil {
		t.Fatalf("RFC3339Nano rejected: %v", err)
	}
	if err := validateDateTime("generated_at", ""); err == nil || !strings.Contains(err.Error(), "is required") {
		t.Fatalf("empty datetime: %v", err)
	}
	if err := validateDateTime("generated_at", "2026-07-07"); err == nil || !strings.Contains(err.Error(), "not RFC3339") {
		t.Fatalf("date-only datetime: %v", err)
	}
}
