// API client: cookie-session based, CSRF header on mutations.
// 响应边界校验在调用侧：查询走 lib/queries.ts fetchValidated，写路径在
// mutation 中 parse contracts zod；本 client 只负责 HTTP/CSRF/401 与错误映射。
// 例外：responseType:"blob"（XLSX 导出）返回原始字节，不做 JSON 校验。
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
  ) {
    super(code);
    this.name = "ApiError";
  }
}

// 401 不整页跳 /login 的公开 auth 端点：/status 是路由守卫的未登录探测，
// login/setup 的 401 是错误凭证或前置条件，logout 在会话过期时也无需再跳。
// 带会话的 /api/auth/password|sessions|events 不在此列，过期 401 仍统一回登录门。
const PUBLIC_AUTH_PATHS = new Set([
  "/api/auth/status",
  "/api/auth/login",
  "/api/auth/setup",
  "/api/auth/logout",
]);

interface ApiOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  signal?: AbortSignal;
  /** "blob" returns raw bytes (e.g. XLSX export) instead of parsing JSON. */
  responseType?: "json" | "blob";
}

export async function api<T>(path: string, opts: ApiOptions = {}): Promise<T> {
  const method = opts.method ?? "GET";
  const headers: Record<string, string> = {};
  if (method !== "GET" && method !== "DELETE") {
    headers["Content-Type"] = "application/json";
  }
  if (method !== "GET") {
    // CSRF tripwire (docs/design/04 §5): required by SessionAuth on mutations.
    headers["X-Fund-Request"] = "fetch";
  }
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers,
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
    signal: opts.signal,
  });
  if (res.status === 401 && !PUBLIC_AUTH_PATHS.has(path)) {
    // Session expired mid-flight: back through the gate.
    window.location.assign("/login");
    throw new ApiError(401, "unauthorized");
  }
  if (!res.ok) {
    let code = `http_${res.status}`;
    try {
      const data = (await res.json()) as { error?: string };
      if (typeof data.error === "string" && data.error.length > 0) code = data.error;
    } catch {
      // non-JSON error body — keep the synthesized code
      console.warn("[api] non-JSON error body", { status: res.status });
    }
    throw new ApiError(res.status, code);
  }
  if (opts.responseType === "blob") {
    return (await res.blob()) as T;
  }
  return (await res.json()) as T;
}
