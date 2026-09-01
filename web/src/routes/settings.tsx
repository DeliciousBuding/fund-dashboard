// 设置 /settings —— 四 tab（06 §3）：安全（改密/会话管理/登录事件）/ 偏好 /
// 数据（导入导出/备份说明）/ Agent（MCP 端点与密钥掩码）。

import {
  AuthEventsResponseSchema,
  AuthOkResponseSchema,
  AuthSessionsResponseSchema,
  SystemAgentResponseSchema,
} from "@fund-dashboard/contracts";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { KeyRound, LogOut, MonitorSmartphone, ShieldCheck } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { EmptyState } from "../components/ui/empty-state";
import { Input, Label } from "../components/ui/input";
import { Segmented } from "../components/ui/segmented";
import { Skeleton } from "../components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs";
import { ApiError, api } from "../lib/api";
import { fetchValidated } from "../lib/queries";
import { cn } from "../lib/utils";
import { useSettings } from "../stores/settings";

// ── 安全 tab ────────────────────────────────────────────────────────

const EVENT_LABEL: Record<string, string> = {
  setup: "初始化密码",
  login_ok: "登录成功",
  login_fail: "登录失败",
  lockout: "触发锁定",
  logout: "退出登录",
  password_change: "修改密码",
  session_revoke: "撤销会话",
};

function fmtTs(ts: number): string {
  return new Date(ts * 1000).toLocaleString("zh-CN", { hour12: false });
}

