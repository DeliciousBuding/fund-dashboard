// Chart semantic-layer library — declarative echarts composition for fund-dashboard v3.0.
// See docs/refactor/analysis/charts-architecture.md (validation) and, once written,
// docs/charts-design-system.md (Phase cleanup).
//
// Centralizes echarts component registration so each chart stops repeating use([...]).
// Charts using only line/bar/scatter call useCoreCharts() once at module top-level;
// charts needing heatmap/radar/sunburst/treemap add their own use() alongside.
import { use } from "echarts/core";
import { LineChart, BarChart, ScatterChart, RadarChart, TreemapChart, SunburstChart, HeatmapChart } from "echarts/charts";
import {
  GridComponent,
  TooltipComponent,
  LegendComponent,
  DataZoomComponent,
  MarkLineComponent,
  MarkPointComponent,
  RadarComponent,
  VisualMapComponent,
} from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";

let coreRegistered = false;

/** Register the common line/bar/scatter/radar/treemap/sunburst/heatmap + grid/tooltip/legend/dataZoom/markline/markpoint
 *  + visualMap + radar + canvas renderer. Idempotent. */
export function useCoreCharts(): void {
  if (coreRegistered) return;
  use([
    LineChart,
    BarChart,
    ScatterChart,
    RadarChart,
    TreemapChart,
    SunburstChart,
    HeatmapChart,
    GridComponent,
    TooltipComponent,
    LegendComponent,
    DataZoomComponent,
    MarkLineComponent,
    MarkPointComponent,
    RadarComponent,
    VisualMapComponent,
    CanvasRenderer,
  ]);
  coreRegistered = true;
}

export { useChartData } from "./useChartData";
export type { UseChartDataOptions, UseChartDataResult } from "./useChartData";

export { ChartShell } from "./ChartShell";
export type { ChartShellProps, RangeOption } from "./ChartShell";

export { chartTooltip, fmtCNY, fmtPct, pnlSpan } from "./ChartTooltip";

export { lineSeries } from "./series/lineSeries";
export type { LineSeriesOptions } from "./series/lineSeries";

export { barSeries } from "./series/barSeries";
export type { BarSeriesOptions } from "./series/barSeries";

export { scatterSeries } from "./series/scatterSeries";
export type { ScatterSeriesOptions } from "./series/scatterSeries";

export { radarSeries } from "./series/radarSeries";
export type {
  RadarSeriesOptions,
  RadarSeriesResult,
  RadarIndicator,
  RadarSeriesData,
} from "./series/radarSeries";

export { tradeMarkers } from "./series/tradeMarkers";
export type {
  TradeMarkersOptions,
  TradeMarkersResult,
  TradeTransaction,
  AlignStrategy,
} from "./series/tradeMarkers";

// ── Shared chart utilities ─────────────────────────────────────────

/** Compute start & end indices into allDates for a given range key.
 *  "tx" → first/last transaction date (nearest existing NAV dates) ± 10 days padding.
 *  "1m"/"3m"/"6m"/"1y" → that many calendar days before the last date.
 *  anything else → full range [0, lastIdx]. */
export function getDateRange(key: string, allDates: string[], txDates: string[]): [number, number] {
  if (key === 'tx' && txDates.length > 1) {
    // find nearest valid dates in NAV data (handles non-trading days)
    let i0 = allDates.findIndex(d => d >= txDates[0]);
    if (i0 < 0) i0 = 0;
    let i1 = allDates.length - 1;
    for (let j = allDates.length - 1; j >= 0; j--) {
      if (allDates[j] <= txDates[txDates.length - 1]) { i1 = j; break; }
    }
    i0 = Math.max(0, i0 - 10);
    i1 = Math.min(allDates.length - 1, i1 + 10);
    return [i0, i1];
  }
  const lastIdx = allDates.length - 1;
  const days: Record<string, number> = { '1m': 30, '3m': 90, '6m': 180, '1y': 365 };
  if (days[key] && allDates.length) {
    const last = allDates[lastIdx];
    const cutoff = new Date(last); cutoff.setDate(cutoff.getDate() - days[key]);
    const cutoffStr = cutoff.toISOString().substring(0, 10);
    let i0 = allDates.findIndex(d => d >= cutoffStr);
    return [i0 < 0 ? 0 : i0, lastIdx];
  }
  return [0, lastIdx];
}
