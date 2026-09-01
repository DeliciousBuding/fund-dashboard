package agenttools

import (
	"fmt"
	"sync"
	"testing"
)

func TestRegistryLookupLazyIndexIsConcurrentSafe(t *testing.T) {
	registry := &Registry{
		SchemaVersion: "tool-registry-v1",
		Tools: []ToolDefinition{{
			Name:        "get_portfolio_summary",
			Category:    "portfolio",
			Description: "portfolio summary",
			Capability: ToolCapability{
				Tool:       "get_portfolio_summary",
				Scope:      ScopeRead,
				Permission: PermissionAllowed,
				RiskLevel:  RiskLow,
				UseFor:     "read",
			},
			Audit: AuditPolicy{EventType: "read"},
		}},
	}

	// byName is intentionally nil so the first concurrent Lookup calls all
	// exercise the lazy index build path.
	const goroutines = 64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			tool, ok := registry.Lookup("get_portfolio_summary")
			if !ok || tool.Name != "get_portfolio_summary" {
				t.Errorf("Lookup = %#v, %v; want indexed tool", tool, ok)
			}
		}()
	}
	wg.Wait()
}

func TestRegistryValidateRejectsConfirmationWithoutPermission(t *testing.T) {
	ttl := 900
	tools := make([]ToolDefinition, 0, 44)
	for i := 0; i < 44; i++ {
		name := fmt.Sprintf("tool_%02d", i)
		tool := ToolDefinition{
			Name:        name,
			Category:    "admin",
			Description: "generated tool",
			Capability: ToolCapability{
				Tool:       name,
				Scope:      ScopeRead,
				Permission: PermissionAllowed,
				RiskLevel:  RiskLow,
				UseFor:     "read",
			},
			Audit: AuditPolicy{EventType: "read"},
		}
		if i == 0 {
			tool.Name = "add_transaction"
			tool.Capability.Tool = "add_transaction"
			tool.Capability.Scope = ScopeWrite
			tool.Capability.Permission = PermissionAllowed
			tool.Capability.RiskLevel = RiskHigh
			tool.Confirmation = ConfirmationPolicy{Required: true, TokenTTLSeconds: &ttl}
		}
		tools = append(tools, tool)
	}
	registry := &Registry{SchemaVersion: "tool-registry-v1", Tools: tools}
	if err := registry.Validate(); err == nil {
		t.Fatal("Validate returned nil, want confirmation/permission mismatch error")
	}
}
