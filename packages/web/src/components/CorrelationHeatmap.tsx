import { useState, useEffect, useMemo, useRef } from "react";
import { useTranslation } from "react-i18next";
import { Text } from "@cloudflare/kumo";
import { use as echartsUse, graphic } from "echarts/core";
import { HeatmapChart } from "echarts/charts";
import { GridComponent, TooltipComponent, VisualMapComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import { getTheme, chartAxis, chartTooltip, hexToRgba } from "../styles/theme";
import { useEChart } from "../hooks/useEChart";
import { Card } from "./ui/Card";
import type { NavPoint } from "../api";

echartsUse([HeatmapChart, GridComponent, TooltipComponent, VisualMapComponent, CanvasRenderer]);

const MAX_FUNDS = 8;
const MIN_OVERLAP = 20;

interface HoldingInfo { code: string; name: string; weight_pct: number }

/** Pearson correlation coefficient */
function pearson(a: number[], b: number[]): number {
  const n = a.length;
  if (n < 3) return 0;
  let sa = 0, sb = 0, saa = 0, sbb = 0, sab = 0;
  for (let i = 0; i < n; i++) {
    sa += a[i]; sb += b[i];
    saa += a[i] * a[i]; sbb += b[i] * b[i]; sab += a[i] * b[i];
  }
  const num = n * sab - sa * sb;
  const da = Math.sqrt(n * saa - sa * sa);
  const db = Math.sqrt(n * sbb - sb * sb);
  return da < 1e-10 || db < 1e-10 ? 0 : num / (da * db);
}

/** Compute daily returns from NAV series */
function dailyReturns(navs: number[]): number[] {
  const r: number[] = [];
  for (let i = 1; i < navs.length; i++) {
    r.push((navs[i] - navs[i - 1]) / navs[i - 1]);
  }
  return r;
}

export default function CorrelationHeatmap({ dark }: { dark: boolean }) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [holdings, setHoldings] = useState<HoldingInfo[]>([]);
  const [matrixData, setMatrixData] = useState<{ matrix: { x: number; y: number; value: number }[]; n: number } | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const theme = getTheme(dark);

  // Fetch holdings
  useEffect(() => {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    setLoading(true);
    setError(null);

    fetch("/api/portfolio/harness", { signal: ctrl.signal })
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json();
      })
      .then((data) => {
        const signals = (data.holding_signals || []) as any[];
        const top = signals
          .sort((a: any, b: any) => b.weight_pct - a.weight_pct)
          .slice(0, MAX_FUNDS)
          .map((s: any) => ({ code: s.code, name: s.name, weight_pct: s.weight_pct }));
        setHoldings(top);
      })
      .catch((e) => {
        if (e.name !== "AbortError") {
          console.warn("[Correlation]", e);
          setError(e.message || t("correlation.errorLoadFailed"));
          setLoading(false);
        }
      });
    return () => ctrl.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Fetch NAV data when holdings loaded, build correlation matrix
  useEffect(() => {
    if (!holdings.length) return;

    const ctrl = new AbortController();
    let cancelled = false;

    Promise.all(
      holdings.map((h) =>
        fetch(`/api/funds/${h.code}/nav`, { signal: ctrl.signal })
          .then((r) => r.ok ? r.json() as Promise<NavPoint[]> : Promise.reject(r.statusText))
          .catch(() => [] as NavPoint[]),
      ),
    )
      .then((allNavs) => {
        if (cancelled) return;

        const maps = allNavs.map((nav) => {
          const m = new Map<string, number>();
          nav.forEach((p) => m.set(p.date.substring(0, 10), p.unit_nav));
          return m;
        });

        const dateCounts = new Map<string, number>();
        maps.forEach((m) => m.forEach((_, d) => dateCounts.set(d, (dateCounts.get(d) || 0) + 1)));
        const commonDates = [...dateCounts.entries()]
          .filter(([, c]) => c === maps.length)
          .map(([d]) => d)
          .sort();
        if (commonDates.length < MIN_OVERLAP) {
          if (!cancelled) { setError(t("correlation.errorNoOverlap")); setLoading(false); }
          return;
        }

        const aligned = maps.map((m) => commonDates.map((d) => m.get(d)!));
        const returnsList = aligned.map((nav) => dailyReturns(nav));

        const n = holdings.length;
        const matrix: { x: number; y: number; value: number }[] = [];
        for (let i = 0; i < n; i++) {
          for (let j = 0; j < n; j++) {
            const r = i === j ? 1 : pearson(returnsList[i], returnsList[j]);
            matrix.push({ x: i, y: j, value: +r.toFixed(4) });
          }
        }

        if (!cancelled) {
          setMatrixData({ matrix, n });
          setLoading(false);
        }
      })
      .catch((e) => {
        if (!cancelled) {
          console.warn("[Correlation]", e);
          setError(e.message || t("correlation.errorLoadFailed"));
          setLoading(false);
        }
      });

    return () => { cancelled = true; ctrl.abort(); };
  }, [holdings, t]);

  const option = useMemo(() => {
    if (!matrixData) return {} as Record<string, unknown>;
    const { matrix, n } = matrixData;
    const labels = holdings.map((h) => (h.name.length > 6 ? h.name.slice(0, 6) + "…" : h.name));

    return {
      tooltip: {
        position: "top",
        ...chartTooltip(theme),
        formatter: (p: any) => {
          const { x, y, value } = p.data;
          return t("correlation.tooltip", { nameX: holdings[x].name, nameY: holdings[y].name, value: value.toFixed(4) });
        },
      },
      grid: { left: 110, right: 40, top: 20, bottom: 80 },
      xAxis: {
        type: "category", data: labels, position: "bottom",
        ...chartAxis(theme),
        axisLabel: { rotate: 45, fontSize: 10, color: theme.textMuted },
        splitArea: { show: true },
      },
      yAxis: {
        type: "category", data: labels,
        ...chartAxis(theme),
        axisLabel: { fontSize: 10, color: theme.textMuted },
        splitArea: { show: true },
      },
      visualMap: {
        min: -1, max: 1, calculable: true, orient: "horizontal",
        left: "center", bottom: 8,
        inRange: {
          color: [hexToRgba(theme.blue, 0.08), hexToRgba(theme.blue, 0.4), theme.blue],
        },
        textStyle: { color: theme.textMuted, fontSize: 11 },
      },
      series: [{
        type: "heatmap", data: matrix,
        label: {
          show: true,
          fontSize: 9,
          color: theme.textSubtle,
          formatter: (p: any) => p.data.value.toFixed(2),
        },
        emphasis: { itemStyle: { shadowBlur: 10, shadowColor: "rgba(0,0,0,0.3)" } },
      }],
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [matrixData, dark]);

  const ref = useEChart(option, [option]);

  const placeholder = (msg: string, testid: string) => (
    <div
      data-testid={testid}
      style={{
        height: 420,
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
          <Text variant="heading3" as="h3">{t("correlation.title")}</Text>
          {!loading && !error && matrixData && (
            <Text variant="secondary" as="span" size="xs">
              {t("correlation.subtitle", { count: holdings.length })}
            </Text>
          )}
        </div>
        {loading
          ? placeholder(t("correlation.loading"), "chart-loading")
          : error
            ? placeholder(error, "chart-error")
            : !matrixData
              ? placeholder(t("common.noData", "暂无数据"), "chart-empty")
              : <div ref={ref} style={{ width: "100%", height: 420 }} />}
      </div>
    </Card>
  );
}
