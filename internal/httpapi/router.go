// Package httpapi provides thin REST handlers and middleware. Business logic lives in
// internal/service; handlers parse HTTP parameters, call services, and write JSON responses.
package httpapi

import (
	"database/sql"
	"io/fs"
	"net/http"
	"strings"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/auth"
	"github.com/DeliciousBuding/fund-dashboard/internal/config"
	"github.com/DeliciousBuding/fund-dashboard/internal/mcp"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/go-chi/chi/v5"
)

// DB is the minimal database handle needed by HTTP handlers.
type DB = *sql.DB

type RouterOption func(*routerDeps)

type routerDeps struct {
	portfolio    *portfoliosvc.Service
	agentOps     *agentops.Service
	auth         *auth.Service
	staticFS     fs.FS
	db           DB
	dbDriver     string
	crawlHandler http.HandlerFunc // optional, registered when set
	navCrawler   mcp.NavCrawler   // optional, for MCP crawl_nav
	snapshots    mcp.SnapshotRecalculator
	holdings     mcp.HoldingsCrawler
}

func WithDB(db DB) RouterOption {
	return func(deps *routerDeps) {
		service := portfoliosvc.NewService(db)
		deps.portfolio = &service
		deps.db = db
	}
}

// WithDBDriver sets the database driver hint ("sqlite" or "pg") so that
// admin services can generate dialect-correct SQL (e.g. PRAGMA vs pg_catalog).
func WithDBDriver(driver string) RouterOption {
	return func(deps *routerDeps) { deps.dbDriver = driver }
}

func WithAgentOps(service *agentops.Service) RouterOption {
	return func(deps *routerDeps) { deps.agentOps = service }
}

func WithStaticFS(staticFS fs.FS) RouterOption {
	return func(deps *routerDeps) { deps.staticFS = staticFS }
}

// WithAuth wires the single-tenant auth service: /api/auth/* routes plus
// session gating on reads and browser writes.
func WithAuth(svc *auth.Service) RouterOption {
	return func(deps *routerDeps) { deps.auth = svc }
}

// WithCrawlHandler registers POST /api/admin/crawl-nav with the given handler.
// The handler is expected to call the price-refresh job and return JSON.
func WithCrawlHandler(fn http.HandlerFunc) RouterOption {
	return func(deps *routerDeps) { deps.crawlHandler = fn }
}

// WithNavCrawler wires the NAV/price crawler into MCP crawl_nav.
func WithNavCrawler(nav mcp.NavCrawler) RouterOption {
	return func(deps *routerDeps) { deps.navCrawler = nav }
}

func WithSnapshotRecalculator(s mcp.SnapshotRecalculator) RouterOption {
	return func(deps *routerDeps) { deps.snapshots = s }
}

func WithHoldingsCrawler(h mcp.HoldingsCrawler) RouterOption {
	return func(deps *routerDeps) { deps.holdings = h }
}

