/** Map raw Error/API messages to short user-facing copy (#188). */
export function sanitizeUserError(err: unknown, fallback: string): string {
  const raw = err instanceof Error ? err.message : String(err ?? '')
  const m = raw.trim()
  if (!m) return fallback
  // Technical dumps / network noise — show fallback only
  if (
    /expected\s|received\s|at path|ECONNREFUSED|ENOTFOUND|network error|Failed to fetch|Load failed|HTTP\s*\d{3}|Zod|TypeError|SyntaxError|Internal Server Error/i.test(
      m,
    )
  ) {
    return fallback
  }
  if (m.length > 160) return fallback
  return m
}
