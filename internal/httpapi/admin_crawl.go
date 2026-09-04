package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/mcp"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
)

// Admin crawl/recalculate requests run the same whole-batch crawls as the
// scheduled jobs, but synchronously inside one HTTP request. Without a
// request-level deadline a slow upstream could pin the connection until the
// server WriteTimeout cuts it. These bounds are the primary defense; the
// server WriteTimeout stays as the last-resort backstop.
//
// Derivation (see internal/jobs/scheduler.go #248/#268 and
// internal/datasource/*.go):
//   - Batch modes (crawl-nav held/stale_only, crawl-holdings held,
//     recalculate-snapshot all) reuse the scheduler's established 45m
//     whole-batch ceiling. Worst case at production scale (~61 held funds):
//     61 × the 30s upstream HTTP client timeout plus the 1.5s per-code backoff
//     ≈ 33m, leaving headroom for slow upstreams/retries.
//   - Single-code modes only need one upstream fetch (30s Eastmoney / 12s
//     Yahoo history) plus DB round-trips; 2m is ample margin.
//
// Package vars are overridable in tests so the timeout path can be exercised
// deterministically with short durations (see admin_crawl_timeout_test.go).
var (
	adminCrawlBatchTimeout  = 45 * time.Minute
	adminCrawlSingleTimeout = 2 * time.Minute
)

