import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ChartShell } from "../ChartShell";

describe("ChartShell", () => {
  it("renders title + subtitle in the header, and children in the ok state", () => {
    render(
      <ChartShell dark={false} title="组合净值走势" subtitle="说明">
        <div data-testid="chart">x</div>
      </ChartShell>,
    );
    expect(screen.getByText("组合净值走势")).toBeInTheDocument();
    expect(screen.getByText("说明")).toBeInTheDocument();
    expect(screen.getByTestId("chart")).toBeInTheDocument();
  });

  it("shows the loading placeholder (testid) and hides children", () => {
    render(
      <ChartShell dark={false} title="t" loading>
        <div data-testid="chart">x</div>
      </ChartShell>,
    );
    expect(screen.getByTestId("chart-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("chart")).not.toBeInTheDocument();
  });

  it("shows the error placeholder (testid)", () => {
    render(
      <ChartShell dark={false} title="t" error="boom">
        <div data-testid="chart">x</div>
      </ChartShell>,
    );
    expect(screen.getByTestId("chart-error")).toBeInTheDocument();
  });

  it("shows the empty placeholder (testid)", () => {
    render(
      <ChartShell dark={false} title="t" empty>
        <div data-testid="chart">x</div>
      </ChartShell>,
    );
    expect(screen.getByTestId("chart-empty")).toBeInTheDocument();
  });

  it("renders range tabs and marks the active value", () => {
    render(
      <ChartShell
        dark={false}
        title="t"
        ranges={[
          { value: "1m", label: "近1月" },
          { value: "3m", label: "近3月" },
        ]}
        range="1m"
      >
        <div data-testid="chart">x</div>
      </ChartShell>,
    );
    expect(screen.getByText("近1月")).toBeInTheDocument();
    expect(screen.getByTestId("kumo-tab-1m")).toHaveAttribute("data-active", "true");
    expect(screen.getByTestId("kumo-tab-3m")).toHaveAttribute("data-active", "false");
  });

  it("respects a custom testid prefix", () => {
    render(
      <ChartShell dark={false} title="t" testidPrefix="pnl" empty>
        <div>x</div>
      </ChartShell>,
    );
    expect(screen.getByTestId("pnl-empty")).toBeInTheDocument();
  });

  it("renders headerExtra alongside the tabs", () => {
    render(
      <ChartShell dark={false} title="t" headerExtra={<span data-testid="extra">cmp</span>}>
        <div>x</div>
      </ChartShell>,
    );
    expect(screen.getByTestId("extra")).toBeInTheDocument();
  });
});
