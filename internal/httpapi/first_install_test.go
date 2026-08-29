package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

// TestFirstInstallEmptySQLiteReadAPIsServeData is the public self-host
// first-boot path: a brand-new empty SQLite DB must serve empty data with
// 200 on the core read APIs instead of a wall of internal_error 500s.
func TestFirstInstallEmptySQLiteReadAPIsServeData(t *testing.T) {
	dbi := testutil.OpenTempDB(t)
	defer dbi.Close()

	if err := db.EnsureSQLiteSchema(context.Background(), dbi); err != nil {
		t.Fatalf("ensure sqlite schema: %v", err)
	}

	router := newAuthedRouter(t, testCfg(), dbi, WithDBDriver("sqlite"))

	for _, path := range []string{
		"/api/portfolio/",
		"/api/transactions",
		"/api/dca/plans",
		"/api/freshness",
		"/api/system/status",
	} {
		doJSONRequest(t, router, http.MethodGet, path, nil, http.StatusOK)
	}
}
