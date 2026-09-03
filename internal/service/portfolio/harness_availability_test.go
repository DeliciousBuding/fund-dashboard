package portfolio

import (
	"context"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
)

// registryGatedNames recomputes the gated set straight from the registry with the same
// two-condition predicate internal/mcp uses, so the assertions below are not restating
// the implementation under test.
func registryGatedNames(t *testing.T) map[string]bool {
	t.Helper()
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	out := map[string]bool{}
	for _, tool := range registry.Tools {
		if tool.Capability.Permission == agenttools.PermissionRequiresConfirmation || tool.Confirmation.Required {
			out[tool.Name] = true
		}
	}
	return out
}

// TestAvailableAgentToolsMatchesRegistrySSOT pins the literal list to the registry.
// availableAgentTools is a hand-ordered projection; without this test it can silently
// drift whenever a tool is added, renamed or retired, and the harness would keep
// advertising a tool that no longer exists.
func TestAvailableAgentToolsMatchesRegistrySSOT(t *testing.T) {
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	want := map[string]bool{}
	for _, tool := range registry.Tools {
		if tool.Capability.Permission == agenttools.PermissionDisabled {
			continue // boundary tools (broker execution, cash transfer, backup producer)
		}
		want[tool.Name] = true
	}

	seen := map[string]bool{}
	for _, name := range availableAgentTools {
		if seen[name] {
			t.Fatalf("availableAgentTools lists %q twice", name)
		}
		seen[name] = true
		if !want[name] {
			t.Fatalf("availableAgentTools advertises %q, which the registry does not carry as an enabled tool", name)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Fatalf("registry tool %q is missing from availableAgentTools", name)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("availableAgentTools has %d entries, registry has %d enabled tools", len(seen), len(want))
	}
}

// TestConfirmationGatedToolNamesCoversEveryGatedRegistryTool guards the derivation itself.
// mark_source_event is the trap: its Capability.Permission is "allowed" and only
// Confirmation.Required marks it gated, so a predicate that looks at Permission alone
// would keep advertising a tool that always fails closed.
func TestConfirmationGatedToolNamesCoversEveryGatedRegistryTool(t *testing.T) {
	gated := confirmationGatedToolNames()
	want := registryGatedNames(t)
	// prepare_confirmation is the one intentional extra: it is permission
	// "allowed" (it prepares the boundary, it is not itself a gated write), but
	// it is still useless without the confirmation service wired.
	if len(gated) != len(want)+1 {
		t.Fatalf("gated count = %d, want %d (+prepare_confirmation)", len(gated), len(want)+1)
	}
	for name := range want {
		if _, ok := gated[name]; !ok {
			t.Fatalf("gated set is missing %q", name)
		}
	}
	if _, ok := gated["prepare_confirmation"]; !ok {
		t.Fatal("prepare_confirmation must be treated as confirmation-service-dependent")
	}
	for name := range gated {
		if name == "prepare_confirmation" {
			continue
		}
		if !want[name] {
			t.Fatalf("gated set has extra %q", name)
		}
	}
	for _, mustBeGated := range []string{"mark_source_event", "check_alerts", "generate_report", "add_transaction"} {
		if _, ok := gated[mustBeGated]; !ok {
			t.Fatalf("%q must be treated as confirmation-gated", mustBeGated)
		}
	}
	// A gated name that is not advertised anywhere would make the filter a silent no-op.
	for name := range gated {
		if !containsString(availableAgentTools, name) {
			t.Fatalf("gated %q is not in availableAgentTools; the filter would never see it", name)
		}
	}
}

func TestToolsWithConfirmationAvailability(t *testing.T) {
	all := append([]string(nil), availableAgentTools...)
	gated := confirmationGatedToolNames()

	// Wired: identity, same slice contents and order.
	if got := toolsWithConfirmationAvailability(all, true); len(got) != len(all) {
		t.Fatalf("wired filter changed the listing: %d -> %d", len(all), len(got))
	}

	unwired := toolsWithConfirmationAvailability(all, false)
	wantLen := len(all) - len(gated)
	if len(unwired) != wantLen {
		t.Fatalf("unwired listing has %d tools, want %d (%d minus %d gated)", len(unwired), wantLen, len(all), len(gated))
	}
	for _, name := range unwired {
		if _, isGated := gated[name]; isGated {
			t.Fatalf("unwired listing still advertises gated %q", name)
		}
	}
	// Maintenance survives: it needs no confirmation and is what still separates an
	// operator from an analyst when AgentOps is off.
	for _, name := range []string{"crawl_nav", "recalculate_snapshot", "crawl_fund_holdings"} {
		if !containsString(unwired, name) {
			t.Fatalf("unwired listing lost maintenance tool %q", name)
		}
	}
	// Order preserved for consumers that diff the listing.
	cursor := 0
	for _, name := range all {
		if _, isGated := gated[name]; isGated {
			continue
		}
		if unwired[cursor] != name {
			t.Fatalf("unwired listing reordered: position %d = %q, want %q", cursor, unwired[cursor], name)
		}
		cursor++
	}
}

func TestCapabilitiesDowngradedNotDropped(t *testing.T) {
	caps := append([]AgentCapability(nil), agentCapabilities...)
	before := len(caps)

	downgraded := capabilitiesWithConfirmationAvailability(caps, false)
	if len(downgraded) != before {
		t.Fatalf("capability count changed %d -> %d; policy entries must survive", before, len(downgraded))
	}
	seen := map[string]string{}
	for _, capability := range downgraded {
		seen[capability.Tool] = capability.Permission
	}
	for name := range registryGatedNames(t) {
		permission, ok := seen[name]
		if !ok {
			continue // not every gated tool is in the harness capability projection
		}
		if permission != string(agenttools.PermissionDisabled) {
			t.Fatalf("gated %q permission = %q, want disabled", name, permission)
		}
	}
	if seen["crawl_nav"] != string(agenttools.PermissionAllowed) {
		t.Fatalf("maintenance crawl_nav permission = %q, want allowed", seen["crawl_nav"])
	}
	// The input must not be mutated: the same slice feeds every later request.
	for _, capability := range agentCapabilities {
		if capability.Tool == "add_transaction" && capability.Permission != string(agenttools.PermissionRequiresConfirmation) {
			t.Fatalf("source capabilities were mutated: add_transaction = %q", capability.Permission)
		}
	}
}

func TestPermissionsWithConfirmationAvailability(t *testing.T) {
	wired := permissionsWithConfirmationAvailability(defaultAgentPermissions(), true)
	if len(wired.RequiresConfirmation) == 0 {
		t.Fatal("wired permissions lost requires_confirmation")
	}
	if containsString(wired.DisabledOperations, ConfirmationUnwiredMarker) {
		t.Fatal("wired permissions must not claim the confirmation flow is unwired")
	}

	unwired := permissionsWithConfirmationAvailability(defaultAgentPermissions(), false)
	if len(unwired.RequiresConfirmation) != 0 {
		t.Fatalf("requires_confirmation = %#v, want empty", unwired.RequiresConfirmation)
	}
	if unwired.RequiresConfirmation == nil {
		t.Fatal("requires_confirmation must serialize as [], not null")
	}
	if len(unwired.WriteScope) != 1 || unwired.WriteScope[0] != "data_refresh" {
		t.Fatalf("write_scope = %#v, want only data_refresh", unwired.WriteScope)
	}
	if !containsString(unwired.DisabledOperations, ConfirmationUnwiredMarker) {
		t.Fatalf("disabled_operations = %#v, want it to name %q", unwired.DisabledOperations, ConfirmationUnwiredMarker)
	}
	// Pre-existing disabled operations must survive the append.
	for _, name := range []string{"broker_trade_execution", "cash_transfer", "backup_producer"} {
		if !containsString(unwired.DisabledOperations, name) {
			t.Fatalf("disabled_operations lost %q", name)
		}
	}
}

// TestServiceFailsClosedWithoutWiringFact covers the default: a Service nobody told
// anything about AgentOps must not promise confirmation-gated tools.
func TestServiceFailsClosedWithoutWiringFact(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()
	seedMixedHarnessData(t, db)
	ensureSourceEventsTable(t, db) // the agent-context pack reads the source event queue

	service := NewService(db)
	op, err := service.GetHarnessSnapshotFor(context.Background(), 1, HarnessAudienceOperator)
	if err != nil {
		t.Fatalf("GetHarnessSnapshotFor operator: %v", err)
	}
	if containsString(op.AvailableAgentTools, "add_transaction") {
		t.Fatalf("unwired operator advertises add_transaction: %#v", op.AvailableAgentTools)
	}
	if !containsString(op.AvailableAgentTools, "crawl_nav") {
		t.Fatalf("unwired operator lost maintenance crawl_nav: %#v", op.AvailableAgentTools)
	}
	if !containsString(op.AgentPermissions.DisabledOperations, ConfirmationUnwiredMarker) {
		t.Fatalf("disabled_operations = %#v, want %q", op.AgentPermissions.DisabledOperations, ConfirmationUnwiredMarker)
	}
	if strings.Contains(op.AgentBrief, "transaction writes require confirmation") {
		t.Fatalf("unwired brief still promises confirmation: %q", op.AgentBrief)
	}

	pack, err := service.GetAgentContextPackFor(context.Background(), AgentContextOptions{PortfolioID: 1}, HarnessAudienceOperator)
	if err != nil {
		t.Fatalf("GetAgentContextPackFor operator: %v", err)
	}
	for _, capability := range pack.Capabilities {
		if capability.Tool == "add_transaction" && capability.Permission != string(agenttools.PermissionDisabled) {
			t.Fatalf("agent-context add_transaction permission = %q, want disabled", capability.Permission)
		}
	}
	if len(pack.Permissions.RequiresConfirmation) != 0 {
		t.Fatalf("agent-context requires_confirmation = %#v, want empty", pack.Permissions.RequiresConfirmation)
	}
	if strings.Contains(pack.AgentBrief, "not wired") == false {
		t.Fatalf("agent-context brief does not explain the gap: %q", pack.AgentBrief)
	}

	// Public audience must be untouched by the wiring fact either way.
	publicBefore, err := service.GetHarnessSnapshotFor(context.Background(), 1, HarnessAudiencePublic)
	if err != nil {
		t.Fatalf("GetHarnessSnapshotFor public: %v", err)
	}
	service.SetConfirmationFlowAvailable(true)
	publicAfter, err := service.GetHarnessSnapshotFor(context.Background(), 1, HarnessAudiencePublic)
	if err != nil {
		t.Fatalf("GetHarnessSnapshotFor public after wiring: %v", err)
	}
	if len(publicBefore.AvailableAgentTools) != len(publicAfter.AvailableAgentTools) {
		t.Fatalf("public surface changed with the wiring fact: %d -> %d", len(publicBefore.AvailableAgentTools), len(publicAfter.AvailableAgentTools))
	}
	if containsString(publicAfter.AgentPermissions.DisabledOperations, ConfirmationUnwiredMarker) {
		t.Fatal("public surface must not leak the deployment wiring detail")
	}

	// And once wired, the operator surface is whole again.
	wired, err := service.GetHarnessSnapshotFor(context.Background(), 1, HarnessAudienceOperator)
	if err != nil {
		t.Fatalf("GetHarnessSnapshotFor operator wired: %v", err)
	}
	if !containsString(wired.AvailableAgentTools, "add_transaction") {
		t.Fatalf("wired operator lost add_transaction: %#v", wired.AvailableAgentTools)
	}
	if containsString(wired.AgentPermissions.DisabledOperations, ConfirmationUnwiredMarker) {
		t.Fatal("wired operator still reports the confirmation flow as unwired")
	}
	if !strings.Contains(wired.AgentBrief, "transaction writes require confirmation") {
		t.Fatalf("wired brief = %q", wired.AgentBrief)
	}
}
