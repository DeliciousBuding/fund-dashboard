package app

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DeliciousBuding/fund-dashboard/internal/config"
)

func TestBuildWiresProductionRuntimeWithAgentOpsDisabledByDefault(t *testing.T) {
	ctx := context.Background()
	built := buildRuntime(t, config.Config{
		DBPath:      filepath.Join(t.TempDir(), "fund.db"),
		ServiceName: "fund-dashboard-go",
		Version:     "test",
	})
	defer built.Close()

	assertAgentStateSchemaExists(t, built.DB)

	health := httptest.NewRecorder()
	built.Handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200; body=%s", health.Code, health.Body.String())
	}
	if !strings.Contains(health.Body.String(), `"backup_producer_enabled":false`) {
		t.Fatalf("health did not expose backup disabled boundary: %s", health.Body.String())
	}

	prepare := httptest.NewRecorder()
	built.Handler.ServeHTTP(prepare, httptest.NewRequest(http.MethodPost, "/api/agent/confirmations/prepare", strings.NewReader(`{}`)))
	if prepare.Code != http.StatusNotFound {
		t.Fatalf("prepare status = %d, want 404 when agent ops disabled; body=%s", prepare.Code, prepare.Body.String())
	}

	if err := built.DB.PingContext(ctx); err != nil {
		t.Fatalf("runtime DB ping failed: %v", err)
	}
}

func TestBuildWiresOptInAgentOpsConfirmationRoutes(t *testing.T) {
	built := buildRuntime(t, config.Config{
		DBPath:                  filepath.Join(t.TempDir(), "fund.db"),
		ServiceName:             "fund-dashboard-go",
		Version:                 "test",
		AgentOpsEnabled:         true,
		AgentConfirmationSecret: "test-secret",
		AdminKey:                "test-admin-key",
	})
	defer built.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/agent/confirmations/prepare", strings.NewReader(`{
		"tool":"add_transaction",
		"role":"operator",
		"caller":"app-test",
		"request_id":"req-app-1",
		"payload":{"fund_code":"AAPL","amount":1},
		"enforce_reviewed":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-admin-key")
	res := httptest.NewRecorder()
	built.Handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("prepare status = %d, want 201 when agent ops enabled; body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"decision_boundary":"confirmation_only"`) {
		t.Fatalf("prepare response missing confirmation boundary: %s", res.Body.String())
	}
}

func TestBuildWiresConfiguredStaticSPA(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte(`<div id="root"></div>`), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	built := buildRuntime(t, config.Config{
		DBPath:      filepath.Join(t.TempDir(), "fund.db"),
		ServiceName: "fund-dashboard-go",
		Version:     "test",
		StaticDir:   staticDir,
	})
	defer built.Close()

	res := httptest.NewRecorder()
	built.Handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("root status = %d, want 200; body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `<div id="root"></div>`) {
		t.Fatalf("root body missing SPA index: %s", res.Body.String())
	}
}

func buildRuntime(t *testing.T, cfg config.Config) *Runtime {
	t.Helper()
	built, err := Build(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	return built
}

func assertAgentStateSchemaExists(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"agent_confirmations", "agent_audit_events"} {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("agent state table %s missing: %v", table, err)
		}
	}
}

func TestResolveDriver(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{"explicit sqlite", config.Config{DBDriver: "sqlite"}, "sqlite"},
		{"explicit pg", config.Config{DBDriver: "pg"}, "pg"},
		{"empty with pg dsn infers pg", config.Config{PGDSN: "postgres://example"}, "pg"},
		{"empty without dsn defaults sqlite", config.Config{}, "sqlite"},
		{"explicit driver wins over dsn", config.Config{DBDriver: "sqlite", PGDSN: "postgres://example"}, "sqlite"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveDriver(tc.cfg); got != tc.want {
				t.Fatalf("resolveDriver() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildRejectsUnknownDriver(t *testing.T) {
	_, err := Build(context.Background(), config.Config{
		DBDriver: "mysql",
		DBPath:   filepath.Join(t.TempDir(), "fund.db"),
	})
	if err == nil {
		t.Fatal("Build with unknown driver = nil error, want fail-closed dialect resolution")
	}
	if !strings.Contains(err.Error(), "mysql") {
		t.Fatalf("Build error = %q, want unsupported driver name", err)
	}
}

// The admin service legacy entrypoint NewServiceWithDriver panics on unknown
// drivers (fail-closed shell). The production assembly must keep validating
// the driver with dialect.NewChecked before wiring the router, otherwise an
// invalid FUND_DB_DRIVER could surface as a panic instead of a boot error.
func TestBootValidatesDriverBeforeRouterWiring(t *testing.T) {
	raw, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatalf("read app.go: %v", err)
	}
	src := string(raw)
	checkedAt := strings.Index(src, "dialect.NewChecked(driver")
	routerAt := strings.Index(src, "httpapi.NewRouter(")
	if checkedAt < 0 {
		t.Fatal("app.go no longer validates the driver via dialect.NewChecked")
	}
	if routerAt < 0 {
		t.Fatal("app.go no longer wires httpapi.NewRouter")
	}
	if checkedAt > routerAt {
		t.Fatal("driver validation must happen before router wiring")
	}
}
