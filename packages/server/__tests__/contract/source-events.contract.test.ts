// Contract tests — verify runtime API shapes match @fund-dashboard/contracts schemas.
// These are pure (no DB, no network): they guard against field drift (G1/G2/G3)
// and the source-events wrapping bug (C-1/G6) fixed in Phase A.
import { describe, it, expect } from "bun:test";
import {
  SourceEventsResponseSchema,
  SourceEventSchema,
  PortfolioSchema,
  InvestmentHarnessSnapshotSchema,
  ApiErrorSchema,
} from "@fund-dashboard/contracts";

describe("source-events contract (fixes C-1/G6 + G3)", () => {
  const validEvent = {
    id: 1,
    title: "Fed holds rates",
    url: "https://example.com/fed",
    source: "websearch",
    snippet: "The Federal Reserve...",
    query: "fed rate decision",
    related_security_code: null,
    related_security_name: null,
    is_read: false,
    is_useful: false,
    fetched_at: "2026-06-21 10:00:00",
    created_at: "2026-06-21 10:00:00",
  };

  it("accepts the wrapped {count, decision_boundary, events} shape", () => {
    const payload = {
      count: 1,
      decision_boundary: "facts_only" as const,
      events: [validEvent],
    };
    const parsed = SourceEventsResponseSchema.safeParse(payload);
    expect(parsed.success).toBe(true);
  });

  it("requires created_at on each event (G3 fix)", () => {
    const { created_at: _omit, ...noCreated } = validEvent;
    expect(SourceEventSchema.safeParse(noCreated).success).toBe(false);
  });

  it("requires booleans for is_read/is_useful (backend must normalize DB 0/1)", () => {
    const intEvent = { ...validEvent, is_read: 1, is_useful: 0 };
    expect(SourceEventSchema.safeParse(intEvent).success).toBe(false);
  });
});

describe("PortfolioSchema (G1 fix)", () => {
  it("declares unique_stocks", () => {
    expect(PortfolioSchema.shape.unique_stocks).toBeDefined();
  });

  it("declares by_security_type", () => {
    expect(PortfolioSchema.shape.by_security_type).toBeDefined();
  });
});

describe("InvestmentHarnessSnapshotSchema (G2 fix)", () => {
  it("declares data_quality.holdings_coverage_pct", () => {
    expect(
      InvestmentHarnessSnapshotSchema.shape.data_quality.shape
        .holdings_coverage_pct,
    ).toBeDefined();
  });
});

describe("ApiErrorSchema (G8 unified error)", () => {
  it("accepts {error} only", () => {
    expect(ApiErrorSchema.safeParse({ error: "boom" }).success).toBe(true);
  });

  it("accepts {error, message, code}", () => {
    expect(
      ApiErrorSchema.safeParse({ error: "boom", message: "detail", code: "E1" })
        .success,
    ).toBe(true);
  });
});