// navCrawlHandler exposes POST /api/admin/crawl-nav.
// Query:
//   - code / fund_code optional — single security via NavCrawler
//   - stale_only=1|true — refresh only held stale/missing NAV codes (#253; MCP #252 parity)
//   - else all held
func navCrawlHandler(n mcp.NavCrawler, admin *adminsvc.Service) http.HandlerFunc {
	type resp struct {
		Status      string   `json:"status"`
		Mode        string   `json:"mode"`
		FundCode    string   `json:"fund_code,omitempty"`
		Securities  int      `json:"securities,omitempty"`
		Added       int      `json:"added"`
		Latest      string   `json:"latest,omitempty"`
		Codes       []string `json:"codes,omitempty"`
		FailedCodes []string `json:"failed_codes,omitempty"`
		Message     string   `json:"message,omitempty"`
		Error       string   `json:"error,omitempty"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			code = strings.TrimSpace(r.URL.Query().Get("fund_code"))
		}
		if len(code) > 32 {
			WriteJSON(w, http.StatusBadRequest, resp{Status: "error", Error: "fund_code too long"})
			return
		}
		if code != "" {
			ctx, cancel := context.WithTimeout(r.Context(), adminCrawlSingleTimeout)
			defer cancel()
			added, latest, err := n.CrawlCode(ctx, code)
			out := resp{Mode: "single", FundCode: code, Added: added, Latest: latest}
			if err != nil {
				status, msg := crawlOpFailure(r, err)
				out.Status = "error"
				out.Error = msg
				WriteJSON(w, status, out)
				return
			}
			out.Status = "complete"
			WriteJSON(w, http.StatusOK, out)
			return
		}

		staleOnly := truthyQuery(r.URL.Query().Get("stale_only"))
		if staleOnly {
			ctx, cancel := context.WithTimeout(r.Context(), adminCrawlBatchTimeout)
			defer cancel()
			if admin == nil {
				WriteJSON(w, http.StatusInternalServerError, resp{
					Status: "error", Mode: "stale_only", Error: "admin_freshness_unavailable",
				})
				return
			}
			// The freshness read, the recommended-code selection, the per-code
			// loop and the status rule all come from the shared crawl service, so
			// this endpoint and the MCP crawl_nav tool cannot drift (#253 parity).
			result, err := adminsvc.RefreshStaleCodes(ctx, *admin, navCodeRefresher(n), adminsvc.BatchPolicy{
				FailureLogMessage: "admin crawl-nav stale_only code failed",
				LogAttrs:          []any{"request_id", RequestIDFromContext(r.Context())},
			})
			if err != nil {
				status, msg := crawlOpFailure(r, err)
				WriteJSON(w, status, resp{
					Status: "error", Mode: "stale_only", Error: msg,
				})
				return
			}
			if len(result.Codes) == 0 {
				WriteJSON(w, http.StatusOK, resp{
					Status:  "complete",
					Mode:    "stale_only",
					Added:   0,
					Codes:   []string{},
					Message: "no_stale_or_missing_held_nav",
				})
				return
			}
			batch := result.Batch
			// The shared batch stops on a deadline without an error return; the
			// whole-request budget is still exhausted, so surface 504 rather
			// than a misleading partial/complete.
			if requestDeadlineHit(ctx) {
				WriteJSON(w, http.StatusGatewayTimeout, resp{
					Status:      "error",
					Mode:        "stale_only",
					Securities:  len(batch.Done),
					Added:       batch.Added,
					Codes:       batch.Done,
					FailedCodes: batch.Failed,
					Error:       "timeout",
				})
				return
			}
			status := batch.Status()
			out := resp{
				Status:      status,
				Mode:        "stale_only",
				Securities:  len(batch.Done),
				Added:       batch.Added,
				Codes:       batch.Done,
				FailedCodes: batch.Failed,
			}
			if status == "error" {
				WriteJSON(w, http.StatusInternalServerError, out)
				return
			}
			WriteJSON(w, http.StatusOK, out)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), adminCrawlBatchTimeout)
		defer cancel()
		securities, added, err := n.CrawlAllHeld(ctx)
		out := resp{Mode: "held", Securities: securities, Added: added}
		if err != nil {
			status, msg := crawlOpFailure(r, err)
			out.Status = "error"
			out.Error = msg
			WriteJSON(w, status, out)
			return
		}
		out.Status = "complete"
		WriteJSON(w, http.StatusOK, out)
	}
}

// navCodeRefresher adapts the mcp.NavCrawler port that the admin crawl routes
// are wired with to the shared crawl service CodeRefresher port. The
// latest-NAV string is a single-code response detail a batch does not need.
func navCodeRefresher(n mcp.NavCrawler) adminsvc.CodeRefresher {
	return func(ctx context.Context, code string) (int, error) {
		added, _, err := n.CrawlCode(ctx, code)
		return added, err
	}
}

func truthyQuery(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// holdingsCrawlHandler exposes POST /api/admin/crawl-holdings.
// Query: code / fund_code optional — when set, crawls one fund; otherwise all held funds.
func holdingsCrawlHandler(h mcp.HoldingsCrawler) http.HandlerFunc {
	type resp struct {
		Status     string `json:"status"`
		Mode       string `json:"mode"`
		FundCode   string `json:"fund_code,omitempty"`
		Funds      int    `json:"funds,omitempty"`
		Added      int    `json:"added"`
		ReportDate string `json:"report_date,omitempty"`
		Error      string `json:"error,omitempty"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			code = strings.TrimSpace(r.URL.Query().Get("fund_code"))
		}
		if len(code) > 32 {
			WriteJSON(w, http.StatusBadRequest, resp{Status: "error", Error: "fund_code too long"})
			return
		}
		if code != "" {
			ctx, cancel := context.WithTimeout(r.Context(), adminCrawlSingleTimeout)
			defer cancel()
			added, reportDate, err := h.CrawlCode(ctx, code)
			out := resp{Mode: "single", FundCode: code, Added: added, ReportDate: reportDate}
			if err != nil {
				status, msg := crawlOpFailure(r, err)
				out.Status = "error"
				out.Error = msg
				WriteJSON(w, status, out)
				return
			}
			out.Status = "complete"
			WriteJSON(w, http.StatusOK, out)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), adminCrawlBatchTimeout)
		defer cancel()
		funds, added, err := h.CrawlAllHeld(ctx)
		out := resp{Mode: "held", Funds: funds, Added: added}
		if err != nil {
			status, msg := crawlOpFailure(r, err)
			out.Status = "error"
			out.Error = msg
			WriteJSON(w, status, out)
			return
		}
		out.Status = "complete"
		WriteJSON(w, http.StatusOK, out)
	}
}

