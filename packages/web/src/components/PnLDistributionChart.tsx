import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Text } from "@cloudflare/kumo";
import { getTheme, chartAxis, chartTooltip, hexToRgba, fontSize, chartHeight, space} from "../styles/theme";
import { useEChart } from "../hooks/useEChart";
import { ChartShell, useChartData, useCoreCharts } from "./charts";
import { useAppStore } from "../stores/appStore";
import { fetchInvestmentHarness } from "../api";

useCoreCharts();

interface HoldingItem {
  code: string;
  name: string;
  pnl_pct: number | null;
  current_value: number;
  security_type: string;
}

// CN convention: red = profit/up, green = loss/down
// Opacity steps: 0.4 (low intensity) → 1.0 (full intensity)
// Half-open [min, max) for intermediate; last bucket is [min, +∞).
export const PNL_BUCKETS = [
  { key: "loss_30plus", label: "< -30%", min: -Infinity, max: -30 },
  { key: "loss_20_30", label: "-30 ~ -20%", min: -30, max: -20 },
  { key: "loss_10_20", label: "-20 ~ -10%", min: -20, max: -10 },
  { key: "loss_0_10", label: "-10 ~ 0%", min: -10, max: 0 },
  { key: "gain_0_10", label: "0 ~ +10%", min: 0, max: 10 },
  { key: "gain_10_20", label: "+10 ~ +20%", min: 10, max: 20 },
  { key: "gain_20_30", label: "+20 ~ +30%", min: 20, max: 30 },
  { key: "gain_30plus", label: "> +30%", min: 30, max: Infinity },
] as const;

/** Resolve which PnL bucket index a value falls into; null if not a finite number. */
export function pnlBucketIndex(pnlPct: number, buckets = PNL_BUCKETS): number | null {
  if (!Number.isFinite(pnlPct)) return null;
  for (let i = 0; i < buckets.length; i++) {
    const b = buckets[i];
    const isLast = i === buckets.length - 1;
    if (isLast ? pnlPct >= b.min : pnlPct >= b.min && pnlPct < b.max) return i;
  }
  return null;
}

const BUCKETS = PNL_BUCKETS;

const LOSS_ALPHA = [1.0, 0.75, 0.5, 0.4]; // darkest for deep loss → lightest for slight loss
const GAIN_ALPHA = [0.4, 0.5, 0.75, 1.0]; // lightest for slight gain → darkest for strong gain

export default function PnLDistributionChart({ dark }: { dark: boolean }) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const portfolioId = useAppStore((s) => s.portfolioId);

  // AbortSignal is the 2nd arg — never pass it as portfolioId.
  const { data, loading, error } = useChartData(
    (signal) => fetchInvestmentHarness(portfolioId, signal),
    [portfolioId],
  );

  const holdings = useMemo<HoldingItem[]>(
    () =>
      (data?.holding_signals || []).map((s: any) => ({
        code: s.code,
        name: s.name,
        // Prefer snapshot unrealized pnl_pct; fall back to cost-deviation for older payloads.
        pnl_pct: s.pnl_pct != null ? s.pnl_pct : s.deviation_pct,
        current_value: s.current_value,
        security_type: s.security_type,
      })),
    [data],
  );
  const totalWithData = holdings.filter((h) => h.pnl_pct != null).length;

  const option = useMemo(() => {
    if (!holdings.length) return {} as Record<string, unknown>;

    // Bucket holdings by PnL (CN convention: red = profit, green = loss)
    // Half-open [min,max) for intermediate buckets; last is [min, +∞].
    // Avoids dropping exact edges (e.g. -10, +10) that the old `> min && <= max` missed.
    const byCount = BUCKETS.map((b, i) => {
      const count = holdings.filter(
        (h) => h.pnl_pct != null && pnlBucketIndex(Number(h.pnl_pct)) === i,
      ).length;
      const value = holdings
        .filter((h) => h.pnl_pct != null && pnlBucketIndex(Number(h.pnl_pct)) === i)
        .reduce((sum, h) => sum + (Number(h.current_value) || 0), 0);
      return { ...b, count, value };
    });

    const labels = byCount.map((b) => b.label);
    const unclassified = holdings.filter((h) => h.pnl_pct == null).length;

    return {
      tooltip: {
        trigger: "axis",
        axisPointer: { type: "shadow" },
        ...chartTooltip(theme),
        formatter: (params: any) => {
          const idx = params[0]?.dataIndex;
          if (idx == null) return "";
          const b = byCount[idx];
          return (
            `<b>${b.label}</b><br/>${t("pnlDist.tooltipCount", { count: b.count })}` +
            (unclassified && idx === 0 ? `<br/>${t("pnlDist.tooltipUnclassified", { count: unclassified })}` : "") +
            `<br/>${t("common.value")}: ¥${Number(b.value).toLocaleString()}`
          );
        },
      },
      grid: { top: 20, right: 20, bottom: 50, left: 50 },
      xAxis: {
        type: "category",
        data: labels,
        ...chartAxis(theme),
        axisLabel: { rotate: 45, fontSize: fontSize.sm, color: theme.textMuted },
      },
      yAxis: {
        type: "value",
        name: t("pnlDist.countAxis"),
        nameTextStyle: { color: theme.textMuted, fontSize: fontSize.sm },
        ...chartAxis(theme),
      },
      series: [
        {
          type: "bar",
          data: byCount.map((b) => {
            const isGain = b.key.startsWith("gain");
            const alphaIdx = isGain
              ? ["gain_0_10", "gain_10_20", "gain_20_30", "gain_30plus"].indexOf(b.key)
              : ["loss_0_10", "loss_10_20", "loss_20_30", "loss_30plus"].indexOf(b.key);
            const alpha = isGain ? GAIN_ALPHA[alphaIdx] ?? 0.5 : LOSS_ALPHA[alphaIdx] ?? 0.5;
            return {
              value: b.count,
              itemStyle: {
                color: hexToRgba(isGain ? theme.up : theme.down, alpha),
                borderRadius: [4, 4, 0, 0],
              },
            };
          }),
          emphasis: { itemStyle: { opacity: 0.85 } },
        },
      ],
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [holdings, dark]);

  const ref = useEChart(option, [option]);

  return (
    <ChartShell
      dark={dark}
      title={t("pnlDist.title")}
      height={chartHeight.distribution}
      marginBottom={space[5]}
      loading={loading}
      error={error}
      empty={!holdings.length}
      headerExtra={
        !loading && !error && holdings.length > 0 ? (
          <Text variant="secondary" as="span" size="xs">
            {t("pnlDist.summary", { total: holdings.length, withData: totalWithData })}
          </Text>
        ) : undefined
      }
    >
      <div ref={ref} style={{ width: "100%", height: chartHeight.distribution }} />
    </ChartShell>
  );
}
