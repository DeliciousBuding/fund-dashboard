// 顶栏：指数 ticker（SSE，无动画更新）+ 新鲜度徽章 + 主题切换 + ⌘K + 账户菜单。
// 移动端隐藏 ticker 细节，只留连接态点。

import { useMutation } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";
import { CircleDot, LogOut, Moon, Search, Sun, SunMoon } from "lucide-react";
import { toast } from "sonner";
import { useMarketStream } from "../../hooks/useMarketStream";
import { ApiError } from "../../lib/api";
import { logout } from "../../lib/auth";
import { refreshAuthStatus } from "../../lib/authQuery";
import { useFreshness } from "../../lib/queries";
import { queryClient } from "../../lib/queryClient";
import { cn } from "../../lib/utils";
import { type ThemeMode, useSettings } from "../../stores/settings";
import { useUi } from "../../stores/ui";
import { Badge } from "../ui/badge";
import { Button } from "../ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../ui/tooltip";

function daysSince(date: string | null): number | null {
  if (!date) return null;
  const then = new Date(`${date}T00:00:00`);
  if (Number.isNaN(then.getTime())) return null;
  return Math.floor((Date.now() - then.getTime()) / 86_400_000);
}

function FreshnessBadge() {
  const { data, isError } = useFreshness();
  if (isError || !data?.last_nav_date) return null;
  const days = daysSince(data.last_nav_date);
  if (days === null) return null;
  // 规格（03 §7）：warn ≥4 天 / critical ≥7 天（badge tone 映射 danger）
  const tone = days >= 7 ? "danger" : days >= 4 ? "warn" : "neutral";
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge tone={tone} className="cursor-default tabular-nums">
            数据截至 {data.last_nav_date.slice(5)}
            {days > 0 && ` · ${days} 天前`}
          </Badge>
        </TooltipTrigger>
        <TooltipContent>
          {data.anomaly_count > 0 ? `含 ${data.anomaly_count} 条异常交易` : "数据新鲜度正常"}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

function MarketTicker() {
  const { indices, connected } = useMarketStream();
  return (
    <div className="hidden min-w-0 flex-1 items-center gap-4 overflow-hidden px-4 lg:flex">
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="flex shrink-0 items-center">
              <CircleDot
                className={cn("size-3", connected ? "text-up" : "text-fg-3")}
                aria-label={connected ? "实时行情已连接" : "实时行情断开"}
              />
            </span>
          </TooltipTrigger>
          <TooltipContent>{connected ? "实时行情已连接" : "实时行情重连中"}</TooltipContent>
        </Tooltip>
      </TooltipProvider>
      {indices.slice(0, 6).map((idx) => (
        <span key={idx.code} className="flex shrink-0 items-baseline gap-1.5 text-xs">
          <span className="text-fg-3">{idx.name}</span>
          <span className="tabular-nums text-fg">
            {idx.price != null
              ? idx.price.toLocaleString("zh-CN", { maximumFractionDigits: 2 })
              : "—"}
          </span>
          {idx.change_pct != null && (
            <span
              className={cn(
                "tabular-nums",
                idx.change_pct > 0 ? "text-up" : idx.change_pct < 0 ? "text-down" : "text-fg-3",
              )}
            >
              {idx.change_pct > 0 ? "+" : ""}
              {idx.change_pct.toFixed(2)}%
            </span>
          )}
        </span>
      ))}
    </div>
  );
}

const THEME_CYCLE: Record<ThemeMode, ThemeMode> = {
  dark: "light",
  light: "system",
  system: "dark",
};
const THEME_LABEL: Record<ThemeMode, string> = { dark: "暗色", light: "亮色", system: "跟随系统" };

export function TopBar({ title }: { title?: string }) {
  const router = useRouter();
  const theme = useSettings((s) => s.theme);
  const setTheme = useSettings((s) => s.setTheme);
  const setPaletteOpen = useUi((s) => s.setPaletteOpen);

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: async () => {
      await refreshAuthStatus(queryClient);
      await router.navigate({ to: "/login" });
    },
    onError: (e) =>
      toast.error("退出登录失败", { description: e instanceof ApiError ? e.code : String(e) }),
  });

  const ThemeIcon = theme === "dark" ? Moon : theme === "light" ? Sun : SunMoon;

  return (
    <header className="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-border bg-surface-1/85 px-4 backdrop-blur">
      {title && <h1 className="shrink-0 text-sm font-medium text-fg">{title}</h1>}

      <MarketTicker />

      <div className="ml-auto flex shrink-0 items-center gap-1.5">
        <FreshnessBadge />
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setPaletteOpen(true)}
          className="hidden gap-2 text-fg-3 sm:flex"
          aria-label="打开命令面板"
        >
          <Search className="size-4" />
          <kbd className="rounded border border-border bg-surface-2 px-1.5 text-[10px] text-fg-3">
            ⌘K
          </kbd>
        </Button>
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setTheme(THEME_CYCLE[theme])}
                aria-label={`主题：${THEME_LABEL[theme]}`}
              >
                <ThemeIcon className="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>主题：{THEME_LABEL[theme]}</TooltipContent>
          </Tooltip>
        </TooltipProvider>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label="账户菜单">
              <span className="grid size-6 place-items-center rounded-full bg-accent text-xs font-medium text-accent-fg">
                我
              </span>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>单租户账户</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => router.navigate({ to: "/settings" })}>
              设置
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={() => logoutMutation.mutate()}
              className="text-down focus:text-down"
            >
              <LogOut className="mr-2 size-3.5" />
              退出登录
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
