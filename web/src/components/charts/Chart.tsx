// Chart — 图表容器壳：统一高度、loading 骨架、empty 占位、主题感知重渲染。
import { useMemo } from "react";
import { cn } from "../../lib/utils";
import { readChartTheme } from "./theme";
import { useEChart } from "./useEChart";

interface ChartProps {
  /** 构建 option；入参是当前主题（readChartTheme 快照）。 */
  option: (theme: ReturnType<typeof readChartTheme>) => Record<string, unknown>;
  /** 除主题外的额外依赖（数据等）。 */
  deps?: unknown[];
  height?: number;
  loading?: boolean;
  empty?: boolean;
  emptyText?: string;
  className?: string;
  onClick?: (params: unknown) => void;
}

export function Chart({
  option,
  deps = [],
  height = 280,
  loading,
  empty,
  emptyText = "暂无数据",
  className,
}: ChartProps) {
  // 主题快照：dataset.theme 变化时 deps 里的 themeKey 驱动重渲染。
  const themeKey =
    typeof document === "undefined" ? "dark" : (document.documentElement.dataset.theme ?? "dark");
  // biome-ignore lint/correctness/useExhaustiveDependencies: themeKey 变化必须重建 option
  const built = useMemo(() => {
    if (loading || empty) return null;
    return option(readChartTheme());
  }, [loading, empty, themeKey, ...deps]);

  const ref = useEChart(built, [built]);

  if (loading) {
    return (
      <div
        className={cn("animate-pulse rounded-lg bg-surface-2", className)}
        style={{ height }}
        aria-busy="true"
      />
    );
  }
  if (empty) {
    return (
      <div
        className={cn(
          "grid place-items-center rounded-lg bg-surface-1 text-sm text-fg-3",
          className,
        )}
        style={{ height }}
      >
        {emptyText}
      </div>
    );
  }
  return <div ref={ref} className={cn("w-full", className)} style={{ height }} role="img" />;
}
