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
	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
	"github.com/DeliciousBuding/fund-dashboard/internal/mcp"
	"github.com/DeliciousBuding/fund-dashboard/internal/oauth"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/go-chi/chi/v5"
)

type RouterOption func(*routerDeps)

type routerDeps struct {
	portfolio  *portfoliosvc.Service
	agentOps   *agentops.Service
	auth       *auth.Service
	staticFS   fs.FS
	db         *sql.DB
	dbDriver   string
	navCrawler mcp.NavCrawler // optional, for MCP crawl_nav
	snapshots  mcp.SnapshotRecalculator
	holdings   mcp.HoldingsCrawler
	jobStatus  func() []jobs.JobStatus // optional, scheduler runtime snapshot
	oauthSvc   *oauth.Service          // optional, OAuth 2.1 server for MCP connectors
}

func WithDB(db *sql.DB) RouterOption {
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

// WithOAuth wires the OAuth 2.1 authorization server that lets remote MCP
// clients (ChatGPT custom connectors, Claude, Cursor) authenticate with scoped
// bearer tokens instead of static API keys.
func WithOAuth(svc *oauth.Service) RouterOption {
	return func(deps *routerDeps) { deps.oauthSvc = svc }
}

// WithJobStatus wires the scheduler runtime snapshot for GET /api/system/jobs.
// nil-safe: routes without it return an empty job list.
func WithJobStatus(fn func() []jobs.JobStatus) RouterOption {
	return func(deps *routerDeps) { deps.jobStatus = fn }
}

func NewRouter(cfg config.Config, opts ...RouterOption) http.Handler {
	deps := &routerDeps{}
	for _, opt := range opts {
		opt(deps)
	}

	// One fact, three surfaces. MCP tools/list derives its advertisement guard from
	// the AgentOps pointer it is handed; the harness snapshot and the agent-context
	// pack are built inside the portfolio service, so hand the same fact to it here
	// rather than letting each surface guess. Nil AgentOps means every
	// confirmation-gated tool would fail closed, so none of them may be advertised.
	if deps.portfolio != nil {
		deps.portfolio.SetConfirmationFlowAvailable(deps.agentOps != nil)
	}

	r := chi.NewRouter()
	r.Use(RequestID)
	r.Use(Recoverer)
	r.Use(SecurityHeaders(cfg.AuthSecureCookie))
	r.Use(AccessLog)

	// 通用 API 限流（design 06 §2.3）：/api/* 组最外层 per-IP 600/min（burst 60，
	// FUND_API_RPM 可调）；昂贵端点额外 60/min 桶。MCP 按 key 在 registerMCPRoutes
	// 内干（认证失败 401 不计费）。
	apiLimiter := NewRateLimiter(float64(cfg.APIRPM), 60)
	expensiveLimiter := NewRateLimiter(60, 60)
	apiKeyFn := func(req *http.Request) string { return "ip:" + clientIP(req, cfg.TrustedProxies) }

	// Everything under /api/* lives inside this group so the limiters run before
	// auth middleware — unauthenticated scans burn tokens too.
	r.Group(func(api chi.Router) {
		api.Use(RateLimit(apiLimiter, apiKeyFn))
		api.Use(rateLimitExpensive(expensiveLimiter, expensiveAPIPaths, apiKeyFn))

		// Public.
		api.Get("/api/health", healthHandler(cfg))

		// Single-tenant auth: /api/auth/status|setup|login are public (rate-limited),
		// logout/password/sessions/events require a session.
		if deps.auth != nil {
			registerAuthRoutes(api, deps.auth, cfg.AuthSecureCookie, cfg.AllowedOrigins, cfg.TrustedProxies)
		}

		// Agent tool registry / authorize simulation — operator Bearer only.
		// Not an execution path, but must not be a public reconnaissance surface.
		api.Group(func(agentTools chi.Router) {
			agentTools.Use(AdminAuth(cfg.AdminKey))
			agentTools.Get("/api/agent/tools", handleAgentTools())
			agentTools.Get("/api/agent/tools/summary", handleAgentToolsSummary())
			agentTools.Get("/api/agent/tools/{tool}/authorize", handleAgentToolAuthorize())
		})

		// Optional: agent ops (requires explicit config enable).
		if deps.agentOps != nil {
			api.Group(func(agent chi.Router) {
				agent.Use(AdminAuth(cfg.AdminKey))
				registerAgentConfirmationRoutes(agent, deps.agentOps)
			})
		}

		// Portfolio & market reads require a session (single-tenant: everything sits
		// behind the login). Browser writes accept session (preferred) or the legacy
		// edge-injected EdgeKey while FUND_EDGE_AUTH_ENABLED stays on.
		if deps.portfolio != nil {
			api.Group(func(authed chi.Router) {
				authed.Use(SessionAuth(deps.auth, cfg.AllowedOrigins))
				registerExportRoutes(authed)
				registerPortfolioRoutes(authed, deps.portfolio)
				registerFundRoutes(authed, deps.portfolio)
				registerMarketRoutes(authed, deps.portfolio)
				registerAnalysisRoutes(authed, deps.portfolio)
			})
			if deps.db != nil {
				adminForSPA := adminsvc.NewServiceWithDriver(deps.db, deps.dbDriver)
				api.Group(func(authedExt chi.Router) {
					authedExt.Use(SessionAuth(deps.auth, cfg.AllowedOrigins))
					registerSPAReadExtensions(authedExt, deps.portfolio, adminForSPA)
					registerSystemReadRoutes(authedExt, cfg, deps, adminForSPA)
				})
				api.Group(func(browserWrites chi.Router) {
					browserWrites.Use(BrowserWriteAuth(deps.auth, cfg.EdgeKey, cfg.EdgeAuthEnabled, cfg.AllowedOrigins))
					registerSPATransactionRoutes(browserWrites, adminForSPA)
					registerOpsDashboardRoutes(browserWrites, deps.db, deps.dbDriver, cfg.Version)
					registerPortfolioWriteRoutes(browserWrites, deps.portfolio)
					registerSPAWriteExtensions(browserWrites, deps.portfolio)
					registerSystemWriteRoutes(browserWrites, cfg, deps, adminForSPA)
				})
			}
		}

		// Admin — protected by Bearer token auth (ops / agent tools).
		api.Route("/api/admin", func(admin chi.Router) {
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
			}
			if deps.holdings != nil {
				admin.Post("/crawl-holdings", holdingsCrawlHandler(deps.holdings))
			}
			if deps.snapshots != nil {
				admin.Post("/recalculate-snapshot", recalculateSnapshotHandler(deps.snapshots))
			}
		})
	})

	// OAuth discovery + endpoints. Mounted before the SPA fallback: the fallback
	// answers any unknown path with index.html, which would turn a well-known
	// metadata probe into "HTTP 200 + HTML" and silently break connector setup.
	if deps.oauthSvc != nil {
		oauthLimiter := NewRateLimiter(float64(cfg.OAuthRPM), 30)
		oauthIPKey := func(req *http.Request) string { return "ip:" + clientIP(req, cfg.TrustedProxies) }
		registerOAuthRoutes(r, deps.oauthSvc, deps.auth, oauthLimiter, oauthIPKey)
	}

	// MCP stays outside the per-IP group: it has its own per-key limiter
	// (see registerMCPRoutes), mounted after Bearer auth so 401s don't burn tokens.
	if deps.portfolio != nil && deps.db != nil {
		mcpLimiter := NewRateLimiter(float64(cfg.MCPRPM), 60)
		registerMCPRoutes(r, cfg, deps.portfolio, deps.db, deps.agentOps, deps.dbDriver, deps.navCrawler, deps.snapshots, deps.holdings, mcpLimiter, deps.oauthSvc)
	}

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
