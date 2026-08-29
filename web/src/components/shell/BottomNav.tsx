// 移动端底部导航（03 §9）：总览/持仓/交易/我的，safe-area 适配，点击目标 ≥ 28px。
import { Link } from "@tanstack/react-router";
import { ChartPie, LayoutDashboard, ListOrdered, User } from "lucide-react";
import { cn } from "../../lib/utils";

const TABS = [
  { to: "/", label: "总览", icon: LayoutDashboard },
  { to: "/holdings", label: "持仓", icon: ChartPie },
  { to: "/transactions", label: "交易", icon: ListOrdered },
  { to: "/settings", label: "我的", icon: User },
] as const;

export function BottomNav() {
  return (
    <nav
      className="fixed inset-x-0 bottom-0 z-40 grid grid-cols-4 border-t border-border bg-surface-1/95 backdrop-blur md:hidden"
      style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
    >
      {TABS.map(({ to, label, icon: Icon }) => (
        <Link
          key={to}
          to={to}
          activeOptions={{ exact: to === "/" }}
          className={cn(
            "flex min-h-12 flex-col items-center justify-center gap-0.5 py-1.5 text-[11px] text-fg-3",
          )}
          activeProps={{ className: "text-accent" }}
        >
          <Icon className="size-5" />
          {label}
        </Link>
      ))}
    </nav>
  );
}
