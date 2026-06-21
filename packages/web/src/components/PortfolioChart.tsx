import { useState, useEffect, useMemo, useRef } from "react";
import { useTranslation } from "react-i18next";
import { Text } from "@cloudflare/kumo";
import { use as echartsUse, graphic } from "echarts/core";
import { LineChart, BarChart } from "echarts/charts";
import { GridComponent, TooltipComponent, LegendComponent, DataZoomComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import { getTheme, chartAxis, chartTooltip, chartLegend, chartDataZoom, hexToRgba } from "../styles/theme";
import { useEChart } from "../hooks/useEChart";
import { Card } from "./ui/Card";

echartsUse([LineChart, BarChart, GridComponent, TooltipComponent, LegendComponent, DataZoomComponent, CanvasRenderer]);

interface TimelinePoint {
  date: string;
  total_value: number;
  total_cost: number;
  pnl: number;
  pnl_pct: number | string;
}

export default function PortfolioChart({ dark, portfolioId }: { dark: boolean; portfolioId?: number }) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const [tl, setTl] = useState<TimelinePoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    setLoading(true);
    setError(null);
    const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : "";
    fetch(`/api/portfolio/timeline${qs}`, { signal: ctrl.signal })
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json() as Promise<TimelinePoint[]>;
      })
      .then((data) => {
        setTl(data);
        setLoading(false);
      })
      .catch((e) => {
        if (e.name !== "AbortError") {
          console.warn("[timeline]", e);
          setError(e.message);
          setLoading(false);
        }
      });
    return () => ctrl.abort();
  }, [portfolioId]);

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
        {
          name: t("portfolio.marketValue"), type: "line", data: values, smooth: true, symbol: "none",
          lineStyle: { color: theme.blue, width: 2.5 },
          areaStyle: {
            color: new graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: hexToRgba(theme.blue, 0.26) },
              { offset: 1, color: hexToRgba(theme.blue, 0) },
            ]),
          },
        },
        {
          name: t("portfolio.cost"), type: "line", data: costs, smooth: true, symbol: "none",
          lineStyle: { color: theme.amber, width: 1.5, type: "dashed" },
        },
        {
          name: t("portfolio.dailyPnL"), type: "bar", data: pnls, yAxisIndex: 1,
          itemStyle: { color: (p: any) => (Number(p.value) || 0) >= 0 ? theme.up : theme.down, borderRadius: [2, 2, 0, 0] },
          barWidth: "60%",
        },
      ],
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tl, dark]);

  const ref = useEChart(option, [option]);

  const placeholder = (msg: string, testid: string) => (
    <div data-testid={testid} style={{ height: 420, display: "flex", alignItems: "center", justifyContent: "center", color: theme.textMuted, fontVariantNumeric: "tabular-nums" }}>
      {msg}
    </div>
  );

  return (
    <Card dark={dark} style={{ marginBottom: 20 }}>
      <div style={{ padding: "4px 0 16px" }}>
        <Text variant="heading3" as="h3">{t("portfolio.titleChart")}</Text>
        <Text variant="secondary" as="span" size="xs" style={{ marginTop: 2, display: "block" }}>
          {t("portfolio.chartDesc")}
        </Text>
      </div>
      {loading
        ? placeholder(t("common.loading", "加载中…"), "chart-loading")
        : error
          ? placeholder(t("common.loadError", "加载失败"), "chart-error")
          : !tl.length
            ? placeholder(t("common.noData", "暂无数据"), "chart-empty")
            : <div ref={ref} style={{ height: 420 }} />}
    </Card>
  );
}
