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

// ECharts visualMap 等需要程序化插值的场景只认 rgb/hsl/hex；oklch token 直接传入会渲染成黑色。
// canvas fillStyle 赋值即触发浏览器 CSS Color 4 解析，读回为标准化 rgb/hex。
let colorCtx: CanvasRenderingContext2D | null | undefined;

function resolveCssColor(color: string): string {
  if (!color || typeof document === "undefined") return color;
  if (!color.startsWith("oklch") && !color.startsWith("oklab")) return color;
  if (colorCtx === undefined) colorCtx = document.createElement("canvas").getContext("2d");
  if (!colorCtx) return color; // jsdom 等无 canvas 实现的环境回退原值
  colorCtx.fillStyle = "#010203"; // 哨兵：fillStyle 赋无效值时保持原值不变
  colorCtx.fillStyle = color;
  return colorCtx.fillStyle === "#010203" ? color : colorCtx.fillStyle;
}

export function currentThemeMode(): "dark" | "light" {
  if (typeof document === "undefined") return "dark";
  return document.documentElement.dataset.theme === "light" ? "light" : "dark";
}

export function readChartTheme(): ChartTheme {
  const mode = currentThemeMode();
  return {
    fg: resolveCssColor(cssVar("--fg")),
    fg2: resolveCssColor(cssVar("--fg-2")),
    fg3: resolveCssColor(cssVar("--fg-3")),
    border: resolveCssColor(cssVar("--border")),
    surface1: resolveCssColor(cssVar("--surface-1")),
    surface2: resolveCssColor(cssVar("--surface-2")),
    accent: resolveCssColor(cssVar("--accent")),
    up: resolveCssColor(cssVar("--up")),
    down: resolveCssColor(cssVar("--down")),
    warn: resolveCssColor(cssVar("--warn")),
    danger: resolveCssColor(cssVar("--danger")),
    info: resolveCssColor(cssVar("--info")),
    palette: (mode === "dark" ? CHART_PALETTE_DARK : CHART_PALETTE_LIGHT).map(resolveCssColor),
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
