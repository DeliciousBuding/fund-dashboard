// Design tokens — single source of truth for fund-dashboard v3.0 visual system.
// Replaces the scattered C constants + chartColors(dark) + per-component inline colors.
// Light/dark themes polished equally; default follows prefers-color-scheme.
//
// Hard constraint (CN convention): red = up/profit, green = down/loss.

export type ThemeMode = "light" | "dark";

// ── Semantic accents (fixed hue; dark mode uses brighter variants below) ──
const ACCENT = {
  up: "#d63649", // 涨/盈利 — red
  down: "#199c63", // 跌/亏损 — green
  blue: "#3172d9", // brand / primary series
  amber: "#e07b2c", // neutral warning
  violet: "#8b5cf6", // series
  cyan: "#06b6d4", // series
};

export interface ThemeTokens {
  mode: ThemeMode;
  // surfaces
  canvas: string;
  surface: string;
  surfaceHover: string;
  border: string;
  borderSubtle: string;
  // text
  text: string;
  textSubtle: string;
  textMuted: string;
  // semantic accents (brighter in dark)
  up: string;
  down: string;
  blue: string;
  amber: string;
  violet: string;
  cyan: string;
  // chart palette (colorblind-aware ordering)
  series: string[];
  hairline: string; // grid splitline (low-contrast, gridline-subtle)
  gridBg: string; // area gradient start
  gridBgEnd: string; // area gradient end
  sliderBorder: string;
  sliderBg: string;
  sliderFill: string;
  // elevation
  shadowCard: string;
  shadowHover: string;
}

export const lightTheme: ThemeTokens = {
  mode: "light",
  canvas: "#f8fafc",
  surface: "#ffffff",
  surfaceHover: "#f1f5f9",
  border: "#e2e8f0",
  borderSubtle: "#f1f5f9",
  text: "#0f172a",
  textSubtle: "#475569",
  textMuted: "#94a3b8",
  up: ACCENT.up,
  down: ACCENT.down,
  blue: ACCENT.blue,
  amber: ACCENT.amber,
  violet: ACCENT.violet,
  cyan: ACCENT.cyan,
  series: [ACCENT.blue, ACCENT.up, ACCENT.down, ACCENT.amber, ACCENT.violet, ACCENT.cyan],
  hairline: "#eef2f7",
  gridBg: "rgba(49,114,217,0.14)",
  gridBgEnd: "rgba(49,114,217,0)",
  sliderBorder: "#e2e8f0",
  sliderBg: "#f8fafc",
  sliderFill: "rgba(49,114,217,0.15)",
  shadowCard: "0 1px 3px rgba(15,23,42,0.06), 0 1px 2px rgba(15,23,42,0.04)",
  shadowHover: "0 8px 24px rgba(15,23,42,0.10)",
};

export const darkTheme: ThemeTokens = {
  mode: "dark",
  canvas: "#0b0f17",
  surface: "#131922",
  surfaceHover: "#1c2433",
  border: "rgba(255,255,255,0.08)",
  borderSubtle: "rgba(255,255,255,0.05)",
  text: "#e5e7eb",
  textSubtle: "#9ca3af",
  textMuted: "#64748b",
  up: "#f87171", // brighter red on dark
  down: "#4ade80", // brighter green on dark
  blue: "#4dabf7",
  amber: "#fbbf24",
  violet: "#a78bfa",
  cyan: "#22d3ee",
  series: ["#4dabf7", "#f87171", "#4ade80", "#fbbf24", "#a78bfa", "#22d3ee"],
  hairline: "rgba(255,255,255,0.06)",
  gridBg: "rgba(77,171,247,0.16)",
  gridBgEnd: "rgba(77,171,247,0)",
  sliderBorder: "rgba(255,255,255,0.12)",
  sliderBg: "rgba(255,255,255,0.04)",
  sliderFill: "rgba(77,171,247,0.18)",
  shadowCard: "0 1px 3px rgba(0,0,0,0.30)",
  shadowHover: "0 8px 24px rgba(0,0,0,0.45)",
};

export function getTheme(dark: boolean): ThemeTokens {
  return dark ? darkTheme : lightTheme;
}

// ── echarts shared option fragments (chartTheme) ──
// Every chart builds its option from these so gridlines, axes, tooltips,
// dataZoom, and series colors stay consistent across components & themes.

export function chartAxis(t: ThemeTokens) {
  return {
    axisLabel: { fontSize: 11, color: t.textMuted },
    axisLine: { show: true, lineStyle: { color: t.border } },
    axisTick: { show: false },
    splitLine: { lineStyle: { color: t.hairline } },
  };
}

export function chartTooltip(t: ThemeTokens) {
  return {
    backgroundColor: t.surface,
    borderColor: t.border,
    borderWidth: 1,
    textStyle: { color: t.text, fontSize: 12 },
    extraCssText: `box-shadow: ${t.shadowHover}; border-radius: 10px; backdrop-filter: blur(6px);`,
  };
}

export function chartLegend(t: ThemeTokens) {
  return {
    textStyle: { color: t.textSubtle, fontSize: 12 },
    inactiveColor: t.textMuted,
    icon: "roundRect",
    itemWidth: 10,
    itemHeight: 10,
    itemGap: 16,
  };
}

export function chartDataZoom(t: ThemeTokens) {
  return [
    { type: "inside" },
    {
      type: "slider",
      height: 18,
      bottom: 6,
      borderColor: t.sliderBorder,
      backgroundColor: t.sliderBg,
      fillerColor: t.sliderFill,
      selectedDataBackground: { lineStyle: { color: t.blue }, areaStyle: { color: t.sliderFill } },
      handleStyle: { color: t.blue, borderColor: t.blue },
      moveHandleStyle: { color: t.textMuted },
      textStyle: { color: t.textMuted, fontSize: 10 },
    },
  ];
}

// Area gradient for line charts (uses current series color via callback in component)
export function areaGradient(t: ThemeTokens, color: string) {
  return {
    type: "linear",
    x: 0, y: 0, x2: 0, y2: 1,
    colorStops: [
      { offset: 0, color: color.replace("rgb", "rgba").includes("rgba") ? color : hexToRgba(color, 0.22) },
      { offset: 1, color: hexToRgba(color, 0) },
    ],
  };
}

export function hexToRgba(hex: string, alpha: number): string {
  const h = hex.replace("#", "");
  const n = h.length === 3
    ? h.split("").map((c) => c + c).join("")
    : h;
  const r = parseInt(n.substring(0, 2), 16);
  const g = parseInt(n.substring(2, 4), 16);
  const b = parseInt(n.substring(4, 6), 16);
  return `rgba(${r},${g},${b},${alpha})`;
}
