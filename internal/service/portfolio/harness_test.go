package portfolio

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestServiceGetHarnessSnapshotReturnsFactsOnlyAgentContext(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()
	seedMixedHarnessData(t, db)

	service := NewService(db)
	snapshot, err := service.GetHarnessSnapshot(context.Background(), 1) // public default
	if err != nil {
		t.Fatalf("GetHarnessSnapshot returned error: %v", err)
	}

	if snapshot.DecisionBoundary != "facts_only" {
		t.Fatalf("DecisionBoundary = %q, want facts_only", snapshot.DecisionBoundary)
	}
	if snapshot.HoldingsCount != 3 {
		t.Fatalf("HoldingsCount = %d, want 3", snapshot.HoldingsCount)
	}
	if snapshot.TotalValue != 830 {
		t.Fatalf("TotalValue = %.2f, want 830", snapshot.TotalValue)
	}
	if snapshot.Allocation == nil || snapshot.Allocation.TotalValue != 830 {
		t.Fatalf("Allocation = %#v, want total 830", snapshot.Allocation)
	}
	if len(snapshot.HoldingSignals) != 3 {
		t.Fatalf("HoldingSignals length = %d, want 3: %#v", len(snapshot.HoldingSignals), snapshot.HoldingSignals)
	}

	aapl := findHoldingSignal(t, snapshot.HoldingSignals, "AAPL")
	if !containsString(aapl.SignalTags, "price_rally_gt_5pct") {
		t.Fatalf("AAPL SignalTags = %#v, want price_rally_gt_5pct", aapl.SignalTags)
	}
	if aapl.WeightPct != 45.78 {
		t.Fatalf("AAPL WeightPct = %.2f, want 45.78", aapl.WeightPct)
	}
	if aapl.ChangePct == nil || *aapl.ChangePct != 6.5 {
		t.Fatalf("AAPL ChangePct = %v, want 6.5", aapl.ChangePct)
	}

	qdii := findHoldingSignal(t, snapshot.HoldingSignals, "019173")
	if !containsString(qdii.SignalTags, "above_cost_gt_10pct") {
		t.Fatalf("019173 SignalTags = %#v, want above_cost_gt_10pct", qdii.SignalTags)
	}
	if qdii.CostPerShare == nil || *qdii.CostPerShare != 1.2 {
		t.Fatalf("019173 CostPerShare = %v, want 1.2", qdii.CostPerShare)
	}
	if qdii.DeviationPct == nil || *qdii.DeviationPct != 25 {
		t.Fatalf("019173 DeviationPct = %v, want 25", qdii.DeviationPct)
	}

	if snapshot.DataQuality.HoldingsCoveragePct != 100 {
		t.Fatalf("HoldingsCoveragePct = %.1f, want 100", snapshot.DataQuality.HoldingsCoveragePct)
	}
	if !containsString(snapshot.AvailableAgentTools, "get_full_dashboard") {
		t.Fatalf("AvailableAgentTools = %#v, want get_full_dashboard", snapshot.AvailableAgentTools)
	}
	// Public harness must not advertise write/maintenance/confirmation-gated tools.
	for _, banned := range []string{"generate_report", "run_dca_auto_invest", "adjust_position", "check_alerts", "recalculate_snapshot", "crawl_nav", "add_transaction", "delete_fund"} {
		if containsString(snapshot.AvailableAgentTools, banned) {
			t.Fatalf("AvailableAgentTools must not include public-banned %s: %#v", banned, snapshot.AvailableAgentTools)
		}
	}
	if !containsString(snapshot.AgentPermissions.DisabledOperations, "backup_producer") {
		t.Fatalf("DisabledOperations = %#v, want backup_producer", snapshot.AgentPermissions.DisabledOperations)
	}
	if capabilityExists(snapshot.AgentCapabilities, "crawl_nav", "maintenance") {
		t.Fatalf("public AgentCapabilities must not include crawl_nav: %#v", snapshot.AgentCapabilities)
	}
	if len(snapshot.AgentPermissions.RequiresConfirmation) != 0 {
		t.Fatalf("public RequiresConfirmation = %#v, want empty", snapshot.AgentPermissions.RequiresConfirmation)
	}
	if len(snapshot.AgentPermissions.WriteScope) != 0 {
		t.Fatalf("public WriteScope = %#v, want empty", snapshot.AgentPermissions.WriteScope)
	}
	if !actionExists(snapshot.RecommendedAgentActions, "get_investment_source_brief") {
		t.Fatalf("RecommendedAgentActions missing get_investment_source_brief: %#v", snapshot.RecommendedAgentActions)
	}
	if !strings.Contains(snapshot.AgentBrief, "read-only") && !strings.Contains(snapshot.AgentBrief, "Public discovery") {
		t.Fatalf("AgentBrief = %q, want public read-only boundary", snapshot.AgentBrief)
	}

	// Operator audience restores full discovery surface. This is the wired deployment:
	// FUND_AGENT_OPS_ENABLED set, so the confirmation flow can complete and the gated
	// tools are honestly advertised. The unwired shape is asserted in
	// harness_availability_test.go and, across surfaces, in internal/mcp.
	service.SetConfirmationFlowAvailable(true)
	op, err := service.GetHarnessSnapshotFor(context.Background(), 1, HarnessAudienceOperator)
	if err != nil {
		t.Fatalf("GetHarnessSnapshotFor operator: %v", err)
	}
	if !containsString(op.AvailableAgentTools, "crawl_nav") || !containsString(op.AvailableAgentTools, "add_transaction") {
		t.Fatalf("operator AvailableAgentTools missing maintenance/write tools: %#v", op.AvailableAgentTools)
	}
	if len(op.AgentPermissions.RequiresConfirmation) == 0 {
		t.Fatalf("operator RequiresConfirmation empty")
	}
	if !strings.Contains(op.AgentBrief, "transaction writes require confirmation") {
		t.Fatalf("operator AgentBrief = %q", op.AgentBrief)
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if strings.Contains(string(payload), "actual_amount") {
		t.Fatalf("snapshot leaked actual_amount: %s", string(payload))
	}
	if strings.Contains(string(payload), "建议买入") || strings.Contains(string(payload), "建议卖出") || strings.Contains(string(payload), "建议扣款") {
		t.Fatalf("snapshot contains investment decision language: %s", string(payload))
	}
}

func seedMixedHarnessData(t *testing.T, db execer) {
	t.Helper()
	seedMixedAllocationData(t, db)
	if _, err := db.ExecContext(context.Background(), `
		DELETE FROM nav_history;
		CREATE TABLE IF NOT EXISTS fund_holdings (
			fund_code TEXT,
			stock_code TEXT,
			stock_name TEXT,
			weight_pct REAL,
			shares REAL,
			market_value REAL,
			report_date TEXT,
			PRIMARY KEY (fund_code, stock_code, report_date)
		);
		DELETE FROM fund_holdings;
		INSERT INTO nav_history (fund_code, date, unit_nav, daily_change_pct, security_type) VALUES
			('019173', '2026-06-18', 1.5, -4.2, 'fund'),
			('AAPL', '2026-06-18', 190, 6.5, 'stock'),
			('00700', '2026-06-18', 30, -1.2, 'stock');
		INSERT INTO fund_holdings (fund_code, stock_code, stock_name, weight_pct, shares, market_value, report_date) VALUES
			('019173', 'NVDA', 'NVIDIA', 8.5, 100, 12000, '2026-03-31'),
			('019173', 'MSFT', 'Microsoft', 7.2, 100, 11000, '2026-03-31');
	`); err != nil {
		t.Fatalf("seed mixed harness data: %v", err)
	}
}

func findHoldingSignal(t *testing.T, rows []HarnessHoldingSignal, code string) HarnessHoldingSignal {
	t.Helper()
	for _, row := range rows {
		if row.Code == code {
			return row
		}
	}
	t.Fatalf("holding signal %s not found in %#v", code, rows)
	return HarnessHoldingSignal{}
}

func containsString(rows []string, want string) bool {
	for _, row := range rows {
		if row == want {
			return true
		}
	}
	return false
}

func capabilityExists(rows []AgentCapability, tool string, scope string) bool {
	for _, row := range rows {
		if row.Tool == tool && row.Scope == scope {
			return true
		}
	}
	return false
}

func actionExists(rows []RecommendedAgentAction, tool string) bool {
	for _, row := range rows {
		if row.Tool == tool {
			return true
		}
	}
	return false
}
