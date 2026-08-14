import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { getTheme, chartAxis, chartTooltip, chartLegend, chartDataZoom, chartHeight, space} from "../styles/theme";
import { useEChart } from "../hooks/useEChart";
import { ChartShell, useChartData, lineSeries, barSeries, useCoreCharts } from "./charts";
import { fetchPortfolioTimeline } from "../api";

useCoreCharts();

export default function PortfolioChart({ dark, portfolioId }: { dark: boolean; portfolioId?: number }) {
  const { t } = useTranslation();
  const theme = getTheme(dark);

  const { data, loading, error } = useChartData(
    (signal) => fetchPortfolioTimeline(portfolioId, signal),
    [portfolioId],
  );
  const tl = data ?? [];

  const option = useMemo(() => {
    if (!tl.length) return {} as Record<string, unknown>;
    const dates = tl.map((d) => d.date);
    const values = tl.map((d) => d.total_value);
    const costs = tl.map((d) => d.total_cost);
    const pnls = tl.map((d) => d.pnl);
    return {
      tooltip: {
        trigger: "axis",
        ...chartTooltip(theme),
        formatter: (params: any) => {
          const d = tl[params[0]?.dataIndex];
          if (!d) return "";
          const pnl = Number(d.pnl) || 0; // fix: was string-compared (pnl_pct as string)
          const pnlPct = Number(d.pnl_pct) || 0;
          const up = pnl >= 0;
          return (
            `<div style="font-weight:600;margin-bottom:4px">${d.date}</div>` +
            `${t("portfolio.marketValue")}: <b style="font-variant-numeric:tabular-nums">¥${Number(d.total_value).toLocaleString(undefined, { maximumFractionDigits: 0 })}</b><br/>` +
            `${t("portfolio.cost")}: <span style="font-variant-numeric:tabular-nums;color:${theme.textSubtle}">¥${Number(d.total_cost).toLocaleString(undefined, { maximumFractionDigits: 0 })}</span><br/>` +
            `${t("portfolio.dailyPnL")}: <span style="font-variant-numeric:tabular-nums;color:${up ? theme.up : theme.down}">${up ? "+" : ""}¥${pnl.toFixed(0)} (${pnlPct >= 0 ? "+" : ""}${pnlPct}%)</span>`
          );
        },
      },
      legend: { data: [t("portfolio.marketValue"), t("portfolio.cost"), t("portfolio.dailyPnL")], top: 4, ...chartLegend(theme) },
      grid: { left: 60, right: 30, top: 36, bottom: 44 },
      xAxis: { type: "category", data: dates, boundaryGap: false, ...chartAxis(theme) },
      yAxis: [
        { type: "value", ...chartAxis(theme), axisLabel: { formatter: (v: number) => `¥${(v / 1000).toFixed(0)}k`, color: theme.textMuted } },
        { type: "value", ...chartAxis(theme), axisLabel: { formatter: (v: number) => `¥${v.toFixed(0)}`, color: theme.textMuted } },
      ],
      dataZoom: chartDataZoom(theme),
      series: [
        lineSeries({ name: t("portfolio.marketValue"), data: values, color: theme.blue, area: true, areaAlpha: 0.26, width: 2.5 }),
        lineSeries({ name: t("portfolio.cost"), data: costs, color: theme.amber, dashed: true, width: 1.5 }),
        barSeries({
          name: t("portfolio.dailyPnL"),
          data: pnls,
          yAxisIndex: 1,
          upDown: { up: theme.up, down: theme.down },
          borderRadius: [2, 2, 0, 0],
          barWidth: "60%",
        }),
      ],
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tl, dark]);

  const ref = useEChart(option, [option]);

  return (
    <ChartShell
      dark={dark}
      title={t("portfolio.titleChart")}
      subtitle={t("portfolio.chartDesc")}
      loading={loading}
      error={error}
      empty={!tl.length}
      height={chartHeight.default}
      marginBottom={space[5]}
    >
      <div ref={ref} style={{ height: chartHeight.default }} />
    </ChartShell>
  );
}
