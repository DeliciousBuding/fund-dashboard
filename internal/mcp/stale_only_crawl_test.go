package mcp

import (
	"context"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
)

func TestRecommendedRefreshCodesDedupes(t *testing.T) {
	report := adminsvc.FreshnessReport{
		StaleSecurities: []adminsvc.StaleSecurity{
			{Code: "019173", Name: "A", LastNAV: "2020-01-01", StaleDays: 10},
			{Code: "016453", Name: "B", LastNAV: "2020-01-01", StaleDays: 8},
		},
		MissingNAVSecurities: []adminsvc.FreshnessItem{
			{Code: "019173", Name: "A", Type: "fund"},
			{Code: "000001", Name: "C", Type: "fund"},
		},
	}
	codes := RecommendedRefreshCodes(report)
	if len(codes) != 3 {
		t.Fatalf("codes=%v want 3 unique", codes)
	}
}

type fakeNavCrawler struct {
	calls []string
}

func (f *fakeNavCrawler) CrawlAllHeld(ctx context.Context) (int, int, error) {
	f.calls = append(f.calls, "*")
	return 9, 0, nil
}

func (f *fakeNavCrawler) CrawlCode(ctx context.Context, code string) (int, string, error) {
	f.calls = append(f.calls, code)
	return 1, "2026-07-18", nil
}

func TestCallCrawlNavStaleOnlyDoesNotUseCrawlAllHeld(t *testing.T) {
	db := openMCPFixture(t)
	defer db.Close()
	portfolio := portfoliosvc.NewService(db)
	admin := adminsvc.NewService(db)
	nav := &fakeNavCrawler{}
	server, err := NewServer(ServerDeps{
		Portfolio: &portfolio,
		Admin:     &admin,
		Nav:       nav,
		Role:      agenttools.RoleOperator,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if _, callErr := server.callCrawlNav(context.Background(), map[string]any{"stale_only": true}); callErr != nil {
		t.Fatalf("callCrawlNav error: %#v", callErr)
	}
	for _, c := range nav.calls {
		if c == "*" {
			t.Fatalf("stale_only should not call CrawlAllHeld, calls=%v", nav.calls)
		}
	}
}
