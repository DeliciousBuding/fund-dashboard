package portfolio

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/contracts"
)

func TestServiceGetAgentContextPackAssemblesVersionedFactsOnlyPack(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()
	seedMixedHarnessData(t, db)
	ensureSourceEventsTable(t, db)

	service := NewService(db)
	appleEvent, err := service.CreateSourceEvent(context.Background(), CreateSourceEventInput{
		Title:               "Apple earnings update",
		Source:              stringPtr("websearch"),
		Snippet:             stringPtr("Apple reported updated revenue facts."),
		Query:               stringPtr("AAPL earnings update"),
		RelatedSecurityCode: stringPtr("AAPL"),
		RelatedSecurityName: stringPtr("Apple Inc."),
	})
	if err != nil {
		t.Fatalf("create apple source event: %v", err)
	}
	if _, err := service.MarkSourceEventRead(context.Background(), appleEvent.ID, MarkSourceEventInput{
		IsRead:   boolPtr(true),
		IsUseful: boolPtr(true),
	}); err != nil {
		t.Fatalf("mark apple event useful: %v", err)
	}
	if _, err := service.CreateSourceEvent(context.Background(), CreateSourceEventInput{
		Title:               "Tencent regulatory note",
		Source:              stringPtr("eastmoney"),
		Query:               stringPtr("00700 regulation"),
		RelatedSecurityCode: stringPtr("00700"),
		RelatedSecurityName: stringPtr("腾讯控股"),
	}); err != nil {
		t.Fatalf("create tencent source event: %v", err)
	}

	pack, err := service.GetAgentContextPack(context.Background(), AgentContextOptions{
		PortfolioID: 1,
		SourceLimit: 4,
		EventLimit:  10,
	})
	if err != nil {
		t.Fatalf("GetAgentContextPack returned error: %v", err)
	}

	if pack.SchemaVersion != "agent-context-pack-v1" {
		t.Fatalf("SchemaVersion = %q, want agent-context-pack-v1", pack.SchemaVersion)
	}
	if pack.DecisionBoundary != "facts_only" {
		t.Fatalf("DecisionBoundary = %q, want facts_only", pack.DecisionBoundary)
	}
	if pack.Identity.PortfolioID != 1 || pack.Identity.BaseCurrency != "CNY" {
		t.Fatalf("Identity = %#v, want portfolio 1 CNY", pack.Identity)
	}
	if pack.Identity.DataVersion == "" {
		t.Fatalf("Identity.DataVersion is empty")
	}
	if pack.Portfolio.Summary == nil || pack.Portfolio.Allocation == nil {
		t.Fatalf("Portfolio missing summary/allocation: %#v", pack.Portfolio)
	}
	if len(pack.Holdings) != 3 {
		t.Fatalf("Holdings length = %d, want 3", len(pack.Holdings))
	}
	aapl := findContextHolding(t, pack.Holdings, "AAPL")
	if aapl.LatestPrice != 190 || aapl.ChangePct == nil || *aapl.ChangePct != 6.5 {
		t.Fatalf("AAPL holding = %#v, want latest price and change pct", aapl)
	}
	if pack.DataQuality.OverallScore < 90 || pack.DataQuality.Level != "good" {
		t.Fatalf("DataQuality = %#v, want good high score", pack.DataQuality)
	}
	if pack.SourceContext.StoredEventsSummary.Total != 2 ||
		pack.SourceContext.StoredEventsSummary.Unread != 1 ||
		pack.SourceContext.StoredEventsSummary.Useful != 1 {
		t.Fatalf("StoredEventsSummary = %#v, want total=2 unread=1 useful=1", pack.SourceContext.StoredEventsSummary)
	}
	if len(pack.SourceContext.RecentEvents) != 2 {
		t.Fatalf("RecentEvents length = %d, want 2", len(pack.SourceContext.RecentEvents))
	}
	if pack.SourceContext.RecentEvents[0].Title == "" || pack.SourceContext.RecentEvents[0].IsRead == nil {
		t.Fatalf("RecentEvents[0] = %#v, want low-sensitive event with read flag", pack.SourceContext.RecentEvents[0])
	}
	if len(pack.SourceContext.Queries) != 4 {
		t.Fatalf("SourceContext.Queries length = %d, want requested limit 4", len(pack.SourceContext.Queries))
	}
	if !containsString(pack.Permissions.DisabledOperations, "backup_producer") {
		t.Fatalf("DisabledOperations = %#v, want backup_producer disabled", pack.Permissions.DisabledOperations)
	}
	// Public agent-context is least-privilege (#65): no write/maintenance discovery.
	if capabilityExists(pack.Capabilities, "crawl_nav", "maintenance") {
		t.Fatalf("public Capabilities must not include crawl_nav: %#v", pack.Capabilities)
	}
	if capabilityWithPermissionExists(pack.Capabilities, "add_transaction", "write", "requires_confirmation") {
		t.Fatalf("public Capabilities must not include add_transaction: %#v", pack.Capabilities)
	}
	if len(pack.Permissions.WriteScope) != 0 || len(pack.Permissions.RequiresConfirmation) != 0 {
		t.Fatalf("public permissions write/confirmation not empty: %#v", pack.Permissions)
	}
	if !actionExists(pack.Maintenance.RecommendedActions, "get_investment_source_brief") {
		t.Fatalf("Maintenance actions missing source brief: %#v", pack.Maintenance.RecommendedActions)
	}
	if pack.PublicProjection == nil ||
		pack.PublicProjection.HoldingsCount != 3 ||
		pack.PublicProjection.SourceEventsUnread != 1 {
		t.Fatalf("PublicProjection = %#v, want low-sensitive counts", pack.PublicProjection)
	}

	// Operator audience restores full registry discovery.
	op, err := service.GetAgentContextPackFor(context.Background(), AgentContextOptions{
		PortfolioID: 1,
		SourceLimit: 4,
		EventLimit:  10,
	}, HarnessAudienceOperator)
	if err != nil {
		t.Fatalf("GetAgentContextPackFor operator: %v", err)
	}
	if !capabilityExists(op.Capabilities, "crawl_nav", "maintenance") {
		t.Fatalf("operator Capabilities missing crawl_nav maintenance: %#v", op.Capabilities)
	}
	if len(op.Capabilities) < 44 {
		t.Fatalf("operator Capabilities length = %d, want full registry coverage", len(op.Capabilities))
	}
	if !capabilityWithPermissionExists(op.Capabilities, "add_transaction", "write", "requires_confirmation") {
		t.Fatalf("operator Capabilities missing add_transaction confirmation policy: %#v", op.Capabilities)
	}

	payload, err := json.Marshal(pack)
	if err != nil {
		t.Fatalf("marshal pack: %v", err)
	}
	if err := contracts.ValidateAgentContextPackJSON(payload); err != nil {
		t.Fatalf("agent context pack contract validation failed: %v\n%s", err, string(payload))
	}
	for _, forbidden := range []string{"actual_amount", "建议买入", "建议卖出", "建议扣款", "cash_transfer_enabled", `"add_transaction"`, `"crawl_nav"`} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("agent context pack leaked forbidden token %q: %s", forbidden, string(payload))
		}
	}
}

func findContextHolding(t *testing.T, rows []AgentContextHolding, code string) AgentContextHolding {
	t.Helper()
	for _, row := range rows {
		if row.Code == code {
			return row
		}
	}
	t.Fatalf("context holding %s not found in %#v", code, rows)
	return AgentContextHolding{}
}

func capabilityWithPermissionExists(rows []AgentCapability, tool string, scope string, permission string) bool {
	for _, row := range rows {
		if row.Tool == tool && row.Scope == scope && row.Permission == permission {
			return true
		}
	}
	return false
}
