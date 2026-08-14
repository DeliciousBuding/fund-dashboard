import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import React from "react";
import { useEChart } from "../useEChart";

const setOption = vi.fn();
const dispose = vi.fn();
const resize = vi.fn();

vi.mock("echarts/core", () => ({
  init: vi.fn(() => ({
    setOption,
    dispose,
    resize,
  })),
}));

import { init } from "echarts/core";

function ChartHost({ option, deps }: { option: Record<string, unknown>; deps: unknown[] }) {
  const ref = useEChart(option, deps);
  return <div data-testid="chart-host" ref={ref} />;
}

describe("useEChart", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      configurable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
    Object.defineProperty(window, "devicePixelRatio", {
      writable: true,
      configurable: true,
      value: 2.5,
    });
  });

  it("inits with canvas renderer and devicePixelRatio", async () => {
    const option = { series: [{ type: "bar", data: [1] }] };
    render(<ChartHost option={option} deps={[option]} />);
    await waitFor(() => {
      expect(init).toHaveBeenCalled();
    });
    const args = (init as unknown as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(args[0]).toBeInstanceOf(HTMLDivElement);
    expect(args[2]).toMatchObject({
      devicePixelRatio: 2.5,
      renderer: "canvas",
    });
    expect(setOption).toHaveBeenCalledWith(option, true);
  });

  it("caps devicePixelRatio at 3", async () => {
    Object.defineProperty(window, "devicePixelRatio", {
      writable: true,
      configurable: true,
      value: 4,
    });
    const option = { series: [{ type: "line", data: [3] }] };
    render(<ChartHost option={option} deps={[option]} />);
    await waitFor(() => expect(init).toHaveBeenCalled());
    const opts = (init as unknown as ReturnType<typeof vi.fn>).mock.calls[0][2];
    expect(opts.devicePixelRatio).toBe(3);
  });

  it("setOption again when option deps change without re-init", async () => {
    const opt1 = { series: [{ type: "line", data: [1] }] };
    const opt2 = { series: [{ type: "line", data: [9] }] };
    const { rerender } = render(<ChartHost option={opt1} deps={[opt1]} />);
    await waitFor(() => expect(setOption).toHaveBeenCalledTimes(1));
    rerender(<ChartHost option={opt2} deps={[opt2]} />);
    await waitFor(() => expect(setOption).toHaveBeenCalledTimes(2));
    expect(init).toHaveBeenCalledTimes(1);
    expect(setOption).toHaveBeenLastCalledWith(opt2, true);
  });

  it("disposes on unmount", async () => {
    const option = { series: [{ type: "line", data: [1] }] };
    const { unmount } = render(<ChartHost option={option} deps={[option]} />);
    await waitFor(() => expect(init).toHaveBeenCalled());
    unmount();
    expect(dispose).toHaveBeenCalled();
  });

  it("skips init for empty option", async () => {
    render(<ChartHost option={{}} deps={[{}]} />);
    // allow effects to flush
    await waitFor(() => {
      expect(init).not.toHaveBeenCalled();
    });
  });
});
