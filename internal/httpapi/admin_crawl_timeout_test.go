package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// blockingNav blocks until the request context ends, then returns ctx.Err() —
// a slow dependency that makes the per-request timeout observable.
type blockingNav struct{}

func (blockingNav) CrawlCode(ctx context.Context, _ string) (int, string, error) {
	<-ctx.Done()
	return 0, "", ctx.Err()
}

func (blockingNav) CrawlAllHeld(ctx context.Context) (int, int, error) {
	<-ctx.Done()
	return 0, 0, ctx.Err()
}

type blockingHoldings struct{}

func (blockingHoldings) CrawlCode(ctx context.Context, _ string) (int, string, error) {
	<-ctx.Done()
	return 0, "", ctx.Err()
}

func (blockingHoldings) CrawlAllHeld(ctx context.Context) (int, int, error) {
	<-ctx.Done()
	return 0, 0, ctx.Err()
}

type blockingSnapshots struct{}

func (blockingSnapshots) RecalcCode(ctx context.Context, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func (blockingSnapshots) RecalcAll(ctx context.Context) (int, []string, error) {
	<-ctx.Done()
	return 0, nil, ctx.Err()
}

// shortCrawlTimeouts makes the handler deadlines fire in tens of milliseconds
// and restores the defaults afterwards.
func shortCrawlTimeouts(t *testing.T) {
	t.Helper()
	oldBatch, oldSingle := adminCrawlBatchTimeout, adminCrawlSingleTimeout
	adminCrawlBatchTimeout, adminCrawlSingleTimeout = 50*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() {
		adminCrawlBatchTimeout, adminCrawlSingleTimeout = oldBatch, oldSingle
	})
}

func adminCrawlRequest(t *testing.T, router http.Handler, path string) map[string]any {
	t.Helper()
	return doJSONRequest(t, router, http.MethodPost, path, nil, http.StatusGatewayTimeout)
}

func TestAdminCrawlRequestTimeouts(t *testing.T) {
	t.Run("crawl-nav held", func(t *testing.T) {
		shortCrawlTimeouts(t)
		router := NewRouter(testCfg(), WithNavCrawler(blockingNav{}))
		body := adminCrawlRequest(t, router, "/api/admin/crawl-nav")
		if body["status"] != "error" || body["mode"] != "held" || body["error"] != "timeout" {
			t.Fatalf("body=%v", body)
		}
	})
	t.Run("crawl-nav single", func(t *testing.T) {
		shortCrawlTimeouts(t)
		router := NewRouter(testCfg(), WithNavCrawler(blockingNav{}))
		body := adminCrawlRequest(t, router, "/api/admin/crawl-nav?code=019173")
		if body["status"] != "error" || body["mode"] != "single" || body["error"] != "timeout" {
			t.Fatalf("body=%v", body)
		}
	})
	t.Run("crawl-nav stale_only", func(t *testing.T) {
		shortCrawlTimeouts(t)
		db := openPortfolioHTTPFixture(t)
		defer db.Close()
		// A held code with no NAV is recommended by freshness, so the crawl
		// loop reaches the slow dependency before the deadline fires.
		if _, err := db.Exec(`INSERT INTO portfolio_snapshot
			(fund_code, fund_name, held_shares, total_cost, latest_nav, current_value, unrealized_pnl, pnl_pct, security_type, portfolio_id)
			VALUES ('SLOW1', 'Slow Fund', 1, -1, NULL, 0, 0, 0, 'fund', 1)`); err != nil {
			t.Fatal(err)
		}
		router := NewRouter(testCfg(), WithDB(db), WithNavCrawler(blockingNav{}))
		body := adminCrawlRequest(t, router, "/api/admin/crawl-nav?stale_only=1")
		if body["status"] != "error" || body["mode"] != "stale_only" || body["error"] != "timeout" {
			t.Fatalf("body=%v", body)
		}
	})
	t.Run("crawl-holdings held", func(t *testing.T) {
		shortCrawlTimeouts(t)
		router := NewRouter(testCfg(), WithHoldingsCrawler(blockingHoldings{}))
		body := adminCrawlRequest(t, router, "/api/admin/crawl-holdings")
		if body["status"] != "error" || body["mode"] != "held" || body["error"] != "timeout" {
			t.Fatalf("body=%v", body)
		}
	})
	t.Run("crawl-holdings single", func(t *testing.T) {
		shortCrawlTimeouts(t)
		router := NewRouter(testCfg(), WithHoldingsCrawler(blockingHoldings{}))
		body := adminCrawlRequest(t, router, "/api/admin/crawl-holdings?code=019173")
		if body["status"] != "error" || body["mode"] != "single" || body["error"] != "timeout" {
			t.Fatalf("body=%v", body)
		}
	})
	t.Run("recalculate-snapshot all", func(t *testing.T) {
		shortCrawlTimeouts(t)
		router := NewRouter(testCfg(), WithSnapshotRecalculator(blockingSnapshots{}))
		body := adminCrawlRequest(t, router, "/api/admin/recalculate-snapshot")
		if body["status"] != "error" || body["mode"] != "all" || body["error"] != "timeout" {
			t.Fatalf("body=%v", body)
		}
	})
	t.Run("recalculate-snapshot single", func(t *testing.T) {
		shortCrawlTimeouts(t)
		router := NewRouter(testCfg(), WithSnapshotRecalculator(blockingSnapshots{}))
		body := adminCrawlRequest(t, router, "/api/admin/recalculate-snapshot?code=019173")
		if body["status"] != "error" || body["mode"] != "single" || body["error"] != "timeout" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestAdminCrawlTimeoutDoesNotLeakInternals(t *testing.T) {
	shortCrawlTimeouts(t)
	router := NewRouter(testCfg(), WithNavCrawler(blockingNav{}))
	body := adminCrawlRequest(t, router, "/api/admin/crawl-nav")
	msg, _ := body["error"].(string)
	if msg != "timeout" {
		t.Fatalf("error = %q, want stable timeout code", msg)
	}
	if _, leaked := body["detail"]; leaked {
		t.Fatalf("response leaked internal detail: %v", body)
	}
}
