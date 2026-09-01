import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "../../lib/api";
import { downloadText, transactionsToCsv } from "../../lib/csv";
import type { TransactionListItem } from "../../lib/queries";

export function useExportMutations(args: { direction: string; fundCode: string; search: string }) {
  const { direction, fundCode, search } = args;
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
      const blob = await api<Blob>("/api/export/transactions-xlsx", { responseType: "blob" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `transactions-${new Date().toISOString().slice(0, 10)}.xlsx`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      // Deferred revoke: immediate revoke races the click and cancels the
      // download in Safari/Firefox.
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    },
    onSuccess: () => toast.success("XLSX 已导出"),
    onError: () => toast.error("XLSX 导出失败"),
  });
  return { exportCsv, exportXlsx };
}
