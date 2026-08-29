import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  redirect,
} from "@tanstack/react-router";
import { type AuthStatus, fetchAuthStatus } from "./lib/auth";
import { queryClient } from "./lib/queryClient";
import { AppShell } from "./routes/AppShell";
import { OverviewPage } from "./routes/index";
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
  component: OverviewPage,
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  setupRoute,
  protectedRoute.addChildren([indexRoute]),
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
