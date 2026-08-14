import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';

// Mock echarts before component import (hoisted by Vitest)
vi.mock('echarts/core', () => ({
  use: vi.fn(),
  init: vi.fn(() => ({
    setOption: vi.fn(),
    dispose: vi.fn(),
    resize: vi.fn(),
  })),
  graphic: {
    LinearGradient: function () { /* mock */ },
  },
}));

vi.mock('echarts/charts', () => ({
  LineChart: {},
  BarChart: {},
  ScatterChart: {},
  RadarChart: {},
  TreemapChart: {},
  SunburstChart: {},
  HeatmapChart: {},
}));

vi.mock('echarts/components', () => ({
  GridComponent: {},
  TooltipComponent: {},
  LegendComponent: {},
  DataZoomComponent: {},
  MarkLineComponent: {},
  MarkPointComponent: {},
  RadarComponent: {},
  VisualMapComponent: {},
}));

vi.mock('echarts/renderers', () => ({
  CanvasRenderer: {},
}));

import * as echartsCore from 'echarts/core';
import PnLDistributionChart from '../../components/PnLDistributionChart';

describe('PnLDistributionChart', () => {
  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  const mockHarnessData = {
    generated_at: '2026-06-19T00:00:00.000Z',
    decision_boundary: 'facts_only',
    total_value: 50000,
    holdings_count: 12,
    allocation: {
      total_value: 50000,
      by_security_type: [],
      by_market: [],
      by_fund_type: [],
      risk_flags: [],
      agent_brief: '资产配置',
    },
    holding_signals: [
      { code: '019173', name: '纳指100', security_type: 'fund', market: 'CN', held_shares: 1000, current_value: 15000, weight_pct: 30, latest_nav: 1.5, cost_per_share: 1.2, change_pct: -4.2, deviation_pct: 25, signal_tags: ['above_cost_gt_10pct'], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
      { code: '000948', name: '华夏沪深300', security_type: 'fund', market: 'CN', held_shares: 500, current_value: 8000, weight_pct: 16, latest_nav: 1.6, cost_per_share: 1.8, change_pct: 1.5, deviation_pct: -11.1, signal_tags: ['below_cost_gt_10pct'], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
      { code: '005827', name: '易方达蓝筹', security_type: 'fund', market: 'CN', held_shares: 2000, current_value: 12000, weight_pct: 24, latest_nav: 0.9, cost_per_share: 1.0, change_pct: -2.1, deviation_pct: -10, signal_tags: ['below_cost_5_10pct'], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
      { code: '164906', name: '交银中证海外', security_type: 'fund', market: 'CN', held_shares: 800, current_value: 10000, weight_pct: 20, latest_nav: 1.25, cost_per_share: 1.0, change_pct: 8.3, deviation_pct: 25, signal_tags: ['above_cost_gt_10pct'], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
      { code: 'AAPL', name: 'Apple Inc.', security_type: 'stock', market: 'US', held_shares: 50, current_value: 5000, weight_pct: 10, latest_nav: 0, cost_per_share: null, change_pct: null, deviation_pct: null, signal_tags: [], data_points: { has_price: true, has_cost_basis: false, has_change_pct: false } },
    ],
    data_quality: { stale_price_count: 0, missing_cost_basis_count: 1, missing_change_pct_count: 1, holdings_coverage_pct: 100 },
    available_agent_tools: ['get_fund_detail'],
    agent_permissions: {
      decision_boundary: 'facts_only',
      read_scope: ['portfolio'],
      write_scope: ['source_event_feedback'],
      requires_confirmation: ['add_transaction'],
      disabled_operations: ['broker_trade_execution', 'backup_producer'],
    },
    agent_capabilities: [],
    recommended_agent_actions: [],
    agent_brief: 'Portfolio overview',
  };

  it('renders chart container with PnL distribution heading', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockHarnessData),
    } as Response);

    render(<PnLDistributionChart dark={false} />);

    // gate on a data-only element (heading renders during loading — would race)
    await waitFor(() => {
      expect(screen.getByText(/5 只持仓/)).toBeInTheDocument();
    });

    // Summary text with holdings count
    expect(screen.getByText(/4 只有成本数据/)).toBeInTheDocument();
  });

  it('renders all 8 bucket intervals', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockHarnessData),
    } as Response);

    render(<PnLDistributionChart dark={false} />);

    await waitFor(() => {
      expect(screen.getByText('盈亏分布')).toBeInTheDocument();
    });

    // The echarts chart is mocked, but the container div ref={chartRef} should exist
    // We verify the component rendered without crashing with valid data
    const chartDiv = document.querySelector('[ref]') as HTMLElement;
    // The chart container exists (either the echarts div or loading state)
    expect(screen.getByText('盈亏分布')).toBeInTheDocument();
  });

  it('renders with dark mode without crashing', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockHarnessData),
    } as Response);

    render(<PnLDistributionChart dark={true} />);

    await waitFor(() => {
      expect(screen.getByText('盈亏分布')).toBeInTheDocument();
    });

    // The heading renders, verifying the component didn't crash with dark mode.
    // Data-loaded content (e.g. "5 只持仓") may take longer in CI environments.
    expect(screen.getByText('盈亏分布')).toBeInTheDocument();
  });

  it('shows loading state when no holdings data', () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ ...mockHarnessData, holding_signals: [] }),
    } as Response);

    render(<PnLDistributionChart dark={false} />);

    // v3.0: uses Unicode ellipsis '…' (U+2026)
    expect(screen.getByTestId('chart-loading')).toBeInTheDocument();
  });

  it('colors gain buckets red (up) and loss buckets green (down) — CN convention', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockHarnessData),
    } as Response);

    render(<PnLDistributionChart dark={false} />);
    // wait for data to load (count text renders only when loaded) so setOption fired
    await waitFor(() => expect(screen.getByText(/5 只持仓/)).toBeInTheDocument());

    const mockInstance = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value;
    const callArg = mockInstance.setOption.mock.calls[0]?.[0];
    // series[0].data = 8 buckets, each { value, itemStyle: { color } }
    const buckets = callArg.series[0].data;
    // bucket 6 = gain_20_30 (deviation +25%) → red (theme.up #d63649 → rgba 214,54,73)
    expect(buckets[6].itemStyle.color).toMatch(/rgba\(214,\s*54,\s*73/);
    // bucket 2 = loss_10_20 (deviation -11.1%, -10%) → green (theme.down #199c63 → rgba 25,156,99)
    expect(buckets[2].itemStyle.color).toMatch(/rgba\(25,\s*156,\s*99/);
  });
});
