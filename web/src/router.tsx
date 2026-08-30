import {
  createRootRoute,
  createRoute,
  createRouter,
  lazyRouteComponent,
  Outlet,
  redirect,
} from "@tanstack/react-router";
import { authStatusQuery } from "./lib/authQuery";
import { queryClient } from "./lib/queryClient";
import { AppShell } from "./routes/AppShell";
import { LoginPage } from "./routes/login";
import { SetupPage } from "./routes/setup";

function getAuthStatus() {
  return queryClient.ensureQueryData(authStatusQuery);
}

const rootRoute = createRootRoute({ component: Outlet });

// 公开面：登录与首次初始化。
const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  beforeLoad: async () => {
    const status = await getAuthStatus();
    if (!status.initialized) throw redirect({ to: "/setup" });
    if (status.authenticated) throw redirect({ to: "/" });
  },
  component: LoginPage,
});

const setupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/setup",
  beforeLoad: async () => {
    const status = await getAuthStatus();
    if (status.initialized) throw redirect({ to: "/login" });
  },
  component: SetupPage,
});

// 受保护面：一切业务页。未初始化 → /setup；未登录 → /login。
const protectedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "protected",
  beforeLoad: async () => {
    const status = await getAuthStatus();
    if (!status.initialized) throw redirect({ to: "/setup" });
    if (!status.authenticated) throw redirect({ to: "/login" });
  },
  component: AppShell,
});

const indexRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/",
  component: lazyRouteComponent(() => import("./routes/index"), "OverviewPage"),
});

// 全部页面已落地（W3–W6）；图表/表格重的路由一律懒加载。
const holdingsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/holdings",
  component: lazyRouteComponent(() => import("./routes/holdings"), "HoldingsPage"),
});

const holdingDetailRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/holdings/$code",
  component: lazyRouteComponent(() => import("./routes/holdings.$code"), "HoldingDetailPage"),
});

const transactionsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/transactions",
  component: lazyRouteComponent(() => import("./routes/transactions"), "TransactionsPage"),
});

const dcaRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/dca",
  component: lazyRouteComponent(() => import("./routes/dca"), "DcaPage"),
});

const analysisRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/analysis",
  component: lazyRouteComponent(() => import("./routes/analysis"), "AnalysisPage"),
});

const marketRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/market",
  component: lazyRouteComponent(() => import("./routes/market"), "MarketPage"),
});

const insightsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/insights",
  component: lazyRouteComponent(() => import("./routes/insights"), "InsightsPage"),
});

const reportsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/reports",
  component: lazyRouteComponent(() => import("./routes/reports"), "ReportsPage"),
});

const systemRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/system",
  component: lazyRouteComponent(() => import("./routes/system"), "SystemPage"),
});

const systemAuditRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/system/audit",
  component: lazyRouteComponent(() => import("./routes/system.audit"), "SystemAuditPage"),
});

const settingsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/settings",
  component: lazyRouteComponent(() => import("./routes/settings"), "SettingsPage"),
});

// 设计系统目录页（03 §10 目视基线）。壳外全宽，便于查密度/断点行为。
const designRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/_design",
  component: lazyRouteComponent(() => import("./routes/_design"), "DesignPage"),
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  setupRoute,
  protectedRoute.addChildren([
    indexRoute,
    holdingsRoute,
    holdingDetailRoute,
    transactionsRoute,
    dcaRoute,
    analysisRoute,
    marketRoute,
    insightsRoute,
    reportsRoute,
    systemRoute,
    systemAuditRoute,
    settingsRoute,
    designRoute,
  ]),
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
