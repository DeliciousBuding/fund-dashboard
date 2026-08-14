import type { CSSProperties } from "react";
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

/** Extended multi-category hues (sector treemap / allocation). Light + dark variants. */
const SECTOR_EXT = {
  light: {
    energy: "#f08c00",
    industrial: "#0ca678",
    materials: "#2b8a3e",
    utilities: "#1c7ed6",
    consumerCyclical: "#f59f00",
    consumerDefensive: "#e8590c",
  },
  dark: {
    energy: "#ffb020",
    industrial: "#38d9a9",
    materials: "#51cf66",
    utilities: "#4dabf7",
    consumerCyclical: "#fcc419",
    consumerDefensive: "#ff922b",
  },
} as const;

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
  /** Text/icon on solid accent chips (badges, offline banner). */
  onAccent: string;
  /** Scatter/marker outline against chart canvas. */
  markerBorder: string;
  // semantic accents (brighter in dark)
  up: string;
  down: string;
  /** Failure / validation — NOT profit red (#107). */
  critical: string;
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
  // frosted glass (Apple-like translucent surfaces) — regular material defaults
  glassSurface: string;
  glassBorder: string;
  glassHighlight: string;
  glassBlur: string;
  glassShadow: string;
  /** Ambient canvas under glass (mesh / soft orbs) so frost has depth to sample. */
  ambientCanvas: string;
  ambientOrbA: string;
  ambientOrbB: string;
  ambientNoise: string;
  // extended sector hues (multi-category charts)
  sectorEnergy: string;
  sectorIndustrial: string;
  sectorMaterials: string;
  sectorUtilities: string;
  sectorConsumerCyclical: string;
  sectorConsumerDefensive: string;
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
  onAccent: "#ffffff",
  markerBorder: "#ffffff",
  up: ACCENT.up,
  down: ACCENT.down,
  critical: ACCENT.up, // same hue as light up for a11y contrast; semantic domain differs
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
  shadowHover: "0 12px 36px rgba(15,23,42,0.12)",
  // Apple-leaning white frost (regular): softer fill + deeper blur/vibrancy
  glassSurface: "rgba(255,255,255,0.66)",
  glassBorder: "rgba(255,255,255,0.62)",
  glassHighlight: "linear-gradient(180deg, rgba(255,255,255,0.72) 0%, rgba(255,255,255,0.14) 42%, rgba(255,255,255,0) 100%)",
  glassBlur: "blur(28px) saturate(185%)",
  glassShadow: "0 10px 36px rgba(15,23,42,0.09), 0 1px 0 rgba(255,255,255,0.85) inset, 0 -1px 0 rgba(15,23,42,0.03) inset",
  ambientCanvas: "linear-gradient(165deg, #f4f7fb 0%, #eef2f8 42%, #f8fafc 100%)",
  ambientOrbA: "radial-gradient(ellipse 55% 45% at 12% 18%, rgba(49,114,217,0.14), transparent 70%)",
  ambientOrbB: "radial-gradient(ellipse 50% 40% at 88% 8%, rgba(214,54,73,0.06), transparent 65%)",
  ambientNoise: "radial-gradient(circle at 50% 50%, rgba(255,255,255,0.35), transparent 60%)",
  sectorEnergy: SECTOR_EXT.light.energy,
  sectorIndustrial: SECTOR_EXT.light.industrial,
  sectorMaterials: SECTOR_EXT.light.materials,
  sectorUtilities: SECTOR_EXT.light.utilities,
  sectorConsumerCyclical: SECTOR_EXT.light.consumerCyclical,
  sectorConsumerDefensive: SECTOR_EXT.light.consumerDefensive,
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
  onAccent: "#0b0f17",
  markerBorder: "#ffffff",
  up: "#f87171", // brighter red on dark
  down: "#4ade80", // brighter green on dark
  critical: "#f87171",
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
  shadowHover: "0 12px 36px rgba(0,0,0,0.50)",
  glassSurface: "rgba(22,28,38,0.58)",
  glassBorder: "rgba(255,255,255,0.12)",
  glassHighlight: "linear-gradient(180deg, rgba(255,255,255,0.14) 0%, rgba(255,255,255,0.03) 40%, rgba(255,255,255,0) 100%)",
  glassBlur: "blur(30px) saturate(165%)",
  glassShadow: "0 12px 40px rgba(0,0,0,0.40), inset 0 1px 0 rgba(255,255,255,0.10)",
  ambientCanvas: "linear-gradient(165deg, #0a0e16 0%, #0d121c 48%, #0b0f17 100%)",
  ambientOrbA: "radial-gradient(ellipse 55% 45% at 14% 16%, rgba(77,171,247,0.12), transparent 70%)",
  ambientOrbB: "radial-gradient(ellipse 48% 38% at 86% 10%, rgba(248,113,113,0.07), transparent 65%)",
  ambientNoise: "radial-gradient(circle at 50% 50%, rgba(255,255,255,0.04), transparent 60%)",
  sectorEnergy: SECTOR_EXT.dark.energy,
  sectorIndustrial: SECTOR_EXT.dark.industrial,
  sectorMaterials: SECTOR_EXT.dark.materials,
  sectorUtilities: SECTOR_EXT.dark.utilities,
  sectorConsumerCyclical: SECTOR_EXT.dark.consumerCyclical,
  sectorConsumerDefensive: SECTOR_EXT.dark.consumerDefensive,
};

