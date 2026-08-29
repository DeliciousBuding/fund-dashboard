// 分析 /analysis —— 四 tab（01 文档 §4）：对比（归一化净值+指标表+雷达）/
// 回测（定投 vs 一次性，继承旧 DcaBacktestChart 语义）/ 高级（相关性热力图+蒙特卡洛扇形，
// 纯函数客户端计算）/ 穿透（底层股票暴露 treemap + 行业聚合）。
// 深链：?tab=compare|backtest|advanced|penetration。

import { useQueries } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import { Chart } from "../components/charts/Chart";
import { baseChartOption } from "../components/charts/theme";
import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { EmptyState } from "../components/ui/empty-state";
import { Input, Label } from "../components/ui/input";
import { Skeleton } from "../components/ui/skeleton";
import { Table, TBody, Td, THead, Th, Tr } from "../components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs";
import { api } from "../lib/api";
import { fmtCNY, fmtPct, fmtSignedPct, pnlTone } from "../lib/format";
import { useCompare, useNavHistory, usePenetration, useSecurities } from "../lib/queries";
import { cn } from "../lib/utils";
import { calcIRR } from "../services/irr";
import { simulatePath } from "../services/montecarlo";
import { dailyReturns, mean, pearson, sampleStd } from "../services/statistics";
import { useUi } from "../stores/ui";

const toneClass = { up: "text-up", down: "text-down", flat: "text-fg-3" } as const;
const MAX_COMPARE = 8;

// ── 标的多选（compare/backtest/advanced 共用）─────────────────────────

