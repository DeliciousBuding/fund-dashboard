// AppShell：受保护区布局壳 —— 侧栏 + 顶栏 + 移动底栏 + 命令面板 + 页面过渡动效。
// 设计规格：03 §4（240px 侧栏/max-w-1440/卡片 padding）、§5（页面进入 160ms fade+y）、§9（移动断点）。
import { Outlet, useRouterState } from "@tanstack/react-router";
import { AnimatePresence, motion } from "motion/react";
import { BottomNav } from "../components/shell/BottomNav";
import { CommandPalette } from "../components/shell/CommandPalette";
import { Sidebar } from "../components/shell/Sidebar";
import { TopBar } from "../components/shell/TopBar";
import { ErrorBoundary } from "../components/ui/error-boundary";
import { useGlobalHotkeys } from "../hooks/useGlobalHotkeys";

const TITLE_BY_PATH: Record<string, string> = {
  "/": "总览",
  "/holdings": "持仓",
  "/transactions": "交易",
  "/dca": "定投",
  "/analysis": "分析",
  "/market": "市场",
  "/insights": "信号",
  "/reports": "报告",
  "/system": "工作台",
  "/system/audit": "审计",
  "/settings": "设置",
};

function pageTitle(pathname: string): string {
  if (pathname in TITLE_BY_PATH) return TITLE_BY_PATH[pathname];
  if (pathname.startsWith("/holdings/")) return "标的详情";
  return "";
}

export function AppShell() {
  useGlobalHotkeys();
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  return (
    <div className="flex min-h-screen bg-bg text-fg">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar title={pageTitle(pathname)} />
        <main className="mx-auto w-full max-w-[1440px] flex-1 px-4 pb-20 pt-4 md:px-6 md:pb-8">
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key={pathname}
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.16, ease: [0.2, 0.8, 0.2, 1] }}
            >
              {/* 面板级错误边界：崩一页不崩全壳（03 §7） */}
              <ErrorBoundary>
                <Outlet />
              </ErrorBoundary>
            </motion.div>
          </AnimatePresence>
        </main>
        <BottomNav />
      </div>
      <CommandPalette />
    </div>
  );
}
