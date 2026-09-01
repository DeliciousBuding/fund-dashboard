// 分析 /analysis —— 四 tab（01 文档 §4）：对比（归一化净值+指标表+雷达）/
// 回测（定投 vs 一次性，继承旧 DcaBacktestChart 语义）/ 高级（相关性热力图+蒙特卡洛扇形，
// 纯函数客户端计算）/ 穿透（底层股票暴露 treemap + 行业聚合）。
// 深链：?tab=compare|backtest|advanced|penetration。

import { useNavigate, useSearch } from "@tanstack/react-router";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs";
import { AdvancedTab } from "./analysis/AdvancedTab";
import { BacktestTab } from "./analysis/BacktestTab";
import { CompareTab } from "./analysis/CompareTab";
import { PenetrationTab } from "./analysis/PenetrationTab";

const TABS = [
  { value: "compare", label: "对比" },
  { value: "backtest", label: "回测" },
  { value: "advanced", label: "高级" },
  { value: "penetration", label: "穿透" },
] as const;

export function AnalysisPage() {
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as { tab?: string };
  const tab = TABS.some((t) => t.value === search.tab) ? (search.tab as string) : "compare";

  return (
    <Tabs
      value={tab}
      onValueChange={(v) => {
        void navigate({ to: ".", search: { tab: v }, replace: true });
      }}
    >
      <TabsList>
        {TABS.map((t) => (
          <TabsTrigger key={t.value} value={t.value}>
            {t.label}
          </TabsTrigger>
        ))}
      </TabsList>
      <TabsContent value="compare" className="pt-4">
        <CompareTab />
      </TabsContent>
      <TabsContent value="backtest" className="pt-4">
        <BacktestTab />
      </TabsContent>
      <TabsContent value="advanced" className="pt-4">
        <AdvancedTab />
      </TabsContent>
      <TabsContent value="penetration" className="pt-4">
        <PenetrationTab />
      </TabsContent>
    </Tabs>
  );
}
