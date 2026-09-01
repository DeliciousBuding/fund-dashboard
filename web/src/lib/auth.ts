// /api/auth/* 写读客户端。响应契约来自 @fund-dashboard/contracts（schemas/auth.ts），
// 读走 fetchValidated（zod 边界校验），写路径经 api 后立即 schema.parse。
import { AuthOkResponseSchema, type AuthStatus, AuthStatusSchema } from "@fund-dashboard/contracts";
import { api } from "./api";
import { fetchValidated } from "./queries";

export type { AuthStatus };

export function fetchAuthStatus(signal?: AbortSignal): Promise<AuthStatus> {
  return fetchValidated("/api/auth/status", AuthStatusSchema, signal);
}

// setup/login/logout 成功都返回 {"ok":true}；parse 同时拒绝 ok 缺失/非 true。
async function authOk(path: string, body: unknown): Promise<{ ok: true }> {
  const data = await api<unknown>(path, { method: "POST", body });
  return AuthOkResponseSchema.parse(data);
}

export function login(password: string): Promise<{ ok: true }> {
  return authOk("/api/auth/login", { password });
}

export function setup(password: string): Promise<{ ok: true }> {
  return authOk("/api/auth/setup", { password });
}

export function logout(): Promise<{ ok: true }> {
  return authOk("/api/auth/logout", {});
}
