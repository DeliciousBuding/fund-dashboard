package httpapi

import (
	"database/sql"
	"net/http"
	"time"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	"github.com/go-chi/chi/v5"
)

func registerAdminStatusRoutes(r chi.Router, db *sql.DB, driver string) {
	service := adminsvc.NewServiceWithDriver(db, driver)
	r.Get("/status", func(w http.ResponseWriter, req *http.Request) {
		started := time.Now()
		status, err := service.GetSystemStatus(req.Context(), adminProcessStartedAt, started)
		if err != nil {
			writeSafeError(w, req, http.StatusInternalServerError, err)
			return
		}
		status.ResponseMS = time.Since(started).Milliseconds()
		WriteJSON(w, http.StatusOK, status)
	})
	r.Get("/status/{code}", func(w http.ResponseWriter, req *http.Request) {
		status, err := service.GetStatusByCode(req.Context(), chi.URLParam(req, "code"))
		if err != nil {
			writeSafeError(w, req, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, status)
	})
}
