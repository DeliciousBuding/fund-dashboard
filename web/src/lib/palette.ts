// 图表分类色板 —— 全站唯一出处（docs/design/03 §2 图表色板）。
// 暗色基调 OKLCH；亮色模式整体降明度。slot -1 表示 muted（非语义中性色）。

export const CHART_PALETTE_DARK = [
  "oklch(0.78 0.11 85)", // 0 gold (accent)
  "oklch(0.72 0.11 200)", // 1 cyan
  "oklch(0.70 0.13 310)", // 2 violet
  "oklch(0.70 0.15 10)", // 3 rose
  "oklch(0.72 0.14 155)", // 4 emerald
  "oklch(0.72 0.13 60)", // 5 orange
  "oklch(0.68 0.12 250)", // 6 blue
  "oklch(0.72 0.13 130)", // 7 lime
  "oklch(0.70 0.12 350)", // 8 pink
  "oklch(0.70 0.11 175)", // 9 teal
];

export const CHART_PALETTE_LIGHT = CHART_PALETTE_DARK.map((c) =>
  c.replace(/oklch\(0\.\d+/, (m) => {
    const l = Number.parseFloat(m.slice(6));
    return `oklch(${(l - 0.14).toFixed(2)}`;
  }),
);

export function chartPalette(mode: "dark" | "light"): string[] {
  return mode === "dark" ? CHART_PALETTE_DARK : CHART_PALETTE_LIGHT;
}

/** paletteColor resolves a slot index (-1 → muted) against the mode palette. */
export function paletteColor(slot: number, mode: "dark" | "light"): string {
  if (slot < 0) return mode === "dark" ? "oklch(0.52 0.015 250)" : "oklch(0.55 0.015 250)";
  const palette = chartPalette(mode);
  const fallback = palette[0] ?? "oklch(0.78 0.11 85)";
  return palette[slot % palette.length] ?? fallback;
}
