// 图表主题 —— 从 CSS 变量读取设计 tokens（docs/design/03 §6），主题切换时重读。
// ECharts 需要具体颜色值；CSS 变量是唯一 SSOT，这里只做读取与适配。

import { CHART_PALETTE_DARK, CHART_PALETTE_LIGHT } from "../../lib/palette";

interface ChartTheme {
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
// canvas 的 fillStyle getter 会把 oklch 原样序列化回 "oklch(...)"（仅把百分比转小数），并不会转成 rgb，
// 所以不能靠读 fillStyle 序列化来转换。正确做法是 1×1 canvas 上填色后读回 getImageData 的真实 RGB。
const colorCtx =
  typeof document === "undefined"
    ? null
    : document.createElement("canvas").getContext("2d", { willReadFrequently: true });
if (colorCtx) {
  colorCtx.canvas.width = 1;
  colorCtx.canvas.height = 1;
}

function resolveCssColor(color: string): string {
  if (!color || !colorCtx) return color;
  if (!color.startsWith("oklch") && !color.startsWith("oklab")) return color;
  colorCtx.fillStyle = color;
  colorCtx.fillRect(0, 0, 1, 1);
  const d = colorCtx.getImageData(0, 0, 1, 1).data;
  return `rgb(${d[0]},${d[1]},${d[2]})`;
}

function currentThemeMode(): "dark" | "light" {
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
