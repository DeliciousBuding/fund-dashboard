import { useState, useEffect, useMemo, useRef } from "react";
import { Text } from "@cloudflare/kumo";
import { use as echartsUse, graphic } from "echarts/core";
import { BarChart } from "echarts/charts";
import { GridComponent, TooltipComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import { getTheme, chartAxis, chartTooltip, hexToRgba } from "../styles/theme";
import { useEChart } from "../hooks/useEChart";
import { Card } from "./ui/Card";

echartsUse([BarChart, GridComponent, TooltipComponent, CanvasRenderer]);

interface HoldingItem {
  code: string;
  name: string;
  pnl_pct: number | null;
  current_value: number;
  security_type: string;
}

// CN convention: red = profit/up, green = loss/down
// Opacity steps: 0.4 (low intensity) → 1.0 (full intensity)
const BUCKETS = [
  { key: "loss_30plus", label: "< -30%", min: -Infinity, max: -30 },
  { key: "loss_20_30",  label: "-30 ~ -20%", min: -30, max: -20 },
  { key: "loss_10_20",  label: "-20 ~ -10%", min: -20, max: -10 },
  { key: "loss_0_10",   label: "-10 ~ 0%", min: -10, max: 0 },
  { key: "gain_0_10",   label: "0 ~ +10%", min: 0, max: 10 },
  { key: "gain_10_20",  label: "+10 ~ +20%", min: 10, max: 20 },
  { key: "gain_20_30",  label: "+20 ~ +30%", min: 20, max: 30 },
  { key: "gain_30plus", label: "> +30%", min: 30, max: Infinity },
];

const LOSS_ALPHA = [1.0, 0.75, 0.5, 0.4];  // darkest for deep loss → lightest for slight loss
const GAIN_ALPHA = [0.4, 0.5, 0.75, 1.0]; // lightest for slight gain → darkest for strong gain

export default function PnLDistributionChart({ dark }: { dark: boolean }) {
  const [holdings, setHoldings] = useState<HoldingItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const theme = getTheme(dark);

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
        const items: HoldingItem[] = (data.holding_signals || []).map((s: any) => ({
          code: s.code,
          name: s.name,
          pnl_pct: s.deviation_pct,
          current_value: s.current_value,
          security_type: s.security_type,
        }));
        setHoldings(items);
        setLoading(false);
      })
      .catch((e) => {
        if (e.name !== "AbortError") {
          console.warn("[PnLDistribution]", e);
          setError(e.message);
          setLoading(false);
        }
      });
    return () => ctrl.abort();
  }, []);

  const totalWithData = holdings.filter((h) => h.pnl_pct != null).length;

  const option = useMemo(() => {
    if (!holdings.length) return {} as Record<string, unknown>;

    // Bucket holdings by PnL (CN convention: red = profit, green = loss)
    const byCount = BUCKETS.map((b) => ({
      ...b,
      count: holdings.filter(
        (h) =>
          h.pnl_pct != null &&
          Number(h.pnl_pct) > b.min &&
          Number(h.pnl_pct) <= b.max,
      ).length,
      value: holdings
        .filter(
          (h) =>
            h.pnl_pct != null &&
            Number(h.pnl_pct) > b.min &&
            Number(h.pnl_pct) <= b.max,
        )
        .reduce((sum, h) => sum + (Number(h.current_value) || 0), 0),
    }));

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
            `<b>${b.label}</b><br/>持仓数: ${b.count} 只` +
            (unclassified && idx === 0
              ? `<br/>未分类(无成本): ${unclassified} 只`
              : "") +
            `<br/>市值: ¥${Number(b.value).toLocaleString()}`
          );
        },
      },
      grid: { top: 20, right: 20, bottom: 50, left: 50 },
      xAxis: {
        type: "category",
        data: labels,
        ...chartAxis(theme),
        axisLabel: { rotate: 45, fontSize: 11, color: theme.textMuted },
      },
      yAxis: {
        type: "value",
        name: "持仓数",
        nameTextStyle: { color: theme.textMuted, fontSize: 11 },
        ...chartAxis(theme),
      },
      series: [
        {
          type: "bar",
          data: byCount.map((b, i) => {
            const isGain = b.key.startsWith("gain");
            const alphaIdx = isGain
              ? ["gain_0_10", "gain_10_20", "gain_20_30", "gain_30plus"].indexOf(
                  b.key,
                )
              : ["loss_0_10", "loss_10_20", "loss_20_30", "loss_30plus"].indexOf(
                  b.key,
                );
            const alpha = isGain
              ? GAIN_ALPHA[alphaIdx] ?? 0.5
              : LOSS_ALPHA[alphaIdx] ?? 0.5;
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

  const placeholder = (msg: string, testid: string) => (
    <div
      data-testid={testid}
      style={{
        height: 280,
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
          <Text variant="heading3" as="h3">
            盈亏分布
          </Text>
          {!loading && !error && holdings.length > 0 && (
            <Text variant="secondary" as="span" size="xs">
              {holdings.length} 只持仓 · {totalWithData} 只有成本数据
            </Text>
          )}
        </div>
        {loading
          ? placeholder("加载中…", "chart-loading")
          : error
            ? placeholder("加载失败", "chart-error")
            : !holdings.length
              ? placeholder("暂无数据", "chart-empty")
              : <div ref={ref} style={{ width: "100%", height: 280 }} />}
      </div>
    </Card>
  );
}
