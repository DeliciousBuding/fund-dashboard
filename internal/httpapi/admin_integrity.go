package httpapi

import (
	"database/sql"
	"net/http"
	"time"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	"github.com/go-chi/chi/v5"
)

func registerAdminIntegrityRoutes(r chi.Router, db *sql.DB, driver string) {
	service := adminsvc.NewServiceWithDriver(db, driver)
	r.Get("/db-integrity", func(w http.ResponseWriter, req *http.Request) {
		report, err := service.GetDBIntegrity(req.Context(), time.Now().UTC())
		if err != nil {
			writeSafeError(w, req, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, report)
	})
}
