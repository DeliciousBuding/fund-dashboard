import { useCallback, useMemo } from "react";
import { useSearchParams } from "react-router-dom";

/**
 * Sync a chart/range tab value to the URL search params (DESIGN.md G4 / Issue #8).
 * Supports shareable links and browser back/forward.
 */
export function useQueryRange(param: string, defaultValue: string, allowed?: readonly string[]) {
  const [searchParams, setSearchParams] = useSearchParams();

  const range = useMemo(() => {
    const raw = searchParams.get(param);
    if (!raw) return defaultValue;
    if (allowed && !allowed.includes(raw)) return defaultValue;
    return raw;
  }, [searchParams, param, defaultValue, allowed]);

  const setRange = useCallback(
    (next: string) => {
      setSearchParams(
        (prev) => {
          const sp = new URLSearchParams(prev);
          if (!next || next === defaultValue) {
            sp.delete(param);
          } else {
            sp.set(param, next);
          }
          return sp;
        },
        { replace: true },
      );
    },
    [setSearchParams, param, defaultValue],
  );

  return [range, setRange] as const;
}
