package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/contracts"
)

func TestRegistryLoadsDraftAndValidatesToolMetadata(t *testing.T) {
	registry := loadDraftRegistry(t)

	if registry.SchemaVersion != "tool-registry-v1" {
		t.Fatalf("SchemaVersion = %q, want tool-registry-v1", registry.SchemaVersion)
	}
	if got := len(registry.Tools); got < 44 {
		t.Fatalf("tool count = %d, want at least 44 current MCP tools", got)
	}
	if err := registry.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	summary, ok := registry.Lookup("get_portfolio_summary")
	if !ok {
		t.Fatalf("get_portfolio_summary not found")
	}
	if summary.Capability.Scope != ScopeRead || summary.Capability.Permission != PermissionAllowed {
		t.Fatalf("summary capability = %#v, want read allowed", summary.Capability)
	}

	addTx, ok := registry.Lookup("add_transaction")
	if !ok {
		t.Fatalf("add_transaction not found")
	}
	if addTx.Capability.Permission != PermissionRequiresConfirmation || !addTx.Confirmation.Required {
		t.Fatalf("add_transaction policy = %#v/%#v, want confirmation required", addTx.Capability, addTx.Confirmation)
	}
	if !addTx.Audit.RecordAttempt || !addTx.Audit.RecordResult {
		t.Fatalf("add_transaction audit = %#v, want attempt and result", addTx.Audit)
	}
}

func TestRegistryAuthorizationEnforcesRolesConfirmationAndDisabledBoundaries(t *testing.T) {
	registry := loadDraftRegistry(t)

	viewSummary := registry.Authorize(AuthorizeRequest{Tool: "get_portfolio_summary", Role: RoleViewer})
	if !viewSummary.Allowed {
		t.Fatalf("viewer summary decision = %#v, want allowed", viewSummary)
	}

	viewCrawl := registry.Authorize(AuthorizeRequest{Tool: "crawl_nav", Role: RoleViewer})
	if viewCrawl.Allowed || viewCrawl.Reason != DenyScope {
		t.Fatalf("viewer crawl decision = %#v, want scope denial", viewCrawl)
	}

	operatorWrite := registry.Authorize(AuthorizeRequest{Tool: "add_transaction", Role: RoleOperator})
	if operatorWrite.Allowed || !operatorWrite.RequiresConfirmation || operatorWrite.Reason != DenyConfirmationRequired {
		t.Fatalf("operator write without confirmation = %#v, want confirmation denial", operatorWrite)
	}
	if operatorWrite.ConfirmationTTLSeconds == nil || *operatorWrite.ConfirmationTTLSeconds != 900 {
		t.Fatalf("operator write confirmation TTL = %#v, want 900", operatorWrite.ConfirmationTTLSeconds)
	}
	if operatorWrite.ConfirmationReason == nil || *operatorWrite.ConfirmationReason == "" {
		t.Fatalf("operator write confirmation reason = %#v, want non-empty", operatorWrite.ConfirmationReason)
	}

	confirmedWrite := registry.Authorize(AuthorizeRequest{Tool: "add_transaction", Role: RoleOperator, Confirmed: true})
	if !confirmedWrite.Allowed {
		t.Fatalf("operator confirmed write = %#v, want allowed", confirmedWrite)
	}

	disabled := registry.Authorize(AuthorizeRequest{Tool: "backup_producer", Role: RoleOperator, Confirmed: true})
	if disabled.Allowed || disabled.Reason != DenyDisabled {
		t.Fatalf("backup_producer decision = %#v, want hard disabled denial", disabled)
	}
}

func TestRegistryEnforcementModeRejectsUnreviewedInferredPolicies(t *testing.T) {
	registry := loadDraftRegistry(t)

	inferredWrite := registry.Authorize(AuthorizeRequest{
		Tool:            "add_security",
		Role:            RoleOperator,
		Confirmed:       true,
		EnforceReviewed: true,
	})
	if inferredWrite.Allowed || inferredWrite.Reason != DenyReviewRequired {
		t.Fatalf("inferred write enforcement decision = %#v, want review_required denial", inferredWrite)
	}
	if !inferredWrite.ReviewRequired || inferredWrite.PermissionSource != "inferred" {
		t.Fatalf("inferred write review metadata = %#v, want inferred review_required", inferredWrite)
	}

	declaredWrite := registry.Authorize(AuthorizeRequest{
		Tool:            "add_transaction",
		Role:            RoleOperator,
		Confirmed:       true,
		EnforceReviewed: true,
	})
	if !declaredWrite.Allowed {
		t.Fatalf("declared write enforcement decision = %#v, want allowed", declaredWrite)
	}
	if declaredWrite.ReviewRequired || declaredWrite.PermissionSource != "harness" {
		t.Fatalf("declared write review metadata = %#v, want harness reviewed", declaredWrite)
	}

	disabled := registry.Authorize(AuthorizeRequest{
		Tool:            "backup_producer",
		Role:            RoleOperator,
		Confirmed:       true,
		EnforceReviewed: true,
	})
	if disabled.Allowed || disabled.Reason != DenyDisabled {
		t.Fatalf("disabled enforcement decision = %#v, want disabled before review gate", disabled)
	}
}

