package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/go-chi/chi/v5"
)

// registerSPATransactionRoutes exposes transaction mutations for the browser SPA.
// The caller must wrap these routes in EdgeAuth; the SPA never holds the shared key.
func registerSPATransactionRoutes(r chi.Router, service adminsvc.Service) {
	r.Post("/api/transactions/import", func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, 2<<20)
		var body importTransactionsRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		result, err := service.ImportTransactions(req.Context(), body.Transactions)
		writeAdminTransactionResult(w, req, result, err)
	})

	r.Put("/api/transactions/{seq}", func(w http.ResponseWriter, req *http.Request) {
		seq, err := strconv.Atoi(chi.URLParam(req, "seq"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "seq required")
			return
		}
		req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
		var body adminsvc.UpdateTransaction
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		result, err := service.UpdateTransaction(req.Context(), seq, body)
		writeAdminTransactionResult(w, req, result, err)
	})

	r.Delete("/api/transactions/{seq}", func(w http.ResponseWriter, req *http.Request) {
		seq, err := strconv.Atoi(chi.URLParam(req, "seq"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "seq required")
			return
		}
		result, err := service.DeleteTransaction(req.Context(), seq)
		writeAdminTransactionResult(w, req, result, err)
	})
}

func registerAnalysisRoutes(r chi.Router, service *portfoliosvc.Service) {
	r.Get("/api/analysis/compare", handleCompareFunds(service))
}

func handleCompareFunds(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimSpace(r.URL.Query().Get("codes"))
		if raw == "" {
			writeError(w, http.StatusBadRequest, "codes required")
			return
		}
		parts := strings.Split(raw, ",")
		codes := make([]string, 0, len(parts))
		for _, part := range parts {
			code := strings.TrimSpace(part)
			if code != "" {
				codes = append(codes, code)
			}
		}
		if len(codes) == 0 {
			writeError(w, http.StatusBadRequest, "codes required")
			return
		}
		// Bound fan-out: each code hits NAV/xirr/drawdown work (#205).
		const maxCompareCodes = 8
		if len(codes) > maxCompareCodes {
			writeError(w, http.StatusBadRequest, "codes max 8")
			return
		}
		results, err := service.CompareFunds(r.Context(), codes, portfolioIDFromRequest(r))
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		// Frontend CompareResultSchema expects { funds: [...] }.
		funds := make([]map[string]any, 0, len(results))
		for _, item := range results {
			funds = append(funds, map[string]any{
				"code":         item.Code,
				"name":         item.Name,
				"market":       item.Market,
				"xirr":         item.XIRR,
				"volatility":   item.Volatility,
				"sharpe":       item.Sharpe,
				"max_drawdown": item.MaxDrawdown,
				"calmar":       item.Calmar,
			})
		}
		WriteJSON(w, http.StatusOK, map[string]any{"funds": funds})
	}
}
