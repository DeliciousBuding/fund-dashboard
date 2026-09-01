// 定投 /dca —— 计划管理（新建/编辑/停用）+ 手动执行（dry-run 预览 → 确认执行）。
// 写路径：POST /api/dca/plans、/disable、/run（session + CSRF）。

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CalendarClock, CircleStop, Pencil, Play, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../components/ui/dialog";
import { EmptyState } from "../components/ui/empty-state";
import { Input, Label } from "../components/ui/input";
import { Skeleton } from "../components/ui/skeleton";
import { Switch } from "../components/ui/switch";
import { Table, TBody, Td, THead, Th, Tr } from "../components/ui/table";
import { ApiError, api } from "../lib/api";
import { fmtCNY } from "../lib/format";
import { useDcaPlans } from "../lib/queries";
import { useUi } from "../stores/ui";

const WEEKDAY_LABELS = ["一", "二", "三", "四", "五", "六", "日"];

function maskLabel(mask: string): string {
  const days = mask
    .split(",")
    .map((s) => Number(s.trim()))
    .filter((n) => n >= 1 && n <= 7);
  if (days.length === 0) return "—";
  if (days.join() === "1,2,3,4,5") return "工作日";
  return `周${days.map((d) => WEEKDAY_LABELS[d - 1]).join("·")}`;
}

// ── 计划表单 ────────────────────────────────────────────────────────

interface PlanForm {
  id: number | null;
  fund_code: string;
  fund_name: string;
  amount: string;
  weekday_mask: string[];
  start_date: string;
}

function PlanFormDialog(props: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  initial: PlanForm;
}) {
  const [form, setForm] = useState(props.initial);
  const [error, setError] = useState<string | null>(null);
  const queryClient = useQueryClient();
  const [prevOpen, setPrevOpen] = useState(false);
  if (props.open && !prevOpen) {
    setForm(props.initial);
    setError(null);
    setPrevOpen(true);
  } else if (!props.open && prevOpen) {
    setPrevOpen(false);
  }

  const save = useMutation({
    mutationFn: async () => {
      const amount = Number(form.amount);
      if (!form.fund_code.trim()) throw new Error("请填写标的代码");
      if (!Number.isFinite(amount) || amount <= 0) throw new Error("金额必须为正数");
      if (form.weekday_mask.length === 0) throw new Error("至少选一个扣款日");
      await api<{ ok: boolean }>("/api/dca/plans", {
        method: "POST",
        body: {
          id: form.id ?? 0,
          fund_code: form.fund_code.trim(),
          fund_name: form.fund_name.trim(),
          amount,
          frequency: "weekday",
          weekday_mask: [...form.weekday_mask].sort().join(","),
          start_date: form.start_date,
        },
      });
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["dca-plans"] });
      toast.success(form.id == null ? "定投计划已创建" : "定投计划已更新");
      props.onOpenChange(false);
    },
    onError: (e) =>
      setError(e instanceof ApiError ? e.code : e instanceof Error ? e.message : "保存失败"),
  });

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{form.id == null ? "新建定投计划" : "编辑定投计划"}</DialogTitle>
          <DialogDescription>每个选中的工作日按金额自动买入。</DialogDescription>
        </DialogHeader>
        <form
          className="grid grid-cols-2 gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            save.mutate();
          }}
        >
          <div>
            <Label htmlFor="dca-code">标的代码</Label>
            <Input
              id="dca-code"
              value={form.fund_code}
              onChange={(e) => setForm({ ...form, fund_code: e.target.value })}
              required
            />
          </div>
          <div>
            <Label htmlFor="dca-name">名称（可选）</Label>
            <Input
              id="dca-name"
              value={form.fund_name}
              onChange={(e) => setForm({ ...form, fund_name: e.target.value })}
            />
          </div>
          <div>
            <Label htmlFor="dca-amount">每次金额（元）</Label>
            <Input
              id="dca-amount"
              inputMode="decimal"
              value={form.amount}
              onChange={(e) => setForm({ ...form, amount: e.target.value })}
              required
            />
          </div>
          <div>
            <Label htmlFor="dca-start">开始日期</Label>
            <Input
              id="dca-start"
              type="date"
              value={form.start_date}
              onChange={(e) => setForm({ ...form, start_date: e.target.value })}
              required
            />
          </div>
          <div className="col-span-2">
            <Label>扣款日</Label>
            <div className="flex flex-wrap gap-1.5">
              {WEEKDAY_LABELS.map((label, i) => {
                const day = String(i + 1);
                const on = form.weekday_mask.includes(day);
                return (
                  <button
                    key={day}
                    type="button"
                    aria-pressed={on}
                    onClick={() =>
                      setForm({
                        ...form,
                        weekday_mask: on
                          ? form.weekday_mask.filter((d) => d !== day)
                          : [...form.weekday_mask, day],
                      })
                    }
                    className={`rounded-lg border px-3 py-1.5 text-sm transition-colors ${
                      on
                        ? "border-accent bg-accent/15 text-accent"
                        : "border-border text-fg-3 hover:text-fg"
                    }`}
                  >
                    周{label}
                  </button>
                );
              })}
            </div>
          </div>
          {error && <p className="col-span-2 text-sm text-danger">{error}</p>}
          <DialogFooter className="col-span-2">
            <Button type="button" variant="ghost" onClick={() => props.onOpenChange(false)}>
              取消
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? "保存中…" : "保存"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// ── 手动执行（dry-run 预览 → 确认）───────────────────────────────────

