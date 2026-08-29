// 交易 /transactions —— 全量台账：过滤（方向/标的/全文）+ 服务端分页 + 排序 +
// 新增/编辑/删除（二次确认）+ 导出（CSV 中文 BOM / XLSX）。
// 写路径走 session + X-Fund-Request（api client 内置），变更后 invalidate 相关查询。

import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  type SortingState,
  useReactTable,
} from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ArrowUpDown, Download, Pencil, Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
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
import { Segmented } from "../components/ui/segmented";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select";
import { Skeleton } from "../components/ui/skeleton";
import { Table, TBody, Td, THead, Th, Tr } from "../components/ui/table";
import { ApiError, api } from "../lib/api";
import { downloadText, transactionsToCsv } from "../lib/csv";
import { fmtCNY } from "../lib/format";
import { type TransactionListItem, useSecurities, useTransactions } from "../lib/queries";
import { cn } from "../lib/utils";
import { useUi } from "../stores/ui";

const PAGE_SIZE = 100;

const DIRECTIONS = [
  { value: "", label: "全部方向" },
  { value: "buy", label: "买入" },
  { value: "sell", label: "卖出" },
  { value: "dividend", label: "分红" },
];

const DIRECTION_BADGE: Record<string, "up" | "down" | "accent" | "neutral"> = {
  buy: "up",
  sell: "down",
  dividend: "accent",
};
const DIRECTION_LABEL: Record<string, string> = {
  buy: "买入",
  sell: "卖出",
  dividend: "分红",
  convert_in: "转换转入",
  convert_out: "转换转出",
  forced_redeem: "强制赎回",
};

// ── 新增/编辑表单 ───────────────────────────────────────────────────

interface TxFormState {
  seq: number | null; // null = 新增
  fund_code: string;
  trade_time: string;
  confirm_date: string;
  direction: string;
  trade_type: string;
  amount: string;
  shares: string;
  fee: string;
}

const EMPTY_FORM: TxFormState = {
  seq: null,
  fund_code: "",
  trade_time: new Date().toISOString().slice(0, 16),
  confirm_date: new Date().toISOString().slice(0, 10),
  direction: "buy",
  trade_type: "用户买入",
  amount: "",
  shares: "",
  fee: "0",
};

