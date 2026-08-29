// 侧栏：主导航 + 持仓速览（市场分组/搜索/仅看持有）+ 组合切换器。
// 设计规格：03 §4（240px/折叠 64px）、06 §3（系统分组）。
import { Link, useParams } from "@tanstack/react-router";
import {
  ChartPie,
  ChevronsLeft,
  ChevronsRight,
  FileText,
  Gauge,
  Landmark,
  LayoutDashboard,
  ListOrdered,
  Radio,
  Repeat,
  ScrollText,
  Search,
  Settings,
  Telescope,
  TrendingUp,
} from "lucide-react";
import { useMemo, useState } from "react";
import { usePortfolios, useSecurities } from "../../lib/queries";
import { cn } from "../../lib/utils";
import { useUi } from "../../stores/ui";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../ui/select";
import { Switch } from "../ui/switch";

interface NavItem {
  to: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
}

const MAIN_NAV: NavItem[] = [
  { to: "/", label: "总览", icon: LayoutDashboard },
  { to: "/holdings", label: "持仓", icon: ChartPie },
  { to: "/transactions", label: "交易", icon: ListOrdered },
  { to: "/dca", label: "定投", icon: Repeat },
  { to: "/analysis", label: "分析", icon: TrendingUp },
  { to: "/market", label: "市场", icon: Radio },
  { to: "/insights", label: "信号", icon: Telescope },
  { to: "/reports", label: "报告", icon: FileText },
];

const SYSTEM_NAV: NavItem[] = [
  { to: "/system", label: "工作台", icon: Gauge },
  { to: "/system/audit", label: "审计", icon: ScrollText },
  { to: "/settings", label: "设置", icon: Settings },
];

const MARKET_LABEL: Record<string, string> = {
  CN: "A 股 / 场内",
  US: "美股",
  HK: "港股",
};
const MARKET_ORDER = ["CN", "US", "HK"];

function marketLabel(market: string): string {
  return MARKET_LABEL[market] ?? (market ? market : "其他");
}

function NavLink({ item, collapsed }: { item: NavItem; collapsed: boolean }) {
  const Icon = item.icon;
  return (
    <Link
      to={item.to}
      title={collapsed ? item.label : undefined}
      className={cn(
        "flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm text-fg-2 transition-colors",
        "hover:bg-surface-3 hover:text-fg",
        collapsed && "justify-center px-0",
      )}
      activeProps={{ className: "bg-surface-3 text-fg font-medium" }}
      activeOptions={{ exact: item.to === "/" }}
    >
      <Icon className="size-4 shrink-0" />
      {!collapsed && <span className="truncate">{item.label}</span>}
    </Link>
  );
}

