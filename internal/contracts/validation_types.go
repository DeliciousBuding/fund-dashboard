package contracts

type agentContextPack struct {
	SchemaVersion    string             `json:"schema_version"`
	GeneratedAt      string             `json:"generated_at"`
	DecisionBoundary string             `json:"decision_boundary"`
	Identity         agentIdentity      `json:"identity"`
	Portfolio        map[string]any     `json:"portfolio"`
	Holdings         []map[string]any   `json:"holdings"`
	DataQuality      agentDataQuality   `json:"data_quality"`
	SourceContext    agentSourceContext `json:"source_context"`
	Permissions      agentPermissions   `json:"permissions"`
	Capabilities     []toolCapability   `json:"capabilities"`
	Maintenance      map[string]any     `json:"maintenance"`
	AgentBrief       string             `json:"agent_brief"`
	PublicProjection map[string]any     `json:"public_projection,omitempty"`
}

type agentIdentity struct {
	PortfolioID   int     `json:"portfolio_id"`
	PortfolioName *string `json:"portfolio_name,omitempty"`
	BaseCurrency  string  `json:"base_currency"`
	DataVersion   string  `json:"data_version"`
}

type agentDataQuality struct {
	OverallScore          int      `json:"overall_score"`
	Level                 string   `json:"level"`
	StalePriceCount       int      `json:"stale_price_count"`
	MissingCostBasisCount int      `json:"missing_cost_basis_count"`
	MissingChangePctCount int      `json:"missing_change_pct_count"`
	HoldingsCoveragePct   float64  `json:"holdings_coverage_pct"`
	IntegrityStatus       *string  `json:"integrity_status,omitempty"`
	Limitations           []string `json:"limitations"`
}

type agentSourceContext struct {
	Queries             []map[string]any    `json:"queries"`
	Targets             []map[string]any    `json:"targets"`
	StoredEventsSummary sourceEventsSummary `json:"stored_events_summary"`
	RecentEvents        []map[string]any    `json:"recent_events"`
}

type sourceEventsSummary struct {
	Total   int `json:"total"`
	Unread  int `json:"unread"`
	Useful  int `json:"useful"`
	Ignored int `json:"ignored"`
}

type agentPermissions struct {
	DecisionBoundary     string   `json:"decision_boundary"`
	ReadScope            []string `json:"read_scope"`
	WriteScope           []string `json:"write_scope"`
	RequiresConfirmation []string `json:"requires_confirmation"`
	DisabledOperations   []string `json:"disabled_operations"`
}

type toolRegistry struct {
	SchemaVersion string           `json:"schema_version"`
	GeneratedAt   string           `json:"generated_at"`
	Tools         []toolDefinition `json:"tools"`
}

type toolDefinition struct {
	Name           string             `json:"name"`
	Category       string             `json:"category"`
	Description    string             `json:"description"`
	InputSchema    map[string]any     `json:"input_schema"`
	OutputEnvelope outputEnvelope     `json:"output_envelope"`
	Capability     toolCapability     `json:"capability"`
	Confirmation   confirmationPolicy `json:"confirmation"`
	Audit          auditPolicy        `json:"audit"`
	MCPAnnotations mcpAnnotations     `json:"mcp_annotations"`
	Migration      *migrationMetadata `json:"migration,omitempty"`
}

type outputEnvelope struct {
	Kind     string  `json:"kind"`
	JSONRoot *string `json:"json_root"`
}

type toolCapability struct {
	Tool       string `json:"tool"`
	Scope      string `json:"scope"`
	Permission string `json:"permission"`
	RiskLevel  string `json:"risk_level"`
	UseFor     string `json:"use_for"`
}

type confirmationPolicy struct {
	Required        bool    `json:"required"`
	Reason          *string `json:"reason"`
	TokenTTLSeconds *int    `json:"token_ttl_seconds"`
}

type auditPolicy struct {
	EventType     string   `json:"event_type"`
	RecordAttempt bool     `json:"record_attempt"`
	RecordResult  bool     `json:"record_result"`
	RedactArgs    []string `json:"redact_args"`
}

type mcpAnnotations struct {
	ReadOnlyHint    bool `json:"read_only_hint"`
	DestructiveHint bool `json:"destructive_hint"`
	OpenWorldHint   bool `json:"open_world_hint"`
}

type migrationMetadata struct {
	SourceFile       string `json:"source_file"`
	PermissionSource string `json:"permission_source"`
	ReviewRequired   bool   `json:"review_required"`
}
