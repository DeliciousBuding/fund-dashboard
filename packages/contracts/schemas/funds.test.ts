// funds.test.ts — NAV history wire-shape tests (node:test).
// 对照 internal/service/portfolio/detail.go GetNavHistory / NavHistoryPoint：
// daily_change_pct 无 omitempty（null 表示无涨跌幅），security_type 经
// COALESCE(security_type,'fund') 后恒为字符串。
import assert from "node:assert/strict";
import { test } from "node:test";

import {
  DeleteTransactionResponseSchema,
  FundDetailSchema,
  ImportTransactionsResponseSchema,
  NavPointSchema,
  TransactionSchema,
  UpdateTransactionResponseSchema,
} from "./funds.ts";

const navWire = {
  date: "2026-08-29",
  unit_nav: 1.2345,
  daily_change_pct: -0.35,
  security_type: "fund",
};

test("NavPointSchema parses the real NAV point shape", () => {
  const parsed = NavPointSchema.parse(navWire);
  assert.equal(parsed.unit_nav, 1.2345);
  assert.equal(parsed.security_type, "fund");
});

test("NavPointSchema accepts null daily_change_pct (Go pointer without omitempty)", () => {
  const parsed = NavPointSchema.parse({ ...navWire, daily_change_pct: null });
  assert.equal(parsed.daily_change_pct, null);
});

test("NavPointSchema rejects a point missing security_type", () => {
  assert.throws(() =>
    NavPointSchema.parse({
      date: "2026-08-29",
      unit_nav: 1.2345,
      daily_change_pct: null,
    }),
  );
});

// --- /api/funds/:code detail + transactions (strict wire contracts) ---
// fundTransaction (internal/httpapi/fund_response.go): confirm_date/
// settlement_days/order_id have omitempty; every other key is always emitted.

const transactionWire = {
  seq: 1,
  trade_time: "2026-08-29T09:30:00+08:00",
  confirm_date: "2026-08-30",
  trade_type: "买入",
  direction: "buy",
  amount: 1000,
  shares: 900.12,
  fee: 1.5,
  nav: null,
  inferred_nav: null,
  settlement_days: 1,
  order_id: "ORD-001",
  anomaly: null,
};

test("TransactionSchema parses the real fundTransaction shape", () => {
  const parsed = TransactionSchema.parse(transactionWire);
  assert.equal(parsed.seq, 1);
  assert.equal(parsed.trade_type, "买入");
});

test("TransactionSchema rejects unknown fields (contract drift)", () => {
  assert.throws(() => TransactionSchema.parse({ ...transactionWire, future_field: "surprise" }));
});

test("TransactionSchema tolerates omitempty fields being absent", () => {
  const { confirm_date, settlement_days, order_id, ...minimal } = transactionWire;
  const parsed = TransactionSchema.parse(minimal);
  assert.equal(parsed.seq, 1);
  assert.equal(parsed.order_id, undefined);
});

const fundDetailWire = {
  code: "000001",
  name: "成长混合基金",
  security_type: "fund",
  market: "cn",
  held_shares: 100,
  total_cost: 5000,
  latest_nav: 1.234,
  current_value: 5100,
  unrealized_pnl: 100,
  pnl_pct: 2.0,
  auto_buy_count: 3,
  manual_buy_count: 1,
  auto_buy_amount: 3000,
  manual_buy_amount: 2000,
  auto_tx: 3,
  manual_tx: 1,
  buy_count: 4,
  sell_count: 0,
  median_settlement: 1,
  transactions: [transactionWire],
};

test("FundDetailSchema parses the real fundDetailJSON shape", () => {
  const parsed = FundDetailSchema.parse(fundDetailWire);
  assert.equal(parsed.transactions.length, 1);
  assert.equal(parsed.total_cost, 5000);
});

test("FundDetailSchema rejects unknown fields (contract drift)", () => {
  assert.throws(() => FundDetailSchema.parse({ ...fundDetailWire, extra_top_level: true }));
});

test("FundDetailSchema tolerates omitempty security_type/market being absent", () => {
  const { security_type, market, ...minimal } = fundDetailWire;
  const parsed = FundDetailSchema.parse(minimal);
  assert.equal(parsed.security_type, undefined);
  assert.equal(parsed.market, undefined);
});

// --- 写响应契约（/api/transactions/import、PUT/DELETE /api/transactions/{seq}）---
// Go 侧 ImportTransactionsResult/UpdateTransactionResult/DeleteTransactionResult
// 全字段无 omitempty，恒下发。

const importWire = { ok: true, imported: 3, total: 3, affected_funds: 2 };

test("ImportTransactionsResponseSchema parses the real import shape", () => {
  const parsed = ImportTransactionsResponseSchema.parse(importWire);
  assert.equal(parsed.imported, 3);
  assert.equal(parsed.affected_funds, 2);
});

test("ImportTransactionsResponseSchema rejects a missing always-emitted key", () => {
  const { affected_funds, ...partial } = importWire;
  assert.throws(() => ImportTransactionsResponseSchema.parse(partial));
});

test("ImportTransactionsResponseSchema rejects unknown fields (contract drift)", () => {
  assert.throws(() => ImportTransactionsResponseSchema.parse({ ...importWire, extra: 1 }));
});

const updateWire = { ok: true, updated: { seq: 42, fields: ["confirm_amount", "fee"] } };

test("UpdateTransactionResponseSchema parses the real update shape", () => {
  const parsed = UpdateTransactionResponseSchema.parse(updateWire);
  assert.equal(parsed.updated.seq, 42);
  assert.equal(parsed.updated.fields.length, 2);
});

test("UpdateTransactionResponseSchema rejects empty fields key", () => {
  assert.throws(() => UpdateTransactionResponseSchema.parse({ ok: true, updated: { seq: 42 } }));
});

test("UpdateTransactionResponseSchema rejects unknown nested fields", () => {
  assert.throws(() =>
    UpdateTransactionResponseSchema.parse({
      ok: true,
      updated: { seq: 42, fields: [], extra: true },
    }),
  );
});

const deleteWire = {
  ok: true,
  deleted: { seq: 42, fund_code: "161725", direction: "buy", amount: 1000 },
};

test("DeleteTransactionResponseSchema parses the real delete shape", () => {
  const parsed = DeleteTransactionResponseSchema.parse(deleteWire);
  assert.equal(parsed.deleted.fund_code, "161725");
});

test("DeleteTransactionResponseSchema rejects a missing deleted key", () => {
  assert.throws(() => DeleteTransactionResponseSchema.parse({ ok: true }));
});

test("DeleteTransactionResponseSchema rejects unknown fields (contract drift)", () => {
  assert.throws(() =>
    DeleteTransactionResponseSchema.parse({
      ...deleteWire,
      deleted: { ...deleteWire.deleted, x: 1 },
    }),
  );
});