function ChangePasswordCard() {
  const [oldPwd, setOldPwd] = useState("");
  const [newPwd, setNewPwd] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const queryClient = useQueryClient();

  const strength =
    newPwd.length === 0
      ? null
      : newPwd.length >= 16 && /[a-zA-Z]/.test(newPwd) && /\d/.test(newPwd)
        ? "强"
        : newPwd.length >= 12 && /[a-zA-Z]/.test(newPwd) && /\d/.test(newPwd)
          ? "合格"
          : "不足（至少 12 位且含字母+数字）";

  const change = useMutation({
    mutationFn: async () => {
      const data = await api<unknown>("/api/auth/password", {
        method: "POST",
        body: { current_password: oldPwd, new_password: newPwd },
      });
      return AuthOkResponseSchema.parse(data);
    },
    onSuccess: async () => {
      toast.success("密码已更新，其他会话已全部退出");
      setOldPwd("");
      setNewPwd("");
      setConfirm("");
      await queryClient.invalidateQueries({ queryKey: ["auth-sessions"] });
    },
    onError: (e) => setError(e instanceof ApiError ? e.code : "修改失败"),
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <KeyRound className="size-4 text-accent" />
          修改密码
        </CardTitle>
      </CardHeader>
      <CardContent>
        <form
          className="grid max-w-sm gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            setError(null);
            if (newPwd !== confirm) {
              setError("两次输入的新密码不一致");
              return;
            }
            change.mutate();
          }}
        >
          <div>
            <Label htmlFor="old-pwd">当前密码</Label>
            <Input
              id="old-pwd"
              type="password"
              autoComplete="current-password"
              value={oldPwd}
              onChange={(e) => setOldPwd(e.target.value)}
              required
            />
          </div>
          <div>
            <Label htmlFor="new-pwd">新密码</Label>
            <Input
              id="new-pwd"
              type="password"
              autoComplete="new-password"
              value={newPwd}
              onChange={(e) => setNewPwd(e.target.value)}
              required
            />
            {strength && (
              <p
                className={cn(
                  "mt-1 text-xs",
                  strength === "强" ? "text-up" : strength === "合格" ? "text-warn" : "text-danger",
                )}
              >
                强度：{strength}
              </p>
            )}
          </div>
          <div>
            <Label htmlFor="confirm-pwd">确认新密码</Label>
            <Input
              id="confirm-pwd"
              type="password"
              autoComplete="new-password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
            />
          </div>
          {error && <p className="text-sm text-danger">{error}</p>}
          <Button type="submit" disabled={change.isPending} className="w-fit">
            {change.isPending ? "提交中…" : "更新密码"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function SessionsCard() {
  const queryClient = useQueryClient();
  const sessions = useQuery({
    queryKey: ["auth-sessions"],
    queryFn: ({ signal }) =>
      fetchValidated("/api/auth/sessions", AuthSessionsResponseSchema, signal),
  });
  const revoke = useMutation({
    mutationFn: async (idPrefix: string) => {
      const data = await api<unknown>(`/api/auth/sessions/${idPrefix}/revoke`, {
        method: "POST",
      });
      return AuthOkResponseSchema.parse(data);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["auth-sessions"] });
      toast.success("会话已撤销");
    },
    onError: () => toast.error("撤销失败"),
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <MonitorSmartphone className="size-4 text-accent" />
          活动会话
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {sessions.isPending ? (
          <Skeleton className="h-24" />
        ) : sessions.isError ? (
          <EmptyState
            title="会话加载失败"
            description="无法读取活动会话，请重试。"
            action={
              <Button size="sm" onClick={() => void sessions.refetch()}>
                重试
              </Button>
            }
          />
        ) : (sessions.data?.sessions ?? []).length === 0 ? (
          <p className="text-sm text-fg-3">暂无活动会话</p>
        ) : (
          <>
            {(sessions.data?.sessions ?? []).map((s) => (
              <div
                key={s.id_prefix}
                className="flex flex-wrap items-center gap-3 rounded-lg border border-border px-3 py-2.5"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 text-sm text-fg">
                    <span className="max-w-72 truncate">{s.user_agent || "未知设备"}</span>
                    {s.current && <Badge tone="accent">当前</Badge>}
                  </div>
                  <div className="mt-0.5 text-[11px] text-fg-3 tabular-nums">
                    {s.ip || "未知 IP"} · 最近活跃 {fmtTs(s.last_seen_at)} · 过期{" "}
                    {fmtTs(s.expires_at)}
                  </div>
                </div>
                {!s.current && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => revoke.mutate(s.id_prefix)}
                    disabled={revoke.isPending}
                  >
                    <LogOut className="size-3.5" />
                    撤销
                  </Button>
                )}
              </div>
            ))}
            {sessions.data?.truncated && (
              <p className="pt-1 text-xs text-fg-3">
                仅显示最近 200 条会话（共 {sessions.data.total} 条）
              </p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}

function AuthEventsCard() {
  const events = useQuery({
    queryKey: ["auth-events"],
    queryFn: ({ signal }) =>
      fetchValidated("/api/auth/events?limit=20", AuthEventsResponseSchema, signal),
  });
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ShieldCheck className="size-4 text-accent" />
          最近登录事件
        </CardTitle>
      </CardHeader>
      <CardContent>
        {events.isPending ? (
          <Skeleton className="h-24" />
        ) : events.isError ? (
          <EmptyState
            title="登录事件加载失败"
            description="无法读取登录事件，请重试。"
            action={
              <Button size="sm" onClick={() => void events.refetch()}>
                重试
              </Button>
            }
          />
        ) : (events.data?.events ?? []).length === 0 ? (
          <p className="text-sm text-fg-3">暂无记录</p>
        ) : (
          <ul className="space-y-1.5">
            {(events.data?.events ?? []).map((e) => (
              <li
                key={`${e.ts}-${e.event}-${e.ip}-${e.detail}`}
                className="flex items-baseline gap-3 text-xs"
              >
                <span className="shrink-0 tabular-nums text-fg-3">{fmtTs(e.ts)}</span>
                <Badge
                  tone={
                    e.event === "login_fail" || e.event === "lockout"
                      ? "danger"
                      : e.event === "login_ok"
                        ? "up"
                        : "neutral"
                  }
                >
                  {EVENT_LABEL[e.event] ?? e.event}
                </Badge>
                <span className="min-w-0 truncate text-fg-3">
                  {e.ip ?? ""} {e.detail ?? ""}
                </span>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

// ── 偏好 tab ────────────────────────────────────────────────────────

function PreferencesTab() {
  const theme = useSettings((s) => s.theme);
  const setTheme = useSettings((s) => s.setTheme);
  const convention = useSettings((s) => s.convention);
  const setConvention = useSettings((s) => s.setConvention);
  const density = useSettings((s) => s.density);
  const setDensity = useSettings((s) => s.setDensity);

  return (
    <div className="grid max-w-2xl gap-4">
      <Card>
        <CardHeader>
          <CardTitle>主题</CardTitle>
        </CardHeader>
        <CardContent>
          <Segmented
            id="pref-theme"
            value={theme}
            onChange={(v) => setTheme(v as typeof theme)}
            options={[
              { value: "dark", label: "暗色" },
              { value: "light", label: "亮色" },
              { value: "system", label: "跟随系统" },
            ]}
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>涨跌色约定</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <Segmented
            id="pref-convention"
            value={convention}
            onChange={(v) => setConvention(v as typeof convention)}
            options={[
              { value: "cn", label: "中式 · 涨红跌绿" },
              { value: "western", label: "西式 · 涨绿跌红" },
            ]}
          />
          <p className="text-xs text-fg-3">全站即时生效，包括图表与买卖点标记。</p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>密度</CardTitle>
        </CardHeader>
        <CardContent>
          <Segmented
            id="pref-density"
            value={density}
            onChange={(v) => setDensity(v as typeof density)}
            options={[
              { value: "comfortable", label: "舒适" },
              { value: "compact", label: "紧凑" },
            ]}
          />
        </CardContent>
      </Card>
    </div>
  );
}

// ── 数据 tab ────────────────────────────────────────────────────────

function DataTab() {
  return (
    <div className="grid max-w-2xl gap-4">
      <Card>
        <CardHeader>
          <CardTitle>导入 / 导出</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm text-fg-2">
          <p>
            交易导入与 CSV/XLSX 导出在
            <Link to="/transactions" className="text-accent hover:underline">
              {" "}
              交易{" "}
            </Link>
            页完成（新增 = 单条导入；导出带中文表头与 BOM）。
          </p>
          <p className="text-xs text-fg-3">批量 JSON 导入走同一入口的幂等语义（order_id 去重）。</p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>备份</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm text-fg-2">
          <p>SQLite 模式：每日 03:00 自动 WAL 检查点 + 三表清扫（过期会话/确认/审计）。</p>
          <p className="text-xs text-fg-3">
            数据库文件级别的备份由部署侧（容器卷快照）负责；一致性校验与体检在
            <Link to="/system" className="text-accent hover:underline">
              {" "}
              工作台{" "}
            </Link>
            触发。
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

// ── Agent tab ───────────────────────────────────────────────────────

function AgentTab() {
  const info = useQuery({
    queryKey: ["system-agent"],
    queryFn: ({ signal }) => fetchValidated("/api/system/agent", SystemAgentResponseSchema, signal),
  });

  if (info.isPending) return <Skeleton className="h-48 max-w-2xl" />;
  if (info.isError) {
    return (
      <EmptyState
        title="Agent 信息加载失败"
        description="无法读取 MCP 端点与密钥掩码，请重试。"
        action={
          <Button size="sm" onClick={() => void info.refetch()}>
            重试
          </Button>
        }
      />
    );
  }
  const d = info.data;

  return (
    <div className="grid max-w-2xl gap-4">
      <Card>
        <CardHeader>
          <CardTitle>MCP 服务</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm text-fg-2">
          <p>
            端点：
            <code className="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-xs">
              {d?.request_method} {d?.endpoint}
            </code>
          </p>
          <p>
            工具面：共 <span className="tabular-nums text-accent">{d?.tools.total_tools ?? 0}</span>{" "}
            个
            {d?.tools.by_scope &&
              `（读 ${d.tools.by_scope.read ?? 0} / 写 ${d.tools.by_scope.write ?? 0}）`}
          </p>
          {d?.tools.by_permission && (
            <p className="text-xs text-fg-3">
              权限分布：
              {Object.entries(d.tools.by_permission)
                .map(([k, v]) => `${k} ${v}`)
                .join(" · ")}
            </p>
          )}
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>API 密钥</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm text-fg-2">
          {(d?.key_env_vars ?? []).map((env) => (
            <div
              key={env}
              className="flex items-center justify-between rounded-lg border border-border px-3 py-2"
            >
              <code className="font-mono text-xs">{env}</code>
              <span className="text-xs text-fg-3">
                {d?.keys[env === "MCP_API_KEY" ? "mcp_api_key" : "public_mcp_key"] ?? "未配置"}
              </span>
            </div>
          ))}
          <p className="text-xs text-fg-3">
            密钥只经环境变量下发、服务端保存，界面仅显示掩码。轮换 = 修改环境变量后重启服务。
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

// ── 页面 ────────────────────────────────────────────────────────────

export function SettingsPage() {
  return (
    <Tabs defaultValue="security">
      <TabsList>
        <TabsTrigger value="security">安全</TabsTrigger>
        <TabsTrigger value="preferences">偏好</TabsTrigger>
        <TabsTrigger value="data">数据</TabsTrigger>
        <TabsTrigger value="agent">Agent</TabsTrigger>
      </TabsList>
      <TabsContent value="security" className="space-y-4 pt-4">
        <ChangePasswordCard />
        <SessionsCard />
        <AuthEventsCard />
      </TabsContent>
      <TabsContent value="preferences" className="pt-4">
        <PreferencesTab />
      </TabsContent>
      <TabsContent value="data" className="pt-4">
        <DataTab />
      </TabsContent>
      <TabsContent value="agent" className="pt-4">
        <AgentTab />
      </TabsContent>
    </Tabs>
  );
}
