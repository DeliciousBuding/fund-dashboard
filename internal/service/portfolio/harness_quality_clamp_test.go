package portfolio

import (
	"strings"
	"testing"
)

func TestBuildRecommendedAgentActionsClampsReason(t *testing.T) {
	// riskFlags with a very long token should still produce bounded Reason.
	long := strings.Repeat("风险提示", 200)
	actions := buildRecommendedAgentActions(1, 1, 50, []string{long})
	if len(actions) == 0 {
		t.Fatal("expected actions")
	}
	for _, a := range actions {
		if len([]rune(a.Reason)) > 500 {
			t.Fatalf("reason too long for tool %s: %d", a.Tool, len([]rune(a.Reason)))
		}
	}
}

func TestBuildRecommendedAgentActionsCrawlNavUsesStaleOnly(t *testing.T) {
	actions := buildRecommendedAgentActions(2, 0, 100, nil)
	var crawl *RecommendedAgentAction
	for i := range actions {
		if actions[i].Tool == "crawl_nav" {
			crawl = &actions[i]
			break
		}
	}
	if crawl == nil {
		t.Fatal("expected crawl_nav recommendation when stale prices exist")
	}
	if crawl.Input == nil || crawl.Input["stale_only"] != true {
		t.Fatalf("crawl_nav Input=%v want {stale_only:true}", crawl.Input)
	}
	if _, ok := crawl.Input["all"]; ok {
		t.Fatalf("crawl_nav Input must not use all=true: %v", crawl.Input)
	}
}
