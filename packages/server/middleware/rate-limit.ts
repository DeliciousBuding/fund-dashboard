/** In-memory rate limiter — per-IP sliding window (60s buckets)
 *
 *  Limits:
 *    /mcp   → 30 req/min
 *    /api/* → 60 req/min
 *
 *  Usage:
 *    import { rateLimiter } from "./middleware/rate-limit";
 *    app.use("/api/*", rateLimiter(60));
 *    app.all("/mcp", rateLimiter(30), ...);
 */

import type { Context, Next } from "hono";

interface RateEntry {
  count: number;
  resetTime: number; // epoch ms — when the current window expires
}

const store = new Map<string, RateEntry>();

/** Extract client IP from request headers or connection */
function getIP(c: Context): string {
  const xff = c.req.header("X-Forwarded-For");
  if (xff) return xff.split(",")[0].trim();
  const xri = c.req.header("X-Real-IP");
  if (xri) return xri.trim();
  // Bun's request.connection is not standard — fall back to hostname
  try {
    const req = c.req.raw as any;
    return req.connection?.remoteAddress || "127.0.0.1";
  } catch {
    return "127.0.0.1";
  }
}

/** Create a rate-limiting middleware for `maxPerMinute` requests per 60s window. */
export function rateLimiter(maxPerMinute: number) {
  return async (c: Context, next: Next) => {
    const ip = getIP(c);
    const now = Date.now();
    const entry = store.get(ip);

    // Reset expired window
    if (!entry || now >= entry.resetTime) {
      store.set(ip, { count: 1, resetTime: now + 60_000 });
      return next();
    }

    // Within window — check limit
    if (entry.count < maxPerMinute) {
      entry.count++;
      return next();
    }

    // Rate limited
    const retryAfter = Math.ceil((entry.resetTime - now) / 1000);
    c.header("Retry-After", String(retryAfter));
    return c.json(
      { error: "rate_limited", message: `Too many requests (max ${maxPerMinute}/min)`, retryAfter },
      429,
    );
  };
}

/** Exposed for tests — reset all counters */
export function __resetStore() {
  store.clear();
}