export function Sidebar() {
  const collapsed = useUi((s) => s.sidebarCollapsed);
  const toggleSidebar = useUi((s) => s.toggleSidebar);
  const portfolioId = useUi((s) => s.portfolioId);
  const setPortfolioId = useUi((s) => s.setPortfolioId);
  const heldOnly = useUi((s) => s.heldOnly);
  const setHeldOnly = useUi((s) => s.setHeldOnly);

  const portfolios = usePortfolios();
  const securities = useSecurities(portfolioId);
  const activeCode = useParams({ strict: false, select: (p) => (p as { code?: string }).code });

  const [query, setQuery] = useState("");

  const grouped = useMemo(() => {
    const list = (securities.data ?? []).filter((s) => {
      if (heldOnly && !(s.held_shares > 0)) return false;
      if (!query) return true;
      const q = query.toLowerCase();
      return s.code.toLowerCase().includes(q) || s.name.toLowerCase().includes(q);
    });
    const map = new Map<string, typeof list>();
    for (const s of list) {
      const key = s.market ?? "";
      const bucket = map.get(key) ?? [];
      bucket.push(s);
      map.set(key, bucket);
    }
    const keys = [...map.keys()].sort((a, b) => {
      const ia = MARKET_ORDER.indexOf(a);
      const ib = MARKET_ORDER.indexOf(b);
      return (ia === -1 ? 99 : ia) - (ib === -1 ? 99 : ib);
    });
    return keys.map((k) => ({ market: k, items: map.get(k) ?? [] }));
  }, [securities.data, heldOnly, query]);

  return (
    <aside
      className={cn(
        "hidden shrink-0 flex-col border-r border-border bg-surface-1 transition-[width] duration-200 md:flex",
        collapsed ? "w-16" : "w-60",
      )}
    >
      {/* 品牌 */}
      <div className={cn("flex items-center gap-2 px-5 py-5", collapsed && "justify-center px-0")}>
        <span className="grid size-7 shrink-0 place-items-center rounded-lg bg-surface-2 text-sm text-accent">
          ◈
        </span>
        {!collapsed && <span className="font-medium text-fg">持仓中枢</span>}
      </div>

      {/* 组合切换器（多组合时才显示选择器） */}
      {!collapsed && (portfolios.data?.length ?? 0) > 1 && (
        <div className="px-3 pb-2">
          <Select value={String(portfolioId)} onValueChange={(v) => setPortfolioId(Number(v))}>
            <SelectTrigger className="h-8 text-xs">
              <SelectValue placeholder="选择组合" />
            </SelectTrigger>
            <SelectContent>
              {(portfolios.data ?? []).map((p) => (
                <SelectItem key={p.id} value={String(p.id)}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      {/* 主导航 */}
      <nav className="space-y-0.5 px-3 py-1">
        {MAIN_NAV.map((item) => (
          <NavLink key={item.to} item={item} collapsed={collapsed} />
        ))}
      </nav>

      {/* 持仓速览 */}
      {!collapsed && (
        <div className="mt-3 flex min-h-0 flex-1 flex-col border-t border-border">
          <div className="space-y-2 px-3 pt-3">
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-fg-3" />
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="搜索标的…"
                className="h-8 w-full rounded-lg border border-border bg-surface-2 pl-8 pr-2 text-xs text-fg outline-none placeholder:text-fg-3 focus:border-accent"
              />
            </div>
            <div className="flex items-center justify-between px-1 text-xs text-fg-3">
              <span id="held-only-label">仅看持有</span>
              <Switch
                checked={heldOnly}
                onCheckedChange={setHeldOnly}
                aria-labelledby="held-only-label"
              />
            </div>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto px-3 py-2">
            {securities.isLoading && (
              <div className="space-y-1.5 px-1 pt-1">
                {["sk1", "sk2", "sk3", "sk4", "sk5"].map((k) => (
                  <div key={k} className="h-6 animate-pulse rounded bg-surface-3" />
                ))}
              </div>
            )}
            {!securities.isLoading && grouped.length === 0 && (
              <p className="px-1 pt-2 text-xs text-fg-3">{query ? "没有匹配的标的" : "暂无持仓"}</p>
            )}
            {grouped.map((g) => (
              <div key={g.market || "other"} className="pt-2">
                <div className="flex items-center gap-1.5 px-1 pb-1 text-[11px] font-medium tracking-wide text-fg-3">
                  <Landmark className="size-3" />
                  {marketLabel(g.market)}
                  <span className="ml-auto tabular-nums">{g.items.length}</span>
                </div>
                {g.items.map((s) => (
                  <Link
                    key={s.code}
                    to="/holdings/$code"
                    params={{ code: s.code }}
                    className={cn(
                      "flex items-center justify-between gap-2 rounded-md px-2 py-1.5 text-xs",
                      "text-fg-2 transition-colors hover:bg-surface-3 hover:text-fg",
                      activeCode === s.code && "bg-surface-3 text-fg",
                    )}
                  >
                    <span className="min-w-0 truncate">{s.name}</span>
                    {s.pnl_pct != null && (
                      <span
                        className={cn(
                          "shrink-0 tabular-nums",
                          s.pnl_pct > 0 ? "text-up" : s.pnl_pct < 0 ? "text-down" : "text-fg-3",
                        )}
                      >
                        {s.pnl_pct > 0 ? "+" : ""}
                        {s.pnl_pct.toFixed(1)}%
                      </span>
                    )}
                  </Link>
                ))}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 系统分组 */}
      <nav className="space-y-0.5 border-t border-border px-3 py-2">
        {SYSTEM_NAV.map((item) => (
          <NavLink key={item.to} item={item} collapsed={collapsed} />
        ))}
      </nav>

      {/* 折叠开关 */}
      <div className="border-t border-border p-2">
        <button
          type="button"
          onClick={toggleSidebar}
          aria-label={collapsed ? "展开侧栏" : "折叠侧栏"}
          className="flex w-full items-center justify-center rounded-lg px-3 py-2 text-fg-3 transition-colors hover:bg-surface-3 hover:text-fg"
        >
          {collapsed ? <ChevronsRight className="size-4" /> : <ChevronsLeft className="size-4" />}
        </button>
      </div>
    </aside>
  );
}
