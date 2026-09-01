import { Chart } from "../../components/charts/Chart";
import { baseChartOption } from "../../components/charts/theme";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../../components/ui/card";
import { EmptyState } from "../../components/ui/empty-state";
import { Skeleton } from "../../components/ui/skeleton";
import { Table, TBody, Td, THead, Th, Tr } from "../../components/ui/table";
import { fmtCNY, fmtPct } from "../../lib/format";
import { usePenetration } from "../../lib/queries";
import { useUi } from "../../stores/ui";

// ── penetration（底层股票暴露）────────────────────────────────────────

export function PenetrationTab() {
  const portfolioId = useUi((s) => s.portfolioId);
  const penetration = usePenetration(portfolioId);
  const rows = penetration.data?.penetration ?? [];

  return (
    <div className="space-y-4">
      {penetration.isPending ? (
        <Skeleton className="h-80" />
      ) : penetration.isError ? (
        <EmptyState
          title="穿透数据加载失败"
          description="请稍后重试；若持续失败可到工作台触发持仓抓取。"
          action={
            <Button size="sm" onClick={() => void penetration.refetch()}>
              重试
            </Button>
          }
        />
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
