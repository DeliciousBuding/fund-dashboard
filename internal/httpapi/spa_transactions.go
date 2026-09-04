package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/DeliciousBuding/fund-dashboard/internal/contracts"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/go-chi/chi/v5"
)

// registerSPATransactionRoutes exposes transaction mutations for the browser SPA.
// The caller mounts these inside the BrowserWriteAuth group: session cookie
// first, with the legacy edge-injected key only while FUND_EDGE_AUTH_ENABLED is
// on. The SPA client itself never holds a key. Handlers are the same
// implementation the admin Bearer surface uses (registerTransactionMutationRoutes).
func registerSPATransactionRoutes(r chi.Router, service adminsvc.Service) {
	registerTransactionMutationRoutes(r, service, transactionMutationPaths{
		importPath: "/api/transactions/import",
		seqPath:    "/api/transactions/{seq}",
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
		// Bound fan-out: each code hits NAV/xirr/drawdown work (#205). The limit
		// is shared with the MCP compare_funds surface via internal/contracts.
		if len(codes) > contracts.MaxCompareCodes {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("codes max %d", contracts.MaxCompareCodes))
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
