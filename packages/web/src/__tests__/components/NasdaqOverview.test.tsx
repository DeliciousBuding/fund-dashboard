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
  ScatterChart: {},
}));

vi.mock('echarts/components', () => ({
  GridComponent: {},
  TooltipComponent: {},
  LegendComponent: {},
  DataZoomComponent: {},
  MarkLineComponent: {},
  MarkPointComponent: {},
}));

vi.mock('echarts/renderers', () => ({
  CanvasRenderer: {},
}));

import NasdaqOverview from '../../components/NasdaqOverview';

const mockFundInfoList = [
  { code: '019173', name: '纳斯达克100指数(QDII)C', type: 'QDII', security_type: 'fund', market: 'CN', held_shares: 1000, current_value: 15000, unrealized_pnl: 3000, pnl_pct: 25, latest_nav: 1.5 },
  { code: '016533', name: '嘉实纳斯达克100ETF联接(QDII)C', type: 'QDII', security_type: 'fund', market: 'CN', held_shares: 500, current_value: 7500, unrealized_pnl: 500, pnl_pct: 7.14, latest_nav: 1.5 },
];

const mockNavData = [
  { date: '2026-06-01', unit_nav: 1.40, daily_change_pct: 0.5 },
  { date: '2026-06-15', unit_nav: 1.45, daily_change_pct: 1.2 },
  { date: '2026-06-19', unit_nav: 1.50, daily_change_pct: 0.8 },
];

const mockFundDetail = {
  code: '019173',
  name: '纳斯达克100指数(QDII)C',
  security_type: 'fund',
  market: 'CN',
  held_shares: 1000,
  total_cost: -12000,
  latest_nav: 1.5,
  current_value: 15000,
  unrealized_pnl: 3000,
  pnl_pct: 25,
  auto_buy_count: 10,
  manual_buy_count: 5,
  auto_buy_amount: 8000,
  manual_buy_amount: 4000,
  auto_tx: 10,
  manual_tx: 5,
  buy_count: 15,
  sell_count: 0,
  median_settlement: 2,
  transactions: [
    { seq: 1, trade_time: '2026-06-01 10:00:00', confirm_date: '2026-06-02', trade_type: '定投买入', direction: 'buy', amount: 800, shares: 571.43, fee: 0, nav: 1.40, inferred_nav: null, anomaly: null },
    { seq: 2, trade_time: '2026-06-15 10:00:00', confirm_date: '2026-06-16', trade_type: '定投买入', direction: 'buy', amount: 800, shares: 551.72, fee: 0, nav: 1.45, inferred_nav: null, anomaly: null },
  ],
};

describe('NasdaqOverview', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders Nasdaq overview heading and summary stats', async () => {
    // 1st: fetchNav for proxy fund (019173)
    // 2nd: fetchFundDetail for 019173
    // 3rd: fetchFundDetail for 016533
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockNavData),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockFundDetail),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          ...mockFundDetail,
          code: '016533',
          name: '嘉实纳斯达克100ETF联接(QDII)C',
          held_shares: 500,
          current_value: 7500,
          unrealized_pnl: 500,
          pnl_pct: 7.14,
          total_cost: -7000,
          transactions: [],
        }),
      } as Response);

    const onSelect = vi.fn();
    render(<NasdaqOverview nasdaqFunds={mockFundInfoList} onSelect={onSelect} dark={false} />);

    await waitFor(() => {
      expect(screen.getByText('纳斯达克总览')).toBeInTheDocument();
    });

    // Verify summary text
    expect(screen.getByText(/2 只纳指基金/)).toBeInTheDocument();

    // Verify stat cards (fallback label from t('nasdaq.funds', '纳指基金'))
    expect(screen.getByText('纳指基金')).toBeInTheDocument();
    expect(screen.getByText('总买入')).toBeInTheDocument();

    // Verify chart section heading (fallback from t('nasdaq.chartTitle', '纳指收益走势'))
    expect(screen.getByText('纳指收益走势')).toBeInTheDocument();
  });

  it('renders fund comparison tables for holdings and cleared funds', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockNavData),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockFundDetail),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          ...mockFundDetail,
          code: '016533',
          name: '嘉实纳斯达克100ETF联接(QDII)C',
          held_shares: 500,
          current_value: 7500,
          unrealized_pnl: 500,
          pnl_pct: 7.14,
          total_cost: -7000,
          transactions: [],
        }),
      } as Response);

    render(<NasdaqOverview nasdaqFunds={mockFundInfoList} onSelect={vi.fn()} dark={false} />);

    await waitFor(() => {
      expect(screen.getByText('纳斯达克总览')).toBeInTheDocument();
    });

    // Holdings table
    expect(screen.getByText('纳指持仓')).toBeInTheDocument();
    expect(screen.getByText('已清仓纳指')).toBeInTheDocument();
  });

  it('renders multiple fund comparison with echarts chart container', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockNavData),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockFundDetail),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          ...mockFundDetail,
          code: '016533',
          name: '嘉实纳斯达克100ETF联接(QDII)C',
          held_shares: 500,
          current_value: 7500,
          unrealized_pnl: 500,
          pnl_pct: 7.14,
          total_cost: -7000,
          transactions: [],
        }),
      } as Response);

    const onSelect = vi.fn();
    render(<NasdaqOverview nasdaqFunds={mockFundInfoList} onSelect={onSelect} dark={false} />);

    await waitFor(() => {
      expect(screen.getByText('纳斯达克总览')).toBeInTheDocument();
    });

    // Verify echarts chart section renders (the mocked chart won't actually draw)
    // v3.0: description uses fallback from t('nasdaq.chartDesc', '累计收益率曲线 (线性插值平滑) + 买卖点标记')
    expect(screen.getByText(/累计收益率曲线/)).toBeInTheDocument();

    // Verify range tabs
    expect(screen.getByText('交易区间')).toBeInTheDocument();
  });

  it('renders with dark mode without crashing', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockNavData),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockFundDetail),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({
          ...mockFundDetail,
          code: '016533',
          name: '嘉实纳斯达克100ETF联接(QDII)C',
          held_shares: 500,
          current_value: 7500,
          unrealized_pnl: 500,
          pnl_pct: 7.14,
          total_cost: -7000,
          transactions: [],
        }),
      } as Response);

    render(<NasdaqOverview nasdaqFunds={mockFundInfoList} onSelect={vi.fn()} dark={true} />);

    await waitFor(() => {
      expect(screen.getByText('纳斯达克总览')).toBeInTheDocument();
    });

    expect(screen.getByText(/2 只纳指基金/)).toBeInTheDocument();
  });
});
