package httpapi

import (
	"database/sql"
	"net/http"
	"time"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	"github.com/go-chi/chi/v5"
)

// process start time shared by status + dashboard uptime.
var adminProcessStartedAt = time.Now()

// registerAdminDashboardRoutes mounts operator dashboard under /api/admin (Bearer).
// buildVersion is FUND_VERSION (git pin / release id); empty omits build_version in JSON.
func registerAdminDashboardRoutes(r chi.Router, db *sql.DB, driver string, buildVersion string) {
	service := adminsvc.NewServiceWithDriver(db, driver)
	r.Get("/dashboard", handleAdminDashboard(service, buildVersion))
}

// registerOpsDashboardRoutes mounts SPA dashboard under EdgeAuth as /api/ops/dashboard.
func registerOpsDashboardRoutes(r chi.Router, db *sql.DB, driver string, buildVersion string) {
	service := adminsvc.NewServiceWithDriver(db, driver)
	r.Get("/api/ops/dashboard", handleAdminDashboard(service, buildVersion))
}

func handleAdminDashboard(service adminsvc.Service, buildVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		started := time.Now()
		report, err := service.GetDashboard(req.Context(), adminProcessStartedAt, started.UTC())
		if err != nil {
			writeSafeError(w, req, http.StatusInternalServerError, err)
			return
		}
		report.ResponseMS = time.Since(started).Milliseconds()
		report.System.BuildVersion = buildVersion
		WriteJSON(w, http.StatusOK, report)
	}
}
