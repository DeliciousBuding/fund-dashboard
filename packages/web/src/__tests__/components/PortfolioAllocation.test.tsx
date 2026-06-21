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
}));

vi.mock('echarts/charts', () => ({ SunburstChart: {} }));
vi.mock('echarts/components', () => ({ TooltipComponent: {} }));
vi.mock('echarts/renderers', () => ({ CanvasRenderer: {} }));

import PortfolioAllocation from '../../components/PortfolioAllocation';

const mockAllocation = {
  total_value: 830,
  by_security_type: [
    { key: 'stock', label: '股票', value: 680, weight_pct: 81.93, count: 2 },
    { key: 'fund', label: '基金', value: 150, weight_pct: 18.07, count: 1 },
  ],
  by_market: [
    { key: 'US', label: '美股', value: 380, weight_pct: 45.78, count: 1 },
    { key: 'HK', label: '港股', value: 300, weight_pct: 36.15, count: 1 },
  ],
  by_fund_type: [
    { key: 'QDII-股票', label: 'QDII-股票', value: 150, weight_pct: 18.07, count: 1 },
  ],
  risk_flags: ['股票资产占比高于 80%'],
  agent_brief: '资产配置：股票 81.93%，基金 18.07%。风险提示：股票资产占比高于 80%。',
};

const mockHarness = {
  generated_at: '2026-01-01T00:00:00Z',
  decision_boundary: 'facts_only' as const,
  total_value: 830,
  holdings_count: 3,
  allocation: mockAllocation,
  holding_signals: [
    { code: 'NVDA', name: 'NVIDIA', security_type: 'stock', market: 'US', held_shares: 10, current_value: 380, weight_pct: 45.78, latest_nav: 110, cost_per_share: 80, change_pct: 37.5, deviation_pct: 37.5, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
    { code: '00700', name: '腾讯控股', security_type: 'stock', market: 'HK', held_shares: 100, current_value: 300, weight_pct: 36.15, latest_nav: 380, cost_per_share: 350, change_pct: 8.57, deviation_pct: 8.57, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
    { code: 'F01', name: '纳指科技ETF', security_type: 'fund', market: 'US', held_shares: 1000, current_value: 150, weight_pct: 18.07, latest_nav: 1.5, cost_per_share: 1.2, change_pct: 25, deviation_pct: 25, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
  ],
  data_quality: { stale_price_count: 0, missing_cost_basis_count: 0, missing_change_pct_count: 0, holdings_coverage_pct: 100 },
  available_agent_tools: [],
  agent_brief: 'test',
};

describe('PortfolioAllocation', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders allocation buckets, risk flags, and agent brief', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(mockAllocation) } as Response)
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(mockHarness) } as Response);

    render(<PortfolioAllocation dark={false} />);

    await waitFor(() => {
      expect(screen.getByText('资产配置')).toBeInTheDocument();
    });
    expect(screen.getByText('股票')).toBeInTheDocument();
    expect(screen.getByText('美股')).toBeInTheDocument();
    expect(screen.getByText('股票资产占比高于 80%')).toBeInTheDocument();
    expect(screen.getByText(/资产配置：股票 81.93%/)).toBeInTheDocument();
  });

  it('renders sunburst chart container when holding data loads', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(mockAllocation) } as Response)
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(mockHarness) } as Response);

    render(<PortfolioAllocation dark={false} />);

    await waitFor(() => {
      expect(screen.getByTestId('sunburst-chart')).toBeInTheDocument();
    });
    expect(screen.getByText('配置层级')).toBeInTheDocument();
    expect(screen.getByText(/内圈: 资产类型/)).toBeInTheDocument();
  });

  it('supports dark mode for sunburst chart', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(mockAllocation) } as Response)
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(mockHarness) } as Response);

    render(<PortfolioAllocation dark={true} />);

    await waitFor(() => {
      expect(screen.getByTestId('sunburst-chart')).toBeInTheDocument();
    });
    expect(screen.getByText('配置层级')).toBeInTheDocument();
  });

  it('omits sunburst when holding signals are empty', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(mockAllocation) } as Response)
      .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve({ ...mockHarness, holding_signals: [] }) } as Response);

    render(<PortfolioAllocation dark={false} />);

    await waitFor(() => {
      expect(screen.getByText('资产配置')).toBeInTheDocument();
    });
    expect(screen.queryByTestId('sunburst-chart')).not.toBeInTheDocument();
  });
});
