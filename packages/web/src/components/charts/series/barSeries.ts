// barSeries — declarative echarts bar-series option factory.
// Supports fixed color OR per-value red-up/green-down coloring (CN convention),
// which is what PnLDistribution / PortfolioChart daily-PnL / MonteCarlo histograms need.
export interface BarSeriesOptions {
  name: string;
  data: (number | null)[];
  /** fixed bar color (overrides upDown). */
  color?: string;
  /** per-value coloring: value >= 0 → up (red/profit), < 0 → down (green/loss). */
  upDown?: { up: string; down: string };
  yAxisIndex?: number;
  barWidth?: string | number;
  borderRadius?: number[];
  opacity?: number;
  /** attach a markLine (e.g. MonteCarlo median reference line). */
  markLine?: Record<string, unknown>;
}

export function barSeries(o: BarSeriesOptions): Record<string, unknown> {
  const itemStyle: Record<string, unknown> = {};
  if (o.upDown) {
    const { up, down } = o.upDown;
    // Number() guard: values may arrive as strings (handoff §6: compare after Number()).
    itemStyle.color = (p: { value: number | string }) =>
      (Number(p.value) || 0) >= 0 ? up : down;
  } else if (o.color) {
    itemStyle.color = o.color;
  }
  if (o.borderRadius) itemStyle.borderRadius = o.borderRadius;
  if (o.opacity != null) itemStyle.opacity = o.opacity;

  const series: Record<string, unknown> = {
    name: o.name,
    type: "bar",
    data: o.data,
    itemStyle,
  };
  if (o.yAxisIndex != null) series.yAxisIndex = o.yAxisIndex;
  if (o.barWidth != null) series.barWidth = o.barWidth;
  if (o.markLine) series.markLine = o.markLine;
  return series;
}
