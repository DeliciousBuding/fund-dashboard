import { useState, useEffect, useMemo, useRef } from "react";
import { useTranslation } from "react-i18next";
import { Text } from "@cloudflare/kumo";
import { use as echartsUse, graphic } from "echarts/core";
import { BarChart } from "echarts/charts";
import { GridComponent, TooltipComponent, MarkLineComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import { getTheme, chartAxis, chartTooltip, hexToRgba } from "../styles/theme";
import { useEChart } from "../hooks/useEChart";
import { Card } from "./ui/Card";
import type { NavPoint } from "../api";

echartsUse([BarChart, GridComponent, TooltipComponent, MarkLineComponent, CanvasRenderer]);

const SIMULATIONS = 10000;
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

/** Box-Muller normal random sampler */
function normalRandom(mean: number, std: number): number {
  let u = 0, v = 0;
  while (u === 0) u = Math.random();
  while (v === 0) v = Math.random();
  return mean + std * Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * v);
}

/** Run one simulation path: cumulative return after TRADING_DAYS */
function simulatePath(dailyMean: number, dailyStd: number): number {
  let cum = 1;
  for (let t = 0; t < TRADING_DAYS; t++) {
    cum *= (1 + normalRandom(dailyMean, dailyStd));
  }
  return cum - 1; // return as fractional return
}

