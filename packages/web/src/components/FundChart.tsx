import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { getTheme, chartAxis, chartTooltip, chartLegend, hexToRgba, fontSize, opacity, chartHeight } from "../styles/theme";
import { useEChart } from "../hooks/useEChart";
import { useQueryRange } from "../hooks/useQueryRange";
import { ChartShell, lineSeries, tradeMarkers, useCoreCharts, getDateRange, type RangeOption } from "./charts";
import type { NavPoint, Transaction } from "../api";

useCoreCharts();

const FUND_RANGES = ["tx", "1m", "3m", "6m", "1y", "all"] as const;

interface FundChartProps {
  navData: NavPoint[];
  transactions: Transaction[];
  heldShares: number;
  totalCost: number;
  chartTitle: string;
  priceLabel: string;
  dark: boolean;
}

export default function FundChart({ navData, transactions, heldShares, totalCost, chartTitle, priceLabel, dark }: FundChartProps) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const [range, setRange] = useQueryRange("range", "tx", FUND_RANGES);

  const RANGE_TABS: RangeOption[] = [
    { value: "tx", label: t("chart.range.tx") },
    { value: "1m", label: t("chart.range.1m") },
    { value: "3m", label: t("chart.range.3m") },
    { value: "6m", label: t("chart.range.6m") },
    { value: "1y", label: t("chart.range.1y") },
    { value: "all", label: t("chart.range.all") },
  ];

  const txDates = useMemo(
    () => [...new Set(transactions.map((tx) => tx.trade_time.substring(0, 10)))].sort(),
    [transactions],
  );
  const dates10 = useMemo(() => navData.map((d) => d.date.substring(0, 10)), [navData]);

  const option = useMemo(() => {
    if (!navData.length) return {} as Record<string, unknown>;
    const navs = navData.map((d) => d.unit_nav);
    const [i0, i1] = getDateRange(range, dates10, txDates);
    const slicedDates = dates10.slice(i0, i1 + 1);
    const slicedNavs = navs.slice(i0, i1 + 1);

    const series: Record<string, any>[] = [];
    const legendItems: string[] = [];

    // NAV line (+ avg-cost markLine)
    const avgCost = heldShares > 0.001 ? Number(Math.abs(totalCost)) / heldShares : null;
    const costLabel = t("fund.costLabel");
    const navMarkLine =
      avgCost && avgCost > 0
        ? {
            silent: true,
            symbol: "none",
            lineStyle: { color: hexToRgba(theme.amber, 0.4), type: "dashed", width: 1 },
            data: [
              {
                yAxis: +avgCost.toFixed(4),
                name: costLabel,
                label: { formatter: `${costLabel} ¥${avgCost.toFixed(4)}`, fontSize: fontSize.xs, color: hexToRgba(theme.amber, 0.6) },
              },
            ],
          }
        : undefined;
    series.push(lineSeries({ name: priceLabel, data: slicedNavs, color: theme.blue, area: true, markLine: navMarkLine }));
    legendItems.unshift(priceLabel);
    if (navMarkLine) legendItems.push(costLabel);

    // MA20 (off-by-one window preserved per refactor constraint — render-layer only)
    if (slicedNavs.length >= 20) {
      const ma20: (number | null)[] = [];
      let sum = 0;
      for (let i = 0; i < slicedNavs.length; i++) {
        sum += slicedNavs[i];
        if (i >= 20) sum -= slicedNavs[i - 20];
        ma20.push(i >= 19 ? +(sum / 20).toFixed(4) : null);
      }
      series.push(lineSeries({ name: t('fundDetail.ma20'), data: ma20, color: theme.amber, width: 1, dotted: true, opacity: opacity.seriesSoft }));
      legendItems.push(t('fundDetail.ma20'));
    }

    // buy/sell markers (extracted into the shared tradeMarkers helper)
    const tm = tradeMarkers({
      dates: slicedDates,
      values: slicedNavs,
      transactions,
      upColor: theme.up,
      downColor: theme.down,
      surfaceColor: theme.surface,
      buyLabel: t("fund.buyLabel"),
      sellLabel: t("fund.sellLabel"),
      buySize: 8,
      sellSize: 10,
      z: 10,
    });
    if (tm.buys.length) legendItems.push(t("fund.buyLabel"));
    if (tm.sells.length) legendItems.push(t("fund.sellLabel"));
    series.push(...tm.series);

    return {
      tooltip: { trigger: "axis", confine: true, ...chartTooltip(theme) },
      legend: { data: legendItems, top: 4, ...chartLegend(theme) },
      grid: { left: 55, right: 30, top: 32, bottom: 36 },
      xAxis: { type: "category", data: slicedDates, boundaryGap: false, ...chartAxis(theme) },
      yAxis: { type: "value", scale: true, ...chartAxis(theme) },
      dataZoom: [{ type: "inside" }],
      series,
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [navData, transactions, range, txDates, dates10, dark, heldShares, totalCost, priceLabel]);

  const chartRef = useEChart(option, [option]);

  return (
    <ChartShell
      dark={dark}
      title={navData.length ? chartTitle : undefined}
      ranges={navData.length ? RANGE_TABS : undefined}
      range={range}
      onRangeChange={setRange}
      empty={!navData.length}
      height={chartHeight.large}
    >
      <div ref={chartRef} style={{ height: chartHeight.large }} />
    </ChartShell>
  );
}
