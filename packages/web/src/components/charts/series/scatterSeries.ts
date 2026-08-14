// scatterSeries — declarative echarts scatter-series option factory.
// Used both directly (Correlation-style scatter) and as the building block for
// TradeMarkers (buy/sell points).
export interface ScatterSeriesOptions {
  name: string;
  data: [string | number, number][] | number[][];
  color: string;
  /** fixed size or per-point size fn (e.g. amount-scaled trade markers). */
  symbolSize?: number | ((...args: unknown[]) => number);
  /** marker shape — "circle" (default), "diamond", "rect", "triangle", "pin", etc. */
  symbol?: string;
  /** marker border (white surface edge makes markers pop over the line). */
  borderColor?: string;
  borderWidth?: number;
  z?: number;
  opacity?: number;
}

export function scatterSeries(o: ScatterSeriesOptions): Record<string, unknown> {
  const itemStyle: Record<string, unknown> = { color: o.color };
  if (o.borderColor) {
    itemStyle.borderColor = o.borderColor;
    itemStyle.borderWidth = o.borderWidth ?? 1.5;
  }
  if (o.opacity != null) itemStyle.opacity = o.opacity;

  const series: Record<string, unknown> = {
    name: o.name,
    type: "scatter",
    data: o.data,
    itemStyle,
    symbolSize: o.symbolSize ?? 8,
  };
  if (o.z != null) series.z = o.z;
  if (o.symbol) series.symbol = o.symbol;
  return series;
}