export default function MonteCarloChart({ dark }: { dark: boolean }) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [stats, setStats] = useState<MonteCarloStats | null>(null);
  const [chartData, setChartData] = useState<ChartBinData | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const theme = getTheme(dark);

  useEffect(() => {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    let cancelled = false;
    setLoading(true);
    setError(null);
    setStats(null);
    setChartData(null);

    fetch("/api/portfolio/harness", { signal: ctrl.signal })
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json();
      })
      .then(async (data) => {
        const signals = (data.holding_signals || []) as any[];
        if (!signals.length) { if (!cancelled) { setError(t("montecarlo.errorNoData")); setLoading(false); } return; }

        const top: HoldingInfo[] = signals
          .sort((a: any, b: any) => b.weight_pct - a.weight_pct)
          .slice(0, MAX_FUNDS)
          .map((s: any) => ({ code: s.code, name: s.name, weight_pct: s.weight_pct }));

        const navResults = await Promise.all(top.map((h) =>
          fetch(`/api/funds/${h.code}/nav`, { signal: ctrl.signal })
            .then((r) => r.ok ? r.json() as Promise<NavPoint[]> : Promise.reject(r.statusText))
            .catch(() => [] as NavPoint[]),
        ));
        if (cancelled) return;

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
        if (commonDates.length < 30) { if (!cancelled) { setError(t("montecarlo.errorNoOverlap")); setLoading(false); } return; }

        const portReturns: number[] = [];
        for (let t = 1; t < commonDates.length; t++) {
          const prev = commonDates[t - 1], curr = commonDates[t];
          let wr = 0;
          for (let i = 0; i < maps.length; i++) {
            const pv = maps[i].get(prev), cv = maps[i].get(curr);
            if (pv && cv && pv > 0) {
              const w = top[i].weight_pct / totalWeight;
              wr += w * ((cv - pv) / pv);
            }
          }
          portReturns.push(wr);
        }
        if (portReturns.length < 30) { if (!cancelled) { setError(t("montecarlo.errorInsufficient")); setLoading(false); } return; }

        const n = portReturns.length;
        const mean = portReturns.reduce((s, v) => s + v, 0) / n;
        const variance = portReturns.reduce((s, v) => s + (v - mean) ** 2, 0) / (n - 1);
        const std = Math.sqrt(variance);

        const results: number[] = [];
        for (let s = 0; s < SIMULATIONS; s++) {
          results.push(simulatePath(mean, std));
        }
        results.sort((a, b) => a - b);
        const median = results[Math.floor(results.length / 2)];
        const p5 = results[Math.floor(results.length * 0.05)];
        const p95 = results[Math.floor(results.length * 0.95)];

        const minR = results[0], maxR = results[results.length - 1];
        const binW = (maxR - minR) / BINS || 0.01;
        const bins = new Array(BINS).fill(0);
        for (const r of results) {
          const idx = Math.min(BINS - 1, Math.floor((r - minR) / binW));
          bins[idx]++;
        }
        const density = bins.map((v) => v / SIMULATIONS);
        const binLabels = bins.map((_, i) => `${((minR + binW * i) * 100).toFixed(0)}%`);

        if (!cancelled) {
          setStats({ mean: mean * TRADING_DAYS, median, p5, p95 });
          setChartData({ labels: binLabels, density, bins, results, dailyMean: mean, minR, binW });
          setLoading(false);
        }
      })
      .catch((e) => {
        if (!cancelled) {
          console.warn("[MonteCarlo]", e);
          setError(e.message || t("correlation.errorLoadFailed"));
          setLoading(false);
        }
      });

    return () => { cancelled = true; ctrl.abort(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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
          const p = ps[0]; if (!p) return "";
          const idx = p.dataIndex;
          return (
            `<b>${t("montecarlo.title")}:</b> ${labels[idx]}<br/>` +
            `<b>${t("montecarlo.density")}:</b> ${(density[idx] * 100).toFixed(2)}%<br/>` +
            `<b>${t("montecarlo.title")}:</b> ${bins[idx]}`
          );
        },
      },
      grid: { left: 55, right: 30, top: 20, bottom: 50 },
      xAxis: {
        type: "category", data: labels,
        ...chartAxis(theme),
        axisLabel: { rotate: 45, fontSize: 9, color: theme.textMuted, interval: Math.floor(BINS / 12) },
      },
      yAxis: {
        type: "value", name: t("montecarlo.density"),
        nameTextStyle: { color: theme.textMuted, fontSize: 11 },
        ...chartAxis(theme),
        axisLabel: { formatter: (v: number) => `${(v * 100).toFixed(0)}%`, color: theme.textMuted },
      },
      series: [{
        type: "bar",
        data: density.map((d) => ({ value: d, itemStyle: { color: theme.blue, borderRadius: [2, 2, 0, 0] } })),
        markLine: {
          silent: true, symbol: "none",
          data: [
            {
              xAxis: labels[medianIdx],
              name: t("montecarlo.medianLine"),
              lineStyle: { color: theme.amber, type: "dashed" },
              label: { formatter: t("montecarlo.medianLine"), fontSize: 10, color: theme.amber },
            },
          ],
        },
      }],
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chartData, dark]);

  const ref = useEChart(option, [option]);

  const annualMean = stats ? stats.mean * 100 : 0;

  const placeholder = (msg: string, testid: string) => (
    <div
      data-testid={testid}
      style={{
        height: 360,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        color: theme.textMuted,
        fontVariantNumeric: "tabular-nums",
      }}
    >
      {msg}
    </div>
  );

  return (
    <Card dark={dark} style={{ marginBottom: 20 }}>
      <div style={{ padding: "4px 0 16px" }}>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "baseline",
            marginBottom: 8,
          }}
        >
          <Text variant="heading3" as="h3">{t("montecarlo.title")}</Text>
          {!loading && !error && stats && (
            <Text variant="secondary" as="span" size="xs">
              {t("montecarlo.subtitle", { sims: SIMULATIONS.toLocaleString(), days: TRADING_DAYS })}
            </Text>
          )}
        </div>
        {loading
          ? placeholder(t("montecarlo.loading"), "chart-loading")
          : error
            ? placeholder(error, "chart-error")
            : !chartData
              ? placeholder(t("common.noData", "暂无数据"), "chart-empty")
              : (
                <>
                  <div style={{ display: "flex", gap: 16, marginBottom: 12, flexWrap: "wrap" }}>
                    {[
                      { label: t("montecarlo.expectedReturn"), value: `${annualMean >= 0 ? "+" : ""}${annualMean.toFixed(2)}%`, color: annualMean >= 0 ? theme.up : theme.down },
                      { label: t("montecarlo.median"), value: `${(stats!.median * 100).toFixed(2)}%`, color: stats!.median >= 0 ? theme.up : theme.down },
                      { label: t("montecarlo.var5"), value: `${(stats!.p5 * 100).toFixed(2)}%`, color: theme.amber },
                      { label: t("montecarlo.p95"), value: `${(stats!.p95 * 100).toFixed(2)}%`, color: theme.blue },
                    ].map((s) => (
                      <div key={s.label} style={{ textAlign: "center" }}>
                        <Text variant="secondary" as="span" size="xs">{s.label}</Text>
                        <div style={{ fontSize: 16, fontWeight: 700, color: s.color, marginTop: 2 }}>{s.value}</div>
                      </div>
                    ))}
                  </div>
                  <div ref={ref} style={{ width: "100%", height: 300 }} />
                </>
              )}
      </div>
    </Card>
  );
}
