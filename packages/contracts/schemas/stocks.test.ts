// stocks.test.ts — /api/stocks/{code} flat SPA wire-shape tests (node:test).
// 对照 internal/httpapi/market.go usStockSPAResponse：19 个键恒下发
// （无 quote 时为 0/""/null/[] 默认值），error/message 仅在
// report.Error != "" 时下发。
import assert from "node:assert/strict";
import { test } from "node:test";

import { USStockHistoryPointSchema, USStockInfoSchema, USStockProfileSchema } from "./stocks.ts";

const profileWire = {
  sector: "Technology",
  industry: "Software",
  market_cap: 3.2e12,
  pe: 36.5,
  description: "Software and devices.",
};

const historyPointWire = {
  date: "2026-08-28",
  close: 410.5,
  change_pct: 1.2,
};

const usStockWire = {
  code: "MSFT",
  name: "Microsoft Corporation",
  market: "us",
  price: 410.5,
  previous_close: 405.4,
  change: 5.1,
  change_pct: 1.26,
  high: 412,
  low: 404,
  open: 406,
  volume: 2.3e7,
  currency: "USD",
  market_time: "2026-08-29T20:00:00Z",
  profile: profileWire,
  history: [historyPointWire],
  source: "yahoo_chart",
  decision_boundary: "facts_only",
  side_effects: "stock_realtime_upsert,stock_kline_upsert",
  external_fetch: "yahoo_chart",
};

test("USStockInfoSchema parses the real full shape", () => {
  const parsed = USStockInfoSchema.parse(usStockWire);
  assert.equal(parsed.price, 410.5);
  assert.equal(parsed.decision_boundary, "facts_only");
  assert.equal(parsed.profile?.market_cap, 3.2e12);
  assert.equal(parsed.history.length, 1);
});

test("USStockInfoSchema parses the real no-data shape (zeros + error/message)", () => {
  const parsed = USStockInfoSchema.parse({
    code: "",
    name: "",
    market: "us",
    price: 0,
    previous_close: 0,
    change: 0,
    change_pct: 0,
    high: 0,
    low: 0,
    open: 0,
    volume: 0,
    currency: "USD",
    market_time: "",
    profile: null,
    history: [],
    source: "not_performed",
    decision_boundary: "facts_only",
    side_effects: "none",
    external_fetch: "not_performed",
    error: "no_data",
    message: "symbol is required",
  });
  assert.equal(parsed.error, "no_data");
  assert.equal(parsed.message, "symbol is required");
  assert.equal(parsed.profile, null);
  assert.deepEqual(parsed.history, []);
});

test("USStockInfoSchema tolerates conditional error/message being absent", () => {
  const parsed = USStockInfoSchema.parse(usStockWire);
  assert.equal(parsed.error, undefined);
  assert.equal(parsed.message, undefined);
});

test("USStockInfoSchema requires decision_boundary (Go always emits it)", () => {
  const { decision_boundary, ...rest } = usStockWire;
  assert.throws(() => USStockInfoSchema.parse(rest));
});

test("USStockInfoSchema requires side_effects (Go always emits it)", () => {
  const { side_effects, ...rest } = usStockWire;
  assert.throws(() => USStockInfoSchema.parse(rest));
});

test("USStockInfoSchema requires external_fetch (Go always emits it)", () => {
  const { external_fetch, ...rest } = usStockWire;
  assert.throws(() => USStockInfoSchema.parse(rest));
});

test("USStockInfoSchema rejects unknown fields (contract drift)", () => {
  assert.throws(() => USStockInfoSchema.parse({ ...usStockWire, future_field: "surprise" }));
});

test("USStockProfileSchema accepts null market_cap/pe (Go nil pointers)", () => {
  const parsed = USStockProfileSchema.parse({
    ...profileWire,
    market_cap: null,
    pe: null,
  });
  assert.equal(parsed.market_cap, null);
  assert.equal(parsed.pe, null);
});

test("USStockProfileSchema requires sector (Go always emits it)", () => {
  const { sector, ...rest } = profileWire;
  assert.throws(() => USStockProfileSchema.parse(rest));
});

test("USStockProfileSchema rejects unknown fields", () => {
  assert.throws(() => USStockProfileSchema.parse({ ...profileWire, future_field: "surprise" }));
});

test("USStockHistoryPointSchema rejects a point missing close", () => {
  const { close, ...rest } = historyPointWire;
  assert.throws(() => USStockHistoryPointSchema.parse(rest));
});

test("USStockHistoryPointSchema rejects unknown fields", () => {
  assert.throws(() =>
    USStockHistoryPointSchema.parse({ ...historyPointWire, future_field: "surprise" }),
  );
});
