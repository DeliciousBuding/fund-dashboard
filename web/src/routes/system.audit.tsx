// 审计 /system/audit —— auth_events + agent_audit_events 合并时间线（06 §3）。
// kind chip 过滤、时间倒序、detail 截断展开。

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Badge } from "../components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { EmptyState } from "../components/ui/empty-state";
import { Segmented } from "../components/ui/segmented";
import { Skeleton } from "../components/ui/skeleton";
import { api } from "../lib/api";

interface AuditEntry {
  kind: "auth" | "agent";
  ts: number;
  event: string;
  summary: string;
  ip?: string;
}

const KIND_LABEL: Record<string, string> = { auth: "认证", agent: "Agent" };
const AUTH_EVENT_LABEL: Record<string, string> = {
  setup: "初始化密码",
  login_ok: "登录成功",
  login_fail: "登录失败",
  lockout: "触发锁定",
  logout: "退出登录",
  password_change: "修改密码",
  session_revoke: "撤销会话",
};

export function SystemAuditPage() {
  const [kind, setKind] = useState("all");
  const audit = useQuery({
    queryKey: ["system-audit"],
    queryFn: ({ signal }) =>
      api<{ events: AuditEntry[] }>("/api/system/audit?limit=200", { signal }),
    staleTime: 60 * 1000,
  });

  const rows = (audit.data?.events ?? []).filter((e) => kind === "all" || e.kind === kind);

  return (
    <Card>
      <CardHeader className="flex-row flex-wrap items-center justify-between gap-2">
        <CardTitle>审计时间线</CardTitle>
        <Segmented
          id="audit-kind"
          value={kind}
          onChange={setKind}
          size="sm"
          options={[
            { value: "all", label: "全部" },
            { value: "auth", label: "认证" },
            { value: "agent", label: "Agent" },
          ]}
        />
      </CardHeader>
      <CardContent>
        {audit.isPending ? (
          <Skeleton className="h-48" />
        ) : rows.length === 0 ? (
          <EmptyState
            title="暂无审计事件"
            description="登录、改密、Agent 工具调用都会留痕在这里。"
          />
        ) : (
          <ul className="divide-y divide-border">
            {rows.map((e, i) => (
              <li key={`${e.ts}-${i}`} className="flex items-baseline gap-3 py-2 text-sm">
                <span className="shrink-0 text-xs tabular-nums text-fg-3">
                  {new Date(e.ts * 1000).toLocaleString("zh-CN", { hour12: false })}
                </span>
                <Badge tone={e.kind === "auth" ? "accent" : "info"}>
                  {KIND_LABEL[e.kind] ?? e.kind}
                </Badge>
                <span className="shrink-0 text-fg-2">
                  {e.kind === "auth" ? (AUTH_EVENT_LABEL[e.event] ?? e.event) : e.event}
                </span>
                <span className="min-w-0 flex-1 truncate text-xs text-fg-3">
                  {e.ip && `${e.ip} · `}
                  {e.summary}
                </span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