func TestRegistrySummaryCountsAgentGovernanceSurface(t *testing.T) {
	registry := loadDraftRegistry(t)

	summary := registry.Summary()
	if summary.TotalTools < 47 {
		t.Fatalf("TotalTools = %d, want current tools plus disabled boundaries", summary.TotalTools)
	}
	if summary.ByScope[ScopeDisabled] != 3 {
		t.Fatalf("disabled scope count = %d, want 3 hard disabled boundaries", summary.ByScope[ScopeDisabled])
	}
	if summary.ReviewRequiredTools < 26 {
		t.Fatalf("ReviewRequiredTools = %d, want inferred rows counted", summary.ReviewRequiredTools)
	}
	if summary.ConfirmationRequiredTools < 14 {
		t.Fatalf("ConfirmationRequiredTools = %d, want confirmation-gated tools counted", summary.ConfirmationRequiredTools)
	}
	if summary.AuditedTools < 19 {
		t.Fatalf("AuditedTools = %d, want audited tools counted", summary.AuditedTools)
	}
}

func TestDefaultRegistryLoadsEmbeddedDraftWithDisabledBoundaries(t *testing.T) {
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry returned error: %v", err)
	}
	if got := len(registry.Tools); got < 47 {
		t.Fatalf("default tool count = %d, want current 44 tools plus disabled boundaries", got)
	}
	if _, ok := registry.Lookup("get_investment_source_brief"); !ok {
		t.Fatalf("default registry missing get_investment_source_brief")
	}
	backup, ok := registry.Lookup("backup_producer")
	if !ok {
		t.Fatalf("default registry missing backup_producer disabled boundary")
	}
	if backup.Capability.Permission != PermissionDisabled || backup.Capability.Scope != ScopeDisabled {
		t.Fatalf("backup_producer capability = %#v, want disabled", backup.Capability)
	}
	payload, err := json.Marshal(registry)
	if err != nil {
		t.Fatalf("marshal default registry: %v", err)
	}
	if err := contracts.ValidateToolRegistryJSON(payload); err != nil {
		t.Fatalf("default registry contract validation failed: %v", err)
	}
}

// loadDraftRegistry loads the canonical embedded registry (the historical
// docs/go-backend-rewrite draft file was removed with the rewrite docs; the
// embedded default_registry.json is the SSOT).
func loadDraftRegistry(t *testing.T) *Registry {
	t.Helper()
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry returned error: %v", err)
	}
	return registry
}

func TestDefaultRegistryInputSchemasAreRealJSONSchema(t *testing.T) {
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	for _, tool := range registry.Tools {
		// Disabled boundary tools are not MCP-advertised; only require type:object.
		if tool.Capability.Permission == PermissionDisabled {
			continue
		}
		schema := tool.InputSchema
		if schema == nil {
			t.Fatalf("tool %s missing input_schema", tool.Name)
		}
		if src, _ := schema["source"].(string); src == "typescript-zod" {
			t.Fatalf("tool %s still has typescript-zod stub schema", tool.Name)
		}
		if typ, _ := schema["type"].(string); typ != "object" {
			t.Fatalf("tool %s input_schema.type = %v, want object", tool.Name, schema["type"])
		}
		if _, ok := schema["properties"]; !ok {
			t.Fatalf("tool %s input_schema missing properties", tool.Name)
		}
	}
	backtest, ok := registry.Lookup("run_backtest")
	if !ok {
		t.Fatal("run_backtest missing")
	}
	props, _ := backtest.InputSchema["properties"].(map[string]any)
	if props == nil {
		t.Fatal("run_backtest properties missing")
	}
	if _, ok := props["code"]; !ok {
		t.Fatal("run_backtest schema must advertise code alias")
	}
	if _, ok := props["fund_code"]; !ok {
		t.Fatal("run_backtest schema must advertise fund_code")
	}
}
