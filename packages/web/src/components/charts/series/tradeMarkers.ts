// tradeMarkers — unified buy/sell marker logic for charts with transactions.
// Extracted from FundChart.getBuySellMarkers (the prototype) so FundChart and
// NasdaqOverview stop each hand-rolling scatter series with diverging symbol/color/tooltip.
//
// CN convention: buy = red (up/profit direction → upColor), sell = green (downColor).
// Markers carry a white surface border + high z so they sit visibly above the line.
import i18n from "../../../i18n";
import { scatterSeries } from "./scatterSeries";

export type AlignStrategy = "exact" | "nearest";

export interface TradeTransaction {
  trade_time: string;
  /** "buy" | "sell" */
  direction: string;
  [k: string]: unknown;
}

export interface TradeMarkersOptions {
  /** chart x-axis dates (already sliced to the visible range). */
  dates: string[];
  /** chart y values aligned 1:1 with `dates`. */
  values: number[];
  transactions: TradeTransaction[];
  /** buy color — theme.up (red). */
  upColor: string;
  /** sell color — theme.down (green). */
  downColor: string;
  /** marker border — theme.surface (white) for the pop edge. */
  surfaceColor: string;
  /**
   * How to match a trade date (YYYY-MM-DD) to a chart date:
   *  - "nearest" (default): exact match, else first chart date >= trade date
   *    (handles non-trading days — preserves FundChart behavior).
   *  - "exact": only match when the chart has the exact trade date.
   */
  align?: AlignStrategy;
  buyLabel?: string;
  sellLabel?: string;
  buySize?: number;
  sellSize?: number;
  z?: number;
}

export interface TradeMarkersResult {
  buys: [string, number][];
  sells: [string, number][];
  /** ready-to-spread echarts scatter series (only populated sides are included). */
  series: Record<string, unknown>[];
}

function matchIndex(dates: string[], td: string, align: AlignStrategy): number {
  const exact = dates.indexOf(td);
  if (exact >= 0) return exact;
  if (align === "exact") return -1;
  // nearest: first chart date on/after the trade date
  return dates.findIndex((d) => d >= td);
}

export function tradeMarkers(o: TradeMarkersOptions): TradeMarkersResult {
  const align = o.align ?? "nearest";
  const buys: [string, number][] = [];
  const sells: [string, number][] = [];

  for (const tx of o.transactions) {
    if (!tx?.trade_time) continue;
    const td = String(tx.trade_time).substring(0, 10);
    const idx = matchIndex(o.dates, td, align);
    if (idx < 0 || idx >= o.values.length) continue;
    const point: [string, number] = [o.dates[idx], o.values[idx]];
    if (tx.direction === "buy") buys.push(point);
    else if (tx.direction === "sell") sells.push(point);
  }

  const z = o.z ?? 100;
  const series: Record<string, unknown>[] = [];
  if (buys.length) {
    series.push(
      scatterSeries({
        name: o.buyLabel ?? i18n.t("fundDetail.dir.buy"),
        data: buys,
        color: o.upColor,
        borderColor: o.surfaceColor,
        symbolSize: o.buySize ?? 10,
        z,
      }),
    );
  }
  if (sells.length) {
    series.push(
      scatterSeries({
        name: o.sellLabel ?? i18n.t("fundDetail.dir.sell"),
        data: sells,
        color: o.downColor,
        borderColor: o.surfaceColor,
        symbolSize: o.sellSize ?? 12,
        z,
      }),
    );
  }

  return { buys, sells, series };
}
