package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/go-chi/chi/v5"
)

func registerPortfolioRoutes(r chi.Router, service *portfoliosvc.Service) {
	r.Route("/api/portfolio", func(r chi.Router) {
		r.Get("/", handlePortfolioSummary(service))
		r.Get("/portfolios", handlePortfolioDefinitions(service))
		r.Get("/penetration", handlePortfolioPenetration(service))
		r.Get("/timeline", handlePortfolioTimeline(service))
		r.Get("/xirr", handlePortfolioXIRR(service))
		r.Get("/allocation", handlePortfolioAllocation(service))
		r.Get("/agent-context", handlePortfolioAgentContext(service))
		r.Get("/harness", handlePortfolioHarness(service))
		r.Get("/source-brief", handlePortfolioSourceBrief(service))
		r.Get("/source-events", handleGetSourceEvents(service))
	})
}

// registerPortfolioWriteRoutes mounts SPA source-event mutations under EdgeAuth.
func registerPortfolioWriteRoutes(r chi.Router, service *portfoliosvc.Service) {
	r.Post("/api/portfolio/source-events", handleCreateSourceEvent(service))
	r.Patch("/api/portfolio/source-events/{id}", handleMarkSourceEvent(service))
}

func handlePortfolioSummary(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary, err := service.GetSummary(r.Context(), portfolioIDFromRequest(r))
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, portfolioSummaryResponse(summary))
	}
}

func handlePortfolioDefinitions(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		portfolios, err := service.ListPortfolioDefinitions(r.Context())
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, portfolios)
	}
}

func handlePortfolioPenetration(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := service.GetPenetration(r.Context(), portfoliosvc.PenetrationOptions{
			PortfolioID: portfolioIDFromRequest(r),
			Limit:       intQueryMax(r, "limit", 30, 200),
			SortBy:      r.URL.Query().Get("sort_by"),
		})
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, penetrationResponse(report))
	}
}

func handlePortfolioTimeline(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timeline, err := service.GetTimeline(r.Context(), portfolioIDFromRequest(r))
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, timeline)
	}
}

func handlePortfolioXIRR(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := service.GetPortfolioXIRR(r.Context(), portfolioIDFromRequest(r))
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, xirrResponse{
			XIRR:    report.XIRRPct,
			Message: report.Message,
		})
	}
}

func handlePortfolioAllocation(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allocation, err := service.GetAllocation(r.Context(), portfolioIDFromRequest(r))
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, allocation)
	}
}

func handlePortfolioAgentContext(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pack, err := service.GetAgentContextPack(r.Context(), portfoliosvc.AgentContextOptions{
			PortfolioID:  portfolioIDFromRequest(r),
			SourceLimit:  intQueryMax(r, "source_limit", 8, 50),
			EventLimit:   intQueryMax(r, "event_limit", 20, 100),
			BaseCurrency: r.URL.Query().Get("base_currency"),
		})
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, pack)
	}
}

func handlePortfolioHarness(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot, err := service.GetHarnessSnapshot(r.Context(), portfolioIDFromRequest(r))
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, snapshot)
	}
}

func handlePortfolioSourceBrief(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		brief, err := service.GetInvestmentSourceBrief(r.Context(), portfoliosvc.SourceBriefOptions{
			PortfolioID: portfolioIDFromRequest(r),
			Limit:       intQueryMax(r, "limit", 20, 100),
		})
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, brief)
	}
}

func handleGetSourceEvents(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opts := portfoliosvc.GetSourceEventsOptions{
			Limit:               intQueryMax(r, "limit", 30, 200),
			RelatedSecurityCode: r.URL.Query().Get("code"),
			Source:              r.URL.Query().Get("source"),
			ShowRead:            boolQuery(r, "show_read"),
		}
		events, err := service.GetSourceEvents(r.Context(), opts)
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"count":             len(events),
			"decision_boundary": "facts_only",
			"events":            sourceEventResponses(events),
		})
	}
}

func handleCreateSourceEvent(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var input sourceEventRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		if input.Title == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}
		if input.URL != nil && !safeHTTPURL(strings.TrimSpace(*input.URL)) {
			writeError(w, http.StatusBadRequest, "url must use http(s) scheme")
			return
		}
		event, err := service.CreateSourceEvent(r.Context(), portfoliosvc.CreateSourceEventInput{
			Title:               input.Title,
			URL:                 input.URL,
			Source:              input.Source,
			Snippet:             input.Snippet,
			Query:               input.Query,
			RelatedSecurityCode: input.RelatedSecurityCode,
			RelatedSecurityName: input.RelatedSecurityName,
		})
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusCreated, sourceEventResponse(*event))
	}
}

func handleMarkSourceEvent(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var input markSourceEventRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		ok, err := service.MarkSourceEventRead(r.Context(), id, portfoliosvc.MarkSourceEventInput{
			IsRead:   input.IsRead,
			IsUseful: input.IsUseful,
		})
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "not found or no fields to update")
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
	}
}

// safeHTTPURL accepts empty or http(s) URLs only. Source events are rendered
// as clickable links by the SPA, so any other scheme (javascript:, data:, …)
// must be rejected at the write boundary to prevent stored-XSS/script execution.
func safeHTTPURL(raw string) bool {
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

type sourceEventRequest struct {
	Title               string  `json:"title"`
	URL                 *string `json:"url"`
	Source              *string `json:"source"`
	Snippet             *string `json:"snippet"`
	Query               *string `json:"query"`
	RelatedSecurityCode *string `json:"related_security_code"`
	RelatedSecurityName *string `json:"related_security_name"`
}

type markSourceEventRequest struct {
	IsRead   *bool `json:"is_read"`
	IsUseful *bool `json:"is_useful"`
}

func portfolioIDFromRequest(r *http.Request) int {
	value, err := strconv.Atoi(r.URL.Query().Get("portfolio_id"))
	if err != nil || value < 1 {
		return 1
	}
	// Single-portfolio prod uses 1; cap absurd values (#214).
	const maxPortfolioID = 1000
	if value > maxPortfolioID {
		return maxPortfolioID
	}
	return value
}

func intQuery(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

// intQueryMax is intQuery with a hard upper bound (and lower bound 1 when positive fallbacks expected).
func intQueryMax(r *http.Request, key string, fallback, max int) int {
	v := intQuery(r, key, fallback)
	if v <= 0 {
		return fallback
	}
	if max > 0 && v > max {
		return max
	}
	return v
}

// intQueryOpt parses an optional integer query param. Empty falls back; a
// present-but-malformed value is a client error (writes 400, ok=false).
func intQueryOpt(w http.ResponseWriter, r *http.Request, key string, fallback int) (int, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query_param: "+key)
		return 0, false
	}
	return v, true
}

func floatQueryOpt(w http.ResponseWriter, r *http.Request, key string, fallback float64) (float64, bool) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, true
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query_param: "+key)
		return 0, false
	}
	return v, true
}
func floatQuery(r *http.Request, key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(r.URL.Query().Get(key), 64)
	if err != nil {
		return fallback
	}
	return value
}

// floatQueryMax clamps to (0, max]; non-positive falls back (#213).
func floatQueryMax(r *http.Request, key string, fallback, max float64) float64 {
	v := floatQuery(r, key, fallback)
	if v <= 0 {
		return fallback
	}
	if max > 0 && v > max {
		return max
	}
	return v
}

func boolQuery(r *http.Request, key string) bool {
	switch r.URL.Query().Get(key) {
	case "1", "true", "TRUE", "True":
		return true
	default:
		return false
	}
}
