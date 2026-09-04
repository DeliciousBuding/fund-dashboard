package httpapi

import (
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/config"
	"github.com/DeliciousBuding/fund-dashboard/internal/dialect"
	"github.com/DeliciousBuding/fund-dashboard/internal/jobs"
	"github.com/DeliciousBuding/fund-dashboard/internal/repository/agentstate"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	"github.com/go-chi/chi/v5"
)

// 工作台系统 API（docs/design/06-security-hardening.md §2.6）。
// 读端点挂 SessionAuth 组；写端点（crawl/verify）挂 BrowserWriteAuth 组
// （session + CSRF 头），由 registerSystemReadRoutes/registerSystemWriteRoutes
// 的调用方负责下装中间件。

// registerSystemReadRoutes mounts the workspace read surface.
func registerSystemReadRoutes(r chi.Router, cfg config.Config, deps *routerDeps, admin adminsvc.Service) {
	r.Get("/api/system/status", handleSystemStatus(cfg, deps, admin))
	r.Get("/api/system/jobs", handleSystemJobs(deps))
	r.Get("/api/system/integrity", handleDBIntegrity(admin))
	r.Get("/api/system/audit", handleSystemAudit(deps))
	r.Get("/api/system/agent", handleSystemAgent(cfg))
}

// registerSystemWriteRoutes mounts the workspace write surface (trigger-type
// endpoints; responses echo the same admin crawl/verify adapters).
func registerSystemWriteRoutes(r chi.Router, cfg config.Config, deps *routerDeps, admin adminsvc.Service) {
	if deps.navCrawler != nil {
		r.Post("/api/system/crawl-nav", navCrawlHandler(deps.navCrawler, &admin))
	}
	if deps.holdings != nil {
		r.Post("/api/system/crawl-holdings", holdingsCrawlHandler(deps.holdings))
	}
	r.Post("/api/system/verify", handleDBIntegrity(admin))
}

// ── GET /api/system/status ─────────────────────────────────────────────

func handleSystemStatus(cfg config.Config, deps *routerDeps, admin adminsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		freshness, err := admin.GetFreshness(r.Context())
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		driver := deps.dbDriver
		if driver == "" {
			driver = "sqlite" // db.Open 的默认(见 internal/repository/db/open.go)
		}
		body := map[string]any{
			"version":    cfg.Version,
			"db_driver":  driver,
			"go_version": runtime.Version(),
			"uptime_sec": int64(time.Since(adminProcessStartedAt).Seconds()),
			"freshness":  map[string]any{"health": freshness.Health},
		}
		// SQLite 附 DB 文件大小（PG 无本地文件，省略字段）。判据必须是上面已解析的
		// driver：cfg.DBDriver 是未解析的原始 FUND_DB_DRIVER（默认部署留空，由
		// app.resolveDriver 推断成 sqlite），拿它当判据会让响应一边报
		// db_driver=sqlite 一边永远不吐 db_size_bytes。
		if strings.EqualFold(driver, "sqlite") && cfg.DBPath != "" {
			info, err := os.Stat(cfg.DBPath)
			if err != nil {
				slog.Warn("system status: db file stat failed", "error", err)
			} else {
				body["db_size_bytes"] = info.Size()
			}
		}
		WriteJSON(w, http.StatusOK, body)
	}
}

// ── GET /api/system/jobs ───────────────────────────────────────────────

func handleSystemJobs(deps *routerDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.jobStatus == nil {
			// scheduler 未接线（测试/最小部署）→ 空清单而非错误。
			WriteJSON(w, http.StatusOK, map[string]any{"jobs": []jobs.JobStatus{}})
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"jobs": deps.jobStatus()})
	}
}

// ── GET /api/system/integrity + POST /api/system/verify ────────────────

// handleDBIntegrity shares the admin db-integrity reader (read-only,
// facts-only). GET /api/system/integrity and POST /api/system/verify both use
// it — POST is the "check now" semantics of the workspace trigger button.
func handleDBIntegrity(service adminsvc.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		report, err := service.GetDBIntegrity(req.Context(), time.Now().UTC())
		if err != nil {
			writeSafeError(w, req, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, report)
	}
}

// ── GET /api/system/audit ──────────────────────────────────────────────

// systemAuditEntry is the merged auth+agent timeline row.
type systemAuditEntry struct {
	Kind    string `json:"kind"` // "auth" | "agent"
	TS      int64  `json:"ts"`
	Event   string `json:"event"`        // auth: event type | agent: tool
	Summary string `json:"summary"`      // auth: detail | agent: "status event_type"
	IP      string `json:"ip,omitempty"` // auth only
}

func handleSystemAudit(deps *routerDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, ok := intQueryOpt(w, r, "limit", 100)
		if !ok {
			return
		}
		if limit <= 0 {
			limit = 100
		}
		if limit > 500 {
			limit = 500
		}

		var out []systemAuditEntry
		if deps.auth != nil {
			events, err := deps.auth.ListAuthEvents(r.Context(), limit)
			if err != nil {
				writeSafeError(w, r, http.StatusInternalServerError, err)
				return
			}
			for _, ev := range events {
				out = append(out, systemAuditEntry{
					Kind:    "auth",
					TS:      ev.TS,
					Event:   ev.Event,
					Summary: ev.Detail,
					IP:      ev.IP,
				})
			}
		}
		// agent_audit_events 可能不存在（旧库/未建表 fixture）——忽略并继续。
		agentEvents, err := agentstate.NewAuditEventRepository(deps.db).List(r.Context(), limit)
		if err != nil && !dialect.IsMissingTableError(err) {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		for _, ev := range agentEvents {
			out = append(out, systemAuditEntry{
				Kind:    "agent",
				TS:      eventTS(ev.CreatedAt),
				Event:   ev.Tool,
				Summary: string(ev.Status) + " " + ev.EventType,
			})
		}

		// 合并时间线：倒序（同秒按 kind 稳定即可），再截断到 limit。
		sort.SliceStable(out, func(i, j int) bool { return out[i].TS > out[j].TS })
		if len(out) > limit {
			out = out[:limit]
		}
		if out == nil {
			out = []systemAuditEntry{}
		}
		WriteJSON(w, http.StatusOK, map[string]any{"events": out})
	}
}

// eventTS parses agent audit created_at (UTC RFC3339Nano) to unix seconds.
func eventTS(createdAt string) int64 {
	t, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// ── GET /api/system/agent ──────────────────────────────────────────────

func handleSystemAgent(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		registry, err := agenttools.DefaultRegistry()
		if err != nil {
			writeSafeError(w, r, http.StatusInternalServerError, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"endpoint":       "/mcp",
			"request_method": "POST",
			"tools":          registry.Summary(),
			"key_env_vars":   []string{"MCP_API_KEY", "PUBLIC_MCP_KEY"},
			"keys": map[string]string{
				"mcp_api_key":    maskSecret(cfg.AdminKey),
				"public_mcp_key": maskSecret(cfg.PublicMCPKey),
			},
		})
	}
}

// maskSecret renders a key as "已配置" / "未配置" only — never expose even a
// fragment of the key material in a response body（设计 06 §2.5）。
func maskSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未配置"
	}
	return "已配置"
}
