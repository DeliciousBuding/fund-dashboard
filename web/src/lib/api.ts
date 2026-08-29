// API client: cookie-session based, CSRF header on mutations, zod-free at the
// boundary for now (contracts wiring lands with W3 pages).
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
  ) {
    super(code);
    this.name = "ApiError";
  }
}

interface ApiOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  signal?: AbortSignal;
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
  if (res.status === 401 && !path.startsWith("/api/auth/")) {
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
    }
    throw new ApiError(res.status, code);
  }
  return (await res.json()) as T;
}