interface DcaRunItem {
  plan_id: number;
  fund_code: string;
  fund_name?: string;
  amount: number;
  status: string;
  message?: string;
}
interface DcaRunResult {
  ok: boolean;
  dry_run: boolean;
  executed: number;
  skipped: number;
  previewed: number;
  items: DcaRunItem[];
}

function RunDialog(props: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const [preview, setPreview] = useState<DcaRunResult | null>(null);
  const queryClient = useQueryClient();

  const run = useMutation({
    mutationFn: (dryRun: boolean) =>
      api<DcaRunResult>("/api/dca/run", { method: "POST", body: { dry_run: dryRun } }),
    onSuccess: async (result, dryRun) => {
      if (dryRun) {
        setPreview(result);
      } else {
        toast.success(`已执行 ${result.executed} 笔定投`);
        await queryClient.invalidateQueries({ queryKey: ["transactions"] });
        await queryClient.invalidateQueries({ queryKey: ["dca-plans"] });
        props.onOpenChange(false);
        setPreview(null);
      }
    },
    onError: (e) =>
      toast.error("执行失败", { description: e instanceof ApiError ? e.code : String(e) }),
  });

  return (
    <Dialog
      open={props.open}
      onOpenChange={(v) => {
        if (!v) setPreview(null);
        props.onOpenChange(v);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>手动执行定投</DialogTitle>
          <DialogDescription>先预览今日到期计划，确认后才会真实写入台账。</DialogDescription>
        </DialogHeader>
        {preview == null ? (
          <DialogFooter>
            <Button variant="ghost" onClick={() => props.onOpenChange(false)}>
              取消
            </Button>
            <Button onClick={() => run.mutate(true)} disabled={run.isPending}>
              {run.isPending ? "计算中…" : "预览今日到期"}
            </Button>
          </DialogFooter>
        ) : (
          <div className="space-y-3">
            {preview.items.length === 0 ? (
              <p className="py-4 text-center text-sm text-fg-3">今日没有到期的定投计划。</p>
            ) : (
              <div className="max-h-64 overflow-y-auto rounded-lg border border-border">
                <Table>
                  <THead>
                    <Tr>
                      <Th>标的</Th>
                      <Th className="text-right">金额</Th>
                      <Th>状态</Th>
                    </Tr>
                  </THead>
                  <TBody>
                    {preview.items.map((item) => (
                      <Tr key={`${item.plan_id}-${item.fund_code}`}>
                        <Td>
                          <span className="text-fg">{item.fund_name || item.fund_code}</span>
                          <span className="ml-1.5 font-mono text-[11px] text-fg-3">
                            {item.fund_code}
                          </span>
                        </Td>
                        <Td className="text-right tabular-nums">{fmtCNY(item.amount)}</Td>
                        <Td>
                          <Badge tone={item.status === "preview" ? "accent" : "neutral"}>
                            {item.status === "preview" ? "待执行" : (item.message ?? item.status)}
                          </Badge>
                        </Td>
                      </Tr>
                    ))}
                  </TBody>
                </Table>
              </div>
            )}
            <DialogFooter>
              <Button variant="ghost" onClick={() => setPreview(null)}>
                返回
              </Button>
              <Button
                onClick={() => run.mutate(false)}
                disabled={run.isPending || preview.items.length === 0}
              >
                {run.isPending ? "执行中…" : `确认执行 ${preview.previewed} 笔`}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

const pad2 = (n: number) => String(n).padStart(2, "0");
const nowLocalDate = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
};

// ── 页面 ────────────────────────────────────────────────────────────

export function DcaPage() {
  const portfolioId = useUi((s) => s.portfolioId);
  const [activeOnly, setActiveOnly] = useState(false);
  const plans = useDcaPlans(activeOnly, portfolioId);
  const queryClient = useQueryClient();

  const [formOpen, setFormOpen] = useState(false);
  const [runOpen, setRunOpen] = useState(false);
  const [formInitial, setFormInitial] = useState<PlanForm>({
    id: null,
    fund_code: "",
    fund_name: "",
    amount: "100",
    weekday_mask: ["1", "2", "3", "4", "5"],
    start_date: nowLocalDate(),
  });

  const disable = useMutation({
    mutationFn: (id: number) => api(`/api/dca/plans/${id}/disable`, { method: "POST" }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["dca-plans"] });
      toast.success("计划已停用");
    },
    onError: (e) =>
      toast.error("停用失败", { description: e instanceof ApiError ? e.code : String(e) }),
  });

  const rows = useMemo(() => plans.data?.plans ?? [], [plans.data]);
  const weeklyTotal = rows
    .filter((p) => p.active === 1)
    .reduce((sum, p) => sum + p.amount * p.weekday_mask.split(",").filter(Boolean).length, 0);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <Button
          size="sm"
          onClick={() => {
            setFormInitial({
              id: null,
              fund_code: "",
              fund_name: "",
              amount: "100",
              weekday_mask: ["1", "2", "3", "4", "5"],
              start_date: nowLocalDate(),
            });
            setFormOpen(true);
          }}
        >
          <Plus className="size-4" />
          新建计划
        </Button>
        <Button variant="outline" size="sm" onClick={() => setRunOpen(true)}>
          <Play className="size-4" />
          手动执行
        </Button>
        <div className="ml-auto flex items-center gap-2 text-xs text-fg-3">
          <span id="dca-active-label">仅看启用</span>
          <Switch
            checked={activeOnly}
            onCheckedChange={setActiveOnly}
            aria-labelledby="dca-active-label"
          />
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <CalendarClock className="size-4 text-accent" />
            启用中 {rows.filter((p) => p.active === 1).length} 个计划 · 预计每周投入{" "}
            <span className="tabular-nums text-accent">{fmtCNY(weeklyTotal)}</span>
          </CardTitle>
        </CardHeader>
        <CardContent style={{ padding: 0 }}>
          {plans.isPending ? (
            <div className="space-y-2 p-4">
              {["p1", "p2", "p3"].map((k) => (
                <Skeleton key={k} className="h-10 w-full" />
              ))}
            </div>
          ) : plans.isError ? (
            <EmptyState
              title="加载失败"
              description="定投计划拉取失败，请重试。"
              action={
                <Button size="sm" onClick={() => void plans.refetch()}>
                  重试
                </Button>
              }
            />
          ) : rows.length === 0 ? (
            <EmptyState
              title="还没有定投计划"
              description="创建计划后，系统在每个选中的工作日自动按金额买入。"
              action={
                <Button size="sm" onClick={() => setFormOpen(true)}>
                  新建计划
                </Button>
              }
            />
          ) : (
            <Table>
              <THead>
                <Tr>
                  <Th>标的</Th>
                  <Th className="text-right">每次金额</Th>
                  <Th>扣款日</Th>
                  <Th>开始日期</Th>
                  <Th>状态</Th>
                  <Th className="text-right">操作</Th>
                </Tr>
              </THead>
              <TBody>
                {rows.map((p) => (
                  <Tr key={p.id} className={p.active === 1 ? undefined : "opacity-50"}>
                    <Td>
                      <div className="text-fg">{p.fund_name || "—"}</div>
                      <div className="font-mono text-[11px] text-fg-3">{p.fund_code}</div>
                    </Td>
                    <Td className="text-right tabular-nums">{fmtCNY(p.amount)}</Td>
                    <Td className="text-fg-2">{maskLabel(p.weekday_mask)}</Td>
                    <Td className="tabular-nums text-fg-2">{p.start_date}</Td>
                    <Td>
                      <Badge tone={p.active === 1 ? "up" : "neutral"}>
                        {p.active === 1 ? "启用" : "已停用"}
                      </Badge>
                    </Td>
                    <Td>
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label="编辑"
                          onClick={() => {
                            setFormInitial({
                              id: p.id,
                              fund_code: p.fund_code,
                              fund_name: p.fund_name ?? "",
                              amount: String(p.amount),
                              weekday_mask: p.weekday_mask.split(",").filter(Boolean),
                              start_date: p.start_date,
                            });
                            setFormOpen(true);
                          }}
                        >
                          <Pencil className="size-3.5" />
                        </Button>
                        {p.active === 1 && (
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="停用"
                            onClick={() => disable.mutate(p.id)}
                            disabled={disable.isPending}
                          >
                            <CircleStop className="size-3.5 text-down" />
                          </Button>
                        )}
                      </div>
                    </Td>
                  </Tr>
                ))}
              </TBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <PlanFormDialog open={formOpen} onOpenChange={setFormOpen} initial={formInitial} />
      <RunDialog open={runOpen} onOpenChange={setRunOpen} />
    </div>
  );
}