function TxFormDialog(props: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  initial: TxFormState;
  onSaved: () => void;
}) {
  const [form, setForm] = useState<TxFormState>(props.initial);
  const [error, setError] = useState<string | null>(null);
  const queryClient = useQueryClient();

  // 打开时同步 initial（复用同一 Dialog 实例）
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
      const shares = Number(form.shares);
      const fee = Number(form.fee) || 0;
      if (!form.fund_code.trim()) throw new Error("请填写标的代码");
      if (!Number.isFinite(amount) || amount <= 0) throw new Error("金额必须为正数");
      if (!Number.isFinite(shares) || shares < 0) throw new Error("份额不能为负");
      const signed = form.direction === "sell" ? -1 : 1;
      if (form.seq == null) {
        // 新增 = 单条导入（import 语义：order_id 缺省时后端生成）
        await api("/api/transactions/import", {
          method: "POST",
          body: {
            transactions: [
              {
                order_id: "",
                fund_code: form.fund_code.trim(),
                fund_name: "",
                trade_time: form.trade_time,
                confirm_date: form.confirm_date,
                trade_type: form.trade_type,
                direction: form.direction,
                confirm_amount: amount,
                confirm_share: shares,
                fee,
                signed_cash_flow: signed * amount,
                signed_share_change: signed * shares,
              },
            ],
          },
        });
      } else {
        await api(`/api/transactions/${form.seq}`, {
          method: "PUT",
          body: {
            trade_time: form.trade_time,
            confirm_date: form.confirm_date,
            direction: form.direction,
            trade_type: form.trade_type,
            confirm_amount: amount,
            confirm_share: shares,
            fee,
          },
        });
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["transactions"] });
      await queryClient.invalidateQueries({ queryKey: ["securities"] });
      toast.success(form.seq == null ? "交易已新增" : "交易已更新");
      props.onSaved();
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.code : e instanceof Error ? e.message : "保存失败");
    },
  });

  const isBuy = form.direction === "buy";
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{form.seq == null ? "新增交易" : `编辑交易 #${form.seq}`}</DialogTitle>
          <DialogDescription>
            {isBuy ? "买入记为负现金流、正份额变更。" : "卖出记为正现金流、负份额变更。"}
          </DialogDescription>
        </DialogHeader>
        <form
          className="grid grid-cols-2 gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            save.mutate();
          }}
        >
          <div className="col-span-2">
            <Label htmlFor="tx-code">标的代码</Label>
            <Input
              id="tx-code"
              value={form.fund_code}
              disabled={form.seq != null}
              onChange={(e) => setForm({ ...form, fund_code: e.target.value })}
              placeholder="如 019173 / AAPL"
              required
            />
          </div>
          <div>
            <Label htmlFor="tx-dir">方向</Label>
            <Select
              value={form.direction}
              onValueChange={(v) =>
                setForm({
                  ...form,
                  direction: v,
                  trade_type: v === "buy" ? "用户买入" : v === "sell" ? "用户卖出" : "分红",
                })
              }
            >
              <SelectTrigger id="tx-dir">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="buy">买入</SelectItem>
                <SelectItem value="sell">卖出</SelectItem>
                <SelectItem value="dividend">分红</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div>
            <Label htmlFor="tx-type">类型</Label>
            <Input
              id="tx-type"
              value={form.trade_type}
              onChange={(e) => setForm({ ...form, trade_type: e.target.value })}
            />
          </div>
          <div>
            <Label htmlFor="tx-time">交易时间</Label>
            <Input
              id="tx-time"
              type="datetime-local"
              value={form.trade_time}
              onChange={(e) => setForm({ ...form, trade_time: e.target.value })}
            />
          </div>
          <div>
            <Label htmlFor="tx-confirm">确认日期</Label>
            <Input
              id="tx-confirm"
              type="date"
              value={form.confirm_date}
              onChange={(e) => setForm({ ...form, confirm_date: e.target.value })}
            />
          </div>
          <div>
            <Label htmlFor="tx-amount">金额（元）</Label>
            <Input
              id="tx-amount"
              inputMode="decimal"
              value={form.amount}
              onChange={(e) => setForm({ ...form, amount: e.target.value })}
              required
            />
          </div>
          <div>
            <Label htmlFor="tx-shares">份额</Label>
            <Input
              id="tx-shares"
              inputMode="decimal"
              value={form.shares}
              onChange={(e) => setForm({ ...form, shares: e.target.value })}
              required
            />
          </div>
          <div className="col-span-2">
            <Label htmlFor="tx-fee">手续费（元）</Label>
            <Input
              id="tx-fee"
              inputMode="decimal"
              value={form.fee}
              onChange={(e) => setForm({ ...form, fee: e.target.value })}
            />
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

// ── 删除确认 ────────────────────────────────────────────────────────

