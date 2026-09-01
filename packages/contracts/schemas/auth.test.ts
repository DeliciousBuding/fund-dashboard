// auth.test.ts — /api/auth/sessions + /api/auth/events wire-shape tests (node:test).
// 对照 internal/auth/service.go (SessionInfo) 与 store.go (AuthEvent) 的
// omitempty 分支：ip/user_agent/detail 为空时从 JSON 对象整体省略。
import assert from "node:assert/strict";
import { test } from "node:test";

import {
  AuthEventSchema,
  AuthEventsResponseSchema,
  AuthOkResponseSchema,
  AuthSessionsResponseSchema,
  AuthStatusSchema,
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
  const parsed = AuthSessionsResponseSchema.parse({
    sessions: [sessionWire],
    total: 1,
    truncated: false,
  });
  assert.equal(parsed.sessions.length, 1);
  assert.equal(parsed.total, 1);
  assert.equal(parsed.truncated, false);
});

test("AuthSessionsResponseSchema passes the truncated page shape (200 of N)", () => {
  const parsed = AuthSessionsResponseSchema.parse({
    sessions: Array.from({ length: 200 }, (_, i) => ({
      ...sessionWire,
      id_prefix: `a1b2c3d${i}`,
      current: false,
    })),
    total: 205,
    truncated: true,
  });
  assert.equal(parsed.sessions.length, 200);
  assert.equal(parsed.total, 205);
  assert.equal(parsed.truncated, true);
});

test("AuthSessionsResponseSchema rejects missing total/truncated", () => {
  assert.throws(() => AuthSessionsResponseSchema.parse({ sessions: [sessionWire] }));
  assert.throws(() =>
    AuthSessionsResponseSchema.parse({
      sessions: [sessionWire],
      total: 1,
    }),
  );
  assert.throws(() =>
    AuthSessionsResponseSchema.parse({
      sessions: [sessionWire],
      truncated: false,
    }),
  );
});

test("AuthSessionsResponseSchema rejects negative/non-integer total", () => {
  assert.throws(() =>
    AuthSessionsResponseSchema.parse({
      sessions: [sessionWire],
      total: -1,
      truncated: true,
    }),
  );
  assert.throws(() =>
    AuthSessionsResponseSchema.parse({
      sessions: [sessionWire],
      total: 1.5,
      truncated: true,
    }),
  );
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

// ── /api/auth/status ────────────────────────────────────────────────

const statusWire = {
  initialized: true,
  env_managed: false,
  authenticated: true,
  session_expires_at: 1756086400,
};

test("AuthStatusSchema parses the authenticated handler payload", () => {
  const parsed = AuthStatusSchema.parse(statusWire);
  assert.equal(parsed.initialized, true);
  assert.equal(parsed.session_expires_at, 1756086400);
});

test("AuthStatusSchema accepts unauthenticated payload (no session_expires_at)", () => {
  // handleAuthStatus only adds session_expires_at when a session authenticates.
  const parsed = AuthStatusSchema.parse({
    initialized: true,
    env_managed: false,
    authenticated: false,
  });
  assert.equal(parsed.session_expires_at, undefined);
});

test("AuthStatusSchema rejects a missing required flag", () => {
  assert.throws(() => AuthStatusSchema.parse({ initialized: true }));
});

// ── /api/auth/setup|login|logout ok envelope ────────────────────────

test("AuthOkResponseSchema passes the {ok:true} success envelope", () => {
  assert.deepEqual(AuthOkResponseSchema.parse({ ok: true }), { ok: true });
});

test("AuthOkResponseSchema rejects ok:false", () => {
  assert.throws(() => AuthOkResponseSchema.parse({ ok: false }));
});
