// market.test.ts — /api/market/* wire-shape tests (node:test).
// 对照 internal/httpapi/market.go 与 internal/service/portfolio/index_history.go：
// indices 7 键恒下发；history 8 键 map 字面量恒下发（error/message 不进该 map）；
// live 7 恒发键 + 6 omitempty 条件键。
import assert from "node:assert/strict";
import { test } from "node:test";

import {
  ExchangeRateSchema,
  IndexHistoryPointSchema,
  IndexHistorySchema,
  IndexLiveSchema,
  MarketIndexSchema,
} from "./market.ts";

const marketIndexWire = {
  code: "^GSPC",
  name: "标普500",
  market: "US",
  price: 5600.5,
  change_pct: 0.42,
  change_amt: 23.5,
  updated_at: "2099-01-01 12:00:00",
};

const indexHistoryPointWire = { date: "2026-08-28", close: 19888.2, change_pct: 1.25 };

const indexHistoryWire = {
  symbol: "^NDX",
  count: 2,
  range: "1y",
  data: [indexHistoryPointWire, { date: "2026-08-29", close: 19910.5, change_pct: 0.11 }],
  source: "yahoo_chart",
  external_fetch: "yahoo_chart",
  decision_boundary: "facts_only",
  side_effects: "none",
};

const indexLiveWire = {
  code: "^GSPC",
  name: "标普500",
  market: "US",
  price: 5600.5,
  change_pct: 0.42,
  change_amt: 23.5,
  updated_at: "2099-01-01 12:00:00",
  source: "indices_cache",
  decision_boundary: "facts_only",
  side_effects: "none",
  external_fetch: "not_performed",
};

const exchangeRateWire = {
  from: "USD",
  to: "CNY",
  rate: 7.2345,
  updated_at: "2026-08-29T09:30:00Z",
  source: "yahoo_chart",
};

// ── /api/market/indices ──────────────────────────────────────────────

test("MarketIndexSchema parses the real indices shape", () => {
  const parsed = MarketIndexSchema.parse(marketIndexWire);
  assert.equal(parsed.code, "^GSPC");
  assert.equal(parsed.price, 5600.5);
  assert.equal(parsed.change_pct, 0.42);
});

test("MarketIndexSchema rejects null price (Go float64 defaults to 0)", () => {
  assert.throws(() => MarketIndexSchema.parse({ ...marketIndexWire, price: null }));
});

test("MarketIndexSchema requires updated_at (Go always emits it)", () => {
  const { updated_at, ...rest } = marketIndexWire;
  assert.throws(() => MarketIndexSchema.parse(rest));
});

test("MarketIndexSchema rejects unknown fields (live keys do not belong here)", () => {
  assert.throws(() => MarketIndexSchema.parse({ ...marketIndexWire, source: "yahoo_chart" }));
});

// ── /api/market/index/{code}/history ─────────────────────────────────

test("IndexHistorySchema parses the real 8-key shape", () => {
  const parsed = IndexHistorySchema.parse(indexHistoryWire);
  assert.equal(parsed.symbol, "^NDX");
  assert.equal(parsed.count, 2);
  assert.equal(parsed.source, "yahoo_chart");
  assert.equal(parsed.decision_boundary, "facts_only");
  assert.equal(parsed.data.length, 2);
});

test("IndexHistorySchema requires source (map literal always emits it)", () => {
  const { source, ...rest } = indexHistoryWire;
  assert.throws(() => IndexHistorySchema.parse(rest));
});

test("IndexHistorySchema requires external_fetch (map literal always emits it)", () => {
  const { external_fetch, ...rest } = indexHistoryWire;
  assert.throws(() => IndexHistorySchema.parse(rest));
});

test("IndexHistorySchema requires decision_boundary (map literal always emits it)", () => {
  const { decision_boundary, ...rest } = indexHistoryWire;
  assert.throws(() => IndexHistorySchema.parse(rest));
});

test("IndexHistorySchema requires side_effects (map literal always emits it)", () => {
  const { side_effects, ...rest } = indexHistoryWire;
  assert.throws(() => IndexHistorySchema.parse(rest));
});

test("IndexHistorySchema requires data (always emitted)", () => {
  const { data, ...rest } = indexHistoryWire;
  assert.throws(() => IndexHistorySchema.parse(rest));
});

test("IndexHistorySchema rejects unknown fields (contract drift)", () => {
  assert.throws(() => IndexHistorySchema.parse({ ...indexHistoryWire, future_field: "surprise" }));
});

test("IndexHistoryPointSchema parses the real point shape", () => {
  const parsed = IndexHistoryPointSchema.parse(indexHistoryPointWire);
  assert.equal(parsed.close, 19888.2);
});

test("IndexHistoryPointSchema rejects a point missing close", () => {
  const { close, ...rest } = indexHistoryPointWire;
  assert.throws(() => IndexHistoryPointSchema.parse(rest));
});

test("IndexHistoryPointSchema rejects unknown fields", () => {
  assert.throws(() =>
    IndexHistoryPointSchema.parse({ ...indexHistoryPointWire, future_field: "surprise" }),
  );
});

// ── /api/market/index/{code}（live）──────────────────────────────────

test("IndexLiveSchema parses the real indices-cache shape", () => {
  const parsed = IndexLiveSchema.parse(indexLiveWire);
  assert.equal(parsed.code, "^GSPC");
  assert.equal(parsed.price, 5600.5);
  assert.equal(parsed.source, "indices_cache");
  assert.equal(parsed.error, undefined);
});

test("IndexLiveSchema tolerates omitempty source being absent", () => {
  const { source, ...rest } = indexLiveWire;
  const parsed = IndexLiveSchema.parse(rest);
  assert.equal(parsed.source, undefined);
});

test("IndexLiveSchema tolerates omitempty error/message being absent", () => {
  const parsed = IndexLiveSchema.parse(indexLiveWire);
  assert.equal(parsed.error, undefined);
  assert.equal(parsed.message, undefined);
});

test("IndexLiveSchema parses the no-data shape (error/message, no source)", () => {
  const parsed = IndexLiveSchema.parse({
    code: "^ZZZ",
    name: "",
    market: "",
    price: 0,
    change_pct: 0,
    change_amt: 0,
    updated_at: "",
    decision_boundary: "facts_only",
    side_effects: "none",
    external_fetch: "not_performed",
    error: "no_data",
    message: "no quote for ^ZZZ",
  });
  assert.equal(parsed.error, "no_data");
  assert.equal(parsed.message, "no quote for ^ZZZ");
  assert.equal(parsed.source, undefined);
});

test("IndexLiveSchema requires code (Go always emits it)", () => {
  const { code, ...rest } = indexLiveWire;
  assert.throws(() => IndexLiveSchema.parse(rest));
});

test("IndexLiveSchema requires updated_at (Go always emits it, even empty)", () => {
  const { updated_at, ...rest } = indexLiveWire;
  assert.throws(() => IndexLiveSchema.parse(rest));
});

test("IndexLiveSchema rejects unknown fields (contract drift)", () => {
  assert.throws(() => IndexLiveSchema.parse({ ...indexLiveWire, future_field: "surprise" }));
});

// ── /api/market/exchange-rate ────────────────────────────────────────

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