export function getTheme(dark: boolean): ThemeTokens {
  return dark ? darkTheme : lightTheme;
}

/** Soft chart emphasis/marker shadow derived from theme (not hard-coded black) (#111). */
export function chartShadowColor(t: ThemeTokens, alpha = 0.3): string {
  return t.mode === "dark" ? `rgba(0,0,0,${alpha})` : `rgba(15,23,42,${alpha})`;
}

/** Glass material tiers (Apple-like ultraThin / regular / thick). Single path with Card glass. */
export type GlassMaterial = "ultraThin" | "regular" | "thick";

export interface GlassMaterialTokens {
  surface: string;
  border: string;
  blur: string;
  shadow: string;
  highlight: string;
}

export function glassMaterial(t: ThemeTokens, material: GlassMaterial = "regular"): GlassMaterialTokens {
  if (t.mode === "light") {
    if (material === "ultraThin") {
      return {
        surface: "rgba(255,255,255,0.48)",
        border: "rgba(255,255,255,0.50)",
        blur: "blur(20px) saturate(170%)",
        shadow: "0 6px 20px rgba(15,23,42,0.06), inset 0 1px 0 rgba(255,255,255,0.70)",
        highlight: "linear-gradient(180deg, rgba(255,255,255,0.55) 0%, rgba(255,255,255,0.06) 50%, transparent 100%)",
      };
    }
    if (material === "thick") {
      return {
        surface: "rgba(255,255,255,0.82)",
        border: "rgba(255,255,255,0.72)",
        blur: "blur(36px) saturate(200%)",
        shadow: "0 14px 44px rgba(15,23,42,0.11), inset 0 1px 0 rgba(255,255,255,0.92)",
        highlight: "linear-gradient(180deg, rgba(255,255,255,0.85) 0%, rgba(255,255,255,0.18) 40%, transparent 100%)",
      };
    }
    return {
      surface: t.glassSurface,
      border: t.glassBorder,
      blur: t.glassBlur,
      shadow: t.glassShadow,
      highlight: t.glassHighlight,
    };
  }
  // dark
  if (material === "ultraThin") {
    return {
      surface: "rgba(22,28,38,0.42)",
      border: "rgba(255,255,255,0.09)",
      blur: "blur(22px) saturate(150%)",
      shadow: "0 8px 24px rgba(0,0,0,0.30), inset 0 1px 0 rgba(255,255,255,0.07)",
      highlight: "linear-gradient(180deg, rgba(255,255,255,0.10) 0%, rgba(255,255,255,0.02) 45%, transparent 100%)",
    };
  }
  if (material === "thick") {
    return {
      surface: "rgba(22,28,38,0.78)",
      border: "rgba(255,255,255,0.14)",
      blur: "blur(36px) saturate(175%)",
      shadow: "0 16px 48px rgba(0,0,0,0.48), inset 0 1px 0 rgba(255,255,255,0.12)",
      highlight: "linear-gradient(180deg, rgba(255,255,255,0.16) 0%, rgba(255,255,255,0.03) 40%, transparent 100%)",
    };
  }
  return {
    surface: t.glassSurface,
    border: t.glassBorder,
    blur: t.glassBlur,
    shadow: t.glassShadow,
    highlight: t.glassHighlight,
  };
}

