import { describe, expect, it } from "vitest";
import { fmtCNY, fmtPct, fmtSignedCNY, fmtSignedPct, pnlTone } from "./format";

describe("format", () => {
  it("fmtCNY renders CNY with grouping and 2dp", () => {
    expect(fmtCNY(1234567.891)).toBe("¥1,234,567.89");
    expect(fmtCNY(0)).toBe("¥0.00");
  });

  it("fmtCNY tolerates null/undefined/NaN", () => {
    expect(fmtCNY(null)).toBe("—");
    expect(fmtCNY(undefined)).toBe("—");
    expect(fmtCNY(Number.NaN)).toBe("—");
  });

  it("fmtPct renders percent with digits", () => {
    expect(fmtPct(12.345)).toBe("12.35%");
    expect(fmtPct(-4.2, 1)).toBe("-4.2%");
    expect(fmtPct(null)).toBe("—");
  });

  it("fmtSigned* add explicit plus for gains", () => {
    expect(fmtSignedPct(1.234)).toBe("+1.23%");
    expect(fmtSignedPct(-1.234)).toBe("-1.23%");
    expect(fmtSignedPct(0)).toBe("0.00%");
    expect(fmtSignedCNY(120)).toBe("+¥120.00");
    expect(fmtSignedCNY(-120)).toBe("-¥120.00");
  });

  it("pnlTone maps sign to CN convention tone", () => {
    expect(pnlTone(1)).toBe("up");
    expect(pnlTone(-1)).toBe("down");
    expect(pnlTone(0)).toBe("flat");
    expect(pnlTone(null)).toBe("flat");
  });
});