// recalculateSnapshotHandler exposes POST /api/admin/recalculate-snapshot.
// Query: code / fund_code optional — when set, one fund; otherwise all transaction codes.
// All-mode: failed_codes always present (empty array when none); status complete|partial|error.
func recalculateSnapshotHandler(s mcp.SnapshotRecalculator) http.HandlerFunc {
	type resp struct {
		Status      string   `json:"status"`
		Mode        string   `json:"mode"`
		FundCode    string   `json:"fund_code,omitempty"`
		Codes       int      `json:"codes,omitempty"`
		FailedCodes []string `json:"failed_codes"`
		Error       string   `json:"error,omitempty"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			code = strings.TrimSpace(r.URL.Query().Get("fund_code"))
		}
		if len(code) > 32 {
			WriteJSON(w, http.StatusBadRequest, resp{Status: "error", Error: "fund_code too long"})
			return
		}
		if code != "" {
			ctx, cancel := context.WithTimeout(r.Context(), adminCrawlSingleTimeout)
			defer cancel()
			err := s.RecalcCode(ctx, code)
			out := resp{Mode: "single", FundCode: code}
			if err != nil {
				status, msg := crawlOpFailure(r, err)
				out.Status = "error"
				out.Error = msg
				WriteJSON(w, status, out)
				return
			}
			out.Status = "complete"
			WriteJSON(w, http.StatusOK, out)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), adminCrawlBatchTimeout)
		defer cancel()
		n, failed, err := s.RecalcAll(ctx)
		if failed == nil {
			failed = []string{}
		}
		out := resp{Mode: "all", Codes: n, FailedCodes: failed}
		if err != nil {
			status, msg := crawlOpFailure(r, err)
			out.Status = "error"
			out.Error = msg
			WriteJSON(w, status, out)
			return
		}
		// RecalcAll soft-fails per code and can return err=nil after a deadline
		// cut the batch short; still report 504 instead of a misleading
		// complete/partial.
		if requestDeadlineHit(ctx) {
			WriteJSON(w, http.StatusGatewayTimeout, resp{
				Status:      "error",
				Mode:        "all",
				Codes:       n,
				FailedCodes: failed,
				Error:       "timeout",
			})
			return
		}
		status := adminsvc.BatchStatus(n, failed)
		out.Status = status
		if status == "error" {
			WriteJSON(w, http.StatusInternalServerError, out)
			return
		}
		WriteJSON(w, http.StatusOK, out)
	}
}

// crawlOpFailure maps an admin crawl/recalculate failure to (HTTP status, safe
// client message). A request deadline returns 504 with the stable "timeout"
// code; every other error keeps the existing 500 safeAdminOpError path so
// internal details (SQL/upstream noise) never leave the server.
func crawlOpFailure(r *http.Request, err error) (int, string) {
	if errors.Is(err, context.DeadlineExceeded) {
		rid := ""
		if r != nil {
			rid = RequestIDFromContext(r.Context())
		}
		slog.Warn("admin crawl request timed out", "request_id", rid, "path", safePath(r), "error", err.Error())
		return http.StatusGatewayTimeout, "timeout"
	}
	return http.StatusInternalServerError, safeAdminOpError(r, err)
}

// requestDeadlineHit reports whether ctx expired because of a deadline (not a
// client disconnect/cancel).
func requestDeadlineHit(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.DeadlineExceeded)
}

// safeAdminOpError logs the full error and returns a stable client-facing code for admin crawl/recalc (#206).
func safeAdminOpError(r *http.Request, err error) string {
	if err == nil {
		return "internal_error"
	}
	rid := ""
	if r != nil {
		rid = RequestIDFromContext(r.Context())
	}
	slog.Error("admin op error", "request_id", rid, "path", safePath(r), "error", err.Error())
	return "internal_error"
}
