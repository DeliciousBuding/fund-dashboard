package httpapi

import (
	"strings"
	"net/http"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/go-chi/chi/v5"
)

func registerFundRoutes(r chi.Router, service *portfoliosvc.Service) {
	r.Get("/api/securities", handleFundList(service))
	r.Route("/api/funds", func(r chi.Router) {
		r.Get("/", handleFundList(service))
		r.Get("/{code}", handleFundDetail(service))
		r.Get("/{code}/nav", handleFundNAV(service))
		r.Get("/{code}/xirr", handleFundXIRR(service))
		r.Get("/{code}/drawdown", handleFundDrawdown(service))
		r.Get("/{code}/dca", handleFundDCA(service))
	})
}

func handleFundList(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := service.ListSecurities(r.Context(), portfolioIDFromRequest(r))
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, items)
	}
}

func handleFundDetail(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := normalizedCodeParam(r)
		detail, err := service.GetFundDetail(r.Context(), code, portfolioIDFromRequest(r))
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		if detail == nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		WriteJSON(w, http.StatusOK, fundDetailResponse(*detail))
	}
}

func handleFundNAV(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := service.GetNavHistory(r.Context(), normalizedCodeParam(r), intQueryMax(r, "limit", 200, 2000))
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, report.Data)
	}
}

func handleFundXIRR(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := normalizedCodeParam(r)
		report, err := service.GetFundXIRR(r.Context(), code, portfolioIDFromRequest(r))
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, xirrResponse{
			XIRR:    report.XIRRPct,
			Message: report.Message,
			Code:    code,
		})
	}
}

func handleFundDrawdown(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := service.GetFundDrawdown(r.Context(), normalizedCodeParam(r))
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		if report == nil {
			writeError(w, http.StatusNotFound, "no nav data")
			return
		}
		WriteJSON(w, http.StatusOK, drawdownResponse(*report))
	}
}

func handleFundDCA(service *portfoliosvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := service.ComputeDCAAmount(r.Context(), portfoliosvc.ComputeDCAAmountOptions{
			Code:        normalizedCodeParam(r),
			BaseAmount:  floatQueryMax(r, "base", 30, 1_000_000),
			Mode:        normalizeDCAMode(r.URL.Query().Get("mode")),
			PortfolioID: portfolioIDFromRequest(r),
		})
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		if report.Error != "" {
			WriteJSON(w, http.StatusBadRequest, report)
			return
		}
		WriteJSON(w, http.StatusOK, report)
	}
}

func normalizedCodeParam(r *http.Request) string {
	return adminsvc.NormalizeSecurityCode(chi.URLParam(r, "code"))
}

func normalizeDCAMode(raw string) string {
	switch strings.TrimSpace(raw) {
	case "change_pct":
		return "change_pct"
	default:
		return "nav_deviation"
	}
}
