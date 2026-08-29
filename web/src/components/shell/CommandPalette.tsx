// ⌘K 命令面板（03 §8）：跳页面 / 跳标的（名称·代码模糊搜）/ 动作。
// cmdk 提供过滤与键盘导航；标的走 /api/securities 缓存（React Query 已有）。

import { useRouter } from "@tanstack/react-router";
import { Command } from "cmdk";
import {
  ChartPie,
  FileText,
  Gauge,
  LayoutDashboard,
  ListOrdered,
  Radio,
  Repeat,
  ScrollText,
  Settings,
  Telescope,
  TrendingUp,
  Wallet,
} from "lucide-react";
import { useSecurities } from "../../lib/queries";
import { useUi } from "../../stores/ui";
import { Dialog, DialogContent } from "../ui/dialog";

const PAGES = [
  { to: "/", label: "总览", icon: LayoutDashboard, keywords: "overview home" },
  { to: "/holdings", label: "持仓", icon: ChartPie, keywords: "holdings positions" },
  { to: "/transactions", label: "交易", icon: ListOrdered, keywords: "transactions ledger" },
  { to: "/dca", label: "定投", icon: Repeat, keywords: "dca autoinvest" },
  { to: "/analysis", label: "分析", icon: TrendingUp, keywords: "analysis compare backtest" },
  { to: "/market", label: "市场", icon: Radio, keywords: "market indices" },
  { to: "/insights", label: "信号", icon: Telescope, keywords: "insights events signals" },
  { to: "/reports", label: "报告", icon: FileText, keywords: "reports" },
  { to: "/system", label: "工作台", icon: Gauge, keywords: "system workbench ops" },
  { to: "/system/audit", label: "审计", icon: ScrollText, keywords: "audit events log" },
  { to: "/settings", label: "设置", icon: Settings, keywords: "settings preferences security" },
] as const;

export function CommandPalette() {
  const open = useUi((s) => s.paletteOpen);
  const setOpen = useUi((s) => s.setPaletteOpen);
  const portfolioId = useUi((s) => s.portfolioId);
  const securities = useSecurities(portfolioId);
  const router = useRouter();

  const go = (to: string) => {
    setOpen(false);
    void router.navigate({ to });
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="top-[18%] max-w-lg translate-y-0 p-0" aria-label="命令面板">
        <Command
          label="命令面板"
          className="flex flex-col"
          filter={(value, search, keywords) => {
            const hay = `${value} ${(keywords ?? []).join(" ")}`.toLowerCase();
            return hay.includes(search.toLowerCase()) ? 1 : 0;
          }}
        >
          <div className="border-b border-border px-3">
            <Command.Input
              autoFocus
              placeholder="跳页面、搜标的、找动作…"
              className="h-11 w-full bg-transparent text-sm text-fg outline-none placeholder:text-fg-3"
            />
          </div>
          <Command.List className="max-h-80 overflow-y-auto p-2">
            <Command.Empty className="px-3 py-6 text-center text-sm text-fg-3">
              没有匹配项
            </Command.Empty>

            <Command.Group
              heading="页面"
              className="text-xs text-fg-3 [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5"
            >
              {PAGES.map(({ to, label, icon: Icon, keywords }) => (
                <Command.Item
                  key={to}
                  value={label}
                  keywords={[...keywords, to]}
                  onSelect={() => go(to)}
                  className="flex cursor-pointer items-center gap-2.5 rounded-lg px-2 py-2 text-sm text-fg-2 aria-selected:bg-surface-3 aria-selected:text-fg"
                >
                  <Icon className="size-4 shrink-0" />
                  {label}
                </Command.Item>
              ))}
            </Command.Group>

            <Command.Group
              heading="标的"
              className="text-xs text-fg-3 [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5"
            >
              {(securities.data ?? []).map((s) => (
                <Command.Item
                  key={s.code}
                  value={`${s.name} ${s.code}`}
                  keywords={[s.market ?? "", s.security_type ?? ""]}
                  onSelect={() => go(`/holdings/${s.code}`)}
                  className="flex cursor-pointer items-center gap-2.5 rounded-lg px-2 py-2 text-sm text-fg-2 aria-selected:bg-surface-3 aria-selected:text-fg"
                >
                  <Wallet className="size-4 shrink-0" />
                  <span className="min-w-0 flex-1 truncate">{s.name}</span>
                  <span className="shrink-0 font-mono text-xs text-fg-3">{s.code}</span>
                </Command.Item>
              ))}
            </Command.Group>
          </Command.List>
        </Command>
      </DialogContent>
    </Dialog>
  );
}
