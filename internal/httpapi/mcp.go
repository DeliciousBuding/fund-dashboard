package httpapi

import (
	"log/slog"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/DeliciousBuding/fund-dashboard/internal/agentops"
	"github.com/DeliciousBuding/fund-dashboard/internal/agenttools"
	"github.com/DeliciousBuding/fund-dashboard/internal/config"
	"github.com/DeliciousBuding/fund-dashboard/internal/mcp"
	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	portfoliosvc "github.com/DeliciousBuding/fund-dashboard/internal/service/portfolio"
	"github.com/go-chi/chi/v5"
)

func registerMCPRoutes(r chi.Router, cfg config.Config, portfolio *portfoliosvc.Service, db *sql.DB, agentOps *agentops.Service, driver string, nav mcp.NavCrawler, snapshots mcp.SnapshotRecalculator, holdings mcp.HoldingsCrawler) {
	admin := adminsvc.NewServiceWithDriver(db, driver)

	// Fail-closed bearer auth: MCP_API_KEY (operator) and/or PUBLIC_MCP_KEY (analyst).
	r.With(MCPAuth(cfg.AdminKey, cfg.PublicMCPKey)).Post("/mcp", func(w http.ResponseWriter, req *http.Request) {
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
		WriteJSON(w, http.StatusOK, server.Handle(req.Context(), request))
	})
}