/** Canonical frosted-glass surface styles (#104). Prefer Card glass; use this for chips/toasts/nav. */
export function glassSurfaceStyle(
  t: ThemeTokens,
  opts?: { borderRadius?: number | string; material?: GlassMaterial },
): CSSProperties {
  const m = glassMaterial(t, opts?.material ?? "regular");
  return {
    background: m.surface,
    border: `1px solid ${m.border}`,
    boxShadow: m.shadow,
    backdropFilter: m.blur,
    WebkitBackdropFilter: m.blur,
    ...(opts?.borderRadius != null ? { borderRadius: opts.borderRadius } : {}),
  };
}

/** Ambient canvas stack for AppLayout — gives glass something to refract. */
export function ambientCanvasStyle(t: ThemeTokens): CSSProperties {
  return {
    backgroundColor: t.canvas,
    backgroundImage: `${t.ambientOrbA}, ${t.ambientOrbB}, ${t.ambientNoise}, ${t.ambientCanvas}`,
    backgroundAttachment: "fixed",
  };
}

// ── Spacing / radius scales (CSS vars also in index.css) ──
export const space = {
  1: 4,
  2: 8,
  3: 12,
  4: 16,
  5: 24,
  6: 32,
  7: 48,
  8: 60,
} as const;

export const radius = {
  /** Square / flush chrome (e.g. mobile bottom nav). */
  none: 0,
  /** Icon chips / micro badges (#133). */
  xs: 2,
  sm: 6,
  md: 10,
  lg: 14,
  /** Apple-leaning continuous surface (cards / hero). */
  xl: 18,
  /** Full pill (insight chips, segmented tabs). */
  pill: 999,
} as const;

export const fontSize = {
  /** badges / meta — bumped for high-DPI readability (was 10) */
  xs: 11,
  /** captions / secondary chips */
  sm: 12,
  /** body secondary / labels */
  md: 13,
  /** body */
  base: 14,
  /** emphasis body / buttons */
  lg: 15,
  /** section stats */
  xl: 16,
  /** large stats */
  '2xl': 18,
  /** StatCard primary */
  '3xl': 20,
  /** hero metric */
  '4xl': 30,
} as const;

/** Font-weight scale — replace raw 400/500/600/700 literals (#120). */
export const fontWeight = {
  regular: 400,
  medium: 500,
  semibold: 600,
  bold: 700,
} as const;

/** Line-height scale — replace raw lineHeight literals (#127).
 *  Numbers are CSS unitless multipliers; pixel heights use strings (React treats
 *  numeric lineHeight as a multiplier, not px). */
export const lineHeight = {
  /** Compact control chrome (icon / language buttons). */
  none: 1,
  /** Single-line badge / market chip. */
  badge: "16px",
} as const;

/** Letter-spacing scale — replace raw letterSpacing literals (#127).
 *  Numbers are px (React convention); em tracking uses strings. */
export const letterSpacing = {
  /** Tight tracking for large tabular metrics (StatCard). */
  tight: "-0.02em",
  /** Slight open tracking for banner / alert text. */
  wide: 0.3,
} as const;

export const hitTarget = {
  /** Desktop / dense chrome min (was 24; +4 for high-DPI touch laptops). */
  min: 28,
  /** iOS/Android primary control (WCAG 2.5.5). */
  mobile: 44,
} as const;

/** Layout chrome sizes — fixed shell dimensions (#147). */
export const layout = {
  /** Fixed mobile bottom navigation bar height. */
  mobileNavHeight: 56,
} as const;

/** Elevation / stacking scale — single source of truth for fixed layers (#119). */
export const zIndex = {
  base: 0,
  /** Local stacking within a positioned parent (e.g. Card content over glass highlight) (#124). */
  local: 1,
  dropdown: 100,
  sticky: 200,
  modal: 1000,
  toast: 2000,
  banner: 9999,
  /** Skip link sits above system banners when focused. */
  skip: 10000,
} as const;

/** Opacity scale — replace raw chrome / series opacity literals (#145).
 *  Chart-only single-use (e.g. PnL bar emphasis 0.85) may stay local. */
export const opacity = {
  /** Busy / deleting / disabled control chrome. */
  disabled: 0.4,
  /** Muted secondary chrome (expand toggle, read events). */
  muted: 0.5,
  /** Soft idle icon-button chrome (refresh rest state). */
  soft: 0.55,
  /** Soft secondary chart series (MA20 dotted). */
  seriesSoft: 0.6,
  /** Strong scatter markers (buy/sell). */
  seriesStrong: 0.9,
  /** Full solid — hover / emphasis restore. */
  solid: 1,
} as const;

