package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminDBIntegrityReportsSQLiteChecksAndRowCounts(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(testCfg(), WithDB(db))

	result := doJSONRequest(t, router, http.MethodGet, "/api/admin/db-integrity", nil, http.StatusOK)
	if result["overall"] != "ok" {
		t.Fatalf("overall = %v, want ok; body=%s", result["overall"], toJSONString(t, result))
	}
	if result["decision_boundary"] != "read_only" {
		t.Fatalf("decision_boundary = %v, want read_only", result["decision_boundary"])
	}
	if _, ok := result["timestamp"].(string); !ok {
		t.Fatalf("timestamp = %#v, want string", result["timestamp"])
	}

	checks := result["checks"].(map[string]any)
	integrity := checks["integrity_check"].(map[string]any)
	if integrity["passed"] != true || integrity["detail"] != "ok" {
		t.Fatalf("integrity_check = %#v, want passed ok", integrity)
	}
	quick := checks["quick_check"].(map[string]any)
	if quick["passed"] != true || quick["result"] != "ok" {
		t.Fatalf("quick_check = %#v, want passed ok", quick)
	}
	foreignKeys := checks["foreign_key_check"].(map[string]any)
	if foreignKeys["passed"] != true || foreignKeys["violations"].(float64) != 0 {
		t.Fatalf("foreign_key_check = %#v, want no violations", foreignKeys)
	}

	rowCounts := result["row_counts"].(map[string]any)
	if rowCounts["fund_details"].(float64) != 2 {
		t.Fatalf("fund_details row count = %v, want 2", rowCounts["fund_details"])
	}
	if rowCounts["transactions"].(float64) != 2 {
		t.Fatalf("transactions row count = %v, want 2", rowCounts["transactions"])
	}

	if strings.Contains(toJSONString(t, result), "restore") || strings.Contains(toJSONString(t, result), "repair") {
		t.Fatalf("read-only integrity response should not recommend repair/restore for clean data: %s", toJSONString(t, result))
	}
}

func TestAdminDBIntegrityRejectsPost(t *testing.T) {
	db := openPortfolioHTTPFixture(t)
	defer db.Close()

	router := NewRouter(testCfg(), WithDB(db))

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/api/admin/db-integrity", nil))
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/admin/db-integrity status = %d, want 401; body=%s", res.Code, res.Body.String())
	}
}
