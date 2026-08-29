package httpapi

import (
	"database/sql"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	"github.com/go-chi/chi/v5"
)

func registerAdminIntegrityRoutes(r chi.Router, db *sql.DB, driver string) {
	service := adminsvc.NewServiceWithDriver(db, driver)
	r.Get("/db-integrity", handleDBIntegrity(service))
}
