// useEChart — unified echarts lifecycle for v3.0.
// Eliminates the per-component init/setOption/resize/dispose boilerplate that
// was copy-pasted inconsistently across 10 chart components (F-CHART-5 memory leak).
// Theme switches go through setOption (not dispose+rebuild) — no flicker, no
// lost dataZoom/legend state.
// High-DPI: init with capped devicePixelRatio so canvas is sharp on Retina /
// external monitors; re-init when OS scale / monitor DPR changes.
import { useEffect, useRef } from "react";
import { init } from "echarts/core";

type ChartInstance = ReturnType<typeof init>;

/** Cap DPR for memory/perf while remaining sharp on 2x–3x displays. */
function chartDpr(): number {
  if (typeof window === "undefined") return 1;
  return Math.min(Math.max(window.devicePixelRatio || 1, 1), 3);
}

function initChart(el: HTMLDivElement): ChartInstance {
  return init(el, undefined, {
    devicePixelRatio: chartDpr(),
    renderer: "canvas",
  });
}

/**
 * @param option  full echarts option (build with getTheme(dark) + chart* helpers)
 * @param deps    values that should trigger a re-render of the chart
 */
export function useEChart(option: Record<string, unknown>, deps: unknown[]) {
  const ref = useRef<HTMLDivElement>(null);
  const inst = useRef<ChartInstance | null>(null);
  const optionRef = useRef(option);
  optionRef.current = option;

  // init (once) + setOption on every dep change (notMerge so theme switches are clean)
  useEffect(() => {
    if (!ref.current || !option || Object.keys(option).length === 0) return;
    if (!inst.current) inst.current = initChart(ref.current);
    inst.current.setOption(option, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  // resize observer + DPR change re-init + dispose on unmount
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

    // Re-init when OS display scale / external monitor DPR changes so canvas stays sharp.
    // jsdom/test envs may lack matchMedia — skip DPR listener there.
    let mq: MediaQueryList | null = null;
    const onDprChange = () => {
      if (!ref.current) return;
      const cur = optionRef.current;
      if (!cur || Object.keys(cur).length === 0) return;
      try {
        inst.current?.dispose();
      } catch {
        /* ignore */
      }
      inst.current = initChart(ref.current);
      inst.current.setOption(cur, true);
    };
    if (typeof window.matchMedia === "function") {
      mq = window.matchMedia(`(resolution: ${window.devicePixelRatio}dppx)`);
      if (typeof mq.addEventListener === "function") {
        mq.addEventListener("change", onDprChange);
      } else {
        mq.addListener(onDprChange);
      }
    }

    return () => {
      ro.disconnect();
      if (mq) {
        if (typeof mq.removeEventListener === "function") {
          mq.removeEventListener("change", onDprChange);
        } else {
          mq.removeListener(onDprChange);
        }
      }
      inst.current?.dispose();
      inst.current = null;
    };
  }, []);

  return ref;
}
