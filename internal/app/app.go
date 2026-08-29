// Package app provides the production dependency assembly path.
// Build opens SQLite, creates data sources, starts the scheduler, wires HTTP routes,
// and returns a Runtime that can be gracefully shut down.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
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

func Build(ctx context.Context, cfg config.Config) (*Runtime, error) {
	dbase, err := db.Open(ctx, db.Options{
		Driver:     cfg.DBDriver,
		SQLitePath: cfg.DBPath,
		DSN:        cfg.PGDSN,
	})
	if err != nil {
		return nil, err
	}

	// On PostgreSQL, ensure the schema exists before startup.
	if cfg.DBDriver == "pg" {
		if err := db.EnsurePGSchema(ctx, dbase); err != nil {
			_ = dbase.Close()
			return nil, fmt.Errorf("ensure pg schema: %w", err)
		}
	}

	runtime, err := buildWithDB(ctx, cfg, dbase)
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

func buildWithDB(ctx context.Context, cfg config.Config, db *sql.DB) (*Runtime, error) {
	// ── agent state tables ───────────────────────────────────────────
	confirmationRepo := agentstate.NewConfirmationRepository(db)
	auditRepo := agentstate.NewAuditEventRepository(db)

	// On SQLite, EnsureSchema creates agent tables with AUTOINCREMENT.
	// On PG, EnsurePGSchema already created them with SERIAL PRIMARY KEY.
	if cfg.DBDriver != "pg" {
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
	refresher := jobs.NewPriceRefresher(db,
		jobs.WithDBDriver(cfg.DBDriver),
		jobs.WithSource(datasource.TypeFund, fundSource),
		jobs.WithSource(datasource.TypeStock, stockSource),
	)

	// ── single-tenant auth (web login) ─────────────────────────────────
	// On PG the auth tables are created by EnsurePGSchema; on SQLite we create
	// them here (same pattern as agentstate). The store also backs the daily
	// auth_events sweep wired into the scheduler below.
	authStore := authpkg.NewStore(db)
	if cfg.DBDriver != "pg" {
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
	scheduler := jobs.NewScheduler(refresher, db)
	scheduler.WithAuthEventSweeper(authStore)
	scheduler.Start()

	// ── HTTP router ──────────────────────────────────────────────────
	options := []httpapi.RouterOption{
		httpapi.WithDB(db),
		httpapi.WithDBDriver(cfg.DBDriver),
		httpapi.WithAuth(authService),
	}

	if cfg.StaticDir != "" {
		if _, err := os.Stat(cfg.StaticDir); err != nil {
			return nil, fmt.Errorf("open static dir: %w", err)
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

	// Crawl handler — thin adapter from HTTP to the job runner.
	options = append(options, httpapi.WithCrawlHandler(
		crawlHandler(refresher),
	))
	// MCP crawl_nav uses the same PriceRefresher instance.
	options = append(options, httpapi.WithNavCrawler(refresher))
	options = append(options, httpapi.WithSnapshotRecalculator(jobs.NewSnapshotService(db)))
	options = append(options, httpapi.WithHoldingsCrawler(jobs.NewHoldingsRefresher(db)))
	// Workspace system API: scheduler runtime snapshot for /api/system/jobs.
	options = append(options, httpapi.WithJobStatus(scheduler.StatusSnapshot))

	return &Runtime{
		DB:        db,
		Handler:   httpapi.NewRouter(cfg, options...),
		scheduler: scheduler,
	}, nil
}

// ── helpers ─────────────────────────────────────────────────────────────

func crawlHandler(refresher *jobs.PriceRefresher) http.HandlerFunc {
	type crawlResponse struct {
		Status     string `json:"status"`
		Securities int    `json:"securities"`
		Added      int    `json:"added"`
		Error      string `json:"error,omitempty"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		securities, added, err := refresher.RefreshAllHeld(r.Context())
		resp := crawlResponse{Securities: securities, Added: added}
		if err != nil {
			// Never leak driver/SQL detail to clients (#233; align safeAdminOpError).
			slog.Error("crawl-nav legacy handler",
				"request_id", httpapi.RequestIDFromContext(r.Context()),
				"path", r.URL.Path,
				"error", err.Error(),
			)
			resp.Status = "error"
			resp.Error = "internal_error"
			httpapi.WriteJSON(w, http.StatusInternalServerError, resp)
			return
		}
		resp.Status = "complete"
		httpapi.WriteJSON(w, http.StatusOK, resp)
	}
}

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
