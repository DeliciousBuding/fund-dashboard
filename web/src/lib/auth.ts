import { api } from "./api";

export interface AuthStatus {
  initialized: boolean;
  env_managed: boolean;
  authenticated: boolean;
  session_expires_at?: number;
}

export function fetchAuthStatus(signal?: AbortSignal): Promise<AuthStatus> {
  return api<AuthStatus>("/api/auth/status", { signal });
}

export function login(password: string): Promise<{ ok: true }> {
  return api("/api/auth/login", { method: "POST", body: { password } });
}

export function setup(password: string): Promise<{ ok: true }> {
  return api("/api/auth/setup", { method: "POST", body: { password } });
}

export function logout(): Promise<{ ok: true }> {
  return api("/api/auth/logout", { method: "POST", body: {} });
}
