package portfolio

import (
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
)

// Why this file exists.
//
// MCP tools/list already refuses to advertise a confirmation-gated tool when the
// deployment never wired the AgentOps confirmation service (internal/mcp/server.go:
// Server.confirmationCompletable). The harness snapshot and the agent-context pack
// are the two other surfaces an agent reads to decide what it can call, and they used
// to advertise the full operator registry unconditionally. On a deployment without
// FUND_AGENT_OPS_ENABLED that told the agent add_transaction exists; calling it
// returned tool_denied: confirmation_service_unavailable. Same drift, two more
// surfaces.
//
// The gated set is derived from the agenttools registry -- the SSOT the MCP guard
// already uses -- instead of being restated here, so the surfaces cannot disagree
// about which tools are gated.

// ConfirmationUnwiredMarker is reported in disabled_operations so a consumer can see
// why the confirmation-gated tools are absent instead of having to infer it. Exported
// because it is part of the harness wire contract: internal/mcp asserts the marker is
// present exactly when tools/list stops advertising those tools.
const ConfirmationUnwiredMarker = "confirmation_gated_tools_unwired"

// confirmationGatedToolNames returns every registry tool whose execution depends on
// the AgentOps confirmation flow. Both conditions matter and they are not redundant:
// mark_source_event is Capability.Permission "allowed" yet Confirmation.Required,
// while check_alerts and generate_report are Permission "requires_confirmation".
//
// Fail-closed: when the registry cannot be loaded every harness tool is reported as
// gated, so a degraded deployment hides tools rather than advertising ones that cannot
// execute. The branch is unreachable in a running server -- internal/httpapi builds
// each MCP server from the same registry and answers 500 when it fails to load.
func confirmationGatedToolNames() map[string]struct{} {
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		all := make(map[string]struct{}, len(availableAgentTools))
		for _, name := range availableAgentTools {
			all[name] = struct{}{}
		}
		return all
	}
	gated := make(map[string]struct{})
	for _, tool := range registry.Tools {
		if tool.Capability.Permission == agenttools.PermissionRequiresConfirmation || tool.Confirmation.Required {
			gated[tool.Name] = struct{}{}
		}
	}
	// prepare_confirmation is permission "allowed" (it is the preparation half of
	// the confirmation boundary, not a gated write itself), but it is useless --
	// and must therefore not be advertised -- when the confirmation service is not
	// wired. Treat it as gated for advertisement purposes.
	gated["prepare_confirmation"] = struct{}{}
	return gated
}

// toolsWithConfirmationAvailability drops confirmation-gated names when the flow is not
// wired. Input order is preserved so consumers that diff the listing stay stable.
func toolsWithConfirmationAvailability(tools []string, confirmationsAvailable bool) []string {
	if confirmationsAvailable {
		return tools
	}
	gated := confirmationGatedToolNames()
	out := make([]string, 0, len(tools))
	for _, name := range tools {
		if _, isGated := gated[name]; isGated {
			continue
		}
		out = append(out, name)
	}
	return out
}

// capabilitiesWithConfirmationAvailability keeps gated capability entries -- they are
// policy: the tool exists in the product and is high risk -- but restates the
// permission as what this deployment can actually do.
func capabilitiesWithConfirmationAvailability(caps []AgentCapability, confirmationsAvailable bool) []AgentCapability {
	if confirmationsAvailable {
		return caps
	}
	gated := confirmationGatedToolNames()
	out := make([]AgentCapability, 0, len(caps))
	for _, capability := range caps {
		if _, isGated := gated[capability.Tool]; isGated {
			downgraded := capability
			downgraded.Permission = string(agenttools.PermissionDisabled)
			out = append(out, downgraded)
			continue
		}
		out = append(out, capability)
	}
	return out
}

// permissionsWithConfirmationAvailability empties the confirmation surface and names the
// reason. Write scopes reachable only through a confirmation disappear; data_refresh
// stays because crawl_nav / recalculate_snapshot / crawl_fund_holdings are maintenance
// tools that need no confirmation.
func permissionsWithConfirmationAvailability(permissions AgentPermissions, confirmationsAvailable bool) AgentPermissions {
	if confirmationsAvailable {
		return permissions
	}
	out := permissions
	// Empty slice, not nil: these are JSON array fields and a null here is the same
	// defect class the empty-portfolio collections had (consumers must not branch on
	// null versus []).
	out.RequiresConfirmation = []string{}
	kept := make([]string, 0, len(permissions.WriteScope))
	for _, scope := range permissions.WriteScope {
		if scope == "data_refresh" {
			kept = append(kept, scope)
		}
	}
	out.WriteScope = kept
	out.DisabledOperations = append(append([]string(nil), permissions.DisabledOperations...), ConfirmationUnwiredMarker)
	return out
}
