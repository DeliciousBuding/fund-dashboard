// 交易 CSV 导出 —— 继承旧 transactionsToCsv 语义：BOM + 全引号包裹 + 中文表头 +
// 方向中文标签 + T+N 结算。i18n 已废弃，直连中文（W0 决策）。
import type { TransactionListItem } from "./queries";

const HEADERS = [
  "交易时间",
  "确认日期",
  "方向",
  "类型",
  "金额",
  "份额",
  "手续费",
  "结算",
  "单号",
  "备注",
] as const;

export const DIRECTION_LABEL: Record<string, string> = {
  buy: "买入",
  sell: "卖出",
  dividend: "分红",
  convert_in: "转换转入",
  convert_out: "转换转出",
  forced_redeem: "强制赎回",
};

// csvText 对可能被 Excel 当公式执行的文本列做防护：以 = + - @ 开头时前置单引号。
function csvText(v: unknown): string {
  const s = v == null ? "" : String(v);
  return /^[=+\-@]/.test(s) ? `'${s}` : s;
}

export function transactionsToCsv(rows: TransactionListItem[]): string {
  const lines = rows.map((tx) =>
    [
      csvText((tx.trade_time ?? "").substring(0, 16)),
      csvText(tx.confirm_date),
      DIRECTION_LABEL[tx.direction ?? ""] ?? csvText(tx.direction),
      csvText(tx.trade_type),
      tx.amount != null ? tx.amount.toFixed(2) : "",
      tx.shares != null ? tx.shares.toFixed(2) : "",
      tx.fee != null && tx.fee > 0 ? tx.fee.toFixed(2) : "",
      tx.settlement_days != null ? `T+${tx.settlement_days}` : "",
      csvText(tx.order_id),
      csvText(tx.anomaly),
    ]
      .map((v) => `"${String(v).replaceAll('"', '""')}"`)
      .join(","),
  );
  // BOM：Excel 打开 UTF-8 不乱码（旧实现同语义）
  return `﻿${[HEADERS.join(","), ...lines].join("\n")}`;
}

export function downloadText(filename: string, content: string, mime = "text/csv;charset=utf-8") {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Deferred revoke: immediate revoke races the click and cancels the
  // download in Safari/Firefox.
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
