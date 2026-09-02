import { describe, expect, it } from "vitest";
import { safeOAuthReturn } from "./oauthReturn";

describe("safeOAuthReturn", () => {
  it("accepts an authorize path with a query string", () => {
    const value = "/oauth/authorize?client_id=abc&state=1";
    expect(safeOAuthReturn(value)).toBe(value);
  });

  it("keeps a legitimate deep oauth path", () => {
    expect(safeOAuthReturn("/oauth/consent?x=1")).toBe("/oauth/consent?x=1");
  });

  it("rejects empty and non-string input", () => {
    expect(safeOAuthReturn(null)).toBeNull();
    expect(safeOAuthReturn(undefined)).toBeNull();
    expect(safeOAuthReturn("")).toBeNull();
    expect(safeOAuthReturn("   ")).toBeNull();
    expect(safeOAuthReturn(42 as unknown as string)).toBeNull();
  });

  it("rejects anything outside the /oauth/ surface", () => {
    expect(safeOAuthReturn("/")).toBeNull();
    expect(safeOAuthReturn("/api/health")).toBeNull();
    expect(safeOAuthReturn("/login")).toBeNull();
    expect(safeOAuthReturn("/oauth")).toBeNull();
  });

  it("rejects scheme-relative and absolute URLs", () => {
    expect(safeOAuthReturn("//evil.example/oauth/authorize")).toBeNull();
    expect(safeOAuthReturn("https://evil.example/oauth/authorize")).toBeNull();
    expect(safeOAuthReturn("http://evil.example/")).toBeNull();
    expect(safeOAuthReturn("javascript:alert(1)")).toBeNull();
    expect(safeOAuthReturn("data:text/html,<script>")).toBeNull();
  });

  // A browser normalizes dot segments before navigating, so validating only the
  // raw prefix would let the login page redirect anywhere on this origin.
  it("rejects dot-segment traversal that escapes /oauth/", () => {
    expect(safeOAuthReturn("/oauth/../api/admin")).toBeNull();
    expect(safeOAuthReturn("/oauth/authorize/../../api/admin")).toBeNull();
    expect(safeOAuthReturn("/oauth/%2e%2e/api/admin")).toBeNull();
  });

  it("rejects header and response splitting characters", () => {
    expect(safeOAuthReturn("/oauth/authorize\r\nSet-Cookie: x=1")).toBeNull();
    expect(safeOAuthReturn("/oauth/authorize\nx")).toBeNull();
    expect(safeOAuthReturn("/oauth/authorize\tx")).toBeNull();
  });

  it("rejects overlong values", () => {
    expect(safeOAuthReturn(`/oauth/${"a".repeat(4000)}`)).toBeNull();
  });

  it("drops a fragment", () => {
    expect(safeOAuthReturn("/oauth/authorize?state=1#frag")).toBe("/oauth/authorize?state=1");
  });

  it("preserves encoded query values", () => {
    const value = "/oauth/authorize?redirect_uri=https%3A%2F%2Fchatgpt.com%2Fcb";
    expect(safeOAuthReturn(value)).toBe(value);
  });
});
