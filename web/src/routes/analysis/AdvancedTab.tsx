import { useMemo, useState } from "react";
import { Chart } from "../../components/charts/Chart";
import { baseChartOption } from "../../components/charts/theme";
import { Button } from "../../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../../components/ui/card";
import { EmptyState } from "../../components/ui/empty-state";
import { Skeleton } from "../../components/ui/skeleton";
import { simulatePath } from "../../services/montecarlo";
import { dailyReturns, mean, pearson, sampleStd } from "../../services/statistics";
import { CodePicker, MAX_COMPARE, useMultiNav } from "./shared";

// ── advanced（相关性热力图 + 蒙特卡洛扇形）────────────────────────────

export function AdvancedTab() {
  const [codes, setCodes] = useState<string[]>([]);
  const navs = useMultiNav(codes.slice(0, MAX_COMPARE));

  // 相关性：按交易日交集对齐日收益率，两两 pearson。
  const correlation = useMemo(() => {
    const returns = codes.map((_, i) => {
      const points = [...(navs[i]?.data ?? [])].sort((a, b) => a.date.localeCompare(b.date));
      const map = new Map<string, number>();
      for (let j = 1; j < points.length; j++) {
        const prev = points[j - 1];
        const cur = points[j];
        if (prev && cur && prev.unit_nav > 0) {
          map.set(cur.date, cur.unit_nav / prev.unit_nav - 1);
        }
      }
      return map;
    });
    const n = codes.length;
    const matrix: number[][] = Array.from({ length: n }, () => Array.from({ length: n }, () => 1));
    for (let i = 0; i < n; i++) {
      for (let j = i + 1; j < n; j++) {
        const a = returns[i];
        const b = returns[j];
        if (!a || !b) continue;
        const common = [...a.keys()].filter((d) => b.has(d)).sort();
        const rowI = matrix[i];
        const rowJ = matrix[j];
        if (!rowI || !rowJ) continue;
        if (common.length < 20) {
          rowI[j] = 0;
          rowJ[i] = 0;
          continue;
        }
        const xs = common.map((d) => a.get(d) ?? 0);
        const ys = common.map((d) => b.get(d) ?? 0);
        const r = pearson(xs, ys);
        rowI[j] = r;
        rowJ[i] = r;
      }
    }
    return matrix;
  }, [codes, navs]);

  // 蒙特卡洛：第一个选中标的的日收益率 → 250 交易日扇形带。
  const mc = useMemo(() => {
    const firstIdx = 0;
    const points = [...(navs[firstIdx]?.data ?? [])].sort((a, b) => a.date.localeCompare(b.date));
    if (!codes[0] || points.length < 60) return null;
    const rets = dailyReturns(points.map((p) => p.unit_nav));
    if (rets.length < 30) return null;
    const mu = mean(rets);
    const sigma = sampleStd(rets);
    const DAYS = 250;
    const SIMS = 500;
    const paths: number[][] = [];
    for (let s = 0; s < SIMS; s++) paths.push(simulatePath(1, mu, sigma, DAYS));
    const band = (p: number) =>
      Array.from({ length: DAYS + 1 }, (_, d) => {
        const col = paths.map((path) => path[d] ?? 1).sort((a, b) => a - b);
        return col[Math.floor(col.length * p)] ?? 1;
      });
    return { p10: band(0.1), p50: band(0.5), p90: band(0.9), code: codes[0] };
  }, [codes, navs]);

  return (
    <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
      <Card>
        <CardHeader>
          <CardTitle>选择标的</CardTitle>
        </CardHeader>
        <CardContent>
          <CodePicker selected={codes} onChange={setCodes} />
          <p className="mt-2 text-[11px] text-fg-3">热力图需要 ≥2 个；蒙特卡洛用第一个标的。</p>
        </CardContent>
      </Card>

      <div className="space-y-4">
        {navs.some((q) => q.isError) && (
          <EmptyState
            title="净值数据加载失败"
            description="部分标的净值拉取失败，请重试。"
            action={
              <Button size="sm" onClick={() => navs.forEach((q) => void q.refetch())}>
                重试
              </Button>
            }
          />
        )}
        <Card>
          <CardHeader>
            <CardTitle>相关性热力图（日收益 pearson）</CardTitle>
          </CardHeader>
          <CardContent>
            {codes.length < 2 ? (
              <EmptyState title="至少选择 2 个标的" />
            ) : (
              <Chart
                loading={navs.some((q) => q.isPending)}
                height={Math.max(240, codes.length * 44)}
                deps={[correlation, codes]}
                option={(t) => ({
                  ...baseChartOption(t),
                  xAxis: {
                    type: "category",
                    data: codes,
                    axisLabel: { color: t.fg3, fontSize: 10 },
                  },
                  yAxis: {
                    type: "category",
                    data: codes,
                    axisLabel: { color: t.fg3, fontSize: 10 },
                  },
                  visualMap: {
                    min: -1,
                    max: 1,
                    orient: "horizontal",
                    left: "center",
                    bottom: 0,
                    textStyle: { color: t.fg3, fontSize: 10 },
                    inRange: { color: [t.down, t.surface2, t.up] },
                  },
                  series: [
                    {
                      type: "heatmap",
                      data: codes.flatMap((_, i) =>
                        codes.map((_, j) => [j, i, Number((correlation[i]?.[j] ?? 0).toFixed(2))]),
                      ),
                      label: { show: true, color: t.fg2, fontSize: 10 },
                    },
                  ],
                })}
              />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>蒙特卡洛扇形（250 交易日 · 500 路径 · P10/P50/P90）</CardTitle>
          </CardHeader>
          <CardContent>
            {navs.some((q) => q.isPending) ? (
              <Skeleton className="h-[300px]" />
            ) : !mc ? (
              <EmptyState title="选择至少 1 个净值历史 ≥60 点的标的" />
            ) : (
              <Chart
                height={300}
                deps={[mc]}
                option={(t) => {
                  const xs = mc.p50.map((_, d) => d);
                  return {
                    ...baseChartOption(t),
                    legend: { top: 0, textStyle: { color: t.fg3, fontSize: 11 } },
                    xAxis: {
                      type: "category",
                      data: xs,
                      axisLabel: { color: t.fg3, fontSize: 11 },
                    },
                    yAxis: {
                      type: "value",
                      scale: true,
                      axisLabel: {
                        color: t.fg3,
                        fontSize: 11,
                        formatter: (v: number) => `${((v - 1) * 100).toFixed(0)}%`,
                      },
                      splitLine: { lineStyle: { color: t.border, type: "dashed" } },
                    },
                    series: [
                      {
                        name: "P90",
                        type: "line",
                        data: mc.p90,
                        showSymbol: false,
                        lineStyle: { opacity: 0 },
                        stack: "band",
                        itemStyle: { color: t.accent },
                      },
                      {
                        name: "band",
                        type: "line",
                        data: mc.p90.map((v, i) => v - (mc.p10[i] ?? v)),
                        showSymbol: false,
                        lineStyle: { opacity: 0 },
                        stack: "band",
                        areaStyle: { color: t.accent, opacity: 0.12 },
                        itemStyle: { color: t.accent },
                      },
                      {
                        name: "P50 中位",
                        type: "line",
                        data: mc.p50,
                        showSymbol: false,
                        lineStyle: { width: 2, color: t.accent },
                        itemStyle: { color: t.accent },
                      },
                      {
                        name: "P10",
                        type: "line",
                        data: mc.p10,
                        showSymbol: false,
                        lineStyle: { width: 1, type: "dashed", color: t.fg3 },
                        itemStyle: { color: t.fg3 },
                      },
                    ],
                  };
                }}
              />
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
