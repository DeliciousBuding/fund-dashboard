import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import { fmtCNY, fmtSignedCNY, fmtSignedPct, pnlTone } from "../lib/format";

// 组合汇总（/api/portfolio/）——响应为 snake_case（httpapi DTO 层转换）。
interface PortfolioSummary {
  current_value: number;
  unrealized_pnl: number;
  pnl_pct: number;
  invested_cost: number;
  held_funds: number;
  unique_funds: number;
  unique_stocks: number;
  total_tx: number;
  last_nav_date?: string | null;
}

const toneClass = { up: "text-up", down: "text-down", flat: "text-fg-3" } as const;

export function OverviewPage() {
  const summary = useQuery({
    queryKey: ["portfolio-summary"],
    queryFn: ({ signal }) => api<PortfolioSummary>("/api/portfolio/", { signal }),
  });

  return (
    <div className="mx-auto max-w-5xl px-6 py-8">
      <header className="mb-8 flex items-baseline justify-between">
        <h1 className="text-lg font-medium text-fg">总览</h1>
        {summary.data?.last_nav_date ? (
          <span className="text-xs text-fg-3">净值截至 {summary.data.last_nav_date}</span>
        ) : null}
      </header>

      {summary.isPending ? (
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {["a", "b", "c", "d"].map((key) => (
            <div
              key={key}
              className="h-28 animate-pulse rounded-xl border border-border bg-surface-1"
            />
          ))}
        </div>
      ) : summary.isError ? (
        <div className="rounded-xl border border-border bg-surface-1 p-6 text-sm text-danger">
          加载失败，请刷新重试
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <KpiCard label="当前市值" value={fmtCNY(summary.data.current_value)} hero />
          <KpiCard
            label="未实现盈亏"
            value={fmtSignedCNY(summary.data.unrealized_pnl)}
            sub={fmtSignedPct(summary.data.pnl_pct)}
            tone={pnlTone(summary.data.unrealized_pnl)}
          />
          <KpiCard
            label="投入成本"
            value={fmtCNY(summary.data.invested_cost)}
            sub={`${summary.data.total_tx} 笔交易`}
          />
          <KpiCard
            label="持有标的"
            value={String(summary.data.held_funds)}
            sub={`基金 ${summary.data.unique_funds} · 股票 ${summary.data.unique_stocks}`}
          />
        </div>
      )}

      <p className="mt-10 text-center text-xs text-fg-3">
        持仓 / 交易 / 分析 / 定投等完整页面按 docs/design/05 路线图 W3–W6 陆续上线
      </p>
    </div>
  );
}

function KpiCard(props: {
  label: string;
  value: string;
  sub?: string;
  hero?: boolean;
  tone?: keyof typeof toneClass;
}) {
  return (
    <section className="rounded-xl border border-border bg-surface-1 p-5">
      <div className="text-xs text-fg-3">{props.label}</div>
      <div
        className={`tnum mt-2 font-medium ${props.hero ? "text-2xl" : "text-xl"} ${
          props.tone ? toneClass[props.tone] : "text-fg"
        }`}
      >
        {props.value}
      </div>
      {props.sub ? <div className="tnum mt-1 text-xs text-fg-2">{props.sub}</div> : null}
    </section>
  );
}
