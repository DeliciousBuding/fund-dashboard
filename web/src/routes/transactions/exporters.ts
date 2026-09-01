import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "../../lib/api";
import { downloadBlob, downloadText, transactionsToCsv } from "../../lib/csv";
import type { TransactionListItem } from "../../lib/queries";

function filteredTxParams(args: {
  portfolioId?: number;
  direction?: string;
  fundCode?: string;
  search?: string;
}) {
  const params = new URLSearchParams();
  if (args.direction) params.set("direction", args.direction);
  if (args.fundCode) params.set("fund_code", args.fundCode);
  if (args.search) params.set("search", args.search);
  if (args.portfolioId && args.portfolioId > 1)
    params.set("portfolio_id", String(args.portfolioId));
  params.set("limit", "5000");
  return params;
}

export function useExportMutations(args: {
  portfolioId?: number;
  direction: string;
  fundCode: string;
  search: string;
}) {
  const { portfolioId, direction, fundCode, search } = args;

  const fetchAll = async () => {
    const params = filteredTxParams({ portfolioId, direction, fundCode, search });
    const all = await api<{ transactions: TransactionListItem[] }>(`/api/transactions?${params}`);
    return all.transactions;
  };

  const exportCsv = useMutation({
    mutationFn: async () => {
      const rows = await fetchAll();
      downloadText(
        `transactions-${new Date().toISOString().slice(0, 10)}.csv`,
        transactionsToCsv(rows),
      );
    },
    onSuccess: () => toast.success("CSV 已导出"),
    onError: () => toast.error("导出失败"),
  });

  const exportXlsx = useMutation({
    mutationFn: async () => {
      const rows = await fetchAll();
      const blob = await api<Blob>("/api/export/transactions-xlsx", {
        method: "POST",
        responseType: "blob",
        body: {
          fundName: "transactions",
          transactions: rows.map((tx) => ({
            trade_time: tx.trade_time ?? "",
            confirm_date: tx.confirm_date ?? "",
            direction: tx.direction ?? "",
            type: tx.trade_type ?? "",
            amount: tx.amount ?? 0,
            shares: tx.shares ?? 0,
            nav: null,
            inferred_nav: null,
            fee: tx.fee ?? 0,
            settlement_days: tx.settlement_days,
            trade_day_type: "",
          })),
        },
      });
      downloadBlob(`transactions-${new Date().toISOString().slice(0, 10)}.xlsx`, blob);
    },
    onSuccess: () => toast.success("XLSX 已导出"),
    onError: () => toast.error("XLSX 导出失败"),
  });

  return { exportCsv, exportXlsx };
}
