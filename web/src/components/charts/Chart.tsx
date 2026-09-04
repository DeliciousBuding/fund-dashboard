// Chart — 图表容器壳：统一高度、loading 骨架、empty 占位、主题感知重渲染。
import { useMemo } from "react";
import { cn } from "../../lib/utils";
import { Button } from "../ui/button";
import { readChartTheme } from "./theme";
import { useEChart } from "./useEChart";
import { useThemeKey } from "./useThemeKey";

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
  /** Accessible name for the rendered chart (role="img"). */
  ariaLabel?: string;
  error?: boolean;
  errorText?: string;
  onRetry?: () => void;
}

export function Chart({
  option,
  deps = [],
  height = 280,
  loading,
  empty,
  emptyText = "暂无数据",
  className,
  ariaLabel,
  error,
  errorText = "加载失败",
  onRetry,
}: ChartProps) {
  // 主题快照（响应式）：data-theme / data-convention 属性变化经
  // useSyncExternalStore 触发重渲染，deps 里的 themeKey 驱动 notMerge 重绘。
  const themeKey = useThemeKey();
  // biome-ignore lint/correctness/useExhaustiveDependencies: themeKey 变化必须重建 option
  const built = useMemo(() => {
    if (loading || empty || error) return null;
    return option(readChartTheme());
  }, [loading, empty, error, themeKey, ...deps]);

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
  if (error) {
    return (
      <div
        className={cn(
          "grid place-items-center gap-2 rounded-lg bg-surface-1 text-sm text-fg-3",
          className,
        )}
        style={{ height }}
      >
        <span>{errorText}</span>
        {onRetry ? (
          <Button size="sm" variant="secondary" onClick={onRetry}>
            重试
          </Button>
        ) : null}
      </div>
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
  return (
    <div
      ref={ref}
      className={cn("w-full", className)}
      style={{ height }}
      role="img"
      aria-label={ariaLabel ?? "图表"}
    />
  );
}
