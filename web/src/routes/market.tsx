// 市场 /market —— 指数看板（开盘状态着色）+ 纳斯达克透视（NDX 历史线 +
// 全体交易散点叠加 + 统计卡）。SSE ticker 在顶栏，本页用轮询快照。

import { useMemo, useState } from "react";
import { Chart } from "../components/charts/Chart";
import { baseChartOption } from "../components/charts/theme";
import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { EmptyState } from "../components/ui/empty-state";
import { Segmented } from "../components/ui/segmented";
import { Skeleton } from "../components/ui/skeleton";
import { fmtCNY, fmtSignedPct, pnlTone } from "../lib/format";
import { useIndexHistory, useIndices, useTransactions } from "../lib/queries";
import { toneClass } from "../lib/tones";
import { cn } from "../lib/utils";
import { isUSMarketOpen } from "../services/marketTime";
import { useUi } from "../stores/ui";

const RANGES = [
  { value: "3mo", label: "3月" },
  { value: "6mo", label: "6月" },
  { value: "1y", label: "1年" },
  { value: "5y", label: "5年" },
];

function IndicesBoard() {
  const indices = useIndices();
  const usOpen = isUSMarketOpen();

  if (indices.isPending) {
    return (
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {["i1", "i2", "i3", "i4"].map((k) => (
          <Skeleton key={k} className="h-20" />
        ))}
      </div>
    );
  }
  const list = indices.data ?? [];
  if (list.length === 0) {
    return <EmptyState title="暂无指数数据" description="行情抓取后这里展示指数看板。" />;
  }
  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      {list.map((idx) => (
        <Card key={idx.code}>
          <div className="flex items-center justify-between">
            <span className="text-xs text-fg-3">{idx.name}</span>
            {idx.market === "US" && (
              <Badge tone={usOpen ? "up" : "neutral"}>{usOpen ? "交易中" : "休市"}</Badge>
            )}
          </div>
          <div className="mt-1.5 text-xl font-medium tabular-nums text-fg">
            {idx.price != null
              ? idx.price.toLocaleString("zh-CN", { maximumFractionDigits: 2 })
              : "—"}
          </div>
          {idx.change_pct != null && (
            <div className={cn("text-xs tabular-nums", toneClass[pnlTone(idx.change_pct)])}>
              {fmtSignedPct(idx.change_pct)}
            </div>
          )}
        </Card>
      ))}
    </div>
  );
}

function NasdaqPanel() {
  const portfolioId = useUi((s) => s.portfolioId);
  const [range, setRange] = useState("1y");
  const history = useIndexHistory("^NDX", range);
  // 全部交易散点叠加在 NDX 线上（继承旧 NasdaqOverview 语义，按当前组合隔离）
  const txs = useTransactions({ limit: 5000, portfolioId });

  const scatter = useMemo(() => {
    const points = history.data?.data ?? [];
    const byDate = new Map(points.map((p) => [p.date, p.close]));
    const buys: [string, number][] = [];
    const sells: [string, number][] = [];
    for (const tx of txs.data?.transactions ?? []) {
      const date = (tx.confirm_date ?? tx.trade_time ?? "").slice(0, 10);
      const close = byDate.get(date);
      if (!date || close == null) continue;
      if (tx.direction === "buy") buys.push([date, close]);
      else if (tx.direction === "sell") sells.push([date, close]);
    }
    return { buys, sells };
  }, [history.data, txs.data]);

  const stats = useMemo(() => {
    const list = txs.data?.transactions ?? [];
    const buyList = list.filter((t) => t.direction === "buy");
    const sellList = list.filter((t) => t.direction === "sell");
    return {
      buyCount: buyList.length,
      sellCount: sellList.length,
      buyAmount: buyList.reduce((s, t) => s + (t.amount ?? 0), 0),
      sellAmount: sellList.reduce((s, t) => s + (t.amount ?? 0), 0),
    };
  }, [txs.data]);

  const points = history.data?.data ?? [];

  return (
    <Card>
      <CardHeader className="flex-row flex-wrap items-center justify-between gap-2">
        <CardTitle className="flex items-center gap-2">
          纳斯达克100 透视
          <span className="text-xs font-normal text-fg-3">
            买 {stats.buyCount} 笔 {fmtCNY(stats.buyAmount)} · 卖 {stats.sellCount} 笔{" "}
            {fmtCNY(stats.sellAmount)}
          </span>
        </CardTitle>
        <Segmented id="ndx-range" value={range} onChange={setRange} options={RANGES} size="sm" />
      </CardHeader>
      <CardContent>
        <Chart
          height={340}
          loading={history.isPending}
          empty={!history.isPending && points.length === 0}
          emptyText="NDX 历史数据不可用"
          deps={[points, scatter]}
          option={(t) => ({
            ...baseChartOption(t),
            xAxis: { type: "time", axisLabel: { color: t.fg3, fontSize: 11 } },
            yAxis: {
              type: "value",
              scale: true,
              splitLine: { lineStyle: { color: t.border, type: "dashed" } },
              axisLabel: { color: t.fg3, fontSize: 11 },
            },
            legend: { top: 0, textStyle: { color: t.fg3, fontSize: 11 } },
            series: [
              {
                name: "NDX",
                type: "line",
                showSymbol: false,
                data: points.map((p) => [p.date, p.close]),
                lineStyle: { width: 1.5, color: t.info },
                itemStyle: { color: t.info },
              },
              {
                name: "买入",
                type: "scatter",
                symbol: "triangle",
                symbolSize: 8,
                itemStyle: { color: t.up },
                data: scatter.buys,
                z: 10,
              },
              {
                name: "卖出",
                type: "scatter",
                symbol: "triangle",
                symbolRotate: 180,
                symbolSize: 8,
                itemStyle: { color: t.down },
                data: scatter.sells,
                z: 10,
              },
            ],
          })}
        />
      </CardContent>
    </Card>
  );
}

export function MarketPage() {
  return (
    <div className="space-y-4">
      <IndicesBoard />
      <NasdaqPanel />
    </div>
  );
}
