import { describe, expect, it } from "vitest";
import { DIRECTION_LABEL, transactionsToCsv } from "./csv";
import type { TransactionListItem } from "./queries";

function makeRow(overrides: Partial<TransactionListItem> = {}): TransactionListItem {
  return {
    seq: 1,
    trade_time: "2026-08-29T10:30:45Z",
    confirm_date: "2026-08-31",
    direction: "buy",
    trade_type: "申购",
    fund_code: "000001",
    fund_name: "示例基金",
    amount: 1000,
    shares: 512.3456,
    fee: 1.5,
    order_id: "ORD-1",
    anomaly: null,
    settlement_days: 2,
    portfolio_id: 1,
    ...overrides,
  };
}

function dataRows(csv: string): string[] {
  return csv.replace("\uFEFF", "").split("\n").slice(1);
}

describe("transactionsToCsv", () => {
  it("emits UTF-8 BOM + Chinese header row", () => {
    expect(transactionsToCsv([])).toBe(
      "\uFEFF交易时间,确认日期,方向,类型,金额,份额,手续费,结算,单号,备注",
    );
  });

  it("renders a full row: quoted cells, truncated time, CN direction, 2dp numbers, T+N", () => {
    const rows = dataRows(transactionsToCsv([makeRow()]));
    expect(rows).toHaveLength(1);
    expect(rows[0]).toBe(
      '"2026-08-29T10:30","2026-08-31","买入","申购","1000.00","512.35","1.50","T+2","ORD-1",""',
    );
  });

  it("escapes embedded double quotes by doubling them", () => {
    const rows = dataRows(transactionsToCsv([makeRow({ order_id: 'A"B' })]));
    expect(rows[0]).toContain('"A""B"');
  });

  it("guards formula injection: = + - @ prefixed text gets a leading quote", () => {
    const csv = transactionsToCsv([
      makeRow({ order_id: "=cmd" }),
      makeRow({ order_id: "+calc" }),
      makeRow({ order_id: "-sum" }),
      makeRow({ anomaly: "@attr" }),
    ]);
    expect(csv).toContain('"\'=cmd"');
    expect(csv).toContain('"\'+calc"');
    expect(csv).toContain('"\'-sum"');
    expect(csv).toContain('"\'@attr"');
  });

  it("maps known directions to CN labels and guards unknown passthrough", () => {
    expect(DIRECTION_LABEL).toEqual({
      buy: "买入",
      sell: "卖出",
      dividend: "分红",
      convert_in: "转换转入",
      convert_out: "转换转出",
      forced_redeem: "强制赎回",
    });
    const rows = dataRows(transactionsToCsv([makeRow({ direction: "sell" })]));
    expect(rows[0]).toContain('"卖出"');
    // 未知方向原样透传，但仍走公式防护
    const weird = dataRows(transactionsToCsv([makeRow({ direction: "@weird" })]));
    expect(weird[0]).toContain('"\'@weird"');
  });

  it("renders null fields as empty cells", () => {
    const rows = dataRows(
      transactionsToCsv([
        makeRow({
          trade_time: null,
          confirm_date: null,
          direction: null,
          trade_type: null,
          amount: null,
          shares: null,
          fee: null,
          order_id: null,
          anomaly: null,
          settlement_days: null,
        }),
      ]),
    );
    expect(rows[0]).toBe('"","","","","","","","","",""');
  });

  it("hides zero/negative fee and missing settlement", () => {
    const rows = dataRows(transactionsToCsv([makeRow({ fee: 0, settlement_days: null })]));
    const cells = rows[0]?.split(",") ?? [];
    expect(cells[6]).toBe('""'); // 手续费列
    expect(cells[7]).toBe('""'); // 结算列
  });
});
