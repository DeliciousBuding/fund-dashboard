import { useCallback, useEffect } from "react";
import { useSearchParams } from "react-router-dom";

/** Default portfolio id omitted from the URL to keep links clean. */
export const DEFAULT_PORTFOLIO_ID = 1;

/**
 * Parse `?portfolio=` — positive finite integer only.
 * Returns null when missing/invalid (caller keeps current store value).
 */
export function parsePortfolioParam(raw: string | null): number | null {
  if (raw == null || raw === "") return null;
  const id = Number(raw);
  if (!Number.isFinite(id) || id <= 0 || !Number.isInteger(id)) return null;
  return id;
}

/**
 * Apply portfolioId to search params (omit default).
 * Returns null when no mutation is needed (caller should keep `prev`).
 */
export function applyPortfolioToSearchParams(
  prev: URLSearchParams,
  portfolioId: number,
  defaultId: number = DEFAULT_PORTFOLIO_ID,
): URLSearchParams | null {
  const cur = prev.get("portfolio");
  if (portfolioId === defaultId) {
    if (cur == null) return null;
    const next = new URLSearchParams(prev);
    next.delete("portfolio");
    return next;
  }
  if (cur === String(portfolioId)) return null;
  const next = new URLSearchParams(prev);
  next.set("portfolio", String(portfolioId));
  return next;
}

/**
 * Two-way sync between Zustand portfolioId and `?portfolio=` (Issue #17 / DESIGN G4).
 * - Mount: initialize store from URL once.
 * - Store changes: write back (replace), omit default id.
 */
export function usePortfolioDeepLink(
  portfolioId: number,
  setPortfolioId: (id: number) => void,
  defaultId: number = DEFAULT_PORTFOLIO_ID,
) {
  const [searchParams, setSearchParams] = useSearchParams();

  // Initialize from URL once (deep-link / share).
  useEffect(() => {
    const id = parsePortfolioParam(searchParams.get("portfolio"));
    if (id != null && id !== portfolioId) {
      setPortfolioId(id);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Keep URL in sync when store portfolio changes.
  useEffect(() => {
    setSearchParams((prev) => {
      const next = applyPortfolioToSearchParams(prev, portfolioId, defaultId);
      return next ?? prev;
    }, { replace: true });
  }, [portfolioId, defaultId, setSearchParams]);

  const handlePortfolioChange = useCallback(
    (id: number) => {
      if (Number.isFinite(id) && id > 0) setPortfolioId(id);
    },
    [setPortfolioId],
  );

  return { handlePortfolioChange };
}
