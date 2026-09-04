import { describe, expect, it } from "vitest";
import { z } from "zod";
import { ApiError } from "../../lib/api";
import { mutationErrorMessage, sanitizeUserError } from "../userError";

describe("sanitizeUserError (#188)", () => {
  it("passes short user-safe messages through", () => {
    expect(sanitizeUserError(new Error("持仓不存在"), "fallback")).toBe("持仓不存在");
  });

  it("replaces technical dumps with fallback", () => {
    expect(
      sanitizeUserError(new Error("Expected string, received number at path.foo"), "加载失败"),
    ).toBe("加载失败");
    expect(sanitizeUserError(new Error("Failed to fetch"), "加载失败")).toBe("加载失败");
    expect(sanitizeUserError(new Error("HTTP 500: Internal Server Error"), "加载失败")).toBe(
      "加载失败",
    );
  });
});

describe("mutationErrorMessage", () => {
  it("maps known API error codes to Chinese copy", () => {
    expect(mutationErrorMessage(new ApiError(400, "weak_password"), "操作失败")).toBe(
      "密码至少 10 个字符",
    );
    expect(mutationErrorMessage(new ApiError(401, "invalid_credentials"), "操作失败")).toBe(
      "密码不正确",
    );
    expect(mutationErrorMessage(new ApiError(429, "rate_limited"), "操作失败")).toBe(
      "尝试过于频繁，请稍后再试",
    );
    expect(mutationErrorMessage(new ApiError(409, "not_initialized"), "操作失败")).toBe(
      "尚未初始化，请先设置密码",
    );
    expect(mutationErrorMessage(new ApiError(409, "already_initialized"), "操作失败")).toBe(
      "已初始化，请直接登录",
    );
    expect(mutationErrorMessage(new ApiError(409, "auth_env_managed"), "操作失败")).toBe(
      "密码由部署环境变量管理，请直接登录",
    );
  });

  it("replaces unknown machine codes and synthesized http_* codes with fallback", () => {
    expect(mutationErrorMessage(new ApiError(500, "http_500"), "操作失败")).toBe("操作失败");
    expect(mutationErrorMessage(new ApiError(500, "internal_failure"), "操作失败")).toBe(
      "操作失败",
    );
  });

  it("passes short human-readable API error text through", () => {
    expect(mutationErrorMessage(new ApiError(409, "组合不存在"), "操作失败")).toBe("组合不存在");
  });

  it("never dumps ZodError payloads", () => {
    const result = z.object({ a: z.string() }).safeParse({ a: 123 });
    if (result.success) throw new Error("expected parse failure");
    const msg = mutationErrorMessage(result.error, "保存失败");
    expect(msg).toBe("保存失败");
    expect(msg).not.toContain("expected");
    expect(msg).not.toContain("string");
  });

  it("replaces network errors with fallback", () => {
    expect(mutationErrorMessage(new TypeError("Failed to fetch"), "保存失败")).toBe("保存失败");
    expect(mutationErrorMessage(undefined, "保存失败")).toBe("保存失败");
    expect(mutationErrorMessage(new Error("Expected string, received number"), "保存失败")).toBe(
      "保存失败",
    );
  });
});
