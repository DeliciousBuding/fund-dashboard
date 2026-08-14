import { describe, expect, it, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { MemoryRouter, useSearchParams } from "react-router-dom";
import type { ReactNode } from "react";
import { useQueryRange } from "../useQueryRange";

function wrapper(initial: string) {
  return function W({ children }: { children: ReactNode }) {
    return <MemoryRouter initialEntries={[initial]}>{children}</MemoryRouter>;
  };
}

describe("useQueryRange", () => {
  it("defaults when param missing", () => {
    const { result } = renderHook(() => useQueryRange("range", "tx", ["tx", "1m", "3m"] as const), {
      wrapper: wrapper("/fund/019173"),
    });
    expect(result.current[0]).toBe("tx");
  });

  it("reads allowed query value", () => {
    const { result } = renderHook(() => useQueryRange("range", "tx", ["tx", "1m", "3m"] as const), {
      wrapper: wrapper("/fund/019173?range=1m"),
    });
    expect(result.current[0]).toBe("1m");
  });

  it("falls back on disallowed value", () => {
    const { result } = renderHook(() => useQueryRange("range", "tx", ["tx", "1m"] as const), {
      wrapper: wrapper("/fund/019173?range=nope"),
    });
    expect(result.current[0]).toBe("tx");
  });

  it("writes and clears default from URL", () => {
    const { result } = renderHook(
      () => {
        const range = useQueryRange("range", "tx", ["tx", "1m", "3m"] as const);
        const [sp] = useSearchParams();
        return { range, qs: sp.toString() };
      },
      { wrapper: wrapper("/fund/019173") },
    );

    act(() => {
      result.current.range[1]("1m");
    });
    expect(result.current.range[0]).toBe("1m");
    expect(result.current.qs).toContain("range=1m");

    act(() => {
      result.current.range[1]("tx");
    });
    expect(result.current.range[0]).toBe("tx");
    expect(result.current.qs).not.toContain("range=");
  });
});
