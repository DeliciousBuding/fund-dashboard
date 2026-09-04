import {
  ImportTransactionsResponseSchema,
  UpdateTransactionResponseSchema,
} from "@fund-dashboard/contracts";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
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
import { Input, Label } from "../../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../components/ui/select";
import { api } from "../../lib/api";
import { mutationErrorMessage } from "../../services/userError";

// ── 新增/编辑表单 ───────────────────────────────────────────────────

export interface TxFormState {
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

const pad2 = (n: number) => String(n).padStart(2, "0");
const nowLocalDateTime = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
};
const nowLocalDate = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
};

export const EMPTY_FORM: TxFormState = {
  seq: null,
  fund_code: "",
  trade_time: nowLocalDateTime(),
  confirm_date: nowLocalDate(),
  direction: "buy",
  trade_type: "用户买入",
  amount: "",
  shares: "",
  fee: "0",
};

export function TxFormDialog(props: {
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
        const data = await api<unknown>("/api/transactions/import", {
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
        ImportTransactionsResponseSchema.parse(data);
      } else {
        const updated = await api<unknown>(`/api/transactions/${form.seq}`, {
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
        UpdateTransactionResponseSchema.parse(updated);
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["transactions"] });
      await queryClient.invalidateQueries({ queryKey: ["securities"] });
      toast.success(form.seq == null ? "交易已新增" : "交易已更新");
      props.onSaved();
    },
    onError: (e) => {
      setError(mutationErrorMessage(e, "保存失败"));
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
