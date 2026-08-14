import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Text } from "@cloudflare/kumo";
import { getTheme, chartAxis, chartTooltip, hexToRgba, chartShadowColor, fontSize, chartHeight, space} from "../styles/theme";
import { useEChart } from "../hooks/useEChart";
import { ChartShell, useChartData, useCoreCharts } from "./charts";
import { useAppStore } from "../stores/appStore";
import { pearson, dailyReturns } from "../services/statistics";
import { fetchInvestmentHarness, fetchNav, type NavPoint } from "../api";

// heatmap + visualMap are now in the core set.
useCoreCharts();

const MAX_FUNDS = 8;
const MIN_OVERLAP = 20;

interface HoldingInfo { code: string; name: string; weight_pct: number }

interface MatrixData {
  matrix: { x: number; y: number; value: number }[];
  n: number;
}

interface CorrelationResult {
  holdings: HoldingInfo[];
  matrixData: MatrixData | null;
}

export default function CorrelationHeatmap({ dark }: { dark: boolean }) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const portfolioId = useAppStore((s) => s.portfolioId);

  const { data, loading, error } = useChartData<CorrelationResult>(async (signal) => {
    // AbortSignal is the 2nd arg — never pass it as portfolioId.
    const payload = await fetchInvestmentHarness(portfolioId, signal);

    const signals = payload.holding_signals || [];
    const top: HoldingInfo[] = [...signals]
      .sort((a, b) => b.weight_pct - a.weight_pct)
      .slice(0, MAX_FUNDS)
      .map((s) => ({ code: s.code, name: s.name, weight_pct: s.weight_pct }));
    if (!top.length) return { holdings: [], matrixData: null };

    const allNavs = await Promise.all(
      top.map((h) =>
        fetchNav(h.code, signal)
          .catch((e: any) => { if (e?.name !== 'AbortError') console.warn('[correlation/nav]', e); return [] as NavPoint[]; }),
      ),
    );

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
    if (commonDates.length < MIN_OVERLAP) throw new Error(t("correlation.errorNoOverlap"));

    const aligned = maps.map((m) => commonDates.map((d) => m.get(d)!));
    const returnsList = aligned.map((nav) => dailyReturns(nav));

    const n = top.length;
    const matrix: { x: number; y: number; value: number }[] = [];
    for (let i = 0; i < n; i++) {
      for (let j = 0; j < n; j++) {
        const r2 = i === j ? 1 : pearson(returnsList[i], returnsList[j]);
        matrix.push({ x: i, y: j, value: +r2.toFixed(4) });
      }
    }
    return { holdings: top, matrixData: { matrix, n } };
  }, [portfolioId, t]);

  const holdings = data?.holdings ?? [];
  const matrixData = data?.matrixData ?? null;

  const option = useMemo(() => {
    if (!matrixData) return {} as Record<string, unknown>;
    const { matrix } = matrixData;
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
        axisLabel: { rotate: 45, fontSize: fontSize.xs, color: theme.textMuted },
        splitArea: { show: true },
      },
      yAxis: {
        type: "category", data: labels,
        ...chartAxis(theme),
        axisLabel: { fontSize: fontSize.xs, color: theme.textMuted },
        splitArea: { show: true },
      },
      visualMap: {
        min: -1, max: 1, calculable: true, orient: "horizontal",
        left: "center", bottom: 8,
        inRange: {
          color: [hexToRgba(theme.blue, 0.08), hexToRgba(theme.blue, 0.4), theme.blue],
        },
        textStyle: { color: theme.textMuted, fontSize: fontSize.sm },
      },
      series: [{
        type: "heatmap", data: matrix,
        label: {
          show: true,
          fontSize: fontSize.xs,
          color: theme.textSubtle,
          formatter: (p: any) => p.data.value.toFixed(2),
        },
        emphasis: { itemStyle: { shadowBlur: 10, shadowColor: chartShadowColor(theme) } },
      }],
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [matrixData, dark]);

  const ref = useEChart(option, [option]);

  return (
    <ChartShell
      dark={dark}
      title={t("correlation.title")}
      height={chartHeight.default}
      marginBottom={space[5]}
      loading={loading}
      loadingText={t("correlation.loading")}
      error={error}
      empty={!matrixData}
      headerExtra={
        !loading && !error && matrixData ? (
          <Text variant="secondary" as="span" size="xs">
            {t("correlation.subtitle", { count: holdings.length })}
          </Text>
        ) : undefined
      }
    >
      <div ref={ref} style={{ width: "100%", height: chartHeight.default }} />
    </ChartShell>
  );
}
