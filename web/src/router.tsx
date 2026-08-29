import {
  createRootRoute,
  createRoute,
  createRouter,
  lazyRouteComponent,
  Outlet,
  redirect,
} from "@tanstack/react-router";
import { type AuthStatus, fetchAuthStatus } from "./lib/auth";
import { queryClient } from "./lib/queryClient";
import { WipPage } from "./routes/_wip";
import { AppShell } from "./routes/AppShell";
import { LoginPage } from "./routes/login";
import { SetupPage } from "./routes/setup";

const authStatusQuery = {
  queryKey: ["auth-status"],
  queryFn: ({ signal }: { signal?: AbortSignal }) => fetchAuthStatus(signal),
  staleTime: 60 * 1000,
} as const;

function getAuthStatus(): Promise<AuthStatus> {
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

// 完整路由表一次到位：未落地页面挂 WipPage 占位（数据 API 已就绪，页面随波次替换）。
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
  component: () => <WipPage title="分析" wave="W5" />,
});

const marketRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/market",
  component: () => <WipPage title="市场" wave="W5" />,
});

const insightsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/insights",
  component: () => <WipPage title="信号" wave="W6" />,
});

const reportsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/reports",
  component: () => <WipPage title="报告" wave="W6" />,
});

const systemRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/system",
  component: () => <WipPage title="工作台" wave="W6" />,
});

const systemAuditRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/system/audit",
  component: () => <WipPage title="审计" wave="W6" />,
});

const settingsRoute = createRoute({
  getParentRoute: () => protectedRoute,
  path: "/settings",
  component: () => <WipPage title="设置" wave="W6" />,
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
