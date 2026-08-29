// 持仓 /holdings —— 持仓表格（排序/占比/盈亏/陈旧度），移动端降级卡片流（03 §8/§9）。
// 陈旧度：/api/freshness 的 stale_securities 客户端 join。

import type { SecurityInfo } from "@fund-dashboard/contracts";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  type SortingState,
  useReactTable,
} from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { useMemo, useState } from "react";
import { Badge } from "../components/ui/badge";
import { Card } from "../components/ui/card";
import { EmptyState } from "../components/ui/empty-state";
import { Skeleton } from "../components/ui/skeleton";
import { Table, TBody, Td, THead, Th, Tr } from "../components/ui/table";
import { api } from "../lib/api";
import { fmtCNY, fmtPct, fmtSignedCNY, fmtSignedPct, pnlTone } from "../lib/format";
import { useSecurities } from "../lib/queries";
import { cn } from "../lib/utils";
import { useUi } from "../stores/ui";

const toneClass = { up: "text-up", down: "text-down", flat: "text-fg-3" } as const;

interface FreshnessReport {
  stale_securities: { code: string; stale_days: number }[];
}

function useStaleMap() {
  const freshness = useQuery({
    queryKey: ["freshness"],
    queryFn: ({ signal }) => api<FreshnessReport>("/api/freshness", { signal }),
    staleTime: 5 * 60 * 1000,
  });
  return useMemo(() => {
    const map = new Map<string, number>();
    for (const s of freshness.data?.stale_securities ?? []) map.set(s.code, s.stale_days);
    return map;
  }, [freshness.data]);
}

type Row = SecurityInfo & { weight_pct: number | null };

const col = createColumnHelper<Row>();

