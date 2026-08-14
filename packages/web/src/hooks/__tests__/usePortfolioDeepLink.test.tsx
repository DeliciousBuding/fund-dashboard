import { describe, expect, it, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { MemoryRouter, useSearchParams } from "react-router-dom";
import type { ReactNode } from "react";
import {
  applyPortfolioToSearchParams,
  parsePortfolioParam,
  usePortfolioDeepLink,
  DEFAULT_PORTFOLIO_ID,
} from "../usePortfolioDeepLink";

function wrapper(initial: string) {
  return function W({ children }: { children: ReactNode }) {
    return <MemoryRouter initialEntries={[initial]}>{children}</MemoryRouter>;
  };
}

describe("parsePortfolioParam", () => {
  it("returns null for missing/empty", () => {
    expect(parsePortfolioParam(null)).toBeNull();
    expect(parsePortfolioParam("")).toBeNull();
  });

  it("accepts positive integers", () => {
    expect(parsePortfolioParam("1")).toBe(1);
    expect(parsePortfolioParam("42")).toBe(42);
  });

  it("rejects non-positive / non-integer / NaN", () => {
    expect(parsePortfolioParam("0")).toBeNull();
    expect(parsePortfolioParam("-3")).toBeNull();
    expect(parsePortfolioParam("1.5")).toBeNull();
    expect(parsePortfolioParam("abc")).toBeNull();
  });
});

describe("applyPortfolioToSearchParams", () => {
  it("omits default id and clears existing param", () => {
    const prev = new URLSearchParams("tab=chart&portfolio=2");
    const next = applyPortfolioToSearchParams(prev, DEFAULT_PORTFOLIO_ID);
    expect(next).not.toBeNull();
    expect(next!.get("portfolio")).toBeNull();
    expect(next!.get("tab")).toBe("chart");
  });

  it("no-ops when default already absent", () => {
    const prev = new URLSearchParams("tab=chart");
    expect(applyPortfolioToSearchParams(prev, 1)).toBeNull();
  });

  it("sets non-default portfolio", () => {
    const prev = new URLSearchParams("tab=allocation");
    const next = applyPortfolioToSearchParams(prev, 3);
    expect(next!.get("portfolio")).toBe("3");
    expect(next!.get("tab")).toBe("allocation");
  });

  it("no-ops when value already matches", () => {
    const prev = new URLSearchParams("portfolio=3");
    expect(applyPortfolioToSearchParams(prev, 3)).toBeNull();
  });
});

describe("usePortfolioDeepLink", () => {
  it("initializes store from ?portfolio=", () => {
    const setId = vi.fn();
    renderHook(() => usePortfolioDeepLink(1, setId), {
      wrapper: wrapper("/?portfolio=2&tab=chart"),
    });
    expect(setId).toHaveBeenCalledWith(2);
  });

  it("ignores invalid portfolio param", () => {
    const setId = vi.fn();
    renderHook(() => usePortfolioDeepLink(1, setId), {
      wrapper: wrapper("/?portfolio=nope"),
    });
    expect(setId).not.toHaveBeenCalled();
  });

  it("writes non-default id into URL and clears default", () => {
    const setId = vi.fn();
    const { result, rerender } = renderHook(
      ({ id }: { id: number }) => {
        const link = usePortfolioDeepLink(id, setId);
        const [sp] = useSearchParams();
        return { link, qs: sp.toString() };
      },
      { wrapper: wrapper("/?tab=chart"), initialProps: { id: 1 } },
    );

    // default: no portfolio= in URL
    expect(result.current.qs).not.toContain("portfolio=");
    expect(result.current.qs).toContain("tab=chart");

    rerender({ id: 4 });
    expect(result.current.qs).toContain("portfolio=4");
    expect(result.current.qs).toContain("tab=chart");

    rerender({ id: 1 });
    expect(result.current.qs).not.toContain("portfolio=");
    expect(result.current.qs).toContain("tab=chart");
  });

  it("handlePortfolioChange forwards valid ids", () => {
    const setId = vi.fn();
    const { result } = renderHook(() => usePortfolioDeepLink(1, setId), {
      wrapper: wrapper("/"),
    });
    act(() => {
      result.current.handlePortfolioChange(7);
    });
    expect(setId).toHaveBeenCalledWith(7);
  });
});
