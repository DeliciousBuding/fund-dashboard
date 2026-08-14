// useChartData — unified data-fetch lifecycle for the chart semantic layer.
// Eliminates the 10× copy-pasted useEffect + AbortController + loading/error
// boilerplate diagnosed in docs/refactor/analysis/charts-module-inventory.md.
//
// The fetcher is a pure async fn receiving an AbortSignal — agnostic to raw
// fetch() vs the api/ layer, so charts that bypass the api layer (PortfolioChart,
// PnLDistribution, MonteCarlo, Correlation) and those using it converge on one hook.
//
// Hard constraint (handoff §6): no silent error swallowing — AbortError is the only
// swallowed rejection; every other error is surfaced via console.warn + setError.
import { useCallback, useEffect, useRef, useState } from "react";
import i18n from "../../i18n";
import { sanitizeUserError } from "../../services/userError";

export interface UseChartDataOptions {
  /** When false, the hook does not fetch (interaction-gated charts, e.g. FundComparison
   *  compare button). The first transition to true triggers a fetch. */
  enabled?: boolean;
  /** Optional initial data (rendered before the first fetch resolves). */
  initialData?: unknown;
}

export interface UseChartDataResult<T> {
  data: T | undefined;
  loading: boolean;
  error: string | null;
  /** Force a re-fetch (bumps an internal nonce so deps-only charts can refresh too). */
  refetch: () => void;
  /** Imperatively override data (for optimistic updates / multi-step fetches). */
  setData: (data: T | undefined) => void;
}

/**
 * @param fetcher async fn that resolves with the chart data; SHOULD honor `signal`
 *               (pass it to fetch) so dep changes/unmount cancel in-flight requests.
 * @param deps    values that trigger a re-fetch (e.g. [portfolioId, range]).
 */
export function useChartData<T>(
  fetcher: (signal: AbortSignal) => Promise<T>,
  deps: unknown[],
  options: UseChartDataOptions = {},
): UseChartDataResult<T> {
  const { enabled = true, initialData } = options;
  const [data, setData] = useState<T | undefined>(initialData as T | undefined);
  const [loading, setLoading] = useState<boolean>(enabled);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  // nonce lets refetch() re-trigger the effect without changing caller deps.
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    if (!enabled) {
      setLoading(false);
      return;
    }
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    setLoading(true);
    setError(null);
    fetcher(ctrl.signal)
      .then((d) => {
        // ignore results from an aborted/stale run
        if (ctrl.signal.aborted) return;
        setData(d);
        setLoading(false);
      })
      .catch((e: unknown) => {
        if (ctrl.signal.aborted) return;
        if (e instanceof DOMException && e.name === "AbortError") return;
        console.warn("[useChartData]", e);
        // #194: never surface raw Zod/HTTP dumps via ChartShell
        setError(sanitizeUserError(e, i18n.t("common.loadError")));
        setLoading(false);
      });
    return () => ctrl.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, enabled, nonce]);

  const refetch = useCallback(() => setNonce((n) => n + 1), []);

  return { data, loading, error, refetch, setData };
}
