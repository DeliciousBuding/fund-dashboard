import { describe, it, expect, vi, afterEach } from 'vitest';
import { screen, waitFor } from '@testing-library/react'
import { renderWithRouter as render } from '../test-utils';

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

import NasdaqOverview from '../../components/NasdaqOverview';
import * as echartsCore from 'echarts/core';

const mockFundInfoList = [
  { code: '019173', name: '纳斯达克100指数(QDII)C', type: 'QDII', security_type: 'fund', market: 'CN', held_shares: 1000, current_value: 15000, unrealized_pnl: 3000, pnl_pct: 25, latest_nav: 1.5 },
  { code: '016533', name: '嘉实纳斯达克100ETF联接(QDII)C', type: 'QDII', security_type: 'fund', market: 'CN', held_shares: 500, current_value: 7500, unrealized_pnl: 500, pnl_pct: 7.14, latest_nav: 1.5 },
];

// Mock ^NDX index history response (matches IndexHistory schema)
const mockIndexHistory = {
  symbol: 'NDX',
  count: 3,
  range: 'max',
  data: [
    { date: '2026-06-01', close: 19000, change_pct: 0.5 },
    { date: '2026-06-15', close: 19500, change_pct: 1.2 },
    { date: '2026-06-19', close: 20000, change_pct: 0.8 },
  ],
};

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
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('renders Nasdaq overview heading and summary stats', async () => {
    // 1st: fetchIndexHistory('NDX', 'max') → GET /api/market/index/NDX/history?range=max
    // 2nd: fetchFundDetail('019173') → GET /api/funds/019173
    // 3rd: fetchFundDetail('016533') → GET /api/funds/016533
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockIndexHistory),
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

    // Verify summary text shows ^NDX benchmark
    expect(screen.getByText(/2 只纳指基金/)).toBeInTheDocument();
    expect(screen.getByText(/基准.*\^NDX/)).toBeInTheDocument();

    // Verify stat cards (fallback label from t('nasdaq.funds', '纳指基金'))
    expect(screen.getByText('纳指基金')).toBeInTheDocument();
    expect(screen.getByText('总买入')).toBeInTheDocument();

    // Verify chart section heading
    expect(screen.getByText('纳指收益走势')).toBeInTheDocument();
  });

  it('renders fund comparison tables for holdings and cleared funds', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockIndexHistory),
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

  it('renders chart section with range tabs and NDX description', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockIndexHistory),
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

    // Verify NDX description in chart header
    expect(screen.getAllByText(/NDX/).length).toBeGreaterThanOrEqual(1);

    // Verify range tabs
    expect(screen.getByText('交易区间')).toBeInTheDocument();
  });

  it('renders with dark mode without crashing', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockIndexHistory),
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

  it('aggregates same-day buy markers while preserving trade counts', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockIndexHistory),
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
          transactions: [
            { seq: 3, trade_time: '2026-06-15 11:00:00', confirm_date: '2026-06-16', trade_type: '用户买入', direction: 'buy', amount: 600, shares: 413.79, fee: 0, nav: 1.45, inferred_nav: null, anomaly: null },
          ],
        }),
      } as Response);

    render(<NasdaqOverview nasdaqFunds={mockFundInfoList} onSelect={vi.fn()} dark={false} />);

    await waitFor(() => {
      const results = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results ?? [];
      const calls = results.at(-1)?.value?.setOption?.mock?.calls ?? [];
      const latest = calls.at(-1)?.[0];
      const buySeries = latest?.series?.find((s: any) => String(s.name).startsWith('买入'));
      expect(buySeries?.name).toContain('3笔');
    });

    const mockInstance = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.at(-1)?.value;
    const option = mockInstance.setOption.mock.calls.at(-1)?.[0];
    const buySeries = option.series.find((s: any) => String(s.name).startsWith('买入'));
    expect(buySeries.data).toHaveLength(2);
    expect(buySeries.data[1].count).toBe(2);
    expect(buySeries.data[1].amt).toBe(1400);
    expect(buySeries.name).toContain('3笔');
  });
});
