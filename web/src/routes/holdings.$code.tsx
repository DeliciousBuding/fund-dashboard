// 标的详情 /holdings/$code —— 四 tab：走势（NAV+成本线+买卖点）/ 定投模拟 / 概览指标 / 交易流水。
// 买卖点随涨跌色约定反转（03 §6：买=▲ --up、卖=▼ --down）。

import { Link, useParams } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { useMemo, useState } from "react";
import { Chart } from "../components/charts/Chart";
import { baseChartOption } from "../components/charts/theme";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { EmptyState } from "../components/ui/empty-state";
import { Input } from "../components/ui/input";
import { Segmented } from "../components/ui/segmented";
import { Skeleton } from "../components/ui/skeleton";
import { Table, TBody, Td, THead, Th, Tr } from "../components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs";
import { fmtCNY, fmtPct, fmtSignedCNY, fmtSignedPct, pnlTone } from "../lib/format";
import {
  useDcaCompute,
  useDrawdown,
  useFundDetail,
  useFundXirr,
  useNavHistory,
} from "../lib/queries";
import { toneClass } from "../lib/tones";
import { cn } from "../lib/utils";
import { useUi } from "../stores/ui";

// ── 走势 tab ────────────────────────────────────────────────────────

function TrendTab({ code }: { code: string }) {
  const nav = useNavHistory(code);
  const detail = useFundDetail(code);

  const avgCost = useMemo(() => {
    const d = detail.data;
    if (!d || d.held_shares <= 0 || d.total_cost === 0) return null;
    return Math.abs(d.total_cost) / d.held_shares;
  }, [detail.data]);

  const markers = useMemo(() => {
    const txs = detail.data?.transactions ?? [];
    const navByDate = new Map((nav.data ?? []).map((p) => [p.date, p.unit_nav]));
    const points: { date: string; nav: number; direction: string; amount: number | null }[] = [];
    for (const tx of txs) {
      const date = (tx.confirm_date ?? tx.trade_time ?? "").slice(0, 10);
      const navValue = navByDate.get(date);
      if (!date || navValue == null) continue;
      const dir = tx.direction;
      if (dir !== "buy" && dir !== "sell") continue;
      points.push({ date, nav: navValue, direction: dir, amount: tx.amount ?? null });
    }
    return points;
  }, [detail.data, nav.data]);

  return (
    <Chart
      height={360}
      loading={nav.isPending || detail.isPending}
      empty={!nav.isPending && (nav.data ?? []).length === 0}
      emptyText="暂无净值历史"
      error={nav.isError || detail.isError}
      errorText="净值历史加载失败"
      onRetry={() => {
        void nav.refetch();
        void detail.refetch();
      }}
      deps={[nav.data, markers, avgCost]}
      option={(t) => ({
        ...baseChartOption(t),
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
            name: "净值",
            type: "line",
            showSymbol: false,
            data: (nav.data ?? []).map((p) => [p.date, p.unit_nav]),
            lineStyle: { width: 2, color: t.accent },
            itemStyle: { color: t.accent },
            areaStyle: { color: t.accent, opacity: 0.08 },
            markLine:
              avgCost != null
                ? {
                    silent: true,
                    symbol: "none",
                    label: {
                      formatter: `成本 ${avgCost.toFixed(4)}`,
                      color: t.fg3,
                      fontSize: 11,
                    },
                    lineStyle: { type: "dashed", color: t.fg3 },
                    data: [{ yAxis: avgCost }],
                  }
                : undefined,
          },
          {
            name: "买入",
            type: "effectScatter",
            symbol: "triangle",
            symbolSize: 9,
            rippleEffect: { scale: 1.6 },
            itemStyle: { color: t.up },
            data: markers.filter((m) => m.direction === "buy").map((m) => [m.date, m.nav]),
            z: 10,
          },
          {
            name: "卖出",
            type: "effectScatter",
            symbol: "triangle",
            symbolRotate: 180,
            symbolSize: 9,
            itemStyle: { color: t.down },
            data: markers.filter((m) => m.direction === "sell").map((m) => [m.date, m.nav]),
            z: 10,
          },
        ],
      })}
    />
  );
}

// ── 定投 tab（智能金额模拟器）────────────────────────────────────────

const DCA_MODES = [
  { value: "nav_deviation", label: "净值偏离" },
  { value: "change_pct", label: "当日涨跌" },
] as const;

