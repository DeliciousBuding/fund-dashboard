// funds.test.ts — NAV history wire-shape tests (node:test).
// 对照 internal/service/portfolio/detail.go GetNavHistory / NavHistoryPoint：
// daily_change_pct 无 omitempty（null 表示无涨跌幅），security_type 经
// COALESCE(security_type,'fund') 后恒为字符串。
import assert from "node:assert/strict";
import { test } from "node:test";

import { NavPointSchema } from "./funds.ts";

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