/** Skeleton bar dimensions for ChartFallback / PageFallback loading shells (#148). */
export const skeleton = {
  barH: {
    /** Caption / subtitle / stat-label skeleton line. */
    sm: 10,
    /** Chart title skeleton line. */
    md: 14,
    /** Page title / stat value skeleton line. */
    lg: 22,
  },
  barW: {
    /** Stat card label. */
    xs: 72,
    /** Stat card value. */
    sm: 100,
    /** Chart title. */
    md: 140,
    /** Page title. */
    lg: 160,
    /** Chart subtitle. */
    xl: 220,
  },
  /** Stat card min height in PageFallback 4-up grid. */
  statMinH: 80,
  /** Default chart body height (matches chartHeight.default / ChartShell). */
  chartH: 420,
} as const;

/** Chart body height scale — replace raw 300–520px chart/shell heights (#162).
 *  `default` matches skeleton.chartH / ChartShell default. */
export const chartHeight = {
  /** PnL distribution histogram. */
  distribution: 280,
  /** Compact embedded series (MonteCarlo histogram). */
  compact: 300,
  /** Fund detail secondary cumulative chart. */
  detail: 340,
  /** MonteCarlo ChartShell. */
  panel: 360,
  /** DCA backtest. */
  backtest: 380,
  /** Allocation sunburst body. */
  medium: 400,
  /** ChartShell / PortfolioChart / Correlation / Allocation shell default. */
  default: 420,
  /** Fund comparison radar/lines. */
  compare: 450,
  /** FundChart / Nasdaq primary series. */
  large: 500,
  /** Penetration treemap. */
  treemap: 520,
} as const;

/** Duration scale (ms) — replace raw .15s / .2s / .3s transition timings (#129). */
export const duration = {
  /** Instant control feedback (hover opacity / background). */
  fast: 150,
  /** Default surface / toast / caret motion. */
  normal: 200,
  /** Status color fades (e.g. SSE indicator). */
  slow: 300,
} as const;

/** Easing scale — replace raw ease / ease-in-out literals (#129). */
export const easing = {
  /** Standard UI ease (CSS default timing). */
  standard: "ease",
  /** Symmetric in-out for loops (shimmer). */
  inOut: "ease-in-out",
} as const;

/** Compose a CSS `transition` value from property + duration/easing tokens (#129). */
export function cssTransition(
  props: string | string[],
  opts?: { duration?: keyof typeof duration; easing?: keyof typeof easing },
): string {
  const d = `${duration[opts?.duration ?? "normal"]}ms`;
  const e = easing[opts?.easing ?? "standard"];
  const list = Array.isArray(props) ? props : [props];
  return list.map((p) => `${p} ${d} ${e}`).join(", ");
}

// ── echarts shared option fragments (chartTheme) ──
// Every chart builds its option from these so gridlines, axes, tooltips,
// dataZoom, and series colors stay consistent across components & themes.

export function chartAxis(t: ThemeTokens) {
  return {
    axisLabel: { fontSize: fontSize.sm, color: t.textMuted },
    axisLine: { show: true, lineStyle: { color: t.border } },
    axisTick: { show: false },
    splitLine: { lineStyle: { color: t.hairline } },
  };
}

export function chartTooltip(t: ThemeTokens) {
  const g = glassMaterial(t, "ultraThin");
  return {
    backgroundColor: g.surface,
    borderColor: g.border,
    borderWidth: 1,
    textStyle: { color: t.text, fontSize: fontSize.md },
    extraCssText: `box-shadow: ${g.shadow}; border-radius: ${radius.md}px; backdrop-filter: ${g.blur}; -webkit-backdrop-filter: ${g.blur};`,
  };
}

export function chartLegend(t: ThemeTokens) {
  return {
    textStyle: { color: t.textSubtle, fontSize: fontSize.md },
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
      textStyle: { color: t.textMuted, fontSize: fontSize.xs },
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

/** Get US stock price change color (red/green) following CN convention */
export function usChangeColor(change: number, dark = false): string {
  const t = getTheme(dark);
  if (change > 0) return t.up;
  if (change < 0) return t.down;
  return t.textMuted;
}
