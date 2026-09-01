// auth.test.ts — /api/auth/sessions + /api/auth/events wire-shape tests (node:test).
// 对照 internal/auth/service.go (SessionInfo) 与 store.go (AuthEvent) 的
// omitempty 分支：ip/user_agent/detail 为空时从 JSON 对象整体省略。
import assert from "node:assert/strict";
import { test } from "node:test";

import {
  AuthEventSchema,
  AuthEventsResponseSchema,
  AuthSessionsResponseSchema,
  SessionInfoSchema,
} from "./auth.ts";

const sessionWire = {
  id_prefix: "a1b2c3d4",
  created_at: 1756000000,
  expires_at: 1756086400,
  last_seen_at: 1756050000,
  ip: "192.0.2.10",
  user_agent: "Mozilla/5.0 Test",
  current: true,
};

test("SessionInfoSchema parses the full redacted session view", () => {
  const parsed = SessionInfoSchema.parse(sessionWire);
  assert.equal(parsed.id_prefix, "a1b2c3d4");
  assert.equal(parsed.current, true);
});

test("SessionInfoSchema accepts empty ip/user_agent (Go stores '' not null)", () => {
  const parsed = SessionInfoSchema.parse({
    ...sessionWire,
    ip: "",
    user_agent: "",
    current: false,
  });
  assert.equal(parsed.ip, "");
  assert.equal(parsed.user_agent, "");
});

test("AuthSessionsResponseSchema passes the handler array shape through", () => {
  const parsed = AuthSessionsResponseSchema.parse({ sessions: [sessionWire] });
  assert.equal(parsed.sessions.length, 1);
});

test("AuthEventSchema accepts omitempty branch (only ts + event present)", () => {
  // Go omitempty drops ip/user_agent/detail when empty.
  const parsed = AuthEventSchema.parse({ ts: 1756000000, event: "login_ok" });
  assert.equal(parsed.ip, undefined);
  assert.equal(parsed.user_agent, undefined);
  assert.equal(parsed.detail, undefined);
});

test("AuthEventSchema accepts full audit row", () => {
  const parsed = AuthEventSchema.parse({
    ts: 1756000001,
    event: "lockout",
    ip: "198.51.100.7",
    user_agent: "cli/1",
    detail: "retry_after=901",
  });
  assert.equal(parsed.event, "lockout");
  assert.equal(parsed.detail, "retry_after=901");
});

test("AuthEventsResponseSchema passes arrays through", () => {
  const parsed = AuthEventsResponseSchema.parse({ events: [{ ts: 1, event: "setup" }] });
  assert.deepEqual(parsed.events, [{ ts: 1, event: "setup" }]);
});

test("AuthSessionsResponseSchema rejects a missing required field", () => {
  assert.throws(() => SessionInfoSchema.parse({ id_prefix: "x" }));
});
