// radarSeries — declarative echarts radar-series option factory.
// Produces both the `radar` axis config AND the `series: [{ type: 'radar', ... }]`
// fragment used by FundComparison and other multi-dimensional comparison charts.
//
// The consumer pre-computes indicators (scales, labels) and data (values, colors)
// from its own business logic; radarSeries handles the echarts wiring.
// Theme-specific axis styling (colors, fonts) stays in the consumer via spread/merge.
import { hexToRgba, lightTheme } from "../../../styles/theme";

export interface RadarIndicator {
  /** Axis label (e.g. "年化收益", "Sharpe"). */
  name: string;
  /** Axis max value — computed dynamically by the consumer (floor + headroom). */
  max: number;
  /** Optional axis min (defaults to 0). */
  min?: number;
}

export interface RadarSeriesData {
  /** Series name (e.g. fund name) shown in legend and tooltip. */
  name: string;
  /** Values in the same order as `indicators`. */
  value: number[];
  /** Ring color (line + point + area). */
  color?: string;
}

export interface RadarSeriesOptions {
  /** Per-axis scale configuration. Order must match `data[].value` order. */
  indicators: RadarIndicator[];
  /** One entry per fund/entity being compared. */
  data: RadarSeriesData[];
  /** Radar shape. Default "polygon". */
  shape?: "circle" | "polygon";
  /** Radar center position. Default ["50%", "48%"]. */
  center?: [string, string];
  /** Radar radius. Default "65%". */
  radius?: string;
}

export interface RadarSeriesResult {
  radar: Record<string, unknown>;
  series: Record<string, unknown>[];
}

export function radarSeries(o: RadarSeriesOptions): RadarSeriesResult {
  return {
    radar: {
      indicator: o.indicators,
      shape: o.shape ?? "polygon",
      center: o.center ?? ["50%", "48%"],
      radius: o.radius ?? "65%",
    },
    series: [
      {
        type: "radar",
        data: o.data.map((d) => ({
          name: d.name,
          value: d.value,
          lineStyle: { color: d.color, width: 2 },
          areaStyle: {
            color: hexToRgba(d.color ?? lightTheme.blue, 0.08),
          },
          symbol: "circle",
          symbolSize: 4,
          itemStyle: { color: d.color },
        })),
      },
    ],
  };
}
