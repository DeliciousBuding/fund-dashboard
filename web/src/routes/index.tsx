// 总览 / —— 01 文档 §4 规格：hero KPI + 净值三线（区间切换）+ 配置 sunburst 三视角
// + 涨跌贡献榜 + 盈亏分布直方图。四态纪律（03 §7）：骨架同形同位，空态有主行动。

import { Link } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import { Chart } from "../components/charts/Chart";
import { baseChartOption } from "../components/charts/theme";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { EmptyState } from "../components/ui/empty-state";
import { Segmented } from "../components/ui/segmented";
import { Skeleton } from "../components/ui/skeleton";
import { fmtCNY, fmtPct, fmtSignedCNY, fmtSignedPct, pnlTone } from "../lib/format";
import {
  useAllocation,
  usePortfolio,
  usePortfolioXirr,
  useSecurities,
  useTimeline,
} from "../lib/queries";
import { cn } from "../lib/utils";
import { useUi } from "../stores/ui";

const toneClass = { up: "text-up", down: "text-down", flat: "text-fg-3" } as const;

// ── KPI 区 ──────────────────────────────────────────────────────────

function KpiCard(props: {
  label: string;
  value: string;
  sub?: string;
  hero?: boolean;
  tone?: keyof typeof toneClass;
}) {
  return (
    <Card className={props.hero ? "sm:col-span-2" : undefined}>
      <CardHeader className="pb-1">
        <CardTitle className="text-xs font-normal text-fg-3">{props.label}</CardTitle>
      </CardHeader>
      <CardContent>
        <div
          className={cn(
            "tabular-nums font-medium",
            props.hero ? "text-4xl" : "text-2xl",
            props.tone ? toneClass[props.tone] : "text-fg",
          )}
        >
          {props.value}
        </div>
        {props.sub ? <div className="mt-1 text-xs text-fg-2 tabular-nums">{props.sub}</div> : null}
      </CardContent>
    </Card>
  );
}

// ── 净值曲线（value/cost/pnl 三线 + 区间切换）────────────────────────

const RANGES = [
  { value: "1M", label: "1月", days: 31 },
  { value: "3M", label: "3月", days: 92 },
  { value: "6M", label: "6月", days: 183 },
  { value: "1Y", label: "1年", days: 366 },
  { value: "ALL", label: "全部", days: 0 },
] as const;

function TimelineChart() {
  const portfolioId = useUi((s) => s.portfolioId);
  const timeline = useTimeline(portfolioId);
  const [range, setRange] = useState<string>("ALL");

  const points = useMemo(() => {
    const all = timeline.data ?? [];
    const r = RANGES.find((x) => x.value === range) ?? RANGES[4];
    if (r.days === 0) return all;
    const cutoff = new Date(Date.now() - r.days * 86_400_000).toISOString().slice(0, 10);
    return all.filter((p) => p.date >= cutoff);
  }, [timeline.data, range]);

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle>净值走势</CardTitle>
        <Segmented
          id="tl-range"
          value={range}
          onChange={setRange}
          options={RANGES.map((r) => ({ value: r.value, label: r.label }))}
        />
      </CardHeader>
      <CardContent>
        <Chart
          height={300}
          loading={timeline.isPending}
          empty={!timeline.isPending && points.length === 0}
          emptyText="还没有净值历史——先同步行情"
          deps={[points]}
          option={(t) => ({
            ...baseChartOption(t),
            legend: { top: 0, right: 0, textStyle: { color: t.fg3, fontSize: 11 }, itemWidth: 14 },
            xAxis: {
              type: "time",
              axisLine: { lineStyle: { color: t.border } },
              axisLabel: { color: t.fg3, fontSize: 11 },
            },
            yAxis: {
              type: "value",
              scale: true,
              splitLine: { lineStyle: { color: t.border, type: "dashed" } },
              axisLabel: { color: t.fg3, fontSize: 11 },
            },
            series: [
              {
                name: "市值",
                type: "line",
                showSymbol: false,
                data: points.map((p) => [p.date, p.total_value]),
                lineStyle: { width: 2, color: t.accent },
                itemStyle: { color: t.accent },
                areaStyle: { color: t.accent, opacity: 0.08 },
              },
              {
                name: "成本",
                type: "line",
                showSymbol: false,
                data: points.map((p) => [p.date, p.total_cost]),
                lineStyle: { width: 1, type: "dashed", color: t.fg3 },
                itemStyle: { color: t.fg3 },
              },
              {
                name: "盈亏",
                type: "line",
                showSymbol: false,
                data: points.map((p) => [p.date, p.pnl]),
                lineStyle: { width: 1.5, color: t.info },
                itemStyle: { color: t.info },
              },
            ],
          })}
        />
      </CardContent>
    </Card>
  );
}

// ── 配置 sunburst（类型/市场/基金类型三视角）──────────────────────────

