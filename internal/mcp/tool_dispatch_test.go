package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

// toolDispatch is now the one list behind both surfaces: tools/list derives its
// advertisement filter from its key set and tools/call dispatches through it.
// These tests pin the directions that were previously unguarded. A name in the
// table that is not a registry tool is unreachable dead weight, and a tool an
// operator surface advertises but cannot execute is the exact failure the merge
// exists to prevent - an agent would call it and get tool_not_implemented.

func TestToolDispatchEntriesAreRegistryTools(t *testing.T) {
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	for name, handler := range toolDispatch {
		if handler == nil {
			t.Errorf("toolDispatch[%q] has a nil handler", name)
		}
		if _, ok := registry.Lookup(name); !ok {
			t.Errorf("toolDispatch[%q] is not a registry tool; the entry can never be reached", name)
		}
	}
	if implemented := implementedMCPTools(); len(implemented) != len(toolDispatch) {
		t.Fatalf("implementedMCPTools reports %d names, toolDispatch has %d", len(implemented), len(toolDispatch))
	}
}

func TestEveryAdvertisedToolIsDispatchable(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()

	surfaces := []struct {
		name   string
		server *Server
	}{
		{"operator", newMCPServerWithRole(t, db, agenttools.RoleOperator)},
		{"analyst", newMCPServer(t, db)},
	}
	for _, surface := range surfaces {
		advertised := listToolNames(t, surface.server)
		if len(advertised) == 0 {
			t.Fatalf("%s: tools/list advertised nothing", surface.name)
		}
		for name := range advertised {
			if _, ok := toolDispatch[name]; !ok {
				t.Errorf("%s: tools/list advertises %q but toolDispatch cannot execute it", surface.name, name)
			}
		}
	}
}

// TestAllowedRegistryToolWithoutDispatchFailsClosed pins the safety net under
// the dispatch table: a tool the registry allows but no handler implements must
// answer -32601 tool_not_implemented rather than panicking on a missing map
// entry or falling through to a zero result.
func TestAllowedRegistryToolWithoutDispatchFailsClosed(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()

	const probe = "get_unimplemented_probe"
	registry := &agenttools.Registry{
		SchemaVersion: "tool-registry-v1",
		Tools: []agenttools.ToolDefinition{
			{
				Name:        probe,
				Category:    "query",
				Description: "探针工具：注册表允许但没有实现",
				Capability: agenttools.ToolCapability{
					Tool:       probe,
					Scope:      agenttools.ScopeRead,
					Permission: agenttools.PermissionAllowed,
					RiskLevel:  agenttools.RiskLow,
				},
			},
		},
	}
	if _, ok := toolDispatch[probe]; ok {
		t.Fatalf("%q must not exist in toolDispatch for this test to mean anything", probe)
	}
	portfolio := portfoliosvc.NewService(db)
	server, err := NewServer(ServerDeps{Registry: registry, Portfolio: &portfolio, Role: agenttools.RoleAnalyst})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"probe"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_unimplemented_probe","arguments":{}}`),
	})
	if resp.Error == nil {
		t.Fatalf("response = %#v, want a tool_not_implemented error", resp)
	}
	if resp.Error.Code != -32601 || resp.Error.Message != "tool_not_implemented: "+probe {
		t.Fatalf("error = %#v, want -32601 tool_not_implemented: %s", resp.Error, probe)
	}
}