func NewRouter(cfg config.Config, opts ...RouterOption) http.Handler {
	deps := &routerDeps{}
	for _, opt := range opts {
		opt(deps)
	}

	r := chi.NewRouter()
	r.Use(RequestID)
	r.Use(Recoverer)
	r.Use(SecurityHeaders)
	r.Use(AccessLog)

	// Public.
	r.Get("/api/health", healthHandler(cfg))

	// Single-tenant auth: /api/auth/status|setup|login are public (rate-limited),
	// logout/password/sessions require a session.
	if deps.auth != nil {
		registerAuthRoutes(r, deps.auth, cfg.AuthSecureCookie, cfg.AllowedOrigins)
	}

	// Agent tool registry / authorize simulation — operator Bearer only.
	// Not an execution path, but must not be a public reconnaissance surface.
	r.Group(func(agentTools chi.Router) {
		agentTools.Use(AdminAuth(cfg.AdminKey))
		agentTools.Get("/api/agent/tools", handleAgentTools())
		agentTools.Get("/api/agent/tools/summary", handleAgentToolsSummary())
		agentTools.Get("/api/agent/tools/{tool}/authorize", handleAgentToolAuthorize())
	})

	// Optional: agent ops (requires explicit config enable).
	if deps.agentOps != nil {
		r.Group(func(agent chi.Router) {
			agent.Use(AdminAuth(cfg.AdminKey))
			registerAgentConfirmationRoutes(agent, deps.agentOps)
		})
	}

	// Portfolio & market reads require a session (single-tenant: everything sits
	// behind the login). Browser writes accept session (preferred) or the legacy
	// edge-injected EdgeKey while FUND_EDGE_AUTH_ENABLED stays on.
	if deps.portfolio != nil {
		r.Group(func(authed chi.Router) {
			authed.Use(SessionAuth(deps.auth, cfg.AllowedOrigins))
			registerExportRoutes(authed)
			registerPortfolioRoutes(authed, deps.portfolio)
			registerFundRoutes(authed, deps.portfolio)
			registerMarketRoutes(authed, deps.portfolio)
			registerAnalysisRoutes(authed, deps.portfolio)
		})
		if deps.db != nil {
			r.Group(func(browserWrites chi.Router) {
				browserWrites.Use(BrowserWriteAuth(deps.auth, cfg.EdgeKey, cfg.EdgeAuthEnabled, cfg.AllowedOrigins))
				registerSPATransactionRoutes(browserWrites, adminsvc.NewServiceWithDriver(deps.db, deps.dbDriver))
				registerOpsDashboardRoutes(browserWrites, deps.db, deps.dbDriver, cfg.Version)
				registerPortfolioWriteRoutes(browserWrites, deps.portfolio)
			})
			registerMCPRoutes(r, cfg, deps.portfolio, deps.db, deps.agentOps, deps.dbDriver, deps.navCrawler, deps.snapshots, deps.holdings)
		}
	}

	// Admin — protected by Bearer token auth (ops / agent tools).
	r.Route("/api/admin", func(admin chi.Router) {
		admin.Use(AdminAuth(cfg.AdminKey))

		// Always available — no DB required.
		registerBackupStatusRoutes(admin, cfg)

		if deps.db != nil {
			adminService := adminsvc.NewServiceWithDriver(deps.db, deps.dbDriver)
			registerAdminDashboardRoutes(admin, deps.db, deps.dbDriver, cfg.Version)
			registerAdminFreshnessRoutes(admin, deps.db, deps.dbDriver)
			registerAdminHoldingsCoverageRoutes(admin, deps.db, deps.dbDriver)
			registerAdminStatusRoutes(admin, deps.db, deps.dbDriver)
			registerAdminTransactionRoutes(admin, adminService)
			registerAdminVerifyRoutes(admin, deps.db, deps.dbDriver)
			registerAdminIntegrityRoutes(admin, deps.db, deps.dbDriver)

		}
		// Maintenance crawlers only need the job adapters (DB owned by jobs).
		var adminForCrawl *adminsvc.Service
		if deps.db != nil {
			svc := adminsvc.NewServiceWithDriver(deps.db, deps.dbDriver)
			adminForCrawl = &svc
		}
		if deps.navCrawler != nil {
			admin.Post("/crawl-nav", navCrawlHandler(deps.navCrawler, adminForCrawl))
		} else if deps.crawlHandler != nil {
			// legacy all-held-only adapter
			admin.Post("/crawl-nav", deps.crawlHandler)
		}
		if deps.holdings != nil {
			admin.Post("/crawl-holdings", holdingsCrawlHandler(deps.holdings))
		}
		if deps.snapshots != nil {
			admin.Post("/recalculate-snapshot", recalculateSnapshotHandler(deps.snapshots))
		}
	})

	// SPA fallback — must be last.
	if deps.staticFS != nil {
		registerStaticRoutes(r, deps.staticFS)
	}

	return r
}

func healthHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Production health is anonymous at the edge — do not leak git SHA / image tag.
		// Non-production keeps version for local smoke and agent diagnostics.
		body := map[string]any{
			"status":                  "ok",
			"service":                 cfg.ServiceName,
			"facts_only":              true,
			"backup_producer_enabled": false,
		}
		if !isProductionEnvironment(cfg.Environment) {
			body["version"] = cfg.Version
		}
		WriteJSON(w, http.StatusOK, body)
	}
}

func isProductionEnvironment(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "production", "prod":
		return true
	default:
		return false
	}
}

// ── agent tools (AdminAuth / operator Bearer) ──────────────────────────────

func handleAgentTools() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		registry, err := agenttools.DefaultRegistry()
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, registry)
	}
}

func handleAgentToolsSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		registry, err := agenttools.DefaultRegistry()
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, registry.Summary())
	}
}

func handleAgentToolAuthorize() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		registry, err := agenttools.DefaultRegistry()
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		role, ok := parseAgentRole(r.URL.Query().Get("role"))
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_role")
			return
		}
		decision := registry.Authorize(agenttools.AuthorizeRequest{
			Tool:            chi.URLParam(r, "tool"),
			Role:            role,
			Confirmed:       boolQuery(r, "confirmed"),
			EnforceReviewed: boolQuery(r, "enforce_reviewed"),
		})
		WriteJSON(w, http.StatusOK, decision)
	}
}

// parseAgentRole accepts only known roles; empty defaults to viewer for simulation (#213).
func parseAgentRole(raw string) (agenttools.Role, bool) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "viewer":
		return agenttools.RoleViewer, true
	case "analyst":
		return agenttools.RoleAnalyst, true
	case "operator":
		return agenttools.RoleOperator, true
	default:
		return "", false
	}
}
