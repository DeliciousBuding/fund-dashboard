import { describe, it, expect } from "vitest";
import { tradeMarkers } from "../series/tradeMarkers";

// Chart dates skip non-trading days (no 2024-01-04 weekend gap pattern here, but
// 2024-01-06/07 are a weekend with no chart point).
const dates = ["2024-01-02", "2024-01-03", "2024-01-05", "2024-01-08"];
const values = [1.0, 1.1, 1.2, 1.3];
const colors = { upColor: "#d63649", downColor: "#199c63", surfaceColor: "#ffffff" };

describe("tradeMarkers", () => {
  it("matches exact trade dates and splits buy(red)/sell(green)", () => {
    const r = tradeMarkers({
      dates,
      values,
      transactions: [
        { trade_time: "2024-01-03", direction: "buy" },
        { trade_time: "2024-01-08", direction: "sell" },
      ],
      ...colors,
    });
    expect(r.buys).toEqual([["2024-01-03", 1.1]]);
    expect(r.sells).toEqual([["2024-01-08", 1.3]]);
    expect(r.series).toHaveLength(2);
    const buy = r.series[0] as any;
    expect(buy.itemStyle.color).toBe("#d63649"); // buy = red (up/profit)
    expect(buy.z).toBe(100);
    const sell = r.series[1] as any;
    expect(sell.itemStyle.color).toBe("#199c63"); // sell = green (down/loss)
  });

  it("nearest-aligns trades on non-trading days to the next chart date", () => {
    // 2024-01-06 (Saturday) has no chart point → nearest = 2024-01-08
    const r = tradeMarkers({
      dates,
      values,
      transactions: [{ trade_time: "2024-01-06", direction: "buy" }],
      align: "nearest",
      ...colors,
    });
    expect(r.buys).toEqual([["2024-01-08", 1.3]]);
  });

  it("exact align drops trades with no matching chart date", () => {
    const r = tradeMarkers({
      dates,
      values,
      transactions: [{ trade_time: "2024-01-06", direction: "buy" }],
      align: "exact",
      ...colors,
    });
    expect(r.buys).toHaveLength(0);
    expect(r.series).toHaveLength(0);
  });

  it("only includes a series for the populated side", () => {
    const r = tradeMarkers({
      dates,
      values,
      transactions: [{ trade_time: "2024-01-02", direction: "buy" }],
      ...colors,
    });
    expect(r.series).toHaveLength(1);
    expect((r.series[0] as any).itemStyle.color).toBe("#d63649");
  });

  it("skips transactions without a trade_time", () => {
    const r = tradeMarkers({
      dates,
      values,
      transactions: [{ direction: "buy" }, { trade_time: "2024-01-02", direction: "buy" }],
      ...colors,
    });
    expect(r.buys).toHaveLength(1);
  });
});
