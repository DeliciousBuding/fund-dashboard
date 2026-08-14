import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Text } from "@cloudflare/kumo";
import { getTheme, chartAxis, chartTooltip, space, radius, fontSize, fontWeight, chartHeight } from "../styles/theme";
import { useEChart } from "../hooks/useEChart";
import { useChartData, barSeries, useCoreCharts, ChartShell } from "./charts";
import { useAppStore } from "../stores/appStore";
import { fetchInvestmentHarness, fetchNav, type NavPoint } from "../api";
import { runMonteCarlo } from "../services/montecarlo";

useCoreCharts();

const SIMULATIONS = 2000;
const TRADING_DAYS = 252;
const MAX_FUNDS = 6;
const BINS = 40;

interface HoldingInfo { code: string; name: string; weight_pct: number }

interface MonteCarloStats {
  mean: number;
  median: number;
  p5: number;
  p95: number;
}

interface ChartBinData {
  labels: string[];
  density: number[];
  bins: number[];
  results: number[];
  dailyMean: number;
  minR: number;
  binW: number;
}

interface SimResult {
  stats: MonteCarloStats;
  chartData: ChartBinData;
}

export default function MonteCarloChart({ dark }: { dark: boolean }) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const portfolioId = useAppStore((s) => s.portfolioId);

  const { data, loading, error } = useChartData<SimResult>(async (signal) => {
    // AbortSignal is the 2nd arg — never pass it as portfolioId.
    const payload = await fetchInvestmentHarness(portfolioId, signal);

    const signals = payload.holding_signals || [];
    if (!signals.length) throw new Error(t("montecarlo.errorNoData"));

    const top: HoldingInfo[] = [...signals]
      .sort((a, b) => b.weight_pct - a.weight_pct)
      .slice(0, MAX_FUNDS)
      .map((s) => ({ code: s.code, name: s.name, weight_pct: s.weight_pct }));

    const navResults = await Promise.all(
      top.map((h) =>
        fetchNav(h.code, signal)
          .catch((e: any) => { if (e?.name !== 'AbortError') console.warn('[montecarlo/nav]', e); return [] as NavPoint[]; }),
      ),
    );

    const maps = navResults.map((nav) => {
      const m = new Map<string, number>();
      nav.forEach((p) => m.set(p.date.substring(0, 10), p.unit_nav));
      return m;
    });
    const totalWeight = top.reduce((s, h) => s + h.weight_pct, 0);
    const dateCounts = new Map<string, number>();
    maps.forEach((m) => m.forEach((_, d) => dateCounts.set(d, (dateCounts.get(d) || 0) + 1)));
    const commonDates = [...dateCounts.entries()]
      .filter(([, c]) => c >= Math.max(2, maps.length / 2))
      .map(([d]) => d).sort();
    if (commonDates.length < 30) throw new Error(t("montecarlo.errorNoOverlap"));

    const portReturns: number[] = [];
    for (let i = 1; i < commonDates.length; i++) {
      const prev = commonDates[i - 1], curr = commonDates[i];
      let wr = 0;
      for (let j = 0; j < maps.length; j++) {
        const pv = maps[j].get(prev), cv = maps[j].get(curr);
        if (pv && cv && pv > 0) {
          const w = top[j].weight_pct / totalWeight;
          wr += w * ((cv - pv) / pv);
        }
      }
      portReturns.push(wr);
    }
    if (portReturns.length < 30) throw new Error(t("montecarlo.errorInsufficient"));

    const mc = runMonteCarlo(portReturns, SIMULATIONS, TRADING_DAYS);
    const results = mc.paths.map((p) => p[0]); // already sorted

    const binLabels = mc.histogram.map((h) => `${(h.bucket * 100).toFixed(0)}%`);
    const density = mc.histogram.map((h) => h.count / SIMULATIONS);
    const bins = mc.histogram.map((h) => h.count);
    const binW = mc.histogram.length > 1 ? mc.histogram[1].bucket - mc.histogram[0].bucket : 0.01;

    return {
      stats: {
        mean: mc.stats.mean * TRADING_DAYS,
        median: mc.percentiles.p50,
        p5: mc.percentiles.p5,
        p95: mc.percentiles.p95,
      },
      chartData: {
        labels: binLabels,
        density,
        bins,
        results,
        dailyMean: mc.stats.mean,
        minR: mc.stats.min,
        binW,
      },
    };
  }, [portfolioId, t]);

  const stats = data?.stats ?? null;
  const chartData = data?.chartData ?? null;

  const option = useMemo(() => {
    if (!chartData) return {} as Record<string, unknown>;
    const { labels, density, bins, results, minR, binW } = chartData;
    const median = results[Math.floor(results.length / 2)];
    const medianIdx = Math.min(Math.max(0, Math.floor((median - minR) / binW)), labels.length - 1);

    return {
      tooltip: {
        trigger: "axis",
        ...chartTooltip(theme),
        formatter: (ps: any) => {
          const p = ps[0];
          if (!p) return "";
          const idx = p.dataIndex;
          return (
            `<b>${t("montecarlo.range")}:</b> ${labels[idx]}<br/>` +
            `<b>${t("montecarlo.density")}:</b> ${(density[idx] * 100).toFixed(2)}%<br/>` +
            `<b>${t("montecarlo.count")}:</b> ${bins[idx]}`
          );
        },
      },
      grid: { left: 55, right: 30, top: 20, bottom: 50 },
      xAxis: {
        type: "category",
        data: labels,
        ...chartAxis(theme),
        axisLabel: { rotate: 45, fontSize: fontSize.xs, color: theme.textMuted, interval: Math.floor(BINS / 12) },
      },
      yAxis: {
        type: "value",
        name: t("montecarlo.density"),
        nameTextStyle: { color: theme.textMuted, fontSize: fontSize.sm },
        ...chartAxis(theme),
        axisLabel: { formatter: (v: number) => `${(v * 100).toFixed(0)}%`, color: theme.textMuted },
      },
      series: [
        barSeries({
          data: density,
          color: theme.blue,
          borderRadius: [2, 2, 0, 0],
          markLine: {
            silent: true,
            symbol: "none",
            data: [
              {
                xAxis: labels[medianIdx],
                name: t("montecarlo.medianLine"),
                lineStyle: { color: theme.amber, type: "dashed" },
                label: { formatter: t("montecarlo.medianLine"), fontSize: fontSize.xs, color: theme.amber },
              },
            ],
          },
        }),
      ],
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chartData, dark]);

  const ref = useEChart(option, [option]);
  const annualMean = stats ? stats.mean * 100 : 0;

  return (
    <ChartShell
      dark={dark}
      title={t("montecarlo.title")}
      subtitle={
        !loading && !error && stats
          ? t("montecarlo.subtitle", { sims: SIMULATIONS.toLocaleString(), days: TRADING_DAYS })
          : undefined
      }
      loading={loading}
      error={error}
      empty={!loading && !error && !chartData}
      loadingText={t("montecarlo.loading")}
      height={chartHeight.panel}
      marginBottom={space[5]}
      testidPrefix="chart"
    >
      {stats && chartData && (
        <>
          <div style={{ display: "flex", gap: space[4], marginBottom: space[3], flexWrap: "wrap" }}>
            {[
              { label: t("montecarlo.expectedReturn"), value: `${annualMean >= 0 ? "+" : ""}${annualMean.toFixed(2)}%`, color: annualMean >= 0 ? theme.up : theme.down },
              { label: t("montecarlo.median"), value: `${(stats.median * 100).toFixed(2)}%`, color: stats.median >= 0 ? theme.up : theme.down },
              { label: t("montecarlo.var5"), value: `${(stats.p5 * 100).toFixed(2)}%`, color: theme.amber },
              { label: t("montecarlo.p95"), value: `${(stats.p95 * 100).toFixed(2)}%`, color: theme.blue },
            ].map((s) => (
              <div key={s.label} style={{ textAlign: "center" }}>
                <Text variant="secondary" as="span" size="xs">{s.label}</Text>
                <div style={{ fontSize: fontSize.xl, fontWeight: fontWeight.bold, color: s.color, marginTop: 2 }}>{s.value}</div>
              </div>
            ))}
          </div>
          <div ref={ref} style={{ width: "100%", height: chartHeight.compact }} />
        </>
      )}
    </ChartShell>
  );
}
