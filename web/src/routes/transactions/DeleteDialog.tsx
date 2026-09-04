import { DeleteTransactionResponseSchema } from "@fund-dashboard/contracts";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Button } from "../../components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../../components/ui/dialog";
import { api } from "../../lib/api";
import { mutationErrorMessage } from "../../services/userError";

// ── 删除确认 ────────────────────────────────────────────────────────

export function DeleteDialog(props: { seq: number | null; onOpenChange: (v: boolean) => void }) {
  const queryClient = useQueryClient();
  const del = useMutation({
    mutationFn: async () =>
      DeleteTransactionResponseSchema.parse(
        await api<unknown>(`/api/transactions/${props.seq}`, { method: "DELETE" }),
      ),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["transactions"] });
      await queryClient.invalidateQueries({ queryKey: ["securities"] });
      toast.success("交易已删除");
      props.onOpenChange(false);
    },
    onError: (e) => toast.error("删除失败", { description: mutationErrorMessage(e, "请稍后重试") }),
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
