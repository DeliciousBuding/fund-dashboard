package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

// newOperatorServerWithoutAgentOps builds the production shape of a deployment whose
// .env never set FUND_AGENT_OPS_ENABLED: app.go does not construct the agentops service,
// httpapi hands NewServer a true nil, and the portfolio service is told the confirmation
// flow is unavailable.
func newOperatorServerWithoutAgentOps(t *testing.T, db *sql.DB) *Server {
	t.Helper()
	portfolio := portfoliosvc.NewService(db)
	admin := adminsvc.NewServiceWithDriver(db, "sqlite")
	server, err := NewServer(ServerDeps{Portfolio: &portfolio, Admin: &admin, Role: agenttools.RoleOperator})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	return server
}

// advertisedHarnessTools calls get_investment_harness_snapshot the way an agent does and
// returns the tool names it advertises in available_agent_tools.
func advertisedHarnessTools(t *testing.T, server *Server) map[string]bool {
	t.Helper()
	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"harness"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_investment_harness_snapshot","arguments":{"portfolio_id":1}}`),
	})
	if resp.Error != nil {
		t.Fatalf("harness tools/call error = %#v", resp.Error)
	}
	decoded := decodeResult(t, resp)
	if decoded["isError"] == true {
		t.Fatalf("harness tools/call isError = true: %#v", decoded["content"])
	}
	structured, ok := decoded["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("harness result has no structuredContent: %#v", decoded)
	}
	raw, ok := structured["available_agent_tools"].([]any)
	if !ok {
		t.Fatalf("available_agent_tools = %#v, want array", structured["available_agent_tools"])
	}
	names := make(map[string]bool, len(raw))
	for _, item := range raw {
		name, ok := item.(string)
		if !ok || name == "" {
			t.Fatalf("available_agent_tools entry = %#v, want non-empty string", item)
		}
		names[name] = true
	}
	return names
}

// TestHarnessSurfaceMatchesToolsList pins the invariant that tools/list established onto
// the harness snapshot. An agent has two in-band ways to learn what this server can do;
// if they disagree the agent will call a tool that cannot succeed. Every deployment shape
// must therefore advertise one identical set on both surfaces.
func TestHarnessSurfaceMatchesToolsList(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()

	cases := []struct {
		name      string
		build     func() *Server
		wantCount int
	}{
		{"analyst static key", func() *Server { return newMCPServer(t, db) }, 26},
		{"operator with AgentOps wired", func() *Server { return newMCPServerWithRole(t, db, agenttools.RoleOperator) }, 44},
		{"operator without AgentOps", func() *Server { return newOperatorServerWithoutAgentOps(t, db) }, 29},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.build()
			listed := listToolNames(t, server)
			advertised := advertisedHarnessTools(t, server)
			if len(listed) != tc.wantCount {
				t.Fatalf("tools/list count = %d, want %d", len(listed), tc.wantCount)
			}
			if len(advertised) != tc.wantCount {
				t.Fatalf("harness available_agent_tools count = %d, want %d", len(advertised), tc.wantCount)
			}
			for name := range listed {
				if !advertised[name] {
					t.Fatalf("tools/list advertises %q but the harness snapshot does not", name)
				}
			}
			for name := range advertised {
				if !listed[name] {
					t.Fatalf("harness snapshot advertises %q but tools/list does not", name)
				}
			}
		})
	}
}

// TestHarnessExplainsUnavailableConfirmations checks the unwired operator surface states
// why the gated tools are gone. Silently omitting them leaves an agent to guess whether
// the tool does not exist, is forbidden for its role, or is broken.
func TestHarnessExplainsUnavailableConfirmations(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()

	server := newOperatorServerWithoutAgentOps(t, db)
	resp := server.Handle(context.Background(), Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"harness"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_investment_harness_snapshot","arguments":{"portfolio_id":1}}`),
	})
	if resp.Error != nil {
		t.Fatalf("harness tools/call error = %#v", resp.Error)
	}
	structured := decodeResult(t, resp)["structuredContent"].(map[string]any)

	permissions, ok := structured["agent_permissions"].(map[string]any)
	if !ok {
		t.Fatalf("agent_permissions = %#v, want object", structured["agent_permissions"])
	}
	switch confirmation := permissions["requires_confirmation"].(type) {
	case nil:
		t.Fatalf("requires_confirmation = null, want an empty JSON array")
	case []any:
		if len(confirmation) != 0 {
			t.Fatalf("requires_confirmation = %#v, want empty when no confirmation service exists", confirmation)
		}
	default:
		t.Fatalf("requires_confirmation = %#v, want array", confirmation)
	}
	disabled, _ := permissions["disabled_operations"].([]any)
	found := false
	for _, item := range disabled {
		if item == portfoliosvc.ConfirmationUnwiredMarker {
			found = true
		}
	}
	if !found {
		t.Fatalf("disabled_operations = %#v, want it to name %q", disabled, portfoliosvc.ConfirmationUnwiredMarker)
	}
	writeScope, _ := permissions["write_scope"].([]any)
	if len(writeScope) != 1 || writeScope[0] != "data_refresh" {
		t.Fatalf("write_scope = %#v, want only data_refresh (maintenance needs no confirmation)", writeScope)
	}

	capabilities, _ := structured["agent_capabilities"].([]any)
	gatedSeen := 0
	for _, item := range capabilities {
		capability, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if capability["tool"] != "add_transaction" {
			continue
		}
		gatedSeen++
		// Policy entry survives, but it must no longer claim a confirmation is enough.
		if capability["permission"] != string(agenttools.PermissionDisabled) {
			t.Fatalf("add_transaction permission = %#v, want disabled", capability["permission"])
		}
	}
	if gatedSeen != 1 {
		t.Fatalf("add_transaction capability seen %d times, want exactly 1", gatedSeen)
	}

	brief, _ := structured["agent_brief"].(string)
	if strings.Contains(brief, "transaction writes require confirmation") {
		t.Fatalf("agent_brief still promises a confirmation flow: %q", brief)
	}
	if !strings.Contains(brief, "not wired") {
		t.Fatalf("agent_brief does not explain the missing writes: %q", brief)
	}
}
