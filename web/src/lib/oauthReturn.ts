// OAuth return-path validation for the login page.
//
// The OAuth authorize endpoint is served by the Go backend, not by the SPA
// router, so after a successful login we hand the browser a real navigation to
// that URL. That makes the login page a potential open redirect, so the return
// target is validated here and again server-side (httpapi.safeOAuthReturn) —
// neither side trusts the other.
//
// The validator is deliberately DOM-free so it can be unit-tested in the node
// environment this project uses; only the thin reader below touches window.

const MAX_RETURN_LENGTH = 2048;

// PARSE_BASE is a fixed, unreachable origin used only so the URL constructor can
// resolve relative input. Absolute URLs still resolve to their own origin and are
// then rejected by the origin comparison.
const PARSE_BASE = "https://fund-dashboard.invalid";

/**
 * Validates a post-login return path. Returns the normalized same-origin path
 * under /oauth/, or null when the value must not be navigated to.
 */
export function safeOAuthReturn(raw: string | null | undefined): string | null {
  if (typeof raw !== "string") return null;
  const value = raw.trim();
  if (!value || value.length > MAX_RETURN_LENGTH) return null;
  // Only the OAuth surface may be a return target.
  if (!value.startsWith("/oauth/")) return null;
  // Reject scheme-relative ("//evil.example") and absolute URLs.
  if (value.startsWith("//")) return null;
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(value)) return null;
  // Reject header/response splitting characters.
  if (/[\r\n\t]/.test(value)) return null;

  let parsed: URL;
  try {
    parsed = new URL(value, PARSE_BASE);
  } catch {
    return null;
  }
  if (parsed.origin !== new URL(PARSE_BASE).origin) return null;
  // The URL constructor resolves dot segments, so "/oauth/../api/admin"
  // normalizes to "/api/admin". Re-check the prefix on the normalized path —
  // validating only the raw input would let this page redirect anywhere on the
  // origin.
  if (!parsed.pathname.startsWith("/oauth/")) return null;
  if (parsed.pathname.startsWith("//")) return null;
  // Drop any hash: the backend never needs it and it is a script-injection
  // surface in some navigation paths.
  return parsed.pathname + parsed.search;
}

/** Reads and validates `?next=` from the current location. */
export function oauthReturnTarget(): string | null {
  try {
    return safeOAuthReturn(new URLSearchParams(window.location.search).get("next"));
  } catch {
    return null;
  }
}