export function HoldingsPage() {
  const portfolioId = useUi((s) => s.portfolioId);
  const securities = useSecurities(portfolioId);
  const heldOnly = useUi((s) => s.heldOnly);
  const staleMap = useStaleMap();
  const [sorting, setSorting] = useState<SortingState>([{ id: "current_value", desc: true }]);

  const rows = useMemo<Row[]>(() => {
    const list = securities.data ?? [];
    const filtered = heldOnly ? list.filter((s) => s.held_shares > 0) : list;
    const total = filtered.reduce((sum, s) => sum + (s.current_value ?? 0), 0);
    return filtered.map((s) => ({
      ...s,
      weight_pct: total > 0 && s.current_value != null ? (s.current_value / total) * 100 : null,
    }));
  }, [securities.data, heldOnly]);

  const columns = useMemo(
    () => [
      col.accessor((r) => r.name, {
        id: "name",
        header: "名称",
        cell: (c) => (
          <div className="min-w-0">
            <Link
              to="/holdings/$code"
              params={{ code: c.row.original.code }}
              className="block truncate font-medium text-fg hover:text-accent"
            >
              {c.getValue()}
            </Link>
            <span className="font-mono text-[11px] text-fg-3">{c.row.original.code}</span>
          </div>
        ),
      }),
      col.accessor((r) => r.current_value ?? 0, {
        id: "current_value",
        header: "市值",
        cell: (c) => <span className="tabular-nums">{fmtCNY(c.row.original.current_value)}</span>,
      }),
      col.accessor((r) => r.weight_pct ?? -1, {
        id: "weight",
        header: "占比",
        cell: (c) => (
          <span className="tabular-nums text-fg-2">{fmtPct(c.row.original.weight_pct, 1)}</span>
        ),
      }),
      col.accessor((r) => r.unrealized_pnl ?? 0, {
        id: "pnl",
        header: "盈亏",
        cell: (c) => (
          <span className={cn("tabular-nums", toneClass[pnlTone(c.row.original.unrealized_pnl)])}>
            {fmtSignedCNY(c.row.original.unrealized_pnl)}
          </span>
        ),
      }),
      col.accessor((r) => r.pnl_pct ?? 0, {
        id: "pnl_pct",
        header: "盈亏%",
        cell: (c) => (
          <span className={cn("tabular-nums", toneClass[pnlTone(c.row.original.pnl_pct)])}>
            {fmtSignedPct(c.row.original.pnl_pct)}
          </span>
        ),
      }),
      col.accessor((r) => staleMap.get(r.code) ?? -1, {
        id: "stale",
        header: "新鲜度",
        cell: (c) => {
          const days = staleMap.get(c.row.original.code);
          if (days == null) return <span className="text-xs text-fg-3">—</span>;
          return <Badge tone={days >= 7 ? "danger" : "warn"}>{days} 天前</Badge>;
        },
      }),
    ],
    [staleMap],
  );

  const table = useReactTable({
    data: rows,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  if (securities.isError) {
    return (
      <EmptyState
        title="持仓数据加载失败"
        description="检查服务状态后重试。"
        action={
          <button
            type="button"
            onClick={() => securities.refetch()}
            className="text-sm text-accent hover:underline"
          >
            重试
          </button>
        }
      />
    );
  }

  return (
    <div className="space-y-3">
      {/* 桌面表格 */}
      <Card className="hidden overflow-x-auto p-0 md:block" style={{ padding: 0 }}>
        <Table>
          <THead>
            {table.getHeaderGroups().map((hg) => (
              <Tr key={hg.id}>
                {hg.headers.map((h) => (
                  <Th key={h.id} className={cn(h.column.id !== "name" && "text-right")}>
                    <button
                      type="button"
                      onClick={h.column.getToggleSortingHandler()}
                      className={cn(
                        "inline-flex cursor-pointer items-center gap-1",
                        h.column.id !== "name" && "flex-row-reverse",
                      )}
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
                  </Th>
                ))}
              </Tr>
            ))}
          </THead>
          <TBody>
            {securities.isPending
              ? ["s1", "s2", "s3", "s4", "s5"].map((k) => (
                  <Tr key={k}>
                    {columns.map((c) => (
                      <Td key={c.id}>
                        <Skeleton className="h-4 w-full" />
                      </Td>
                    ))}
                  </Tr>
                ))
              : table.getRowModel().rows.map((row) => (
                  <Tr key={row.original.code}>
                    {row.getVisibleCells().map((cell) => (
                      <Td
                        key={cell.id}
                        className={cn(cell.column.id !== "name" && "text-right tabular-nums")}
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </Td>
                    ))}
                  </Tr>
                ))}
          </TBody>
        </Table>
        {!securities.isPending && rows.length === 0 && (
          <EmptyState
            title={heldOnly ? "暂无持仓" : "标的列表为空"}
            description={
              heldOnly ? "关闭侧栏「仅看持有」可查看全部已入册标的。" : "先入册标的或导入交易。"
            }
          />
        )}
      </Card>

      {/* 移动卡片流 */}
      <div className="space-y-2 md:hidden">
        {securities.isPending ? (
          ["c1", "c2", "c3"].map((k) => <Skeleton key={k} className="h-20" />)
        ) : rows.length === 0 ? (
          <EmptyState title={heldOnly ? "暂无持仓" : "标的列表为空"} />
        ) : (
          rows.map((s) => (
            <Link key={s.code} to="/holdings/$code" params={{ code: s.code }}>
              <Card className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="truncate text-sm font-medium text-fg">{s.name}</div>
                  <div className="mt-0.5 font-mono text-[11px] text-fg-3">
                    {s.code}
                    {staleMap.get(s.code) != null && (
                      <span className="ml-2 text-warn">{staleMap.get(s.code)} 天前</span>
                    )}
                  </div>
                </div>
                <div className="shrink-0 text-right">
                  <div className="text-sm tabular-nums text-fg">{fmtCNY(s.current_value)}</div>
                  <div className={cn("text-xs tabular-nums", toneClass[pnlTone(s.pnl_pct)])}>
                    {fmtSignedPct(s.pnl_pct)}
                  </div>
                </div>
              </Card>
            </Link>
          ))
        )}
      </div>
    </div>
  );
}
