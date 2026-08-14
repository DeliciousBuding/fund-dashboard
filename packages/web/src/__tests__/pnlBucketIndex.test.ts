import { describe, it, expect } from "vitest";
import { pnlBucketIndex, PNL_BUCKETS } from "../components/PnLDistributionChart";

describe("pnlBucketIndex", () => {
  it("places exact edges into the higher-band bucket (half-open)", () => {
    // -30 lands in loss_20_30 (not deep-loss)
    expect(PNL_BUCKETS[pnlBucketIndex(-30)!].key).toBe("loss_20_30");
    // -10 lands in loss_0_10
    expect(PNL_BUCKETS[pnlBucketIndex(-10)!].key).toBe("loss_0_10");
    // 0 lands in gain_0_10
    expect(PNL_BUCKETS[pnlBucketIndex(0)!].key).toBe("gain_0_10");
    // +10 lands in gain_10_20
    expect(PNL_BUCKETS[pnlBucketIndex(10)!].key).toBe("gain_10_20");
    // +30 lands in gain_30plus
    expect(PNL_BUCKETS[pnlBucketIndex(30)!].key).toBe("gain_30plus");
  });

  it("covers deep loss and strong gain interiors", () => {
    expect(PNL_BUCKETS[pnlBucketIndex(-50)!].key).toBe("loss_30plus");
    expect(PNL_BUCKETS[pnlBucketIndex(45)!].key).toBe("gain_30plus");
    expect(PNL_BUCKETS[pnlBucketIndex(5)!].key).toBe("gain_0_10");
    expect(PNL_BUCKETS[pnlBucketIndex(-5)!].key).toBe("loss_0_10");
  });

  it("returns null for non-finite", () => {
    expect(pnlBucketIndex(Number.NaN)).toBeNull();
    expect(pnlBucketIndex(Number.POSITIVE_INFINITY)).toBeNull();
  });
});
