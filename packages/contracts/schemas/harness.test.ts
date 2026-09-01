// harness.test.ts - source events strict wire-shape tests (node:test).
// Mirrors internal/httpapi/fund_response.go sourceEventJSON: all 12 keys are
// emitted (no omitempty; pointer fields are null); strict() rejects unknown keys.
import assert from "node:assert/strict";
import { test } from "node:test";

import { SourceEventSchema, SourceEventsResponseSchema } from "./harness.ts";

const sourceEventWire = {
  id: 7,
  title: "FOMC keeps rates unchanged",
  url: null,
  source: "web_search",
  snippet: "Decision summary",
  query: "rate decision",
  related_security_code: null,
  related_security_name: null,
  is_read: false,
  is_useful: false,
  fetched_at: "2026-08-29T10:00:00Z",
  created_at: "2026-08-29T09:59:00Z",
};

test("SourceEventSchema parses the real sourceEventJSON shape", () => {
  const parsed = SourceEventSchema.parse(sourceEventWire);
  assert.equal(parsed.id, 7);
  assert.equal(parsed.is_read, false);
  assert.equal(parsed.url, null);
});

test("SourceEventSchema rejects unknown fields (contract drift)", () => {
  assert.throws(() => SourceEventSchema.parse({ ...sourceEventWire, new_field: true }));
});

test("SourceEventsResponseSchema parses the REST wrapper", () => {
  const parsed = SourceEventsResponseSchema.parse({
    count: 1,
    decision_boundary: "facts_only",
    events: [sourceEventWire],
  });
  assert.equal(parsed.count, 1);
  assert.equal(parsed.events[0].title, "FOMC keeps rates unchanged");
});
