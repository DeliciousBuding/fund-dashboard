import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";

// 401 语义边界：公开 auth 端点（探测/登录/退出/初始化）的 401 是业务态，
// 不触发整页跳转；其余端点（含带会话的 /api/auth/password|sessions|events）
// 一律 window.location.assign("/login") 回登录门。
const PUBLIC_AUTH_PATHS = [
  "/api/auth/status",
  "/api/auth/login",
  "/api/auth/setup",
  "/api/auth/logout",
];

const PROTECTED_PATHS = [
  "/api/portfolio/",
  "/api/auth/password",
  "/api/auth/sessions",
  "/api/auth/sessions/abc123/revoke",
  "/api/auth/events",
];

afterEach(() => {
  vi.unstubAllGlobals();
});

function stubUnauthorized() {
  const assign = vi.fn();
  vi.stubGlobal("window", { location: { assign } });
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "unauthorized" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      }),
    ),
  );
  return assign;
}

describe.each(PUBLIC_AUTH_PATHS)("401 on public auth endpoint %s", (path) => {
  it("does not redirect to /login", async () => {
    const assign = stubUnauthorized();
    await expect(api(path)).rejects.toMatchObject({ status: 401, code: "unauthorized" });
    expect(assign).not.toHaveBeenCalled();
  });
});

describe.each(PROTECTED_PATHS)("401 on protected endpoint %s", (path) => {
  it("redirects to /login", async () => {
    const assign = stubUnauthorized();
    await expect(api(path)).rejects.toMatchObject({ status: 401, code: "unauthorized" });
    expect(assign).toHaveBeenCalledOnce();
    expect(assign).toHaveBeenCalledWith("/login");
  });
});

it("surfaces the server error code for non-401 failures", async () => {
  vi.stubGlobal("window", { location: { assign: vi.fn() } });
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "not_found" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
    ),
  );
  await expect(api("/api/portfolio/")).rejects.toMatchObject({ status: 404, code: "not_found" });
});
