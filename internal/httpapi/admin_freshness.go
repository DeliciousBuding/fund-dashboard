package httpapi

import (
	"database/sql"
	"net/http"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	"github.com/go-chi/chi/v5"
)

func registerAdminFreshnessRoutes(r chi.Router, db *sql.DB, driver string) {
	service := adminsvc.NewServiceWithDriver(db, driver)
	r.Get("/freshness", func(w http.ResponseWriter, req *http.Request) {
		report, err := service.GetFreshness(req.Context())
		if err != nil {
			writeSafeError(w, req, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, report)
	})
}
