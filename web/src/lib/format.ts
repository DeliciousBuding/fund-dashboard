// 数字格式化：全站唯一出处。金额右对齐 + tabular-nums 由调用方 CSS 保证。

const cnyFormatter = new Intl.NumberFormat("zh-CN", {
  style: "currency",
  currency: "CNY",
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

export function fmtCNY(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return "—";
  return cnyFormatter.format(value);
}

export function fmtPct(value: number | null | undefined, digits = 2): string {
  if (value == null || !Number.isFinite(value)) return "—";
  return `${value.toFixed(digits)}%`;
}

// fmtSignedPct: +1.23% / -1.23%（涨跌色由调用方按符号着色）
export function fmtSignedPct(value: number | null | undefined, digits = 2): string {
  if (value == null || !Number.isFinite(value)) return "—";
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(digits)}%`;
}

export function fmtSignedCNY(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return "—";
  const sign = value > 0 ? "+" : "";
  return `${sign}${cnyFormatter.format(value)}`;
}

// pnlTone maps a signed value to the CN-convention semantic color token.
export function pnlTone(value: number | null | undefined): "up" | "down" | "flat" {
  if (value == null || !Number.isFinite(value) || value === 0) return "flat";
  return value > 0 ? "up" : "down";
}
