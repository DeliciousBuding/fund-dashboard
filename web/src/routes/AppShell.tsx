import { useMutation } from "@tanstack/react-query";
import { Outlet, useRouter } from "@tanstack/react-router";
import { logout } from "../lib/auth";
import { queryClient } from "../lib/queryClient";

// AppShell：受保护区域的布局壳（侧栏导航 + 顶栏）。W2 设计系统落地时升级为完整
// 侧栏（市场分组/搜索/组合切换），当前先保证可用与顺眼。
export function AppShell() {
  const router = useRouter();
  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["auth-status"] });
      await router.navigate({ to: "/login" });
    },
  });

  return (
    <div className="flex min-h-screen">
      <aside className="hidden w-60 flex-col border-r border-border bg-surface-1 md:flex">
        <div className="flex items-center gap-2 px-5 py-5">
          <span className="grid size-7 place-items-center rounded-lg bg-surface-2 text-sm text-accent">
            ◈
          </span>
          <span className="font-medium text-fg">持仓中枢</span>
        </div>
        <nav className="flex-1 space-y-1 px-3 py-2 text-sm">
          <span className="block rounded-lg bg-surface-3 px-3 py-2 font-medium text-fg">总览</span>
          {["持仓", "交易", "分析", "定投", "市场", "信号", "报告", "设置"].map((label) => (
            <span
              key={label}
              className="block cursor-not-allowed rounded-lg px-3 py-2 text-fg-3"
              title="即将到来（见 docs/design/05 路线图）"
            >
              {label}
              <span className="float-right text-xs">W3+</span>
            </span>
          ))}
        </nav>
        <div className="border-t border-border p-3">
          <button
            type="button"
            onClick={() => logoutMutation.mutate()}
            className="w-full rounded-lg px-3 py-2 text-left text-sm text-fg-2 hover:bg-surface-3 hover:text-fg"
          >
            退出登录
          </button>
        </div>
      </aside>
      <div className="min-w-0 flex-1">
        <Outlet />
      </div>
    </div>
  );
}
