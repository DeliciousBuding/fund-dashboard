package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

func TestHealthEndpointReportsAgentSafeRuntimeBoundary(t *testing.T) {
	cfg, err := config.Parse(map[string]string{
		"FUND_VERSION": "test-sha",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	res := httptest.NewRecorder()
	NewRouter(cfg).ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", res.Code, res.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("health response is not JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %v, want ok", body["status"])
	}
	if body["service"] != "fund-dashboard-go" {
		t.Fatalf("service field = %v, want fund-dashboard-go", body["service"])
	}
	if body["version"] != "test-sha" {
		t.Fatalf("version field = %v, want test-sha", body["version"])
	}
	if body["facts_only"] != true {
		t.Fatalf("facts_only field = %v, want true", body["facts_only"])
	}
	if body["backup_producer_enabled"] != false {
		t.Fatalf("backup_producer_enabled field = %v, want false", body["backup_producer_enabled"])
	}
}

func TestHealthEndpointRedactsVersionInProduction(t *testing.T) {
	cfg, err := config.Parse(map[string]string{
		"FUND_VERSION": "prod-sha-should-hide",
		"FUND_ENV":     "production",
		// production secret floors so Parse succeeds
		"MCP_API_KEY":   "0123456789abcdef",
		"FUND_EDGE_KEY": "fedcba9876543210",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	res := httptest.NewRecorder()
	NewRouter(cfg).ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", res.Code, res.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("health response is not JSON: %v", err)
	}
	if _, ok := body["version"]; ok {
		t.Fatalf("production health must omit version, got %#v", body["version"])
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %v, want ok", body["status"])
	}
}

func TestHealthEndpointRejectsUnsupportedMethods(t *testing.T) {
	cfg, err := config.Parse(map[string]string{})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	res := httptest.NewRecorder()
	NewRouter(cfg).ServeHTTP(res, req)

	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", res.Code)
	}
}
