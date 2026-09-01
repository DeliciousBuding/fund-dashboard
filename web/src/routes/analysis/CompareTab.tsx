import { useMemo, useState } from "react";
import { Chart } from "../../components/charts/Chart";
import { baseChartOption } from "../../components/charts/theme";
import { Button } from "../../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../../components/ui/card";
import { EmptyState } from "../../components/ui/empty-state";
import { Skeleton } from "../../components/ui/skeleton";
import { Table, TBody, Td, THead, Th, Tr } from "../../components/ui/table";
import { fmtSignedPct, pnlTone } from "../../lib/format";
import { useCompare } from "../../lib/queries";
import { toneClass } from "../../lib/tones";
import { cn } from "../../lib/utils";
import { useUi } from "../../stores/ui";
import { CodePicker, MAX_COMPARE, useMultiNav } from "./shared";

// ── compare ─────────────────────────────────────────────────────────

export function CompareTab() {
  const portfolioId = useUi((s) => s.portfolioId);
  const [codes, setCodes] = useState<string[]>([]);
  const compare = useCompare(codes, portfolioId);
  const navs = useMultiNav(codes);

  const normalized = useMemo(() => {
    // 归一化：各自首日 = 100，按日期对齐（取交集日期）。
    const series = codes.map((code, i) => {
      const points = navs[i]?.data ?? [];
      if (points.length === 0) return { code, data: [] as [string, number][] };
      const first = points[0]?.unit_nav ?? 1;
      return {
        code,
        data: points.map((p) => [p.date, (p.unit_nav / first) * 100] as [string, number]),
      };
    });
    return series;
  }, [codes, navs]);

  const funds = compare.data?.funds ?? [];
  const METRICS: {
    key: "xirr" | "volatility" | "sharpe" | "max_drawdown" | "calmar";
    label: string;
  }[] = [
    { key: "xirr", label: "XIRR" },
    { key: "volatility", label: "波动率" },
    { key: "sharpe", label: "Sharpe" },
    { key: "max_drawdown", label: "最大回撤" },
    { key: "calmar", label: "Calmar" },
  ];

  return (
    <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
      <Card>
        <CardHeader>
          <CardTitle>选择标的（≤{MAX_COMPARE}）</CardTitle>
        </CardHeader>
        <CardContent>
          <CodePicker selected={codes} onChange={setCodes} />
        </CardContent>
      </Card>

      <div className="space-y-4">
        {codes.length === 0 ? (
          <EmptyState
            title="选择标的开始对比"
            description="最多 8 个，归一化到同一基点观察相对表现。"
          />
        ) : (
          <>
            <Card>
              <CardHeader>
                <CardTitle>归一化净值（首日 = 100）</CardTitle>
              </CardHeader>
              <CardContent>
                <Chart
                  height={300}
                  loading={navs.some((q) => q.isPending)}
                  empty={normalized.every((s) => s.data.length === 0)}
                  emptyText="所选标的暂无净值数据"
                  error={navs.some((q) => q.isError)}
                  errorText="净值对比加载失败"
                  onRetry={() => {
                    navs.forEach((q) => void q.refetch());
                    void compare.refetch();
                  }}
                  deps={[normalized]}
                  option={(t) => ({
                    ...baseChartOption(t),
                    legend: { top: 0, textStyle: { color: t.fg3, fontSize: 11 } },
                    xAxis: { type: "time", axisLabel: { color: t.fg3, fontSize: 11 } },
                    yAxis: {
                      type: "value",
                      scale: true,
                      splitLine: { lineStyle: { color: t.border, type: "dashed" } },
                      axisLabel: { color: t.fg3, fontSize: 11 },
                    },
                    series: normalized.map((s, i) => ({
                      name: s.code,
                      type: "line",
                      showSymbol: false,
                      data: s.data,
                      lineStyle: { width: 1.5, color: t.palette[i % t.palette.length] },
                      itemStyle: { color: t.palette[i % t.palette.length] },
                    })),
                  })}
                />
              </CardContent>
            </Card>

            <div className="grid gap-4 xl:grid-cols-2">
              <Card className="overflow-x-auto" style={{ padding: 0 }}>
                <Table>
                  <THead>
                    <Tr>
                      <Th>指标</Th>
                      {funds.map((f) => (
                        <Th key={f.code} className="text-right">
                          <span className="font-mono text-[11px]">{f.code}</span>
                        </Th>
                      ))}
                    </Tr>
                  </THead>
                  <TBody>
                    {compare.isPending ? (
                      <Tr>
                        <Td colSpan={codes.length + 1}>
                          <Skeleton className="h-6 w-full" />
                        </Td>
                      </Tr>
                    ) : compare.isError ? (
                      <Tr>
                        <Td colSpan={codes.length + 1}>
                          <EmptyState
                            title="指标加载失败"
                            description="无法读取对比指标，请重试。"
                            action={
                              <Button size="sm" onClick={() => void compare.refetch()}>
                                重试
                              </Button>
                            }
                          />
                        </Td>
                      </Tr>
                    ) : (
                      METRICS.map((m) => (
                        <Tr key={m.key}>
                          <Td className="text-fg-3">{m.label}</Td>
                          {funds.map((f) => {
                            const v = f[m.key];
                            return (
                              <Td
                                key={f.code}
                                className={cn(
                                  "text-right tabular-nums",
                                  (m.key === "xirr" || m.key === "sharpe") && toneClass[pnlTone(v)],
                                )}
                              >
                                {v == null
                                  ? "—"
                                  : m.key === "sharpe" || m.key === "calmar"
                                    ? v.toFixed(2)
                                    : fmtSignedPct(m.key === "max_drawdown" ? -Math.abs(v) : v)}
                              </Td>
                            );
                          })}
                        </Tr>
                      ))
                    )}
                  </TBody>
                </Table>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>指标雷达</CardTitle>
                </CardHeader>
                <CardContent>
                  <Chart
                    height={280}
                    loading={compare.isPending}
                    empty={funds.length === 0}
                    error={compare.isError}
                    errorText="指标对比加载失败"
                    onRetry={() => void compare.refetch()}
                    deps={[funds]}
                    option={(t) => {
                      // 归一化到 0-100：每项指标全组 min-max 映射（回撤反向——越小越好）
                      const norm = (vals: number[], invert = false) => {
                        const lo = Math.min(...vals);
                        const hi = Math.max(...vals);
                        if (hi === lo) return vals.map(() => 60);
                        return vals.map((v) => {
                          const x = ((v - lo) / (hi - lo)) * 100;
                          return invert ? 100 - x : x;
                        });
                      };
                      const cols = {
                        xirr: norm(funds.map((f) => f.xirr ?? 0)),
                        sharpe: norm(funds.map((f) => f.sharpe ?? 0)),
                        calmar: norm(funds.map((f) => f.calmar ?? 0)),
                        volatility: norm(
                          funds.map((f) => f.volatility ?? 0),
                          true,
                        ),
                        max_drawdown: norm(
                          funds.map((f) => f.max_drawdown ?? 0),
                          true,
                        ),
                      };
                      return {
                        ...baseChartOption(t),
                        radar: {
                          indicator: [
                            { name: "XIRR", max: 100 },
                            { name: "Sharpe", max: 100 },
                            { name: "Calmar", max: 100 },
                            { name: "低波动", max: 100 },
                            { name: "低回撤", max: 100 },
                          ],
                          axisName: { color: t.fg3, fontSize: 11 },
                          splitLine: { lineStyle: { color: t.border } },
                          splitArea: { show: false },
                        },
                        legend: { bottom: 0, textStyle: { color: t.fg3, fontSize: 11 } },
                        series: [
                          {
                            type: "radar",
                            data: funds.map((f, i) => ({
                              name: f.code,
                              value: [
                                cols.xirr[i],
                                cols.sharpe[i],
                                cols.calmar[i],
                                cols.volatility[i],
                                cols.max_drawdown[i],
                              ],
                              lineStyle: { color: t.palette[i % t.palette.length], width: 1.5 },
                              itemStyle: { color: t.palette[i % t.palette.length] },
                              areaStyle: { opacity: 0.08 },
                            })),
                          },
                        ],
                      };
                    }}
                  />
                </CardContent>
              </Card>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
