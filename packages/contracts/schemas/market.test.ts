// market.test.ts — /api/market/exchange-rate wire-shape tests (node:test).
// 对照 internal/service/portfolio/exchange_rate.go ExchangeRateReport：
// from/to/rate/updated_at 恒下发，source 带 omitempty（条件下发）。
import assert from "node:assert/strict";
import { test } from "node:test";

import { ExchangeRateSchema } from "./market.ts";

const exchangeRateWire = {
  from: "USD",
  to: "CNY",
  rate: 7.2345,
  updated_at: "2026-08-29T09:30:00Z",
  source: "yahoo_chart",
};

test("ExchangeRateSchema parses the real report shape", () => {
  const parsed = ExchangeRateSchema.parse(exchangeRateWire);
  assert.equal(parsed.from, "USD");
  assert.equal(parsed.to, "CNY");
  assert.equal(parsed.rate, 7.2345);
  assert.equal(parsed.source, "yahoo_chart");
});

test("ExchangeRateSchema tolerates omitempty source being absent", () => {
  const { source, ...withoutSource } = exchangeRateWire;
  const parsed = ExchangeRateSchema.parse(withoutSource);
  assert.equal(parsed.source, undefined);
  assert.equal(parsed.rate, 7.2345);
});

test("ExchangeRateSchema rejects a report missing rate", () => {
  const { rate, ...rest } = exchangeRateWire;
  assert.throws(() => ExchangeRateSchema.parse(rest));
});

test("ExchangeRateSchema rejects unknown fields (contract drift)", () => {
  assert.throws(() => ExchangeRateSchema.parse({ ...exchangeRateWire, future_field: "surprise" }));
});