function DcaTab({ code }: { code: string }) {
  const [base, setBase] = useState("100");
  const [mode, setMode] = useState<string>("nav_deviation");
  const baseAmount = Number(base) || 0;
  const sim = useDcaCompute(code, baseAmount, mode);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>智能金额模拟器</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap items-end gap-3">
          <div className="w-40">
            <div className="mb-1.5 text-xs text-fg-3">基础金额</div>
            <Input
              inputMode="decimal"
              value={base}
              onChange={(e) => setBase(e.target.value)}
              aria-label="基础金额"
            />
          </div>
          <Segmented id="dca-mode" value={mode} onChange={setMode} options={[...DCA_MODES]} />
        </CardContent>
      </Card>

      {sim.isPending && baseAmount > 0 ? (
        <Skeleton className="h-24" />
      ) : sim.isError ? (
        <EmptyState
          title="模拟计算失败"
          description="无法计算建议金额，请重试。"
          action={
            <Button size="sm" onClick={() => void sim.refetch()}>
              重试
            </Button>
          }
        />
      ) : sim.data &&
        !sim.data.error &&
        sim.data.actual_amount !== undefined &&
        sim.data.dca_rate !== undefined ? (
        <Card>
          <CardContent className="flex flex-wrap items-center gap-6 py-5">
            <div>
              <div className="text-xs text-fg-3">建议金额</div>
              <div className="mt-1 text-3xl font-medium tabular-nums text-accent">
                {fmtCNY(sim.data.actual_amount)}
              </div>
            </div>
            <div>
              <div className="text-xs text-fg-3">倍数</div>
              <div className="mt-1 text-xl tabular-nums text-fg">×{sim.data.dca_rate}</div>
            </div>
            {sim.data.signal && <Badge tone="accent">{sim.data.signal}</Badge>}
            {sim.data.explanation && (
              <p className="w-full text-xs text-fg-2">{sim.data.explanation}</p>
            )}
          </CardContent>
        </Card>
      ) : sim.data?.error ? (
        <EmptyState title="无法计算建议金额" description={sim.data.error} />
      ) : (
        <EmptyState
          title="输入基础金额开始模拟"
          description="按净值偏离或当日涨跌自动放大/缩小扣款。"
        />
      )}
    </div>
  );
}

// ── 概览 tab ────────────────────────────────────────────────────────

