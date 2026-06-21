// useEChart — unified echarts lifecycle for v3.0.
// Eliminates the per-component init/setOption/resize/dispose boilerplate that
// was copy-pasted inconsistently across 10 chart components (F-CHART-5 memory leak).
// Theme switches go through setOption (not dispose+rebuild) — no flicker, no
// lost dataZoom/legend state.
import { useEffect, useRef } from "react";
import { init } from "echarts/core";

type ChartInstance = ReturnType<typeof init>;

/**
 * @param option  full echarts option (build with getTheme(dark) + chart* helpers)
 * @param deps    values that should trigger a re-render of the chart
 */
export function useEChart(option: Record<string, unknown>, deps: unknown[]) {
  const ref = useRef<HTMLDivElement>(null);
  const inst = useRef<ChartInstance | null>(null);

  // init (once) + setOption on every dep change (notMerge so theme switches are clean)
  useEffect(() => {
    if (!ref.current || !option || Object.keys(option).length === 0) return;
    if (!inst.current) inst.current = init(ref.current);
    inst.current.setOption(option, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  // resize observer + dispose on unmount
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(() => {
      try {
        inst.current?.resize();
      } catch {
        /* instance may be disposed during teardown */
      }
    });
    ro.observe(el);
    return () => {
      ro.disconnect();
      inst.current?.dispose();
      inst.current = null;
    };
  }, []);

  return ref;
}
