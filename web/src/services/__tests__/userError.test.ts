import { describe, expect, it } from "vitest";
import { sanitizeUserError } from "../userError";

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
