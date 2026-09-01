// 交易 /transactions —— 全量台账：过滤（方向/标的/全文）+ 服务端分页 + 排序 +
// 新增/编辑/删除（二次确认）+ 导出（CSV 中文 BOM / XLSX）。
// 写路径走 session + X-Fund-Request（api client 内置），变更后 invalidate 相关查询。

import {
  flexRender,
  getCoreRowModel,
  type SortingState,
  useReactTable,
} from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ArrowUpDown, Download, Plus } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { EmptyState } from "../components/ui/empty-state";
import { Input } from "../components/ui/input";
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
import { type TransactionListItem, useSecurities, useTransactions } from "../lib/queries";
import { cn } from "../lib/utils";
import { useUi } from "../stores/ui";
import { buildTransactionColumns } from "./transactions/columns";
import { DIRECTIONS, PAGE_SIZE } from "./transactions/constants";
import { DeleteDialog } from "./transactions/DeleteDialog";
import { useExportMutations } from "./transactions/exporters";
import { EMPTY_FORM, TxFormDialog, type TxFormState } from "./transactions/TxForm";

export function TransactionsPage() {
  const portfolioId = useUi((s) => s.portfolioId);
  const securities = useSecurities(portfolioId);
  const [direction, setDirection] = useState("");
  const [fundCode, setFundCode] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const [sorting, setSorting] = useState<SortingState>([]);
  const activeSort = sorting[0];

  const filter = useMemo(
    () => ({
      portfolioId,
      direction: direction || undefined,
      fundCode: fundCode || undefined,
      search: search || undefined,
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
      sortBy: activeSort?.id,
      sortDesc: activeSort?.desc === true,
    }),
    [portfolioId, direction, fundCode, search, page, activeSort],
  );
  const data = useTransactions(filter);

  const [formOpen, setFormOpen] = useState(false);
  const [formInitial, setFormInitial] = useState<TxFormState>(EMPTY_FORM);
  const [deleteSeq, setDeleteSeq] = useState<number | null>(null);

  const { exportCsv, exportXlsx } = useExportMutations({
    portfolioId,
    direction,
    fundCode,
    search,
  });

  const openEdit = useCallback((tx: TransactionListItem) => {
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
  }, []);

  const columns = useMemo(
    () => buildTransactionColumns({ onEdit: openEdit, onDelete: setDeleteSeq }),
    [openEdit],
  );

  const table = useReactTable({
    data: data.data?.transactions ?? [],
    columns,
    state: { sorting },
    onSortingChange: (updater) => {
      setSorting(updater);
      setPage(0);
    },
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
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
