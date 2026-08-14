package httpapi

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
	"github.com/DeliciousBuding/fund-dashboard/internal/mcp"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
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
			added, latest, err := n.CrawlCode(r.Context(), code)
			out := resp{Mode: "single", FundCode: code, Added: added, Latest: latest}
			if err != nil {
				out.Status = "error"
				out.Error = safeAdminOpError(r, err)
				WriteJSON(w, http.StatusInternalServerError, out)
				return
			}
			out.Status = "complete"
			WriteJSON(w, http.StatusOK, out)
			return
		}

		staleOnly := truthyQuery(r.URL.Query().Get("stale_only"))
		if staleOnly {
			if admin == nil {
				WriteJSON(w, http.StatusInternalServerError, resp{
					Status: "error", Mode: "stale_only", Error: "admin_freshness_unavailable",
				})
				return
			}
			report, err := admin.GetFreshness(r.Context())
			if err != nil {
				WriteJSON(w, http.StatusInternalServerError, resp{
					Status: "error", Mode: "stale_only", Error: safeAdminOpError(r, err),
				})
				return
			}
			codes := mcp.RecommendedRefreshCodes(report)
			if len(codes) == 0 {
				WriteJSON(w, http.StatusOK, resp{
					Status:  "complete",
					Mode:    "stale_only",
					Added:   0,
					Codes:   []string{},
					Message: "no_stale_or_missing_held_nav",
				})
				return
			}
			totalAdded := 0
			done := make([]string, 0, len(codes))
			failed := make([]string, 0)
			for _, c := range codes {
				if err := r.Context().Err(); err != nil {
					break
				}
				added, _, err := n.CrawlCode(r.Context(), c)
				if err != nil {
					slog.Error("admin crawl-nav stale_only code failed",
						"request_id", RequestIDFromContext(r.Context()),
						"code", c,
						"error", err.Error(),
					)
					failed = append(failed, c)
					continue
				}
				totalAdded += added
				done = append(done, c)
			}
			status := "complete"
			if len(failed) > 0 && len(done) == 0 {
				status = "error"
			} else if len(failed) > 0 {
				status = "partial"
			}
			out := resp{
				Status:      status,
				Mode:        "stale_only",
				Securities:  len(done),
				Added:       totalAdded,
				Codes:       done,
				FailedCodes: failed,
			}
			if status == "error" {
				WriteJSON(w, http.StatusInternalServerError, out)
				return
			}
			WriteJSON(w, http.StatusOK, out)
			return
		}

		securities, added, err := n.CrawlAllHeld(r.Context())
		out := resp{Mode: "held", Securities: securities, Added: added}
		if err != nil {
			out.Status = "error"
			out.Error = safeAdminOpError(r, err)
			WriteJSON(w, http.StatusInternalServerError, out)
			return
		}
		out.Status = "complete"
		WriteJSON(w, http.StatusOK, out)
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
			added, reportDate, err := h.CrawlCode(r.Context(), code)
			out := resp{Mode: "single", FundCode: code, Added: added, ReportDate: reportDate}
			if err != nil {
				out.Status = "error"
				out.Error = safeAdminOpError(r, err)
				WriteJSON(w, http.StatusInternalServerError, out)
				return
			}
			out.Status = "complete"
			WriteJSON(w, http.StatusOK, out)
			return
		}
		funds, added, err := h.CrawlAllHeld(r.Context())
		out := resp{Mode: "held", Funds: funds, Added: added}
		if err != nil {
			out.Status = "error"
			out.Error = safeAdminOpError(r, err)
			WriteJSON(w, http.StatusInternalServerError, out)
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
			err := s.RecalcCode(r.Context(), code)
			out := resp{Mode: "single", FundCode: code}
			if err != nil {
				out.Status = "error"
				out.Error = safeAdminOpError(r, err)
				WriteJSON(w, http.StatusInternalServerError, out)
				return
			}
			out.Status = "complete"
			WriteJSON(w, http.StatusOK, out)
			return
		}
		n, failed, err := s.RecalcAll(r.Context())
		if failed == nil {
			failed = []string{}
		}
		out := resp{Mode: "all", Codes: n, FailedCodes: failed}
		if err != nil {
			out.Status = "error"
			out.Error = safeAdminOpError(r, err)
			WriteJSON(w, http.StatusInternalServerError, out)
			return
		}
		status := jobs.RecalcAllStatus(n, failed)
		out.Status = status
		if status == "error" {
			WriteJSON(w, http.StatusInternalServerError, out)
			return
		}
		WriteJSON(w, http.StatusOK, out)
	}
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
