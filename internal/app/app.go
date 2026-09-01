// Package app provides the production dependency assembly path.
// Build opens the configured database (SQLite or PostgreSQL), creates data
// sources, starts the scheduler, wires HTTP routes, and returns a Runtime that
// can be gracefully shut down.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	authpkg "github.com/DeliciousBuding/fund-dashboard/internal/auth"
	"github.com/DeliciousBuding/fund-dashboard/internal/config"
	"github.com/DeliciousBuding/fund-dashboard/internal/confirmations"
	"github.com/DeliciousBuding/fund-dashboard/internal/datasource"
	"github.com/DeliciousBuding/fund-dashboard/internal/httpapi"
	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/agentstate"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/db"
	"github.com/DeliciousBuding/fund-dashboard/internal/webui"
)

type Runtime struct {
	DB        *sql.DB
	Handler   http.Handler
	scheduler *jobs.Scheduler
}

// resolveDriver mirrors db.Open's driver inference so schema bootstrapping and
// every downstream driver option agree on one value for the process lifetime.
// config.Parse already lowercases and trims FUND_DB_DRIVER; an empty value means
// "infer from the connection inputs" exactly like db.Open.
func resolveDriver(cfg config.Config) string {
	if cfg.DBDriver != "" {
		return cfg.DBDriver
	}
	if cfg.PGDSN != "" {
		return "pg"
	}
	return "sqlite"
}

func Build(ctx context.Context, cfg config.Config) (*Runtime, error) {
	driver := resolveDriver(cfg)
	dbase, err := db.Open(ctx, db.Options{
		Driver:     driver,
		SQLitePath: cfg.DBPath,
		DSN:        cfg.PGDSN,
	})
	if err != nil {
		return nil, err
	}

	// On PostgreSQL, ensure the schema exists before startup.
	if driver == "pg" {
		if err := db.EnsurePGSchema(ctx, dbase); err != nil {
			_ = dbase.Close()
			return nil, fmt.Errorf("ensure pg schema: %w", err)
		}
	} else {
		// SQLite first install: a fresh self-hosted DB must boot into a usable
		// empty state, not a wall of internal_error 500s. Agent and auth tables
		// come from buildWithDB's EnsureSchema calls.
		if err := db.EnsureSQLiteSchema(ctx, dbase); err != nil {
			_ = dbase.Close()
			return nil, fmt.Errorf("ensure sqlite schema: %w", err)
		}
	}

	runtime, err := buildWithDB(ctx, cfg, driver, dbase)
	if err != nil {
		_ = dbase.Close()
		return nil, err
	}
	return runtime, nil
}

func (r *Runtime) Close() error {
	if r.scheduler != nil {
		r.scheduler.Stop()
	}
	if r.DB != nil {
		return r.DB.Close()
	}
	return nil
}

func buildWithDB(ctx context.Context, cfg config.Config, driver string, dbase *sql.DB) (*Runtime, error) {
	// ── agent state tables ───────────────────────────────────────────
	confirmationRepo := agentstate.NewConfirmationRepository(dbase)
	auditRepo := agentstate.NewAuditEventRepository(dbase)

	// On SQLite, EnsureSchema creates agent tables with AUTOINCREMENT.
	// On PG, EnsurePGSchema already created them with SERIAL PRIMARY KEY.
	if driver != "pg" {
		if err := confirmationRepo.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		if err := auditRepo.EnsureSchema(ctx); err != nil {
			return nil, err
		}
	}

	// ── data sources ─────────────────────────────────────────────────
	fundSource := datasource.NewEastmoneyFund()
	stockSource := datasource.NewYahooStock()
	refresher := jobs.NewPriceRefresher(dbase,
		jobs.WithDBDriver(driver),
		jobs.WithSource(datasource.TypeFund, fundSource),
		jobs.WithSource(datasource.TypeStock, stockSource),
	)

	// ── single-tenant auth (web login) ─────────────────────────────────
	// On PG the auth tables are created by EnsurePGSchema; on SQLite we create
	// them here (same pattern as agentstate). The store also backs the daily
	// auth_events sweep wired into the scheduler below.
	authStore := authpkg.NewStore(dbase)
	if driver != "pg" {
		if err := authStore.EnsureSchema(ctx); err != nil {
			return nil, fmt.Errorf("ensure auth schema: %w", err)
		}
	}
	authService := authpkg.NewService(authStore, authpkg.Options{
		EnvHash: cfg.AuthPasswordHash,
		TTL:     cfg.AuthSessionTTL,
		MaxAge:  cfg.AuthSessionMaxAge,
	})

	// ── scheduler ────────────────────────────────────────────────────
	// Started only after the router is fully wired so a wiring error cannot
	// leave background jobs running against a database that Build closes.
	scheduler := jobs.NewScheduler(refresher, dbase)
	scheduler.WithAuthEventSweeper(authStore)
	scheduler.WithAuthSessionSweeper(authService)

	// ── HTTP router ──────────────────────────────────────────────────
	options := []httpapi.RouterOption{
		httpapi.WithDB(dbase),
		httpapi.WithDBDriver(driver),
		httpapi.WithAuth(authService),
	}

	if cfg.StaticDir != "" {
		info, err := os.Stat(cfg.StaticDir)
		if err != nil {
			return nil, fmt.Errorf("open static dir: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("static dir is not a directory: %s", cfg.StaticDir)
		}
		options = append(options, httpapi.WithStaticFS(os.DirFS(cfg.StaticDir)))
	} else {
		// Default: embedded SPA (dist/ when built, placeholder otherwise).
		options = append(options, httpapi.WithStaticFS(webui.FS()))
	}

	if cfg.AgentOpsEnabled {
		agentOps, err := newAgentOpsService(cfg, confirmationRepo, auditRepo)
		if err != nil {
			return nil, err
		}
		options = append(options, httpapi.WithAgentOps(agentOps))
	}

	// MCP crawl_nav uses the same PriceRefresher instance.
	options = append(options, httpapi.WithNavCrawler(refresher))
	options = append(options, httpapi.WithSnapshotRecalculator(jobs.NewSnapshotService(dbase)))
	options = append(options, httpapi.WithHoldingsCrawler(jobs.NewHoldingsRefresher(dbase)))
	// Workspace system API: scheduler runtime snapshot for /api/system/jobs.
	options = append(options, httpapi.WithJobStatus(scheduler.StatusSnapshot))

	handler := httpapi.NewRouter(cfg, options...)
	scheduler.Start()

	return &Runtime{
		DB:        dbase,
		Handler:   handler,
		scheduler: scheduler,
	}, nil
}

// ── helpers ─────────────────────────────────────────────────────────────

func newAgentOpsService(
	cfg config.Config,
	confirmationRepo agentstate.ConfirmationRepository,
	auditRepo agentstate.AuditEventRepository,
) (*agentops.Service, error) {
	registry, err := agenttools.DefaultRegistry()
	if err != nil {
		return nil, fmt.Errorf("load agent tool registry: %w", err)
	}
	manager, err := confirmations.NewManager([]byte(cfg.AgentConfirmationSecret))
	if err != nil {
		return nil, fmt.Errorf("build confirmation manager: %w", err)
	}
	return agentops.NewService(agentops.ServiceDeps{
		Registry:         registry,
		Confirmations:    manager,
		ConfirmationRepo: confirmationRepo,
		AuditRepo:        auditRepo,
	}), nil
}