function DeleteDialog(props: { seq: number | null; onOpenChange: (v: boolean) => void }) {
  const queryClient = useQueryClient();
  const del = useMutation({
    mutationFn: () => api(`/api/transactions/${props.seq}`, { method: "DELETE" }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["transactions"] });
      await queryClient.invalidateQueries({ queryKey: ["securities"] });
      toast.success("交易已删除");
      props.onOpenChange(false);
    },
    onError: (e) =>
      toast.error("删除失败", { description: e instanceof ApiError ? e.code : String(e) }),
  });
  return (
    <Dialog open={props.seq != null} onOpenChange={props.onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>删除交易 #{props.seq}</DialogTitle>
          <DialogDescription>删除后持仓快照会从台账重算。此操作不可撤销。</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={() => props.onOpenChange(false)}>
            取消
          </Button>
          <Button variant="danger" onClick={() => del.mutate()} disabled={del.isPending}>
            {del.isPending ? "删除中…" : "确认删除"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ── 页面 ────────────────────────────────────────────────────────────

const col = createColumnHelper<TransactionListItem>();

export function TransactionsPage() {
  const portfolioId = useUi((s) => s.portfolioId);
  const securities = useSecurities(portfolioId);
  const [direction, setDirection] = useState("");
  const [fundCode, setFundCode] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const [sorting, setSorting] = useState<SortingState>([]);

  const filter = useMemo(
    () => ({
      portfolioId,
      direction: direction || undefined,
      fundCode: fundCode || undefined,
      search: search || undefined,
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
    }),
    [portfolioId, direction, fundCode, search, page],
  );
  const data = useTransactions(filter);

  const [formOpen, setFormOpen] = useState(false);
  const [formInitial, setFormInitial] = useState<TxFormState>(EMPTY_FORM);
  const [deleteSeq, setDeleteSeq] = useState<number | null>(null);

  const exportCsv = useMutation({
    mutationFn: async () => {
      // 导出当前过滤条件的全量（上限 5000，后端硬顶）
      const params = new URLSearchParams();
      if (direction) params.set("direction", direction);
      if (fundCode) params.set("fund_code", fundCode);
      if (search) params.set("search", search);
      params.set("limit", "5000");
      const all = await api<{ transactions: TransactionListItem[] }>(`/api/transactions?${params}`);
      downloadText(
        `transactions-${new Date().toISOString().slice(0, 10)}.csv`,
        transactionsToCsv(all.transactions),
      );
    },
    onSuccess: () => toast.success("CSV 已导出"),
    onError: () => toast.error("导出失败"),
  });

  const exportXlsx = useMutation({
    mutationFn: async () => {
      const res = await fetch("/api/export/transactions-xlsx", { credentials: "same-origin" });
      if (!res.ok) throw new Error(`http_${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `transactions-${new Date().toISOString().slice(0, 10)}.xlsx`;
      a.click();
      URL.revokeObjectURL(url);
    },
    onSuccess: () => toast.success("XLSX 已导出"),
    onError: () => toast.error("XLSX 导出失败"),
  });

  const columns = useMemo(
    () => [
      col.accessor((r) => r.confirm_date ?? r.trade_time ?? "", {
        id: "date",
        header: "日期",
        cell: (c) => (
          <span className="tabular-nums text-fg-2">
            {(c.row.original.confirm_date ?? c.row.original.trade_time ?? "").slice(0, 10)}
          </span>
        ),
      }),
      col.accessor((r) => r.fund_name ?? r.fund_code, {
        id: "fund",
        header: "标的",
        cell: (c) => (
          <div className="min-w-0">
            <div className="truncate text-fg">{c.getValue()}</div>
            <div className="font-mono text-[11px] text-fg-3">{c.row.original.fund_code}</div>
          </div>
        ),
      }),
      col.accessor((r) => r.direction ?? "", {
        id: "direction",
        header: "方向",
        cell: (c) => (
          <Badge tone={DIRECTION_BADGE[c.getValue()] ?? "neutral"}>
            {DIRECTION_LABEL[c.getValue()] ?? c.getValue() ?? "—"}
          </Badge>
        ),
      }),
      col.accessor((r) => r.trade_type ?? "", {
        id: "trade_type",
        header: "类型",
        cell: (c) => <span className="text-fg-2">{c.getValue() || "—"}</span>,
      }),
      col.accessor((r) => r.amount ?? 0, {
        id: "amount",
        header: "金额",
        cell: (c) => (
          <span
            className={cn(
              "tabular-nums",
              c.row.original.direction === "sell" ? "text-down" : "text-up",
            )}
          >
            {c.row.original.direction === "sell" ? "+" : "-"}
            {fmtCNY(Math.abs(c.getValue() ?? 0))}
          </span>
        ),
      }),
      col.accessor((r) => r.shares ?? 0, {
        id: "shares",
        header: "份额",
        cell: (c) => <span className="tabular-nums text-fg-2">{c.getValue()?.toFixed(2)}</span>,
      }),
      col.accessor((r) => r.fee ?? 0, {
        id: "fee",
        header: "手续费",
        cell: (c) => <span className="tabular-nums text-fg-3">{c.getValue()?.toFixed(2)}</span>,
      }),
      col.display({
        id: "actions",
        header: "",
        cell: (c) => (
          <div className="flex justify-end gap-1">
            <Button
              variant="ghost"
              size="icon"
              aria-label="编辑"
              onClick={() => {
                const tx = c.row.original;
                setFormInitial({
                  seq: tx.seq,
                  fund_code: tx.fund_code,
                  trade_time: (tx.trade_time ?? "").slice(0, 16),
                  confirm_date: tx.confirm_date ?? "",
                  direction: tx.direction ?? "buy",
                  trade_type: tx.trade_type ?? "用户买入",
                  amount: tx.amount != null ? String(tx.amount) : "",
                  shares: tx.shares != null ? String(tx.shares) : "",
                  fee: tx.fee != null ? String(tx.fee) : "0",
                });
                setFormOpen(true);
              }}
            >
              <Pencil className="size-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label="删除"
              onClick={() => setDeleteSeq(c.row.original.seq)}
            >
              <Trash2 className="size-3.5 text-down" />
            </Button>
          </div>
        ),
      }),
    ],
    [],
  );

  const table = useReactTable({
    data: data.data?.transactions ?? [],
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    manualPagination: true,
  });

  const total = data.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="space-y-3">
      {/* 工具栏 */}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          size="sm"
          onClick={() => {
            setFormInitial(EMPTY_FORM);
            setFormOpen(true);
          }}
        >
          <Plus className="size-4" />
          新增交易
        </Button>
        <div className="w-44">
          <Select
            value={fundCode || "__all__"}
            onValueChange={(v) => {
              setFundCode(v === "__all__" ? "" : v);
              setPage(0);
            }}
          >
            <SelectTrigger aria-label="按标的过滤">
              <SelectValue placeholder="全部标的" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">全部标的</SelectItem>
              {(securities.data ?? []).map((s) => (
                <SelectItem key={s.code} value={s.code}>
                  {s.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Segmented
          id="tx-direction"
          value={direction}
          onChange={(v) => {
            setDirection(v);
            setPage(0);
          }}
          options={DIRECTIONS.map((d) => ({ value: d.value, label: d.label }))}
          size="sm"
        />
        <div className="w-52">
          <Input
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(0);
            }}
            placeholder="搜索名称 / 代码…"
            aria-label="全文搜索"
          />
        </div>
        <div className="ml-auto flex gap-1.5">
          <Button
            variant="outline"
            size="sm"
            onClick={() => exportCsv.mutate()}
            disabled={exportCsv.isPending}
          >
            <Download className="size-3.5" />
            CSV
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => exportXlsx.mutate()}
            disabled={exportXlsx.isPending}
          >
            <Download className="size-3.5" />
            XLSX
          </Button>
        </div>
      </div>

      {/* 表格 */}
      <Card className="overflow-x-auto" style={{ padding: 0 }}>
        <Table>
          <THead>
            {table.getHeaderGroups().map((hg) => (
              <Tr key={hg.id}>
                {hg.headers.map((h) => (
                  <Th
                    key={h.id}
                    className={cn(
                      ["amount", "shares", "fee", "actions"].includes(h.column.id) && "text-right",
                    )}
                  >
                    {h.column.getCanSort() ? (
                      <button
                        type="button"
                        onClick={h.column.getToggleSortingHandler()}
                        className="inline-flex cursor-pointer items-center gap-1"
                      >
                        {flexRender(h.column.columnDef.header, h.getContext())}
                        {h.column.getIsSorted() === "asc" ? (
                          <ArrowUp className="size-3" />
                        ) : h.column.getIsSorted() === "desc" ? (
                          <ArrowDown className="size-3" />
                        ) : (
                          <ArrowUpDown className="size-3 opacity-40" />
                        )}
                      </button>
                    ) : (
                      flexRender(h.column.columnDef.header, h.getContext())
                    )}
                  </Th>
                ))}
              </Tr>
            ))}
          </THead>
          <TBody>
            {data.isPending ? (
              ["r1", "r2", "r3", "r4", "r5", "r6"].map((k) => (
                <Tr key={k}>
                  {columns.map((c) => (
                    <Td key={c.id}>
                      <Skeleton className="h-4 w-full" />
                    </Td>
                  ))}
                </Tr>
              ))
            ) : table.getRowModel().rows.length === 0 ? (
              <Tr>
                <Td colSpan={columns.length}>
                  <EmptyState
                    title="没有匹配的交易"
                    description="调整过滤条件，或新增一笔交易。"
                    action={
                      <Button
                        size="sm"
                        onClick={() => {
                          setFormInitial(EMPTY_FORM);
                          setFormOpen(true);
                        }}
                      >
                        新增交易
                      </Button>
                    }
                  />
                </Td>
              </Tr>
            ) : (
              table.getRowModel().rows.map((row) => (
                <Tr key={row.original.seq}>
                  {row.getVisibleCells().map((cell) => (
                    <Td
                      key={cell.id}
                      className={cn(
                        ["amount", "shares", "fee", "actions"].includes(cell.column.id) &&
                          "text-right",
                      )}
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </Td>
                  ))}
                </Tr>
              ))
            )}
          </TBody>
        </Table>
      </Card>

      {/* 分页 */}
      <div className="flex items-center justify-between text-xs text-fg-3">
        <span className="tabular-nums">
          共 {total} 条 · 第 {page + 1} / {pageCount} 页
        </span>
        <div className="flex gap-1.5">
          <Button
            variant="outline"
            size="sm"
            disabled={page === 0}
            onClick={() => setPage(page - 1)}
          >
            上一页
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={page + 1 >= pageCount}
            onClick={() => setPage(page + 1)}
          >
            下一页
          </Button>
        </div>
      </div>

      <TxFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        initial={formInitial}
        onSaved={() => setFormOpen(false)}
      />
      <DeleteDialog seq={deleteSeq} onOpenChange={() => setDeleteSeq(null)} />
    </div>
  );
}
