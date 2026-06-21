/** Fund Dashboard Backend — Bun + Hono + native SQLite
 *  v2.3 — Full penetration analysis: portfolio/penetration with sector breakdown,
 *         sector_map table, Yahoo Finance US stocks & indices, MCP sector awareness.
 *
 *  Start: bun main.ts
 *  Dev:   bun --watch main.ts
 *  Port:  8765 (set PORT env to override)
 */

import { Hono } from "hono";
import { cors } from "hono/cors";
import portfolioRoutes from "./routes/portfolio";
import fundRoutes from "./routes/funds";
import analysisRoutes from "./routes/analysis";
import adminRoutes from "./routes/admin";
import marketRoutes from "./routes/market";
import stockRoutes from "./routes/stocks";
import sseRoutes from "./routes/sse";
import reportRoutes from "./routes/report";
import exportRoutes from "./routes/export";
import dashboardRoutes from "./routes/admin/dashboard";
import docsRoutes from "./routes/docs";
import { log, reqLogger } from "./middleware/logger";
import { rateLimiter } from "./middleware/rate-limit";
import { getDb, initSchema, getRwDb } from "./db";
import { mcpHandler } from "./mcp/server";
import { startScheduler } from "./crawler/scheduler";
import { startIntegrityMonitor } from "./services/db-integrity";
import { ApiError } from "./utils/errors";

const app = new Hono();
const API_KEY = process.env.MCP_API_KEY;           // admin scope — REST /api/admin + full MCP
const PUBLIC_MCP_KEY = process.env.PUBLIC_MCP_KEY;  // public scope — MCP only (your-mcp-domain.example.com)

// ── Auth helper ─────────────────────────────────────────────────────
// Two key scopes (see projects/fund-dashboard/active/2026-06-21-mcp-public-exposure.md):
//   "admin"  — MCP_API_KEY: full control (REST admin + all 34 MCP tools)
//   "public" — PUBLIC_MCP_KEY: MCP tools only, for the public exposure. Physically
//              isolated from admin so a leaked public key can be rotated/revoked
//              without touching admin access.
function requireKey(c: any): "admin" | "public" | null {
  const auth = c.req.header("Authorization") || "";
  const token = auth.startsWith("Bearer ") ? auth.slice(7) : null;
  if (token && API_KEY && token === API_KEY) return "admin";
  if (token && PUBLIC_MCP_KEY && token === PUBLIC_MCP_KEY) return "public";
  return null;
}

/** Admin-only guard for REST /api/admin/*. A public key must NOT reach admin REST. */
function requireAdmin(c: any): boolean {
  if (!API_KEY) {
    log.warn("MCP_API_KEY not configured — admin endpoints disabled");
    return false;
  }
  if (requireKey(c) === "admin") return true;
  log.warn("admin auth failed", { path: c.req.path });
  return false;
}

/** Best-effort client IP (nginx sets X-Real-IP; fall back to XFF). */
function clientIp(c: any): string {
  return c.req.header("x-real-ip")
    || (c.req.header("x-forwarded-for") || "").split(",")[0].trim()
    || "unknown";
}

// ── Middleware ──────────────────────────────────────────────────────
app.use(cors({ origin: ["http://localhost:5176", "http://127.0.0.1:5176", "https://your-fund-domain.example.com"] }));

// Rate limiting — /api 60 req/min, /mcp 30 req/min
app.use("/api/*", rateLimiter(60));

// ── Routes ──────────────────────────────────────────────────────────
// Primary routes
app.route("/api/portfolio", portfolioRoutes);
app.route("/api/funds", fundRoutes);

// Analysis — cross-fund comparison & metrics
app.route("/api/analysis", analysisRoutes);

// Securities alias — same router, backwards compat
app.route("/api/securities", fundRoutes);

// V2 routes — market indices, stock details
app.route("/api/market", marketRoutes);
app.route("/api/stocks", stockRoutes);

// SSE real-time price stream
app.route("/api/market", sseRoutes);

// Admin routes with auth
app.use("/api/admin/*", async (c, next) => {
  if (!requireAdmin(c)) return c.json({ error: "unauthorized" }, 401);
  return next();
});
app.route("/api/admin", adminRoutes);

// Export routes — CSV/Excel download (no auth, receives data from frontend)
app.route("/api/export", exportRoutes);

// Report routes — PDF investment reports
app.route("/api/report", reportRoutes);

// Dashboard — public dev endpoint (no auth, aggregated system metrics)
app.route("/api/dashboard", dashboardRoutes);

// OpenAPI / Swagger docs
app.route("/api", docsRoutes);

// Real DB healthcheck
app.get("/api/health", c => {
  try { getDb().query("SELECT 1").get(); return c.json({ status: "ok", uptime: process.uptime() }); }
  catch (e: any) { return c.json({ status: "error", error: e.message }, 500); }
});
app.get("/api/status", c => c.redirect("/api/admin/status"));
app.get("/api/summary", c => c.json(getDb().query("SELECT * FROM summary_by_fund").all()));

// Request logging (API only)
app.use("/api/*", async (c, next) => {
  const rid = Math.random().toString(36).substring(2, 10);
  const rl = reqLogger(c.req.raw, rid);
  c.set("reqId", rid);
  await next();
  rl.done(c.res.status);
});

// MCP endpoint — accepts admin OR public key. The public exposure
// (your-mcp-domain.example.com) lives behind nginx with its own limit_req + fail2ban;
// here we authenticate by scope and audit every call (scope + IP + UA) for
// forensics. Full system control is exposed to both scopes per the all-public
// decision — defense is layered at nginx/CF/key-isolation, not tool filtering.
app.all("/mcp", rateLimiter(30), async (c, next) => {
  const scope = requireKey(c);
  if (!scope) {
    log.warn("mcp auth failed", { path: c.req.path, ip: clientIp(c) });
    return c.json({ error: "unauthorized" }, 401);
  }
  log.info("mcp call", { scope, ip: clientIp(c), ua: c.req.header("user-agent") });
  return next();
}, (c) => mcpHandler(c.req.raw));

// Global error handler
app.onError((err, c) => {
  if (err instanceof ApiError) {
    const isDev = process.env.NODE_ENV === "development";
    const body: Record<string, unknown> = { error: err.code, message: err.message };
    if (isDev) body.stack = err.stack;
    return c.json(body, err.status as any);
  }
  log.error("unhandled error", { error: err.message, path: c.req.path, stack: err.stack });
  const isDev = process.env.NODE_ENV === "development";
  const body: Record<string, unknown> = { error: "internal", message: "Internal server error" };
  if (isDev) body.stack = err.stack;
  return c.json(body, 500);
});

// Unhandled rejection safety net
process.on("unhandledRejection", (reason) => {
  log.error("unhandled rejection", { error: String(reason) });
});

// ── Startup ─────────────────────────────────────────────────────────
const port = parseInt(process.env.PORT || "8765");

try {
  initSchema(getRwDb());
} catch (e: any) {
  log.error("schema init failed — DB may be corrupted", { error: e.message });
  process.exit(1);
}
getDb();
startScheduler();
startIntegrityMonitor(60); // hourly integrity check
log.info(`fund-backend v2.3 starting`, { port, apiKeyConfigured: !!API_KEY, publicKeyConfigured: !!PUBLIC_MCP_KEY });

export default { port, fetch: app.fetch };
