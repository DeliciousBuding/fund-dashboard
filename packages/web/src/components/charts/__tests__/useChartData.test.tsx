import { describe, it, expect, vi, afterEach } from "vitest";
import { render, waitFor, act } from "@testing-library/react";
import { useChartData } from "../useChartData";

// Minimal harness exposing hook state via testids + a refetch button.
function Harness({
  fetcher,
  enabled,
}: {
  fetcher: (signal: AbortSignal) => Promise<unknown>;
  enabled?: boolean;
}) {
  const r = useChartData<unknown>(fetcher, [0], { enabled });
  return (
    <div>
      <span data-testid="loading">{String(r.loading)}</span>
      <span data-testid="error">{r.error ?? "none"}</span>
      <span data-testid="data">{r.data != null ? JSON.stringify(r.data) : "none"}</span>
      <button data-testid="refetch" onClick={() => r.refetch()}>
        refetch
      </button>
    </div>
  );
}

describe("useChartData", () => {
  afterEach(() => vi.restoreAllMocks());

  it("exposes resolved data and clears loading", async () => {
    const fetcher = vi.fn(() => Promise.resolve([{ a: 1 }]));
    const { getByTestId } = render(<Harness fetcher={fetcher} />);
    await waitFor(() => expect(getByTestId("loading").textContent).toBe("false"));
    expect(getByTestId("data").textContent).toBe(JSON.stringify([{ a: 1 }]));
    expect(getByTestId("error").textContent).toBe("none");
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("surfaces non-Abort errors (never silent) and warns", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    const fetcher = vi.fn(() => Promise.reject(new Error("boom")));
    const { getByTestId } = render(<Harness fetcher={fetcher} />);
    await waitFor(() => expect(getByTestId("error").textContent).toBe("boom"));
    expect(getByTestId("loading").textContent).toBe("false");
    expect(warn).toHaveBeenCalled();
  });

  it("does not fetch when enabled=false", () => {
    const fetcher = vi.fn(() => Promise.resolve("x"));
    const { getByTestId } = render(<Harness fetcher={fetcher} enabled={false} />);
    expect(getByTestId("loading").textContent).toBe("false");
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("refetch() re-runs the fetcher", async () => {
    const fetcher = vi.fn(() => Promise.resolve(1));
    const { getByTestId } = render(<Harness fetcher={fetcher} />);
    await waitFor(() => expect(getByTestId("data").textContent).toBe("1"));
    expect(fetcher).toHaveBeenCalledTimes(1);
    await act(async () => {
      getByTestId("refetch").click();
    });
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
  });
});
