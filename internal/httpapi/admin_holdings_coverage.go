package httpapi

import (
	"database/sql"
	"net/http"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	"github.com/go-chi/chi/v5"
)

func registerAdminHoldingsCoverageRoutes(r chi.Router, db *sql.DB, driver string) {
	service := adminsvc.NewServiceWithDriver(db, driver)
	r.Get("/holdings-coverage", func(w http.ResponseWriter, req *http.Request) {
		report, err := service.GetHoldingsCoverage(req.Context(), intQueryMax(req, "portfolio_id", 1, 1000))
		if err != nil {
			writeSafeError(w, req, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, report)
	})
}
