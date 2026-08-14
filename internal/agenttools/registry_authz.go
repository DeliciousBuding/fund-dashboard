package agenttools

import (
	"errors"
	"fmt"
	"slices"
)

func (r *Registry) Validate() error {
	if r.SchemaVersion != "tool-registry-v1" {
		return fmt.Errorf("unexpected schema_version %q", r.SchemaVersion)
	}
	if len(r.Tools) < 44 {
		return fmt.Errorf("tool registry has %d tools, want at least 44", len(r.Tools))
	}
	seen := map[string]struct{}{}
	for _, tool := range r.Tools {
		if tool.Name == "" {
			return errors.New("tool registry contains empty tool name")
		}
		if _, ok := seen[tool.Name]; ok {
			return fmt.Errorf("duplicate tool %q", tool.Name)
		}
		seen[tool.Name] = struct{}{}
		if tool.Capability.Tool != tool.Name {
			return fmt.Errorf("tool %q capability.tool = %q", tool.Name, tool.Capability.Tool)
		}
		if tool.Capability.Scope == "" || tool.Capability.Permission == "" || tool.Capability.RiskLevel == "" {
			return fmt.Errorf("tool %q missing capability metadata", tool.Name)
		}
		if tool.Confirmation.Required && tool.Confirmation.TokenTTLSeconds == nil {
			return fmt.Errorf("tool %q requires confirmation but has no token TTL", tool.Name)
		}
		if tool.Capability.Permission == PermissionRequiresConfirmation && !tool.Confirmation.Required {
			return fmt.Errorf("tool %q permission requires confirmation but policy does not", tool.Name)
		}
		if tool.Audit.EventType == "" {
			return fmt.Errorf("tool %q missing audit event_type", tool.Name)
		}
	}
	r.reindex()
	return nil
}

func (r *Registry) Lookup(name string) (ToolDefinition, bool) {
	if r.byName == nil {
		r.reindex()
	}
	tool, ok := r.byName[name]
	return tool, ok
}

type AuthorizeRequest struct {
	Tool            string
	Role            Role
	Confirmed       bool
	EnforceReviewed bool
}

type AuthorizeDecision struct {
	Allowed                bool       `json:"allowed"`
	Reason                 DenyReason `json:"reason,omitempty"`
	RequiresConfirmation   bool       `json:"requires_confirmation"`
	ConfirmationReason     *string    `json:"confirmation_reason,omitempty"`
	ConfirmationTTLSeconds *int       `json:"confirmation_ttl_seconds,omitempty"`
	ReviewRequired         bool       `json:"review_required"`
	PermissionSource       string     `json:"permission_source,omitempty"`
	Tool                   string     `json:"tool"`
	Scope                  Scope      `json:"scope,omitempty"`
	Permission             Permission `json:"permission,omitempty"`
	RiskLevel              RiskLevel  `json:"risk_level,omitempty"`
}

func (r *Registry) Authorize(request AuthorizeRequest) AuthorizeDecision {
	tool, ok := r.Lookup(request.Tool)
	if !ok {
		return AuthorizeDecision{Allowed: false, Reason: DenyUnknownTool, Tool: request.Tool}
	}
	decision := AuthorizeDecision{
		Tool:             tool.Name,
		Scope:            tool.Capability.Scope,
		Permission:       tool.Capability.Permission,
		RiskLevel:        tool.Capability.RiskLevel,
		ReviewRequired:   toolRequiresReview(tool),
		PermissionSource: permissionSource(tool),
	}
	if tool.Capability.Permission == PermissionDisabled || tool.Capability.Scope == ScopeDisabled {
		decision.Reason = DenyDisabled
		return decision
	}
	if request.EnforceReviewed && toolRequiresReview(tool) {
		decision.Reason = DenyReviewRequired
		return decision
	}
	if !roleAllowsScope(request.Role, tool.Capability.Scope) {
		decision.Reason = DenyScope
		return decision
	}
	if tool.Capability.Permission == PermissionRequiresConfirmation && !request.Confirmed {
		decision.RequiresConfirmation = true
		decision.ConfirmationReason = tool.Confirmation.Reason
		decision.ConfirmationTTLSeconds = tool.Confirmation.TokenTTLSeconds
		decision.Reason = DenyConfirmationRequired
		return decision
	}
	decision.Allowed = true
	return decision
}

type RegistrySummary struct {
	SchemaVersion             string             `json:"schema_version"`
	GeneratedAt               string             `json:"generated_at"`
	TotalTools                int                `json:"total_tools"`
	DisabledTools             int                `json:"disabled_tools"`
	ReviewRequiredTools       int                `json:"review_required_tools"`
	ConfirmationRequiredTools int                `json:"confirmation_required_tools"`
	AuditedTools              int                `json:"audited_tools"`
	ByScope                   map[Scope]int      `json:"by_scope"`
	ByPermission              map[Permission]int `json:"by_permission"`
	ByRiskLevel               map[RiskLevel]int  `json:"by_risk_level"`
	ByPermissionSource        map[string]int     `json:"by_permission_source"`
	DisabledBoundaries        []string           `json:"disabled_boundaries"`
	ReviewRequiredByCategory  map[string]int     `json:"review_required_by_category"`
	ConfirmationByCategory    map[string]int     `json:"confirmation_required_by_category"`
	AuditRedactionKeys        []string           `json:"audit_redaction_keys"`
}

