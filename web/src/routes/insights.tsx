// 信号 /insights —— 组合级投资支架卡（harness 快照）+ 来源事件流（已读/有用标记）
// + 告警扫描。facts-only 边界：只呈现事实与推荐动作，不做决策措辞。
import { AuthOkResponseSchema } from "@fund-dashboard/contracts";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { BellRing, BookOpenCheck, ThumbsUp } from "lucide-react";
import { toast } from "sonner";
import { AlertList } from "../components/AlertList";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { EmptyState } from "../components/ui/empty-state";
import { Skeleton } from "../components/ui/skeleton";
import { api } from "../lib/api";
import { fmtCNY } from "../lib/format";
import { useAlerts, useHarness, useSourceEvents } from "../lib/queries";
import { mutationErrorMessage } from "../services/userError";
import { useUi } from "../stores/ui";

// isSafeHttp allows only http(s) URLs to be used as clickable hrefs. Source
// event URLs originate from crawled pages and are rendered as links, so any
// other scheme (javascript:, data:, vbscript:) must be neutralised.
function isSafeHttp(value: string | null | undefined): boolean {
  if (!value) return false;
  try {
    const u = new URL(value);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
}

// ── 支架卡 ──────────────────────────────────────────────────────────

function HarnessCard() {
  const portfolioId = useUi((s) => s.portfolioId);
  const harness = useHarness(portfolioId);

  if (harness.isPending) return <Skeleton className="h-48" />;
  if (harness.isError) {
    return (
      <EmptyState
        title="支架快照加载失败"
        description="无法读取投资支架，请重试。"
        action={
          <Button size="sm" onClick={() => void harness.refetch()}>
            重试
          </Button>
        }
      />
    );
  }
  const h = harness.data;
  if (!h) return null;

  const dq = h.data_quality;
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between">
        <CardTitle>投资支架快照</CardTitle>
        <span className="text-[11px] text-fg-3 tabular-nums">
          {new Date(h.generated_at).toLocaleString("zh-CN", { hour12: false })}
        </span>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div>
            <div className="text-[11px] text-fg-3">总市值</div>
            <div className="text-lg font-medium tabular-nums text-fg">{fmtCNY(h.total_value)}</div>
          </div>
          <div>
            <div className="text-[11px] text-fg-3">持仓数</div>
            <div className="text-lg font-medium tabular-nums text-fg">{h.holdings_count}</div>
          </div>
          <div>
            <div className="text-[11px] text-fg-3">数据覆盖</div>
            <div className="text-lg font-medium tabular-nums text-fg">
              {dq.holdings_coverage_pct.toFixed(0)}%
            </div>
          </div>
          <div>
            <div className="text-[11px] text-fg-3">数据缺口</div>
            <div className="text-lg font-medium tabular-nums text-fg">
              {dq.stale_price_count + dq.missing_cost_basis_count + dq.missing_change_pct_count}
            </div>
          </div>
        </div>

        {h.holding_signals.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {h.holding_signals
              .filter((s) => s.signal_tags.length > 0)
              .slice(0, 8)
              .map((s) => (
                <Badge key={s.code} tone="neutral" title={s.name}>
                  {s.name}: {s.signal_tags.join(" / ")}
                </Badge>
              ))}
          </div>
        )}

        {h.recommended_agent_actions.length > 0 && (
          <div className="rounded-lg border border-border bg-surface-2 p-3">
            <div className="pb-1.5 text-xs font-medium text-fg-2">推荐动作（供 Agent 参考）</div>
            <ul className="space-y-1">
              {h.recommended_agent_actions.slice(0, 5).map((a) => (
                <li
                  key={`${a.priority}-${a.tool}-${a.reason}`}
                  className="flex items-baseline gap-2 text-xs"
                >
                  <Badge
                    tone={
                      a.priority === "high"
                        ? "danger"
                        : a.priority === "medium"
                          ? "warn"
                          : "neutral"
                    }
                  >
                    {a.priority}
                  </Badge>
                  <code className="shrink-0 font-mono text-fg-3">{a.tool}</code>
                  <span className="min-w-0 truncate text-fg-2">{a.reason}</span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ── 告警卡 ──────────────────────────────────────────────────────────

function AlertsCard() {
  const alerts = useAlerts();
  if (alerts.isPending) return <Skeleton className="h-32" />;
  if (alerts.isError) {
    return (
      <EmptyState
        title="告警扫描加载失败"
        description="无法读取告警，请重试。"
        action={
          <Button size="sm" onClick={() => void alerts.refetch()}>
            重试
          </Button>
        }
      />
    );
  }
  const list = alerts.data?.alerts ?? [];
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <BellRing className="size-4 text-warn" />
          告警扫描
          <span className="text-xs font-normal text-fg-3 tabular-nums">
            {alerts.data?.count ?? 0} 条
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {list.length === 0 ? (
          <p className="text-sm text-fg-3">无告警：涨跌、回撤、陈旧、定投命中均正常。</p>
        ) : (
          <AlertList alerts={list} />
        )}
      </CardContent>
    </Card>
  );
}

// ── 事件流 ──────────────────────────────────────────────────────────

function EventsCard() {
  const events = useSourceEvents();
  const queryClient = useQueryClient();
  const mark = useMutation({
    mutationFn: async ({
      id,
      is_read,
      is_useful,
    }: {
      id: number;
      is_read?: boolean;
      is_useful?: boolean;
    }) => {
      const data = await api<unknown>(`/api/portfolio/source-events/${id}`, {
        method: "PATCH",
        body: { is_read, is_useful },
      });
      return AuthOkResponseSchema.parse(data);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["source-events"] });
    },
    onError: (e) => toast.error("标记失败", { description: mutationErrorMessage(e, "请稍后重试") }),
  });

  const list = events.data?.events ?? [];

  return (
    <Card>
      <CardHeader>
        <CardTitle>来源事件流</CardTitle>
      </CardHeader>
      <CardContent>
        {events.isPending ? (
          <Skeleton className="h-32" />
        ) : events.isError ? (
          <EmptyState
            title="来源事件加载失败"
            description="无法读取来源事件，请重试。"
            action={
              <Button size="sm" onClick={() => void events.refetch()}>
                重试
              </Button>
            }
          />
        ) : list.length === 0 ? (
          <EmptyState title="暂无事件" description="Agent 抓取的相关资讯会出现在这里。" />
        ) : (
          <ul className="space-y-2.5">
            {list.map((e) => (
              <li
                key={e.id}
                className={`rounded-lg border border-border p-3 ${e.is_read ? "opacity-55" : ""}`}
              >
                <div className="flex items-start gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      {e.url ? (
                        <a
                          href={isSafeHttp(e.url) ? e.url : undefined}
                          target="_blank"
                          rel="noreferrer noopener"
                          className="truncate text-sm font-medium text-fg hover:text-accent"
                        >
                          {e.title}
                        </a>
                      ) : (
                        <span className="truncate text-sm font-medium text-fg">{e.title}</span>
                      )}
                      {e.is_useful && <ThumbsUp className="size-3.5 shrink-0 text-up" />}
                    </div>
                    {e.snippet && (
                      <p className="mt-1 line-clamp-2 text-xs text-fg-3">{e.snippet}</p>
                    )}
                    <div className="mt-1 text-[11px] text-fg-3">
                      {e.source}
                      {e.related_security_name && ` · ${e.related_security_name}`} ·{" "}
                      {new Date(e.fetched_at).toLocaleString("zh-CN", { hour12: false })}
                    </div>
                  </div>
                  <div className="flex shrink-0 gap-1">
                    {!e.is_read && (
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="标记已读"
                        onClick={() => mark.mutate({ id: e.id, is_read: true })}
                      >
                        <BookOpenCheck className="size-4" />
                      </Button>
                    )}
                    {!e.is_useful && (
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="标记有用"
                        onClick={() => mark.mutate({ id: e.id, is_useful: true, is_read: true })}
                      >
                        <ThumbsUp className="size-4" />
                      </Button>
                    )}
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

export function InsightsPage() {
  return (
    <div className="space-y-4">
      <HarnessCard />
      <div className="grid gap-4 lg:grid-cols-2">
        <AlertsCard />
        <EventsCard />
      </div>
    </div>
  );
}
