// auth.ts — /api/auth/sessions and /api/auth/events settings read surfaces
// v3.0 contracts SSOT — derived from internal/httpapi/auth.go,
// internal/auth/service.go (SessionInfo) and internal/auth/store.go (AuthEvent).
import { z } from "zod";

// SessionInfo is the redacted session view (design 06 §3). Every field is
// always emitted: no omitempty, and ip/user_agent degrade to "" rather than
// being dropped.
export const SessionInfoSchema = z.object({
  id_prefix: z.string(),
  created_at: z.number(),
  expires_at: z.number(),
  last_seen_at: z.number(),
  ip: z.string(),
  user_agent: z.string(),
  current: z.boolean(),
});
export type SessionInfo = z.infer<typeof SessionInfoSchema>;

// Go handler already normalizes a nil slice to [] before marshal, so the wire
// is always an array; the schema documents that invariant.
export const AuthSessionsResponseSchema = z.object({
  sessions: z.array(SessionInfoSchema),
});
export type AuthSessionsResponse = z.infer<typeof AuthSessionsResponseSchema>;

// ip/user_agent/detail carry omitempty in Go: empty values are omitted from
// the JSON object rather than serialized as "" or null.
export const AuthEventSchema = z.object({
  ts: z.number(),
  event: z.string(),
  ip: z.string().optional(),
  user_agent: z.string().optional(),
  detail: z.string().optional(),
});
export type AuthEvent = z.infer<typeof AuthEventSchema>;

export const AuthEventsResponseSchema = z.object({
  events: z.array(AuthEventSchema),
});
export type AuthEventsResponse = z.infer<typeof AuthEventsResponseSchema>;

// AuthStatus is GET /api/auth/status (internal/httpapi/auth.go handleAuthStatus).
// initialized/env_managed/authenticated are always emitted; session_expires_at is
// added only when a valid session cookie authenticates the request, so it is
// optional (unix seconds, session TTL).
export const AuthStatusSchema = z.object({
  initialized: z.boolean(),
  env_managed: z.boolean(),
  authenticated: z.boolean(),
  session_expires_at: z.number().optional(),
});
export type AuthStatus = z.infer<typeof AuthStatusSchema>;

// setup/login/logout all write {"ok":true} on success (map[string]any in Go).
export const AuthOkResponseSchema = z.object({
  ok: z.literal(true),
});
export type AuthOkResponse = z.infer<typeof AuthOkResponseSchema>;