type AllocView = "by_security_type" | "by_market" | "by_fund_type";
const ALLOC_VIEWS: { value: AllocView; label: string }[] = [
  { value: "by_security_type", label: "证券类型" },
  { value: "by_market", label: "市场" },
  { value: "by_fund_type", label: "主题" },
];

function AllocationSunburst() {
  const portfolioId = useUi((s) => s.portfolioId);
  const allocation = useAllocation(portfolioId);
  const [view, setView] = useState<AllocView>("by_market");

  const buckets = allocation.data?.[view] ?? [];

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle>配置结构</CardTitle>
        <Segmented
          id="alloc-view"
          value={view}
          onChange={(v) => setView(v as AllocView)}
          options={ALLOC_VIEWS}
        />
      </CardHeader>
      <CardContent>
        <Chart
          height={280}
          loading={allocation.isPending}
          empty={!allocation.isPending && buckets.length === 0}
          emptyText="暂无配置数据"
          deps={[buckets]}
          option={(t) => ({
            ...baseChartOption(t),
            series: [
              {
                type: "sunburst",
                radius: ["18%", "88%"],
                label: { color: t.fg2, fontSize: 11, overflow: "truncate", minAngle: 8 },
                itemStyle: { borderColor: t.surface1, borderWidth: 2 },
                emphasis: { itemStyle: { opacity: 0.92 } },
                data: buckets.map((b, i) => ({
                  name: b.label || b.key,
                  value: Math.max(b.value, 0),
                  itemStyle: { color: t.palette[i % t.palette.length] },
                })),
              },
            ],
          })}
        />
        {buckets.length > 0 && (
          <ul className="mt-2 space-y-1">
            {buckets.slice(0, 5).map((b) => (
              <li key={b.key} className="flex items-center justify-between text-xs">
                <span className="truncate text-fg-2">{b.label || b.key}</span>
                <span className="tabular-nums text-fg-3">{fmtPct(b.weight_pct, 1)}</span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

// ── 涨跌贡献榜（summary top + securities 排序榜）───────────────────────

function ContributorsCard() {
  const portfolioId = useUi((s) => s.portfolioId);
  const portfolio = usePortfolio(portfolioId);
  const securities = useSecurities(portfolioId);

  const held = useMemo(
    () =>
      (securities.data ?? [])
        .filter((s) => s.held_shares > 0 && s.pnl_pct != null)
        .sort((a, b) => (b.pnl_pct ?? 0) - (a.pnl_pct ?? 0)),
    [securities.data],
  );
  const gainers = held.slice(0, 3);
  const losers = [...held].reverse().slice(0, 3);

  const loading = portfolio.isPending || securities.isPending;
  if (loading) return <Skeleton className="h-72 w-full" />;

  const top = portfolio.data?.top_gainer;
  const bottom = portfolio.data?.top_loser;

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between space-y-0">
        <CardTitle>涨跌贡献</CardTitle>
        <div className="flex gap-1.5">
          {top && (
            <Badge tone="up" className="max-w-40 truncate">
              ▲ {top.name}
            </Badge>
          )}
          {bottom && (
            <Badge tone="down" className="max-w-40 truncate">
              ▼ {bottom.name}
            </Badge>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {held.length === 0 ? (
          <EmptyState title="暂无持仓" description="有持仓后这里展示各标的涨跌贡献榜。" />
        ) : (
          <div className="grid grid-cols-2 gap-4">
            {[
              { title: "贡献最多", rows: gainers },
              { title: "拖累最多", rows: losers },
            ].map(({ title, rows }) => (
              <div key={title}>
                <div className="pb-2 text-xs text-fg-3">{title}</div>
                <ul className="space-y-1.5">
                  {rows.map((s) => (
                    <li key={s.code}>
                      <Link
                        to="/holdings/$code"
                        params={{ code: s.code }}
                        className="group flex items-center justify-between gap-2 rounded-md px-1.5 py-1 hover:bg-surface-3"
                      >
                        <span className="min-w-0 truncate text-xs text-fg-2 group-hover:text-fg">
                          {s.name}
                        </span>
                        <span
                          className={cn(
                            "shrink-0 text-xs tabular-nums",
                            toneClass[pnlTone(s.pnl_pct)],
                          )}
                        >
                          {fmtSignedPct(s.pnl_pct, 1)}
                        </span>
                      </Link>
                    </li>
                  ))}
                  {rows.length === 0 && <li className="text-xs text-fg-3">—</li>}
                </ul>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ── 盈亏分布直方图（半开区间分桶 [a,b) —— 旧 pnl_dist 语义随迁）────────

const BUCKET_EDGES = [-Infinity, -20, -10, -5, 0, 5, 10, 20, Infinity];
const BUCKET_LABELS = ["<-20%", "-20~-10", "-10~-5", "-5~0", "0~5", "5~10", "10~20", ">20%"];

function bucketIndex(pct: number): number {
  for (let i = 0; i < BUCKET_EDGES.length - 1; i++) {
    const lo = BUCKET_EDGES[i] as number;
    const hi = BUCKET_EDGES[i + 1] as number;
    if (pct >= lo && pct < hi) return i;
  }
  return BUCKET_EDGES.length - 2;
}

function PnlDistribution() {
  const portfolioId = useUi((s) => s.portfolioId);
  const securities = useSecurities(portfolioId);

  const counts = useMemo(() => {
    const c = new Array(BUCKET_LABELS.length).fill(0) as number[];
    for (const s of securities.data ?? []) {
      if (s.held_shares > 0 && s.pnl_pct != null) c[bucketIndex(s.pnl_pct)]++;
    }
    return c;
  }, [securities.data]);

  const empty = !securities.isPending && counts.every((c) => c === 0);

  return (
    <Card>
      <CardHeader>
        <CardTitle>盈亏分布</CardTitle>
      </CardHeader>
      <CardContent>
        <Chart
          height={220}
          loading={securities.isPending}
          empty={empty}
          emptyText="暂无持仓盈亏数据"
          deps={[counts]}
          option={(t) => ({
            ...baseChartOption(t),
            xAxis: {
              type: "category",
              data: BUCKET_LABELS,
              axisLabel: { color: t.fg3, fontSize: 10, interval: 0 },
              axisLine: { lineStyle: { color: t.border } },
            },
            yAxis: {
              type: "value",
              minInterval: 1,
              splitLine: { lineStyle: { color: t.border, type: "dashed" } },
              axisLabel: { color: t.fg3, fontSize: 11 },
            },
            series: [
              {
                type: "bar",
                data: counts.map((c, i) => ({
                  value: c,
                  itemStyle: {
                    color: i < 4 ? t.down : i > 4 ? t.up : t.fg3,
                    opacity: c === 0 ? 0.25 : 0.9,
                  },
                })),
                barMaxWidth: 28,
              },
            ],
          })}
        />
      </CardContent>
    </Card>
  );
}

// ── 页面 ────────────────────────────────────────────────────────────

export function OverviewPage() {
  const portfolioId = useUi((s) => s.portfolioId);
  const summary = usePortfolio(portfolioId);
  const xirr = usePortfolioXirr(portfolioId);

  if (summary.isError) {
    return (
      <EmptyState
        title="组合数据加载失败"
        description="检查服务状态后重试；若持续失败请到工作台查看系统状态。"
        action={
          <Button size="sm" onClick={() => summary.refetch()}>
            重试
          </Button>
        }
      />
    );
  }

  const s = summary.data;
  const xirrValue = xirr.data?.xirr;

  return (
    <div className="space-y-4">
      {/* Hero KPI */}
      {summary.isPending ? (
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {["k1", "k2", "k3", "k4"].map((k) => (
            <Skeleton key={k} className="h-24" />
          ))}
        </div>
      ) : s ? (
        <>
          <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
            <KpiCard
              label="当前市值"
              value={fmtCNY(s.current_value)}
              hero
              sub={s.last_nav_date ? `净值截至 ${s.last_nav_date}` : undefined}
            />
            <KpiCard
              label="未实现盈亏"
              value={fmtSignedCNY(s.unrealized_pnl)}
              sub={fmtSignedPct(s.pnl_pct)}
              tone={pnlTone(s.unrealized_pnl)}
            />
            <KpiCard
              label="组合 XIRR"
              value={xirrValue != null ? fmtSignedPct(xirrValue) : "—"}
              sub={xirr.data?.message ?? "年化收益率"}
              tone={pnlTone(xirrValue)}
            />
            <KpiCard
              label="持有标的"
              value={String(s.held_funds)}
              sub={`基金 ${s.unique_funds} · 股票 ${s.unique_stocks}`}
            />
          </div>
          {/* 次级 4up */}
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            {[
              { label: "累计买入", value: fmtCNY(s.total_buy) },
              { label: "累计卖出", value: fmtCNY(s.total_sell) },
              { label: "定投投入", value: fmtCNY(s.auto_amount), sub: `${s.auto_tx} 笔` },
              { label: "手动投入", value: fmtCNY(s.manual_amount), sub: `${s.manual_tx} 笔` },
            ].map((item) => (
              <div
                key={item.label}
                className="rounded-xl border border-border bg-surface-1 px-4 py-3"
              >
                <div className="text-[11px] text-fg-3">{item.label}</div>
                <div className="mt-0.5 text-sm font-medium tabular-nums text-fg">
                  {item.value}
                  {item.sub && (
                    <span className="ml-1.5 text-xs font-normal text-fg-3">{item.sub}</span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </>
      ) : null}

      <TimelineChart />

      <div className="grid gap-4 lg:grid-cols-2">
        <AllocationSunburst />
        <ContributorsCard />
      </div>

      <PnlDistribution />
    </div>
  );
}
