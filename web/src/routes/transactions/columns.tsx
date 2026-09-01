import { createColumnHelper } from "@tanstack/react-table";
import { Pencil, Trash2 } from "lucide-react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { DIRECTION_LABEL } from "../../lib/csv";
import { fmtCNY } from "../../lib/format";
import type { TransactionListItem } from "../../lib/queries";
import { cn } from "../../lib/utils";
import { DIRECTION_BADGE } from "./constants";

const col = createColumnHelper<TransactionListItem>();

export function buildTransactionColumns(opts: {
  onEdit: (tx: TransactionListItem) => void;
  onDelete: (seq: number | null) => void;
}) {
  return [
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
            onClick={() => opts.onEdit(c.row.original)}
          >
            <Pencil className="size-3.5" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            aria-label="删除"
            onClick={() => opts.onDelete(c.row.original.seq)}
          >
            <Trash2 className="size-3.5 text-down" />
          </Button>
        </div>
      ),
    }),
  ];
}
