// ChartTooltip — shared tooltip styling + formatter helpers.
// Re-exports theme.chartTooltip (the box style) and adds pure value formatters
// reused across chart tooltip formatters (CNY / percent / colored PnL span).
export { chartTooltip } from "../../styles/theme";
import type { ThemeTokens } from "../../styles/theme";

/** Format a CNY amount, e.g. ¥12,345 (no decimals by default). */
export function fmtCNY(v: number, maximumFractionDigits = 0): string {
  return `¥${Number(v).toLocaleString(undefined, { maximumFractionDigits })}`;
}

/** Format a signed percent, e.g. +11.2% / -3.0%. */
export function fmtPct(v: number): string {
  const n = Number(v) || 0;
  return `${n >= 0 ? "+" : ""}${n}%`;
}

/** Build an HTML span for a PnL value, colored red (up) / green (down) per CN convention. */
export function pnlSpan(value: number, t: ThemeTokens, inner: string): string {
  const up = (Number(value) || 0) >= 0;
  const color = up ? t.up : t.down;
  return `<span style="font-variant-numeric:tabular-nums;color:${color}">${inner}</span>`;
}