function CodePicker(props: {
  selected: string[];
  onChange: (codes: string[]) => void;
  max?: number;
}) {
  const portfolioId = useUi((s) => s.portfolioId);
  const securities = useSecurities(portfolioId);
  const [query, setQuery] = useState("");

  const list = (securities.data ?? []).filter((s) => {
    if (!query) return true;
    const q = query.toLowerCase();
    return s.code.toLowerCase().includes(q) || s.name.toLowerCase().includes(q);
  });

  return (
    <div className="space-y-2">
      <Input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="搜索标的加入对比…"
        aria-label="搜索标的"
      />
      <div className="flex flex-wrap gap-1.5">
        {props.selected.map((code) => {
          const sec = (securities.data ?? []).find((s) => s.code === code);
          return (
            <Badge
              key={code}
              tone="accent"
              className="cursor-pointer"
              onClick={() => props.onChange(props.selected.filter((c) => c !== code))}
            >
              {sec?.name ?? code} ✕
            </Badge>
          );
        })}
      </div>
      <div className="max-h-44 overflow-y-auto rounded-lg border border-border">
        {list.map((s) => {
          const on = props.selected.includes(s.code);
          return (
            <button
              key={s.code}
              type="button"
              disabled={!on && props.selected.length >= (props.max ?? MAX_COMPARE)}
              onClick={() =>
                props.onChange(
                  on ? props.selected.filter((c) => c !== s.code) : [...props.selected, s.code],
                )
              }
              className={cn(
                "flex w-full items-center justify-between px-3 py-1.5 text-left text-xs transition-colors",
                on ? "bg-accent/10 text-accent" : "text-fg-2 hover:bg-surface-3",
                !on &&
                  props.selected.length >= (props.max ?? MAX_COMPARE) &&
                  "cursor-not-allowed opacity-40",
              )}
            >
              <span className="truncate">{s.name}</span>
              <span className="ml-2 shrink-0 font-mono text-fg-3">{s.code}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

// 多标的 NAV 并行拉取
function useMultiNav(codes: string[]) {
  return useQueries({
    queries: codes.map((code) => ({
      queryKey: ["nav", code, 2000],
      queryFn: ({ signal }: { signal?: AbortSignal }) =>
        api<{ date: string; unit_nav: number }[]>(
          `/api/funds/${encodeURIComponent(code)}/nav?limit=2000`,
          { signal },
        ),
      staleTime: 5 * 60 * 1000,
    })),
  });
}

// ── compare ─────────────────────────────────────────────────────────

function CompareTab() {
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

// ── backtest（定投 vs 一次性——旧 DcaBacktestChart 语义）────────────────

function BacktestTab() {
  const portfolioId = useUi((s) => s.portfolioId);
  const securities = useSecurities(portfolioId);
  const [code, setCode] = useState("");
  const [amount, setAmount] = useState("1000");
  const nav = useNavHistory(code);
  const baseAmount = Number(amount) || 1000;

  const result = useMemo(() => {
    const points = [...(nav.data ?? [])].sort((a, b) => a.date.localeCompare(b.date));
    if (points.length < 2) return null;
    const firstNav = points[0]?.unit_nav ?? 1;

    // 定投：每月首个数据点买入 baseAmount
    let dcaShares = 0;
    let dcaInvested = 0;
    let lastMonth = "";
    const cashflows: number[] = [];
    const cfDates: Date[] = [];
    const dcaValues: [string, number][] = [];
    for (const p of points) {
      const month = p.date.substring(0, 7);
      if (month !== lastMonth) {
        dcaShares += baseAmount / p.unit_nav;
        dcaInvested += baseAmount;
        cashflows.push(-baseAmount);
        cfDates.push(new Date(p.date));
        lastMonth = month;
      }
      dcaValues.push([p.date, dcaShares * p.unit_nav]);
    }
    const lumpShares = dcaInvested / firstNav;
    const lumpValues: [string, number][] = points.map((p) => [p.date, lumpShares * p.unit_nav]);

    const lastPoint = points[points.length - 1];
    if (!lastPoint) return null;
    const lastDate = lastPoint.date;
    cashflows.push(dcaValues[dcaValues.length - 1]?.[1] ?? 0);
    cfDates.push(new Date(lastDate));
    const dcaIrrRaw = calcIRR(cashflows, cfDates);

    return {
      dcaValues,
      lumpValues,
      dcaInvested,
      dcaFinal: dcaValues[dcaValues.length - 1]?.[1] ?? 0,
      lumpFinal: lumpValues[lumpValues.length - 1]?.[1] ?? 0,
      dcaIrr: dcaIrrRaw == null ? null : dcaIrrRaw * 100,
    };
  }, [nav.data, baseAmount]);

  return (
    <div className="space-y-4">
      <Card>
        <CardContent className="flex flex-wrap items-end gap-3 py-4">
          <div className="w-64">
            <Label htmlFor="bt-code">标的</Label>
            <Input
              id="bt-code"
              list="bt-codes"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="输入代码，如 019173"
            />
            <datalist id="bt-codes">
              {(securities.data ?? []).map((s) => (
                <option key={s.code} value={s.code}>
                  {s.name}
                </option>
              ))}
            </datalist>
          </div>
          <div className="w-40">
            <Label htmlFor="bt-amount">每月定投金额</Label>
            <Input
              id="bt-amount"
              inputMode="decimal"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
            />
          </div>
        </CardContent>
      </Card>

      {!code ? (
        <EmptyState
          title="输入标的代码开始回测"
          description="每月定投 vs 首日一次性投入的对照模拟。"
        />
      ) : nav.isPending ? (
        <Skeleton className="h-72" />
      ) : !result ? (
        <EmptyState title="净值数据不足" description="该标的至少需要 2 个净值点。" />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            {[
              { label: "定投投入", value: fmtCNY(result.dcaInvested) },
              {
                label: "定投终值",
                value: fmtCNY(result.dcaFinal),
                tone: pnlTone(result.dcaFinal - result.dcaInvested),
                sub: fmtSignedPct((result.dcaFinal / result.dcaInvested - 1) * 100),
              },
              {
                label: "一次性终值",
                value: fmtCNY(result.lumpFinal),
                tone: pnlTone(result.lumpFinal - result.dcaInvested),
                sub: fmtSignedPct((result.lumpFinal / result.dcaInvested - 1) * 100),
              },
              {
                label: "定投 XIRR",
                value: result.dcaIrr != null ? fmtSignedPct(result.dcaIrr) : "—",
                tone: pnlTone(result.dcaIrr),
              },
            ].map((m) => (
              <Card key={m.label}>
                <div className="text-[11px] text-fg-3">{m.label}</div>
                <div
                  className={cn(
                    "mt-1 text-xl font-medium tabular-nums",
                    m.tone ? toneClass[m.tone] : "text-fg",
                  )}
                >
                  {m.value}
                </div>
                {m.sub && <div className="text-[11px] tabular-nums text-fg-3">{m.sub}</div>}
              </Card>
            ))}
          </div>
          <Card>
            <CardHeader>
              <CardTitle>资产曲线对照</CardTitle>
            </CardHeader>
            <CardContent>
              <Chart
                height={300}
                deps={[result]}
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
                  series: [
                    {
                      name: "每月定投",
                      type: "line",
                      showSymbol: false,
                      data: result.dcaValues,
                      lineStyle: { width: 2, color: t.accent },
                      itemStyle: { color: t.accent },
                    },
                    {
                      name: "一次性投入",
                      type: "line",
                      showSymbol: false,
                      data: result.lumpValues,
                      lineStyle: { width: 1.5, type: "dashed", color: t.fg3 },
                      itemStyle: { color: t.fg3 },
                    },
                  ],
                })}
              />
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}

// ── advanced（相关性热力图 + 蒙特卡洛扇形）────────────────────────────

function AdvancedTab() {
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
        <Card>
          <CardHeader>
            <CardTitle>相关性热力图（日收益 pearson）</CardTitle>
          </CardHeader>
          <CardContent>
            {codes.length < 2 ? (
              <EmptyState title="至少选择 2 个标的" />
            ) : (
              <Chart
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
            {!mc ? (
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

// ── penetration（底层股票暴露）────────────────────────────────────────

function PenetrationTab() {
  const portfolioId = useUi((s) => s.portfolioId);
  const penetration = usePenetration(portfolioId);
  const rows = penetration.data?.penetration ?? [];

  return (
    <div className="space-y-4">
      {penetration.isPending ? (
        <Skeleton className="h-80" />
      ) : rows.length === 0 ? (
        <EmptyState
          title="暂无穿透数据"
          description="持仓基金披露季报持仓后，这里展示底层股票暴露。可到工作台触发持仓抓取。"
        />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <Card>
              <div className="text-[11px] text-fg-3">穿透总市值</div>
              <div className="mt-1 text-xl font-medium tabular-nums text-fg">
                {fmtCNY(penetration.data?.total_portfolio_value ?? 0)}
              </div>
            </Card>
            <Card>
              <div className="text-[11px] text-fg-3">权益基金数</div>
              <div className="mt-1 text-xl font-medium tabular-nums text-fg">
                {penetration.data?.equity_fund_count ?? 0}
              </div>
            </Card>
            <Card>
              <div className="text-[11px] text-fg-3">底层股票数</div>
              <div className="mt-1 text-xl font-medium tabular-nums text-fg">
                {penetration.data?.unique_stocks ?? 0}
              </div>
            </Card>
            <Card>
              <div className="text-[11px] text-fg-3">最大单项暴露</div>
              <div className="mt-1 truncate text-xl font-medium tabular-nums text-fg">
                {rows.length > 0
                  ? `${rows[0]?.stock_name ?? "—"} ${fmtPct(rows[0]?.weight_pct ?? 0, 1)}`
                  : "—"}
              </div>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle>底层股票暴露</CardTitle>
            </CardHeader>
            <CardContent>
              <Chart
                height={360}
                deps={[rows]}
                option={(t) => ({
                  ...baseChartOption(t),
                  series: [
                    {
                      type: "treemap",
                      roam: false,
                      breadcrumb: { show: false },
                      label: { color: t.fg, fontSize: 11, overflow: "truncate" },
                      itemStyle: { borderColor: t.surface1, borderWidth: 2, gapWidth: 2 },
                      data: rows.slice(0, 30).map((r, i) => ({
                        name: r.stock_name,
                        value: r.total_exposure_cny,
                        itemStyle: { color: t.palette[i % t.palette.length] },
                      })),
                    },
                  ],
                })}
              />
            </CardContent>
          </Card>

          <Card className="overflow-x-auto" style={{ padding: 0 }}>
            <Table>
              <THead>
                <Tr>
                  <Th>股票</Th>
                  <Th className="text-right">暴露金额</Th>
                  <Th className="text-right">占组合</Th>
                  <Th>持有基金</Th>
                </Tr>
              </THead>
              <TBody>
                {rows.map((r) => (
                  <Tr key={r.stock_code}>
                    <Td>
                      <span className="text-fg">{r.stock_name}</span>
                      <span className="ml-1.5 font-mono text-[11px] text-fg-3">{r.stock_code}</span>
                    </Td>
                    <Td className="text-right tabular-nums">{fmtCNY(r.total_exposure_cny)}</Td>
                    <Td className="text-right tabular-nums text-fg-2">{fmtPct(r.weight_pct, 2)}</Td>
                    <Td className="max-w-64">
                      <div className="flex flex-wrap gap-1">
                        {r.held_by_funds.slice(0, 3).map((f) => (
                          <Badge key={f.fund_code} className="max-w-40 truncate">
                            {f.fund_name}
                          </Badge>
                        ))}
                        {r.held_by_funds.length > 3 && <Badge>+{r.held_by_funds.length - 3}</Badge>}
                      </div>
                    </Td>
                  </Tr>
                ))}
              </TBody>
            </Table>
          </Card>
        </>
      )}
    </div>
  );
}

// ── 页面 ────────────────────────────────────────────────────────────

const TABS = [
  { value: "compare", label: "对比" },
  { value: "backtest", label: "回测" },
  { value: "advanced", label: "高级" },
  { value: "penetration", label: "穿透" },
] as const;

export function AnalysisPage() {
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as { tab?: string };
  const tab = TABS.some((t) => t.value === search.tab) ? (search.tab as string) : "compare";

  return (
    <Tabs
      value={tab}
      onValueChange={(v) => {
        void navigate({ to: ".", search: { tab: v }, replace: true });
      }}
    >
      <TabsList>
        {TABS.map((t) => (
          <TabsTrigger key={t.value} value={t.value}>
            {t.label}
          </TabsTrigger>
        ))}
      </TabsList>
      <TabsContent value="compare" className="pt-4">
        <CompareTab />
      </TabsContent>
      <TabsContent value="backtest" className="pt-4">
        <BacktestTab />
      </TabsContent>
      <TabsContent value="advanced" className="pt-4">
        <AdvancedTab />
      </TabsContent>
      <TabsContent value="penetration" className="pt-4">
        <PenetrationTab />
      </TabsContent>
    </Tabs>
  );
}
