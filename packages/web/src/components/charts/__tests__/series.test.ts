import { describe, it, expect } from "vitest";
import { opacity } from "../../../styles/theme";
import { lineSeries } from "../series/lineSeries";
import { barSeries } from "../series/barSeries";
import { scatterSeries } from "../series/scatterSeries";
import { radarSeries } from "../series/radarSeries";

describe("lineSeries", () => {
  it("builds a smooth line with no symbol and the given color", () => {
    const s = lineSeries({ name: "市值", data: [1, 2, 3], color: "#3172d9" }) as any;
    expect(s.type).toBe("line");
    expect(s.smooth).toBe(true);
    expect(s.symbol).toBe("none");
    expect(s.lineStyle.color).toBe("#3172d9");
    expect(s.areaStyle).toBeUndefined();
  });

  it("adds a gradient area when area=true (absorbs AreaSeries)", () => {
    const s = lineSeries({ name: "净值", data: [1, 2], color: "#3172d9", area: true }) as any;
    expect(s.areaStyle).toBeDefined();
    const stops = s.areaStyle.color.colorStops;
    expect(stops).toHaveLength(2);
    expect(stops[0].color).toMatch(/rgba\(49,\s*114,\s*217,\s*0\.22\)/);
    expect(stops[1].color).toMatch(/rgba\(49,\s*114,\s*217,\s*0\)/);
  });

  it("respects dashed/dotted/yAxisIndex/markLine", () => {
    const dotted = lineSeries({ name: "MA20", data: [1], color: "#e07b2c", dotted: true, opacity: opacity.seriesSoft }) as any;
    expect(dotted.lineStyle.type).toBe("dotted");
    expect(dotted.lineStyle.opacity).toBe(opacity.seriesSoft);

    const dashed = lineSeries({ name: "成本", data: [1], color: "#e07b2c", dashed: true, yAxisIndex: 1, markLine: { data: [] } }) as any;
    expect(dashed.lineStyle.type).toBe("dashed");
    expect(dashed.yAxisIndex).toBe(1);
    expect(dashed.markLine).toEqual({ data: [] });
  });

  it("forwards markPoint and z", () => {
    const s = lineSeries({
      name: "收益", data: [1, 2], color: "#3172d9",
      markPoint: { data: [{ type: "max" }] },
      z: 10,
    }) as any;
    expect(s.markPoint).toEqual({ data: [{ type: "max" }] });
    expect(s.z).toBe(10);
  });
});

describe("barSeries", () => {
  it("colors per-value red-up/green-down (CN convention), comparing after Number()", () => {
    const s = barSeries({ name: "盈亏", data: [100, -50, "20"], upDown: { up: "#d63649", down: "#199c63" } }) as any;
    const color = s.itemStyle.color;
    expect(color({ value: 100 })).toBe("#d63649");
    expect(color({ value: -50 })).toBe("#199c63");
    // string values compared after Number() (handoff §6)
    expect(color({ value: "20" })).toBe("#d63649");
    expect(color({ value: "-5" })).toBe("#199c63");
  });

  it("supports a fixed color + borderRadius", () => {
    const s = barSeries({ name: "x", data: [1], color: "#abcdef", borderRadius: [2, 2, 0, 0] }) as any;
    expect(s.itemStyle.color).toBe("#abcdef");
    expect(s.itemStyle.borderRadius).toEqual([2, 2, 0, 0]);
  });

  it("forwards yAxisIndex + barWidth", () => {
    const s = barSeries({ name: "x", data: [1], upDown: { up: "u", down: "d" }, yAxisIndex: 1, barWidth: "60%" }) as any;
    expect(s.yAxisIndex).toBe(1);
    expect(s.barWidth).toBe("60%");
  });
});

describe("scatterSeries", () => {
  it("builds a scatter with default symbolSize 8", () => {
    const s = scatterSeries({ name: "买入", data: [["d", 1]], color: "#d63649" }) as any;
    expect(s.type).toBe("scatter");
    expect(s.symbolSize).toBe(8);
    expect(s.itemStyle.color).toBe("#d63649");
  });

  it("applies a surface border + z", () => {
    const s = scatterSeries({ name: "x", data: [], color: "#d63649", borderColor: "#ffffff", borderWidth: 2, z: 100 }) as any;
    expect(s.itemStyle.borderColor).toBe("#ffffff");
    expect(s.itemStyle.borderWidth).toBe(2);
    expect(s.z).toBe(100);
  });

  it("forwards symbol shape", () => {
    const circle = scatterSeries({ name: "buy", data: [], color: "#d63649", symbol: "circle" }) as any;
    expect(circle.symbol).toBe("circle");
    const diamond = scatterSeries({ name: "sell", data: [], color: "#199c63", symbol: "diamond" }) as any;
    expect(diamond.symbol).toBe("diamond");
  });
});

describe("radarSeries", () => {
  it("builds radar + series with indicators and colored rings", () => {
    const result = radarSeries({
      indicators: [
        { name: "Return", max: 50 },
        { name: "Volatility", max: 40 },
        { name: "Sharpe", max: 3 },
      ],
      data: [
        { name: "Fund A", value: [1, 2, 3], color: "#3172d9" },
        { name: "Fund B", value: [4, 5, 6], color: "#d63649" },
      ],
    }) as any;

    // radar axis config
    expect(result.radar.indicator).toEqual([
      { name: "Return", max: 50 },
      { name: "Volatility", max: 40 },
      { name: "Sharpe", max: 3 },
    ]);
    expect(result.radar.shape).toBe("polygon");
    expect(result.radar.center).toEqual(["50%", "48%"]);
    expect(result.radar.radius).toBe("65%");

    // series
    expect(result.series).toHaveLength(1);
    const s = result.series[0];
    expect(s.type).toBe("radar");
    expect(s.data).toHaveLength(2);

    const a = s.data[0];
    expect(a.name).toBe("Fund A");
    expect(a.value).toEqual([1, 2, 3]);
    expect(a.lineStyle).toEqual({ color: "#3172d9", width: 2 });
    expect(a.symbol).toBe("circle");
    expect(a.symbolSize).toBe(4);
    expect(a.itemStyle).toEqual({ color: "#3172d9" });
    expect(a.areaStyle.color).toMatch(/rgba\(49,\s*114,\s*217,\s*0\.08\)/);

    const b = s.data[1];
    expect(b.name).toBe("Fund B");
    expect(b.areaStyle.color).toMatch(/rgba\(214,\s*54,\s*73,\s*0\.08\)/);
  });

  it("respects shape / center / radius overrides", () => {
    const result = radarSeries({
      indicators: [{ name: "X", max: 10 }],
      data: [{ name: "A", value: [5], color: "#3172d9" }],
      shape: "circle",
      center: ["50%", "50%"],
      radius: "80%",
    }) as any;
    expect(result.radar.shape).toBe("circle");
    expect(result.radar.center).toEqual(["50%", "50%"]);
    expect(result.radar.radius).toBe("80%");
  });

  it("uses fallback color when none provided", () => {
    const result = radarSeries({
      indicators: [{ name: "X", max: 10 }],
      data: [{ name: "NoColor", value: [10] }],
    }) as any;
    expect(result.series[0].data[0].lineStyle.color).toBeUndefined();
    expect(result.series[0].data[0].areaStyle.color).toMatch(/rgba\(49,\s*114,\s*217,\s*0\.08\)/);
  });
});
