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
}));

vi.mock('echarts/renderers', () => ({
  CanvasRenderer: {},
}));

import FundDetailView from '../../components/FundDetailView';

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

const mockNavData = [
  { date: '2026-06-01', unit_nav: 1.40, daily_change_pct: 0.5 },
  { date: '2026-06-15', unit_nav: 1.45, daily_change_pct: 1.2 },
  { date: '2026-06-19', unit_nav: 1.50, daily_change_pct: 0.8 },
];

const mockXirr = { xirr: 12.5, code: '019173' };
const mockDrawdown = { max_drawdown: 8.3, peak_date: '2026-01-15', trough_date: '2026-03-20', code: '019173' };

describe('FundDetailView', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  function mockAllFetches() {
    // Fetch order in useEffect:
    // 1. fetchFundDetail(code) → /api/funds/019173
    // 2. fetchNav(code) → /api/funds/019173/nav
    // 3. fetchXirr(code) → /api/funds/019173/xirr
    // 4. fetchDrawdown(code) → /api/funds/019173/drawdown
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockFundDetail),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockNavData),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockXirr),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockDrawdown),
      } as Response);
  }

  it('renders StatCards with holding shares, cost, nav, and market value', async () => {
    mockAllFetches();

    render(<FundDetailView code="019173" dark={false} />);

    // Wait for loading to complete and detail to render
    await waitFor(() => {
      expect(screen.getByText('纳斯达克100指数(QDII)C')).toBeInTheDocument();
    });

    // StatCards
    expect(screen.getByText('持有份额')).toBeInTheDocument();
    expect(screen.getByText('投入成本')).toBeInTheDocument();
    expect(screen.getByText('最新净值')).toBeInTheDocument();
    expect(screen.getByText('当前市值')).toBeInTheDocument();
    expect(screen.getByText('未实现盈亏')).toBeInTheDocument();
    expect(screen.getByText('年化收益 (XIRR)')).toBeInTheDocument();
    expect(screen.getByText('最大回撤')).toBeInTheDocument();

    // Value checks
    expect(screen.getByText('1000.00')).toBeInTheDocument();
    expect(screen.getByText('¥ 12000.00')).toBeInTheDocument();
    expect(screen.getByText('1.5000')).toBeInTheDocument();
  });

  it('renders "导出 CSV" and "导入 CSV" buttons', async () => {
    mockAllFetches();

    render(<FundDetailView code="019173" dark={false} />);

    await waitFor(() => {
      expect(screen.getByText('纳斯达克100指数(QDII)C')).toBeInTheDocument();
    });

    expect(screen.getByText('导出')).toBeInTheDocument();
    expect(screen.getByText('导入 CSV')).toBeInTheDocument();
  });

  it('renders tab navigation with chart, dca, overview, and transactions tabs', async () => {
    mockAllFetches();

    render(<FundDetailView code="019173" dark={false} />);

    await waitFor(() => {
      expect(screen.getByText('纳斯达克100指数(QDII)C')).toBeInTheDocument();
    });

    // Tab labels - "净值走势" appears both as tab label and FundChart heading
    expect(screen.getAllByText('净值走势').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('定投')).toBeInTheDocument();
    expect(screen.getByText('概览')).toBeInTheDocument();
    expect(screen.getByText(/交易记录/)).toBeInTheDocument();
  });

  it('renders without crash in dark mode', async () => {
    mockAllFetches();

    render(<FundDetailView code="019173" dark={true} />);

    await waitFor(() => {
      expect(screen.getByText('纳斯达克100指数(QDII)C')).toBeInTheDocument();
    });

    // Core elements still present in dark mode
    expect(screen.getByText('持有份额')).toBeInTheDocument();
    expect(screen.getByText('年化收益 (XIRR)')).toBeInTheDocument();
    expect(screen.getByText('导出')).toBeInTheDocument();
  });
});
