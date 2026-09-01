// 工作台 /system —— 系统状态卡 + 调度任务（触发/最近运行）+ 告警扫描。
// 写操作（抓取/校验）全部二次确认（06 §3）。

import {
  CheckAlertsResponseSchema,
  SystemJobsResponseSchema,
  SystemStatusSchema,
} from "@fund-dashboard/contracts";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, Database, Play, RefreshCw, ShieldCheck } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../components/ui/dialog";
import { Skeleton } from "../components/ui/skeleton";
import { Table, TBody, Td, THead, Th, Tr } from "../components/ui/table";
import { ApiError, api } from "../lib/api";
import { fetchValidated } from "../lib/queries";

function fmtTs(ts?: number): string {
  if (!ts) return "—";
  return new Date(ts * 1000).toLocaleString("zh-CN", { hour12: false });
}

function fmtUptime(sec: number): string {
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d} 天 ${h} 小时`;
  if (h > 0) return `${h} 小时 ${m} 分`;
  return `${m} 分`;
}

// 后端 freshnessHealth（internal/service/admin/freshness.go）返回 fresh / stale / degraded；
// neutral 兜底任何缺失或未知值，禁止裸索引导致渲染崩溃。
const HEALTH_BADGE: Record<string, { tone: "up" | "warn" | "danger" | "neutral"; label: string }> =
  {
    fresh: { tone: "up", label: "健康" },
    stale: { tone: "warn", label: "陈旧数据较多" },
    degraded: { tone: "danger", label: "数据不完整" },
    neutral: { tone: "neutral", label: "状态未知" },
  };

type ActionKey = "crawl-nav" | "crawl-holdings" | "verify";
const ACTIONS: { key: ActionKey; label: string; description: string }[] = [
  { key: "crawl-nav", label: "抓取净值", description: "立即刷新全部持有标的的最新净值。" },
  {
    key: "crawl-holdings",
    label: "抓取持仓",
    description: "拉取基金最新季报持仓（穿透分析数据源）。",
  },
  { key: "verify", label: "一致性校验", description: "台账 vs 快照 vs 净值的一致性检查。" },
];

export function SystemPage() {
  const status = useQuery({
    queryKey: ["system-status"],
    queryFn: ({ signal }) => fetchValidated("/api/system/status", SystemStatusSchema, signal),
    refetchInterval: 30_000,
  });
  const jobs = useQuery({
    queryKey: ["system-jobs"],
    queryFn: ({ signal }) => fetchValidated("/api/system/jobs", SystemJobsResponseSchema, signal),
    refetchInterval: 30_000,
  });
  const alerts = useQuery({
    queryKey: ["system-alerts"],
    queryFn: ({ signal }) => fetchValidated("/api/alerts", CheckAlertsResponseSchema, signal),
    staleTime: 5 * 60 * 1000,
  });

  const [confirmAction, setConfirmAction] = useState<ActionKey | null>(null);
  const queryClient = useQueryClient();
  const trigger = useMutation({
    mutationFn: (key: ActionKey) => api(`/api/system/${key}`, { method: "POST" }),
    onSuccess: async (_d, key) => {
      toast.success(`${ACTIONS.find((a) => a.key === key)?.label} 已触发`);
      setConfirmAction(null);
      await queryClient.invalidateQueries({ queryKey: ["system-jobs"] });
      await queryClient.invalidateQueries({ queryKey: ["system-status"] });
    },
    onError: (e) =>
      toast.error("触发失败", { description: e instanceof ApiError ? e.code : String(e) }),
  });

  const s = status.data;
  const health = HEALTH_BADGE[s?.freshness?.health ?? ""] ?? HEALTH_BADGE.neutral;

  return (
    <div className="space-y-4">
      {/* 状态卡 */}
      {status.isPending ? (
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          {["a", "b", "c", "d"].map((k) => (
            <Skeleton key={k} className="h-20" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <Card>
            <div className="flex items-center gap-1.5 text-[11px] text-fg-3">
              <Activity className="size-3" /> 版本
            </div>
            <div className="mt-1 text-lg font-medium text-fg">{s?.version ?? "—"}</div>
            <div className="text-[11px] text-fg-3">{s?.go_version}</div>
          </Card>
          <Card>
            <div className="flex items-center gap-1.5 text-[11px] text-fg-3">
              <Database className="size-3" /> 数据库
            </div>
            <div className="mt-1 text-lg font-medium uppercase text-fg">{s?.db_driver ?? "—"}</div>
            <div className="text-[11px] text-fg-3 tabular-nums">
              {s?.db_size_bytes ? `${(s.db_size_bytes / 1048576).toFixed(1)} MB` : ""}
            </div>
          </Card>
          <Card>
            <div className="flex items-center gap-1.5 text-[11px] text-fg-3">
              <RefreshCw className="size-3" /> 运行时长
            </div>
            <div className="mt-1 text-lg font-medium text-fg">
              {s ? fmtUptime(s.uptime_sec) : "—"}
            </div>
          </Card>
          <Card>
            <div className="flex items-center gap-1.5 text-[11px] text-fg-3">
              <ShieldCheck className="size-3" /> 数据新鲜度
            </div>
            <div className="mt-1.5">
              <Badge tone={health.tone}>{health.label}</Badge>
            </div>
          </Card>
        </div>
      )}

      {/* 任务中心 */}
      <Card className="overflow-x-auto" style={{ padding: 0 }}>
        <div className="flex items-center justify-between px-5 pt-4 pb-3">
          <h3 className="text-sm font-medium text-fg">调度任务</h3>
          <div className="flex gap-1.5">
            {ACTIONS.map((a) => (
              <Button
                key={a.key}
                variant="outline"
                size="sm"
                onClick={() => setConfirmAction(a.key)}
              >
                <Play className="size-3.5" />
                {a.label}
              </Button>
            ))}
          </div>
        </div>
        <Table>
          <THead>
            <Tr>
              <Th>任务</Th>
              <Th>计划</Th>
              <Th>上次运行</Th>
              <Th>下次运行</Th>
              <Th>状态</Th>
            </Tr>
          </THead>
          <TBody>
            {jobs.isPending ? (
              <Tr>
                <Td colSpan={5}>
                  <Skeleton className="h-6 w-full" />
                </Td>
              </Tr>
            ) : (jobs.data?.jobs ?? []).length === 0 ? (
              <Tr>
                <Td colSpan={5} className="text-center text-sm text-fg-3">
                  调度器未启用或暂无任务
                </Td>
              </Tr>
            ) : (
              (jobs.data?.jobs ?? []).map((j) => (
                <Tr key={j.name}>
                  <Td className="font-medium text-fg">{j.name}</Td>
                  <Td className="text-fg-2">{j.schedule}</Td>
                  <Td className="tabular-nums text-fg-2">{fmtTs(j.last_run)}</Td>
                  <Td className="tabular-nums text-fg-2">
                    {j.next_run > 0 ? fmtTs(j.next_run) : "—"}
                  </Td>
                  <Td>
                    {j.last_error ? (
                      <Badge tone="danger" title={j.last_error}>
                        失败
                      </Badge>
                    ) : j.last_run ? (
                      <Badge tone="up">正常</Badge>
                    ) : (
                      <Badge>未运行</Badge>
                    )}
                  </Td>
                </Tr>
              ))
            )}
          </TBody>
        </Table>
      </Card>

      {/* 告警扫描 */}
      <Card>
        <CardHeader>
          <CardTitle>
            告警扫描
            {alerts.data && (
              <span className="ml-2 text-xs font-normal text-fg-3 tabular-nums">
                {alerts.data.count} 条
              </span>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {alerts.isPending ? (
            <Skeleton className="h-20" />
          ) : (alerts.data?.alerts ?? []).length === 0 ? (
            <p className="text-sm text-fg-3">
              当前无告警——涨跌、回撤、陈旧、定投命中四档扫描均正常。
            </p>
          ) : (
            <ul className="space-y-2">
              {(alerts.data?.alerts ?? []).map((a) => (
                <li
                  key={`${a.kind}-${a.code}-${a.severity}-${a.message}`}
                  className="flex items-center gap-3 text-sm"
                >
                  <Badge
                    tone={
                      a.severity === "high"
                        ? "danger"
                        : a.severity === "medium" || a.severity === "low"
                          ? "warn"
                          : "neutral"
                    }
                  >
                    {a.kind}
                  </Badge>
                  <span className="min-w-0 flex-1 truncate text-fg-2">
                    {a.name || a.code} · {a.message}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <Dialog open={confirmAction != null} onOpenChange={() => setConfirmAction(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{ACTIONS.find((a) => a.key === confirmAction)?.label}</DialogTitle>
            <DialogDescription>
              {ACTIONS.find((a) => a.key === confirmAction)?.description}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirmAction(null)}>
              取消
            </Button>
            <Button
              onClick={() => confirmAction && trigger.mutate(confirmAction)}
              disabled={trigger.isPending}
            >
              {trigger.isPending ? "触发中…" : "确认执行"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
