// Package agenttools manages the Go-native tool registry with 44 MCP tools.
// It provides typed loading, validation, authorization checks, and a summary endpoint
// for agent-facing tool discovery and governance.
package agenttools

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
)

//go:embed default_registry.json
var defaultRegistryFS embed.FS

type Scope string
type Permission string
type RiskLevel string
type Role string
type DenyReason string

const (
	ScopeRead            Scope = "read"
	ScopeWrite           Scope = "write"
	ScopeMaintenance     Scope = "maintenance"
	ScopeExternalContext Scope = "external_context"
	ScopeDisabled        Scope = "disabled"

	PermissionAllowed              Permission = "allowed"
	PermissionRequiresConfirmation Permission = "requires_confirmation"
	PermissionDisabled             Permission = "disabled"

	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"

	RoleViewer     Role = "viewer"
	RoleAnalyst    Role = "analyst"
	RoleMaintainer Role = "maintainer"
	RoleOperator   Role = "operator"

	DenyUnknownTool          DenyReason = "unknown_tool"
	DenyDisabled             DenyReason = "disabled"
	DenyReviewRequired       DenyReason = "review_required"
	DenyScope                DenyReason = "scope_not_allowed"
	DenyConfirmationRequired DenyReason = "confirmation_required"
)

type Registry struct {
	SchemaVersion string           `json:"schema_version"`
	GeneratedAt   string           `json:"generated_at"`
	Tools         []ToolDefinition `json:"tools"`

	byName map[string]ToolDefinition
}

type ToolDefinition struct {
	Name           string             `json:"name"`
	Category       string             `json:"category"`
	Description    string             `json:"description"`
	InputSchema    map[string]any     `json:"input_schema"`
	OutputEnvelope OutputEnvelope     `json:"output_envelope"`
	Capability     ToolCapability     `json:"capability"`
	Confirmation   ConfirmationPolicy `json:"confirmation"`
	Audit          AuditPolicy        `json:"audit"`
	MCPAnnotations MCPAnnotations     `json:"mcp_annotations"`
	Migration      *MigrationMetadata `json:"migration,omitempty"`
}

type OutputEnvelope struct {
	Kind     string  `json:"kind"`
	JSONRoot *string `json:"json_root"`
}

type ToolCapability struct {
	Tool       string     `json:"tool"`
	Scope      Scope      `json:"scope"`
	Permission Permission `json:"permission"`
	RiskLevel  RiskLevel  `json:"risk_level"`
	UseFor     string     `json:"use_for"`
}

type ConfirmationPolicy struct {
	Required        bool    `json:"required"`
	Reason          *string `json:"reason"`
	TokenTTLSeconds *int    `json:"token_ttl_seconds"`
}

type AuditPolicy struct {
	EventType     string   `json:"event_type"`
	RecordAttempt bool     `json:"record_attempt"`
	RecordResult  bool     `json:"record_result"`
	RedactArgs    []string `json:"redact_args"`
}

type MCPAnnotations struct {
	ReadOnlyHint    bool `json:"read_only_hint"`
	DestructiveHint bool `json:"destructive_hint"`
	OpenWorldHint   bool `json:"open_world_hint"`
}

type MigrationMetadata struct {
	SourceFile       string `json:"source_file"`
	PermissionSource string `json:"permission_source"`
	ReviewRequired   bool   `json:"review_required"`
}

type LoadOption func(*loadOptions)

type loadOptions struct {
	disabledBoundaries bool
}

func WithDisabledBoundaries() LoadOption {
	return func(opts *loadOptions) {
		opts.disabledBoundaries = true
	}
}

func DefaultRegistry() (*Registry, error) {
	payload, err := defaultRegistryFS.ReadFile("default_registry.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded tool registry: %w", err)
	}
	return LoadJSON(payload, WithDisabledBoundaries())
}

func LoadJSON(payload []byte, options ...LoadOption) (*Registry, error) {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil, errors.New("tool registry JSON is empty")
	}
	var registry Registry
	if err := json.Unmarshal(payload, &registry); err != nil {
		return nil, fmt.Errorf("decode tool registry: %w", err)
	}

	opts := loadOptions{}
	for _, option := range options {
		option(&opts)
	}
	if opts.disabledBoundaries {
		registry.Tools = append(registry.Tools, disabledBoundaryTools()...)
	}
	registry.reindex()
	if err := registry.Validate(); err != nil {
		return nil, err
	}
	return &registry, nil
}
