/** MCP Server — Fund & Stock data tools for AI agents (Hermes/Claude)
 *
 *  Mounted at /mcp, exposes full system control as MCP tools.
 *  Uses mcp-lite (zero-deps, Hono-compatible).
 *  34 tools (v2.5) — complete parity with REST API.
 *
 *  v2.5: Split into focused modules (tools/{query,portfolio,transactions,admin,operations,securities,market,analysis}.ts).
 *  This file is a thin registry — S.U.P.E.R. Single Responsibility compliant.
 */

import { McpServer, StreamableHttpTransport } from "mcp-lite";
import { z } from "zod";

import { registerQueryTools } from "./tools/query";
import { registerPortfolioTools } from "./tools/portfolio";
import { registerTransactionTools } from "./tools/transactions";
import { registerAdminTools } from "./tools/admin";
import { registerOperationsTools } from "./tools/operations";
import { registerSecurityTools } from "./tools/securities";
import { registerMarketTools } from "./tools/market";
import { registerAnalysisTools } from "./tools/analysis";
import { registerReportTools } from "./tools/report";

const server = new McpServer({
  name: "fund-dashboard",
  version: "2.5.0",
  schemaAdapter: (schema) => (schema as z.ZodType).toJSONSchema(),
});

// Register all tool groups
registerQueryTools(server);
registerPortfolioTools(server);
registerTransactionTools(server);
registerAdminTools(server);
registerOperationsTools(server);
registerSecurityTools(server);
registerMarketTools(server);
registerAnalysisTools(server);
registerReportTools(server);

const transport = new StreamableHttpTransport();
export const mcpHandler = transport.bind(server);
