import { useMemo, useState } from "react";
import { Chart } from "../../components/charts/Chart";
import { baseChartOption } from "../../components/charts/theme";
import { Button } from "../../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../../components/ui/card";
import { EmptyState } from "../../components/ui/empty-state";
import { Input, Label } from "../../components/ui/input";
import { Skeleton } from "../../components/ui/skeleton";
import { fmtCNY, fmtSignedPct, pnlTone } from "../../lib/format";
import { useNavHistory, useSecurities } from "../../lib/queries";
import { toneClass } from "../../lib/tones";
import { cn } from "../../lib/utils";
import { calcIRR } from "../../services/irr";
import { useUi } from "../../stores/ui";

// ── backtest（定投 vs 一次性——旧 DcaBacktestChart 语义）────────────────

export function BacktestTab() {
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
      ) : nav.isError ? (
        <EmptyState
          title="净值历史加载失败"
          description="无法读取该标的净值，请重试。"
          action={
            <Button size="sm" onClick={() => void nav.refetch()}>
              重试
            </Button>
          }
        />
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
