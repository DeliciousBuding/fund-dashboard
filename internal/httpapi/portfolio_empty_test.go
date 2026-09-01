package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
	"github.com/DeliciousBuding/fund-dashboard/internal/testutil"
)

// TestEmptyPortfolioCollectionsSerializeAsArrays pins the SPA contract that
// bare arrays stay arrays on an empty database: zod z.array rejects null, so a
// nil Go slice serialized as null breaks fetchValidated.
func TestEmptyPortfolioCollectionsSerializeAsArrays(t *testing.T) {
	dbi := testutil.OpenTempDB(t)
	defer dbi.Close()
	if err := db.EnsureSQLiteSchema(context.Background(), dbi); err != nil {
		t.Fatalf("ensure sqlite schema: %v", err)
	}
	router := newAuthedRouter(t, testCfg(), dbi, WithDBDriver("sqlite"))

	// GET /api/portfolio/portfolios must be [] (was null when the table was empty).
	req := httptest.NewRequest(http.MethodGet, "/api/portfolio/portfolios", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("portfolios status = %d, want 200; body=%s", res.Code, res.Body.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(res.Body.String()), "[") {
		t.Fatalf("portfolios must serialize as a JSON array, got %s", res.Body.String())
	}
	var portfolios []any
	if err := json.Unmarshal(res.Body.Bytes(), &portfolios); err != nil || portfolios == nil {
		t.Fatalf("portfolios decode = %v, value=%s (nil slice serialized as null)", err, res.Body.String())
	}

	// GET /api/portfolio/allocation by_* arrays must be [] (was null with no rows).
	req = httptest.NewRequest(http.MethodGet, "/api/portfolio/allocation", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("allocation status = %d, want 200; body=%s", res.Code, res.Body.String())
	}
	var allocation map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &allocation); err != nil {
		t.Fatalf("allocation decode: %v; body=%s", err, res.Body.String())
	}
	for _, key := range []string{"by_security_type", "by_market", "by_fund_type"} {
		value, ok := allocation[key]
		if !ok {
			t.Fatalf("allocation missing %q; body=%s", key, res.Body.String())
		}
		if _, isArray := value.([]any); !isArray {
			t.Fatalf("allocation %q must be a JSON array, got %T (%v); body=%s", key, value, value, res.Body.String())
		}
	}
}