function OverviewTab({ code }: { code: string }) {
  const detail = useFundDetail(code);
  const xirr = useFundXirr(code);
  const drawdown = useDrawdown(code);

  if (detail.isPending) return <Skeleton className="h-40" />;
  if (detail.isError) {
    return (
      <EmptyState
        title="标的信息加载失败"
        description="无法读取标的详情，请重试。"
        action={
          <Button size="sm" onClick={() => void detail.refetch()}>
            重试
          </Button>
        }
      />
    );
  }
  const d = detail.data;
  if (!d) return <EmptyState title="标的不存在或已被删除" />;

  const metrics: { label: string; value: string; tone?: keyof typeof toneClass; sub?: string }[] = [
    {
      label: "持有份额",
      value: d.held_shares.toLocaleString("zh-CN", { maximumFractionDigits: 2 }),
    },
    { label: "最新净值", value: d.latest_nav != null ? String(d.latest_nav) : "—" },
    { label: "当前市值", value: fmtCNY(d.current_value) },
    {
      label: "未实现盈亏",
      value: fmtSignedCNY(d.unrealized_pnl),
      tone: pnlTone(d.unrealized_pnl),
      sub: fmtSignedPct(d.pnl_pct),
    },
    {
      label: "XIRR",
      value: xirr.data?.xirr != null ? fmtSignedPct(xirr.data.xirr) : "—",
      tone: pnlTone(xirr.data?.xirr),
    },
    {
      label: "最大回撤",
      value: drawdown.data ? fmtPct(-Math.abs(drawdown.data.max_drawdown)) : "—",
      sub: drawdown.data ? `${drawdown.data.peak_date} → ${drawdown.data.trough_date}` : undefined,
    },
    { label: "买入次数", value: String(d.buy_count) },
    { label: "卖出次数", value: String(d.sell_count) },
    {
      label: "定投 / 手动",
      value: `${d.auto_buy_count} / ${d.manual_buy_count}`,
      sub: `中位结算 ${d.median_settlement} 天`,
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
      {metrics.map((m) => (
        <Card key={m.label}>
          <div className="text-[11px] text-fg-3">{m.label}</div>
          <div
            className={cn(
              "mt-1 text-lg font-medium tabular-nums",
              m.tone ? toneClass[m.tone] : "text-fg",
            )}
          >
            {m.value}
          </div>
          {m.sub && <div className="mt-0.5 text-[11px] text-fg-3 tabular-nums">{m.sub}</div>}
        </Card>
      ))}
    </div>
  );
}

// ── 交易 tab（只读流水 + 搜索 + 定投/手动过滤）─────────────────────────

function TransactionsTab({ code }: { code: string }) {
  const detail = useFundDetail(code);
  const [query, setQuery] = useState("");
  const [source, setSource] = useState("all");

  const rows = useMemo(() => {
    let txs = detail.data?.transactions ?? [];
    if (source === "auto")
      txs = txs.filter((t) => t.trade_type?.includes("定投") || t.trade_type?.includes("auto"));
    if (source === "manual")
      txs = txs.filter((t) => !(t.trade_type?.includes("定投") || t.trade_type?.includes("auto")));
    if (query) {
      const q = query.toLowerCase();
      txs = txs.filter(
        (t) => t.trade_type?.toLowerCase().includes(q) || t.order_id?.toLowerCase().includes(q),
      );
    }
    return txs;
  }, [detail.data, query, source]);

  if (detail.isPending) return <Skeleton className="h-64" />;
  if (detail.isError) {
    return (
      <EmptyState
        title="交易流水加载失败"
        description="无法读取交易明细，请重试。"
        action={
          <Button size="sm" onClick={() => void detail.refetch()}>
            重试
          </Button>
        }
      />
    );
  }

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        <div className="w-56">
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索类型 / 单号…"
            aria-label="搜索交易"
          />
        </div>
        <Segmented
          id="tx-source"
          value={source}
          onChange={setSource}
          options={[
            { value: "all", label: "全部" },
            { value: "auto", label: "定投" },
            { value: "manual", label: "手动" },
          ]}
        />
      </div>
      {rows.length === 0 ? (
        <EmptyState title="没有匹配的交易" description="调整搜索或过滤条件。" />
      ) : (
        <Card className="overflow-x-auto" style={{ padding: 0 }}>
          <Table>
            <THead>
              <Tr>
                <Th>日期</Th>
                <Th>类型</Th>
                <Th className="text-right">金额</Th>
                <Th className="text-right">份额</Th>
                <Th className="text-right">手续费</Th>
              </Tr>
            </THead>
            <TBody>
              {rows.map((tx) => (
                <Tr key={tx.seq ?? tx.order_id ?? `${tx.trade_time}-${tx.trade_type}`}>
                  <Td className="tabular-nums text-fg-2">
                    {(tx.confirm_date ?? tx.trade_time ?? "").slice(0, 10)}
                  </Td>
                  <Td>
                    <Badge
                      tone={
                        tx.direction === "buy" ? "up" : tx.direction === "sell" ? "down" : "neutral"
                      }
                    >
                      {tx.trade_type}
                    </Badge>
                  </Td>
                  <Td className="text-right tabular-nums">{fmtCNY(tx.amount)}</Td>
                  <Td className="text-right tabular-nums text-fg-2">{tx.shares}</Td>
                  <Td className="text-right tabular-nums text-fg-3">{tx.fee}</Td>
                </Tr>
              ))}
            </TBody>
          </Table>
        </Card>
      )}
    </div>
  );
}

// ── 页面 ────────────────────────────────────────────────────────────

export function HoldingDetailPage() {
  const { code } = useParams({ from: "/protected/holdings/$code" });
  const portfolioId = useUi((s) => s.portfolioId);
  const detail = useFundDetail(code, portfolioId);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Link
          to="/holdings"
          aria-label="返回持仓"
          className="inline-flex size-9 items-center justify-center rounded-lg text-fg-2 transition-colors hover:bg-surface-3 hover:text-fg"
        >
          <ArrowLeft className="size-4" />
        </Link>
        <div className="min-w-0">
          <h1 className="truncate text-lg font-medium text-fg">{detail.data?.name ?? code}</h1>
          <div className="font-mono text-xs text-fg-3">{code}</div>
        </div>
        {detail.data?.pnl_pct != null && (
          <Badge
            tone={
              pnlTone(detail.data.pnl_pct) === "up"
                ? "up"
                : pnlTone(detail.data.pnl_pct) === "down"
                  ? "down"
                  : "neutral"
            }
            className="ml-auto"
          >
            {fmtSignedPct(detail.data.pnl_pct)}
          </Badge>
        )}
      </div>

      <Tabs defaultValue="trend">
        <TabsList>
          <TabsTrigger value="trend">走势</TabsTrigger>
          <TabsTrigger value="dca">定投</TabsTrigger>
          <TabsTrigger value="overview">概览</TabsTrigger>
          <TabsTrigger value="transactions">交易</TabsTrigger>
        </TabsList>
        <TabsContent value="trend" className="pt-4">
          <TrendTab code={code} />
        </TabsContent>
        <TabsContent value="dca" className="pt-4">
          <DcaTab code={code} />
        </TabsContent>
        <TabsContent value="overview" className="pt-4">
          <OverviewTab code={code} />
        </TabsContent>
        <TabsContent value="transactions" className="pt-4">
          <TransactionsTab code={code} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
