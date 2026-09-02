package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/config"
	"github.com/DeliciousBuding/fund-dashboard/internal/mcp"
	"github.com/DeliciousBuding/fund-dashboard/internal/oauth"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/go-chi/chi/v5"
)

func registerMCPRoutes(r chi.Router, cfg config.Config, portfolio *portfoliosvc.Service, db *sql.DB, agentOps *agentops.Service, driver string, nav mcp.NavCrawler, snapshots mcp.SnapshotRecalculator, holdings mcp.HoldingsCrawler, mcpLimiter *RateLimiter, oauthSvc *oauth.Service) {
	admin := adminsvc.NewServiceWithDriver(db, driver)

	// Fail-closed bearer auth: MCP_API_KEY (operator) and/or PUBLIC_MCP_KEY (analyst).
	// Per-key limiter is mounted AFTER auth (chi middleware order) so failed
	// auth (401) requests never burn the key's bucket (design 06 §2.3).
	r.With(MCPAuth(cfg.AdminKey, cfg.PublicMCPKey, oauthSvc), RateLimit(mcpLimiter, mcpRateLimitKeyFn)).Post("/mcp", func(w http.ResponseWriter, req *http.Request) {
		role := agenttools.RoleAnalyst
		if scope, ok := mcpAuthFromContext(req.Context()); ok {
			role = scope.Role
		}

		deps := mcp.ServerDeps{
			Portfolio: portfolio,
			Admin:     &admin,
			Nav:       nav,
			Snapshots: snapshots,
			Holdings:  holdings,
			Role:      role,
		}
		// Avoid typed-nil *agentops.Service in confirmationConsumer interface
		// (makes iface==nil false and panics on method dispatch).
		if agentOps != nil {
			deps.AgentOps = agentOps
			// Same service implements mcp.ExecutionAuditSink: execution
			// outcomes persist as event_type "execution" audit rows.
			deps.ExecutionAudit = agentOps
		}
		server, err := mcp.NewServer(deps)
		if err != nil {
			slog.Error("mcp server init", "request_id", RequestIDFromContext(req.Context()), "error", err.Error())
			WriteJSON(w, http.StatusInternalServerError, mcp.Response{
				JSONRPC: "2.0",
				Error:   &mcp.Error{Code: -32000, Message: "internal_error"},
			})
			return
		}

		var request mcp.Request
		req.Body = http.MaxBytesReader(w, req.Body, 2<<20)
		if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
			WriteJSON(w, http.StatusBadRequest, mcp.Response{
				JSONRPC: "2.0",
				Error:   &mcp.Error{Code: -32700, Message: "parse_error"},
			})
			return
		}
		// JSON-RPC notifications (no "id" member, e.g. notifications/initialized,
		// notifications/cancelled, notifications/progress) must not produce a
		// response: swallow with 202 Accepted + empty body (JSON-RPC 2.0 §2.2;
		// MCP spec: "if the message is a notification, the server should return
		// 202 Accepted with no message body"). Requests with an id still get a
		// JSON-RPC envelope, error responses included.
		if mcp.IsNotification(request) {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		// Correlate execution audit rows with the HTTP request id.
		handleCtx := mcp.WithRequestID(req.Context(), RequestIDFromContext(req.Context()))
		WriteJSON(w, http.StatusOK, server.Handle(handleCtx, request))
	})
}

// mcpRateLimitKeyFn derives the per-key bucket key from the Bearer token:
// sha256 前 16 hex — no raw key retained in memory beyond the request (design
// 06 §2.3 per-key 限流)。空 token（必然 401）也不会到达这里(限流在 auth 后)。
func mcpRateLimitKeyFn(r *http.Request) string {
	sum := sha256.Sum256([]byte(bearerToken(r.Header.Get("Authorization"))))
	return "mcp:" + hex.EncodeToString(sum[:])[:16]
}
