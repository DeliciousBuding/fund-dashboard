// useEChart — 统一 echarts 生命周期（移植自旧前端同名 hook，fbeafd9^）。
// init DPR cap 3 / ResizeObserver / OS DPR 变化重初始化 / 卸载 dispose。
// 主题切换由调用方把主题编进 option 依赖即可（notMerge setOption，不闪、
// 不丢 dataZoom/legend 状态）。
import { useEffect, useRef } from "react";
import { echarts, registerCharts } from "./register";

export type ChartInstance = ReturnType<typeof echarts.init>;

/** Cap DPR for memory/perf while remaining sharp on 2x–3x displays. */
function chartDpr(): number {
  if (typeof window === "undefined") return 1;
  return Math.min(Math.max(window.devicePixelRatio || 1, 1), 3);
}

function initChart(el: HTMLDivElement): ChartInstance {
  registerCharts();
  return echarts.init(el, undefined, {
    devicePixelRatio: chartDpr(),
    renderer: "canvas",
  });
}

export function useEChart(option: Record<string, unknown> | null, deps: unknown[]) {
  const ref = useRef<HTMLDivElement>(null);
  const inst = useRef<ChartInstance | null>(null);
  const optionRef = useRef(option);
  optionRef.current = option;

  // init (once) + setOption on every dep change (notMerge so theme switches are clean)
  useEffect(() => {
    if (!ref.current || !option || Object.keys(option).length === 0) return;
    if (!inst.current) inst.current = initChart(ref.current);
    inst.current.setOption(option, true);
    // biome-ignore lint/correctness/useExhaustiveDependencies: deps 由调用方显式声明
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