func (r *Registry) Summary() RegistrySummary {
	summary := RegistrySummary{
		SchemaVersion:            r.SchemaVersion,
		GeneratedAt:              r.GeneratedAt,
		TotalTools:               len(r.Tools),
		ByScope:                  map[Scope]int{},
		ByPermission:             map[Permission]int{},
		ByRiskLevel:              map[RiskLevel]int{},
		ByPermissionSource:       map[string]int{},
		ReviewRequiredByCategory: map[string]int{},
		ConfirmationByCategory:   map[string]int{},
		AuditRedactionKeys:       defaultRedactArgs(),
	}
	for _, tool := range r.Tools {
		summary.ByScope[tool.Capability.Scope]++
		summary.ByPermission[tool.Capability.Permission]++
		summary.ByRiskLevel[tool.Capability.RiskLevel]++
		source := permissionSource(tool)
		if source == "" {
			source = "runtime"
		}
		summary.ByPermissionSource[source]++
		if tool.Capability.Permission == PermissionDisabled || tool.Capability.Scope == ScopeDisabled {
			summary.DisabledTools++
			summary.DisabledBoundaries = append(summary.DisabledBoundaries, tool.Name)
		}
		if toolRequiresReview(tool) {
			summary.ReviewRequiredTools++
			summary.ReviewRequiredByCategory[tool.Category]++
		}
		if tool.Confirmation.Required {
			summary.ConfirmationRequiredTools++
			summary.ConfirmationByCategory[tool.Category]++
		}
		if tool.Audit.RecordAttempt || tool.Audit.RecordResult {
			summary.AuditedTools++
		}
	}
	return summary
}

func toolRequiresReview(tool ToolDefinition) bool {
	if tool.Migration == nil {
		return false
	}
	return tool.Migration.ReviewRequired || tool.Migration.PermissionSource == "inferred"
}

func permissionSource(tool ToolDefinition) string {
	if tool.Migration == nil {
		return ""
	}
	return tool.Migration.PermissionSource
}

// RoleAllowsScope reports whether role may use tools of the given capability scope.
func RoleAllowsScope(role Role, scope Scope) bool {
	allowed := map[Role][]Scope{
		RoleViewer:     {ScopeRead},
		RoleAnalyst:    {ScopeRead, ScopeExternalContext},
		RoleMaintainer: {ScopeRead, ScopeMaintenance, ScopeExternalContext},
		RoleOperator:   {ScopeRead, ScopeMaintenance, ScopeExternalContext, ScopeWrite},
	}
	return slices.Contains(allowed[role], scope)
}

func roleAllowsScope(role Role, scope Scope) bool {
	return RoleAllowsScope(role, scope)
}

func (r *Registry) reindex() {
	r.byName = map[string]ToolDefinition{}
	for _, tool := range r.Tools {
		r.byName[tool.Name] = tool
	}
}

func disabledBoundaryTools() []ToolDefinition {
	return []ToolDefinition{
		disabledBoundaryTool("broker_trade_execution", "本系统不连接券商执行层"),
		disabledBoundaryTool("cash_transfer", "本系统不执行现金划转"),
		disabledBoundaryTool("backup_producer", "按当前运维边界，备份生产器未启用"),
	}
}

func disabledBoundaryTool(name string, useFor string) ToolDefinition {
	return ToolDefinition{
		Name:        name,
		Category:    "admin",
		Description: useFor,
		InputSchema: map[string]any{"type": "object", "additionalProperties": false},
		OutputEnvelope: OutputEnvelope{
			Kind:     "json",
			JSONRoot: nil,
		},
		Capability: ToolCapability{
			Tool:       name,
			Scope:      ScopeDisabled,
			Permission: PermissionDisabled,
			RiskLevel:  RiskHigh,
			UseFor:     useFor,
		},
		Confirmation: ConfirmationPolicy{Required: false},
		Audit: AuditPolicy{
			EventType:     "disabled",
			RecordAttempt: true,
			RecordResult:  true,
			RedactArgs:    defaultRedactArgs(),
		},
		MCPAnnotations: MCPAnnotations{
			ReadOnlyHint:    false,
			DestructiveHint: true,
			OpenWorldHint:   false,
		},
	}
}

func defaultRedactArgs() []string {
	return []string{"api_key", "token", "cookie", "authorization", "webhook", "password", "secret"}
}
