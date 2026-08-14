// lineSeries — declarative echarts line-series option factory.
// Merges the old AreaSeries concept: pass `area: true` for a gradient fill.
// Consumes theme tokens for color so red-up/green-down / palette stay consistent.
//
// Returned objects are spread into `option.series` (see ChartShell consumers).
import { hexToRgba } from "../../../styles/theme";

export interface LineSeriesOptions {
  name: string;
  data: (number | null)[] | [string, number][];
  /** line color — a theme token (e.g. theme.blue / theme.up). */
  color: string;
  /** smooth curve. Default true (daily series). Intraday charts may lower this. */
  smooth?: boolean;
  /** gradient area fill (absorbs the former AreaSeries). Default false. */
  area?: boolean;
  /** area fill color; defaults to the line color. */
  areaColor?: string;
  /** top-of-gradient alpha. Default 0.22 (matches theme.areaGradient). */
  areaAlpha?: number;
  /** line width. Default 2. */
  width?: number;
  dashed?: boolean;
  dotted?: boolean;
  opacity?: number;
  /** point symbol. Default "none" (clean financial lines). */
  symbol?: string;
  yAxisIndex?: number;
  /** attach a markLine (e.g. avg-cost reference line). */
  markLine?: Record<string, unknown>;
  /** attach markPoint annotations (e.g. max/min pins). */
  markPoint?: Record<string, unknown>;
  /** echarts z-level. Default unset (echarts auto-layers). */
  z?: number;
}

export function lineSeries(o: LineSeriesOptions): Record<string, unknown> {
  const width = o.width ?? 2;
  const lineType = o.dashed ? "dashed" : o.dotted ? "dotted" : "solid";
  const lineStyle: Record<string, unknown> = { color: o.color, width, type: lineType };
  if (o.opacity != null) lineStyle.opacity = o.opacity;

  const series: Record<string, unknown> = {
    name: o.name,
    type: "line",
    data: o.data,
    smooth: o.smooth ?? true,
    symbol: o.symbol ?? "none",
    lineStyle,
  };

  if (o.area) {
    const c = o.areaColor ?? o.color;
    const alpha = o.areaAlpha ?? 0.22;
    series.areaStyle = {
      color: {
        type: "linear",
        x: 0, y: 0, x2: 0, y2: 1,
        colorStops: [
          { offset: 0, color: hexToRgba(c, alpha) },
          { offset: 1, color: hexToRgba(c, 0) },
        ],
      },
    };
  }

  if (o.yAxisIndex != null) series.yAxisIndex = o.yAxisIndex;
  if (o.markLine) series.markLine = o.markLine;
  if (o.markPoint) series.markPoint = o.markPoint;
  if (o.z != null) series.z = o.z;
  return series;
}
