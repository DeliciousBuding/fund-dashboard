package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

func TestAgentToolsRoutesRequireAdminAuth(t *testing.T) {
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test", AdminKey: testAdminKey})
	for _, path := range []string{
		"/api/agent/tools",
		"/api/agent/tools/summary",
		"/api/agent/tools/add_transaction/authorize?role=operator",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("%s without key status=%d want 401 body=%s", path, res.Code, res.Body.String())
		}
	}
}

func TestAgentToolsRouteExposesPermissionRegistry(t *testing.T) {
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test", AdminKey: testAdminKey})

	registry := doJSONRequest(t, router, http.MethodGet, "/api/agent/tools", nil, http.StatusOK)
	if registry["schema_version"] != "tool-registry-v1" {
		t.Fatalf("schema_version = %v, want tool-registry-v1", registry["schema_version"])
	}
	tools, ok := registry["tools"].([]any)
	if !ok {
		t.Fatalf("tools = %#v, want array", registry["tools"])
	}
	if len(tools) < 47 {
		t.Fatalf("tools length = %d, want 44 current tools plus disabled boundaries", len(tools))
	}
	payload := toJSONString(t, registry)
	for _, want := range []string{"get_investment_source_brief", "add_transaction", "backup_producer", `"permission":"disabled"`} {
		if !strings.Contains(payload, want) {
			t.Fatalf("registry missing %q: %s", want, payload)
		}
	}
}

func TestAgentToolsSummaryRouteExposesGovernanceCounts(t *testing.T) {
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test", AdminKey: testAdminKey})

	summary := doJSONRequest(t, router, http.MethodGet, "/api/agent/tools/summary", nil, http.StatusOK)
	if summary["schema_version"] != "tool-registry-v1" {
		t.Fatalf("schema_version = %v, want tool-registry-v1", summary["schema_version"])
	}
	if summary["total_tools"].(float64) < 47 {
		t.Fatalf("summary = %#v, want current tools plus disabled boundaries", summary)
	}
	if summary["review_required_tools"].(float64) < 26 {
		t.Fatalf("summary = %#v, want inferred rows counted", summary)
	}
	if summary["disabled_tools"].(float64) != 3 {
		t.Fatalf("summary = %#v, want three hard disabled boundaries", summary)
	}
}

func TestAgentToolAuthorizeRouteUsesReviewedEnforcementGate(t *testing.T) {
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test", AdminKey: testAdminKey})

	inferred := doJSONRequest(t, router, http.MethodGet, "/api/agent/tools/add_security/authorize?role=operator&confirmed=true&enforce_reviewed=true", nil, http.StatusOK)
	if inferred["allowed"] != false || inferred["reason"] != "review_required" {
		t.Fatalf("inferred authorization = %#v, want review_required denial", inferred)
	}
	if inferred["review_required"] != true || inferred["permission_source"] != "inferred" {
		t.Fatalf("inferred authorization = %#v, want review metadata", inferred)
	}

	declared := doJSONRequest(t, router, http.MethodGet, "/api/agent/tools/add_transaction/authorize?role=operator&confirmed=true&enforce_reviewed=true", nil, http.StatusOK)
	if declared["allowed"] != true {
		t.Fatalf("declared authorization = %#v, want allowed", declared)
	}
	if declared["review_required"] != false || declared["permission_source"] != "harness" {
		t.Fatalf("declared authorization = %#v, want reviewed harness metadata", declared)
	}

	needsConfirmation := doJSONRequest(t, router, http.MethodGet, "/api/agent/tools/add_transaction/authorize?role=operator", nil, http.StatusOK)
	if needsConfirmation["allowed"] != false || needsConfirmation["requires_confirmation"] != true {
		t.Fatalf("confirmation authorization = %#v, want confirmation denial", needsConfirmation)
	}
	if needsConfirmation["confirmation_ttl_seconds"].(float64) != 900 || needsConfirmation["confirmation_reason"] == "" {
		t.Fatalf("confirmation authorization = %#v, want confirmation instructions", needsConfirmation)
	}

	viewerDenied := doJSONRequest(t, router, http.MethodGet, "/api/agent/tools/crawl_nav/authorize?role=viewer", nil, http.StatusOK)
	if viewerDenied["allowed"] != false || viewerDenied["reason"] != "scope_not_allowed" {
		t.Fatalf("viewer authorization = %#v, want scope denial", viewerDenied)
	}

	disabled := doJSONRequest(t, router, http.MethodGet, "/api/agent/tools/backup_producer/authorize?role=operator&confirmed=true&enforce_reviewed=true", nil, http.StatusOK)
	if disabled["allowed"] != false || disabled["reason"] != "disabled" {
		t.Fatalf("disabled authorization = %#v, want disabled denial", disabled)
	}
}

func TestAgentToolAuthorizeRejectsUnknownRole(t *testing.T) {
	router := NewRouter(config.Config{ServiceName: "fund-dashboard-go", Version: "test", AdminKey: testAdminKey})
	req := httptest.NewRequest(http.MethodGet, "/api/agent/tools/add_transaction/authorize?role=superadmin&confirmed=true", nil)
	req.Header.Set("Authorization", "Bearer "+testAdminKey)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "invalid_role") {
		t.Fatalf("body=%s want invalid_role", res.Body.String())
	}
}
