// /_design — 组件目录页（dev-only，03 §10）：全组件 × 全状态一页看全，
// 是 UI 评审与回归的目视基线。生产构建保留路由但侧边栏不露出。
import { useState } from "react";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "../components/ui/dialog";
import { EmptyState } from "../components/ui/empty-state";
import { Input } from "../components/ui/input";
import { Segmented } from "../components/ui/segmented";
import { Skeleton } from "../components/ui/skeleton";
import { Switch } from "../components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/tabs";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../components/ui/tooltip";

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-3">
      <h2 className="text-sm font-medium text-fg-2">{title}</h2>
      <div className="rounded-xl border border-border bg-surface-1 p-5">{children}</div>
    </section>
  );
}

export function DesignPage() {
  const [switchOn, setSwitchOn] = useState(true);
  const [seg, setSeg] = useState("a");

  return (
    <div className="space-y-8 pb-16">
      <header>
        <h1 className="text-xl font-semibold text-fg">设计系统目录</h1>
        <p className="mt-1 text-sm text-fg-3">
          「静水流深」全部原语与状态。改 token 后以此页做目视回归。
        </p>
      </header>

      <Section title="按钮 Button">
        <div className="flex flex-wrap items-center gap-3">
          <Button>主要</Button>
          <Button variant="secondary">次要</Button>
          <Button variant="outline">描边</Button>
          <Button variant="ghost">幽灵</Button>
          <Button variant="danger">危险</Button>
          <Button disabled>禁用</Button>
          <Button size="sm">小号</Button>
          <Button size="lg">大号</Button>
        </div>
      </Section>

      <Section title="徽章 Badge">
        <div className="flex flex-wrap gap-2">
          <Badge>neutral</Badge>
          <Badge tone="up">+12.4%</Badge>
          <Badge tone="down">-3.2%</Badge>
          <Badge tone="accent">accent</Badge>
          <Badge tone="warn">warn</Badge>
          <Badge tone="danger">danger</Badge>
          <Badge tone="info">info</Badge>
        </div>
      </Section>

      <Section title="输入 Input">
        <div className="grid max-w-md gap-3">
          <Input placeholder="默认占位…" />
          <Input defaultValue="已填充" />
          <Input disabled placeholder="禁用" />
        </div>
      </Section>

      <Section title="开关 / 分段 Switch · Segmented">
        <div className="flex flex-wrap items-center gap-6">
          <Switch checked={switchOn} onCheckedChange={setSwitchOn} aria-label="示例开关" />
          <Segmented
            value={seg}
            onChange={setSeg}
            options={[
              { value: "a", label: "近一月" },
              { value: "b", label: "近一年" },
              { value: "c", label: "全部" },
            ]}
          />
        </div>
      </Section>

      <Section title="卡片 Card">
        <div className="grid gap-4 sm:grid-cols-3">
          {["总资产", "累计盈亏", "XIRR"].map((t) => (
            <Card key={t}>
              <CardHeader>
                <CardTitle className="text-xs font-normal text-fg-3">{t}</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-2xl font-medium tabular-nums text-fg">¥1,234,567</p>
                <p className="mt-1 text-xs text-up">+2.34%</p>
              </CardContent>
            </Card>
          ))}
        </div>
      </Section>

      <Section title="骨架 Skeleton（loading 态）">
        <div className="space-y-2">
          <Skeleton className="h-8 w-1/3" />
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-8 w-2/3" />
        </div>
      </Section>

      <Section title="空态 EmptyState">
        <EmptyState
          title="还没有交易记录"
          description="导入交易或添加一条记录后，这里会出现台账。"
          action={<Button size="sm">导入交易</Button>}
        />
      </Section>

      <Section title="弹层 Dialog / Tooltip / Tabs">
        <div className="flex flex-wrap items-center gap-4">
          <Dialog>
            <DialogTrigger asChild>
              <Button variant="outline">打开对话框</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>确认操作</DialogTitle>
                <DialogDescription>这是对话框的描述文字，说明操作后果。</DialogDescription>
              </DialogHeader>
            </DialogContent>
          </Dialog>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost">悬停提示</Button>
              </TooltipTrigger>
              <TooltipContent>提示内容</TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
        <Tabs defaultValue="a" className="mt-4">
          <TabsList>
            <TabsTrigger value="a">走势</TabsTrigger>
            <TabsTrigger value="b">持仓</TabsTrigger>
            <TabsTrigger value="c">交易</TabsTrigger>
          </TabsList>
          <TabsContent value="a" className="pt-3 text-sm text-fg-2">
            走势面板
          </TabsContent>
          <TabsContent value="b" className="pt-3 text-sm text-fg-2">
            持仓面板
          </TabsContent>
          <TabsContent value="c" className="pt-3 text-sm text-fg-2">
            交易面板
          </TabsContent>
        </Tabs>
      </Section>

      <Section title="色彩语义（随约定轴切换）">
        <div className="grid grid-cols-2 gap-2 text-xs sm:grid-cols-4">
          {[
            ["bg-up", "涨 up"],
            ["bg-down", "跌 down"],
            ["bg-accent", "accent"],
            ["bg-warn", "warn"],
            ["bg-danger", "danger"],
            ["bg-info", "info"],
            ["bg-surface-3", "surface-3"],
            ["bg-border", "border"],
          ].map(([cls, label]) => (
            <div key={cls} className="flex items-center gap-2">
              <span className={`size-6 rounded-md ${cls}`} />
              <span className="text-fg-2">{label}</span>
            </div>
          ))}
        </div>
      </Section>
    </div>
  );
}
