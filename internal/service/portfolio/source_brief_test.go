package portfolio

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestServiceGetInvestmentSourceBriefBuildsFactsOnlyQueries(t *testing.T) {
	db := openSummaryFixture(t)
	defer db.Close()
	seedMixedHarnessData(t, db)

	service := NewService(db)
	brief, err := service.GetInvestmentSourceBrief(context.Background(), SourceBriefOptions{
		PortfolioID: 1,
		Limit:       6,
	})
	if err != nil {
		t.Fatalf("GetInvestmentSourceBrief returned error: %v", err)
	}

	if brief.DecisionBoundary != "source_queries_only" {
		t.Fatalf("DecisionBoundary = %q, want source_queries_only", brief.DecisionBoundary)
	}
	if len(brief.Queries) == 0 {
		t.Fatalf("Queries is empty, want source query candidates")
	}
	if len(brief.Queries) > 6 {
		t.Fatalf("Queries length = %d, want capped to 6", len(brief.Queries))
	}
	if !queryContains(brief.Queries, "Apple") && !queryContains(brief.Queries, "AAPL") {
		t.Fatalf("Queries = %#v, want Apple/AAPL holding query", brief.Queries)
	}
	if !queryContains(brief.Queries, "NVIDIA") && !queryContains(brief.Queries, "NVDA") {
		t.Fatalf("Queries = %#v, want NVIDIA/NVDA underlying query", brief.Queries)
	}
	if brief.Coverage.HoldingsScanned != 3 {
		t.Fatalf("HoldingsScanned = %d, want 3", brief.Coverage.HoldingsScanned)
	}
	if brief.Coverage.UnderlyingScanned != 2 {
		t.Fatalf("UnderlyingScanned = %d, want 2", brief.Coverage.UnderlyingScanned)
	}
	if brief.Coverage.MaxQueries != 6 {
		t.Fatalf("MaxQueries = %d, want 6", brief.Coverage.MaxQueries)
	}
	if !targetNameExists(brief.SourceTargets, "DSA search providers") {
		t.Fatalf("SourceTargets missing DSA search providers: %#v", brief.SourceTargets)
	}
	if !targetNameExists(brief.SourceTargets, "DSA market context") {
		t.Fatalf("SourceTargets missing DSA market context: %#v", brief.SourceTargets)
	}
	if !targetKindExists(brief.SourceTargets, "web_search") {
		t.Fatalf("SourceTargets missing web_search kind: %#v", brief.SourceTargets)
	}
	if !strings.Contains(brief.AgentBrief, "Hermes") || !strings.Contains(brief.AgentBrief, "DSA search providers") {
		t.Fatalf("AgentBrief = %q, want Hermes and DSA target context", brief.AgentBrief)
	}

	payload, err := json.Marshal(brief)
	if err != nil {
		t.Fatalf("marshal source brief: %v", err)
	}
	for _, forbidden := range []string{"买入", "卖出", "加仓", "减仓", "建议扣款"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("source brief contains decision language %q: %s", forbidden, string(payload))
		}
	}
}

func queryContains(rows []InvestmentSourceQuery, term string) bool {
	for _, row := range rows {
		if strings.Contains(row.Query, term) {
			return true
		}
	}
	return false
}

func targetNameExists(rows []InvestmentSourceTarget, name string) bool {
	for _, row := range rows {
		if row.Name == name {
			return true
		}
	}
	return false
}

func targetKindExists(rows []InvestmentSourceTarget, kind string) bool {
	for _, row := range rows {
		if row.Kind == kind {
			return true
		}
	}
	return false
}
