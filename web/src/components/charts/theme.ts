// 图表主题 —— 从 CSS 变量读取设计 tokens（docs/design/03 §6），主题切换时重读。
// ECharts 需要具体颜色值；CSS 变量是唯一 SSOT，这里只做读取与适配。

import { CHART_PALETTE_DARK, CHART_PALETTE_LIGHT } from "../../lib/palette";

export interface ChartTheme {
  fg: string;
  fg2: string;
  fg3: string;
  border: string;
  surface1: string;
  surface2: string;
  accent: string;
  up: string;
  down: string;
  warn: string;
  danger: string;
  info: string;
  palette: string[];
  mode: "dark" | "light";
}

function cssVar(name: string): string {
  if (typeof window === "undefined") return "";
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

export function currentThemeMode(): "dark" | "light" {
  if (typeof document === "undefined") return "dark";
  return document.documentElement.dataset.theme === "light" ? "light" : "dark";
}

export function readChartTheme(): ChartTheme {
  const mode = currentThemeMode();
  return {
    fg: cssVar("--fg"),
    fg2: cssVar("--fg-2"),
    fg3: cssVar("--fg-3"),
    border: cssVar("--border"),
    surface1: cssVar("--surface-1"),
    surface2: cssVar("--surface-2"),
    accent: cssVar("--accent"),
    up: cssVar("--up"),
    down: cssVar("--down"),
    warn: cssVar("--warn"),
    danger: cssVar("--danger"),
    info: cssVar("--info"),
    palette: mode === "dark" ? CHART_PALETTE_DARK : CHART_PALETTE_LIGHT,
    mode,
  };
}

/** 基础 option 片段：网格/轴/提示框全站统一。 */
export function baseChartOption(t: ChartTheme) {
  return {
    backgroundColor: "transparent",
    color: t.palette,
    textStyle: { color: t.fg2, fontSize: 12 },
    grid: { left: 8, right: 8, top: 28, bottom: 24, containLabel: true },
    tooltip: {
      backgroundColor: t.surface2,
      borderColor: t.border,
      textStyle: { color: t.fg, fontSize: 12 },
      trigger: "axis",
      axisPointer: { lineStyle: { color: t.border } },
    },
    xAxis: {
      axisLine: { lineStyle: { color: t.border } },
      axisLabel: { color: t.fg3, fontSize: 11 },
      splitLine: { show: false },
      axisTick: { show: false },
    },
    yAxis: {
      axisLabel: { color: t.fg3, fontSize: 11 },
      splitLine: { lineStyle: { color: t.border, type: "dashed" as const } },
      axisLine: { show: false },
      axisTick: { show: false },
    },
    legend: { textStyle: { color: t.fg2, fontSize: 11 }, itemWidth: 12, itemHeight: 8 },
  };
}
