import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import { renderWithRouter as render } from '../test-utils'
import userEvent from '@testing-library/user-event'
import * as echartsCore from 'echarts/core'

const { mockFetchCompare } = vi.hoisted(() => ({
  mockFetchCompare: vi.fn(),
}));

// Mock echarts before component import
vi.mock('echarts/core', () => ({
  use: vi.fn(),
  init: vi.fn(() => ({
    setOption: vi.fn(),
    dispose: vi.fn(),
    resize: vi.fn(),
  })),
}))

vi.mock('echarts/charts', () => ({
  RadarChart: {},
  LineChart: {},
  BarChart: {},
  ScatterChart: {},
  TreemapChart: {},
  SunburstChart: {},
  HeatmapChart: {},
}))
vi.mock('echarts/components', () => ({
  RadarComponent: {},
  TooltipComponent: {},
  LegendComponent: {},
  GridComponent: {},
  DataZoomComponent: {},
  MarkLineComponent: {},
  MarkPointComponent: {},
  VisualMapComponent: {},
}))
vi.mock('echarts/renderers', () => ({ CanvasRenderer: {} }))

// Mock the fetchCompare API
vi.mock('../../api', () => ({
  fetchCompare: mockFetchCompare,
}));

import FundComparison from '../../components/FundComparison';
import type { FundInfo } from '../../api';

const mockFunds: FundInfo[] = [
  { code: '164906', name: '交银海外中国互联网', type: 'QDII', security_type: 'fund', market: '', held_shares: 1000, current_value: 1500, unrealized_pnl: 500, pnl_pct: 50, latest_nav: 1.5000 },
  { code: '161128', name: '标普信息科技', type: 'QDII', security_type: 'fund', market: '', held_shares: 2000, current_value: 3000, unrealized_pnl: 1000, pnl_pct: 50, latest_nav: 1.5000 },
  { code: '000614', name: '华安德国30(DAX)', type: 'QDII', security_type: 'fund', market: '', held_shares: 500, current_value: 750, unrealized_pnl: 250, pnl_pct: 50, latest_nav: 1.5000 },
  { code: '000000', name: '未持仓基金', type: '混合', security_type: 'fund', market: '', held_shares: 0, current_value: 0, unrealized_pnl: 0, pnl_pct: 0, latest_nav: null },
];

const compareResponse = {
  funds: [
    { code: '164906', name: '交银海外中国互联网', market: '', xirr: 12.35, volatility: 22.10, sharpe: 0.5588, max_drawdown: 35.20, calmar: 0.3509 },
    { code: '161128', name: '标普信息科技', market: '', xirr: 18.50, volatility: 18.30, sharpe: 1.0109, max_drawdown: 25.10, calmar: 0.7371 },
    { code: '000614', name: '华安德国30(DAX)', market: '', xirr: 8.20, volatility: 15.60, sharpe: 0.5256, max_drawdown: 28.40, calmar: 0.2887 },
  ],
};

describe('FundComparison', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchCompare.mockResolvedValue(compareResponse);
  });

  it('renders the page title', () => {
    render(<FundComparison funds={mockFunds} dark={false} />);
    expect(screen.getByText('基金对比')).toBeInTheDocument();
  });

  it('shows fund selection buttons for held funds only', () => {
    render(<FundComparison funds={mockFunds} dark={false} />);
    expect(screen.getByText('交银海外中国互联网')).toBeInTheDocument();
    expect(screen.getByText('标普信息科技')).toBeInTheDocument();
    expect(screen.getByText('华安德国30(DAX)')).toBeInTheDocument();
    expect(screen.queryByText('未持仓基金')).not.toBeInTheDocument();
  });

  it('toggles fund selection on button click', async () => {
    const user = userEvent.setup();
    render(<FundComparison funds={mockFunds} dark={false} />);

    const btn = screen.getByText('交银海外中国互联网');
    await user.click(btn);
    expect(screen.getByText('对比 (1)')).toBeInTheDocument();

    await user.click(btn);
    const compareBtn = screen.getByRole('button', { name: /对比/ });
    expect(compareBtn).toBeDisabled();
  });

  it('fetches and displays comparison data', async () => {
    const user = userEvent.setup();
    render(<FundComparison funds={mockFunds} dark={false} />);

    await user.click(screen.getByText('交银海外中国互联网'));
    await user.click(screen.getByText('标普信息科技'));

    expect(screen.getByText('对比 (2)')).toBeInTheDocument();

    await user.click(screen.getByText('对比 (2)'));

    await waitFor(() => {
      expect(mockFetchCompare).toHaveBeenCalledWith(['164906', '161128'], 1, expect.any(AbortSignal));
    });

    await waitFor(() => {
      expect(screen.getByText('指标对比表')).toBeInTheDocument();
      expect(screen.getByText('年化收益')).toBeInTheDocument();
      expect(screen.getByText('波动率')).toBeInTheDocument();
      expect(screen.getByText('Sharpe')).toBeInTheDocument();
      expect(screen.getByText('最大回撤')).toBeInTheDocument();
      expect(screen.getByText('Calmar')).toBeInTheDocument();
    });
  });

  it('shows star marker for best metric', async () => {
    const user = userEvent.setup();
    render(<FundComparison funds={mockFunds} dark={false} />);

    await user.click(screen.getByText('交银海外中国互联网'));
    await user.click(screen.getByText('标普信息科技'));
    await user.click(screen.getByText('对比 (2)'));

    await waitFor(() => {
      const cells = screen.getAllByText(/★/);
      expect(cells.length).toBeGreaterThan(0);
    });
  });

  it('renders in dark mode without errors', () => {
    render(<FundComparison funds={mockFunds} dark={true} />);
    expect(screen.getByText('基金对比')).toBeInTheDocument();
    expect(screen.getByText('选择对比基金')).toBeInTheDocument();
  });

  it('shows empty state when no held funds', () => {
    render(<FundComparison funds={[]} dark={false} />);
    expect(screen.getByText('暂无持仓基金数据')).toBeInTheDocument();
  });

  // Helper: drive a compare and capture the first setOption call's option object.
  async function compareAndGetOption(funds: FundInfo[], response: typeof compareResponse) {
    mockFetchCompare.mockResolvedValueOnce(response);
    const user = userEvent.setup();
    render(<FundComparison funds={funds} dark={false} />);
    // select exactly the funds present in the response for determinism
    for (const f of response.funds) {
      const btn = screen.queryByText(f.name);
      if (btn) await user.click(btn);
    }
    const compareBtn = await screen.findByRole('button', { name: /对比/ });
    await user.click(compareBtn);
    await waitFor(() => {
      const calls = echartsCore.init.mock?.results?.[0]?.value?.setOption?.mock?.calls ?? [];
      expect(calls.length).toBeGreaterThan(0);
    });
    const instance = echartsCore.init.mock.results[0].value;
    return instance.setOption.mock.calls[0][0] as Record<string, any>;
  }

  it('radar indicator max scales dynamically so values never clip', async () => {
    // xirr=120% and volatility=60% — both exceed the hardcoded floors (50 / 40).
    // A fixed scale would clip these; dynamic max must grow beyond them.
    const highPerf = {
      funds: [
        { code: '164906', name: '交银海外中国互联网', market: '', xirr: 120.00, volatility: 60.00, sharpe: 2.0, max_drawdown: 20.00, calmar: 1.0 },
        { code: '161128', name: '标普信息科技', market: '', xirr: 30.00, volatility: 18.00, sharpe: 1.5, max_drawdown: 15.00, calmar: 0.9 },
      ],
    };
    const funds: FundInfo[] = [
      { code: '164906', name: '交银海外中国互联网', type: 'QDII', security_type: 'fund', market: '', held_shares: 1000, current_value: 1500, unrealized_pnl: 500, pnl_pct: 50, latest_nav: 1.5 },
      { code: '161128', name: '标普信息科技', type: 'QDII', security_type: 'fund', market: '', held_shares: 2000, current_value: 3000, unrealized_pnl: 1000, pnl_pct: 50, latest_nav: 1.5 },
    ];
    const opt = await compareAndGetOption(funds, highPerf);
    const indicator = opt.radar.indicator as { name: string; max: number }[];
    const byName = (n: string) => indicator.find(i => i.name === n)!;
    // dynamic max must be >= the largest observed value (no clipping)
    expect(byName('年化收益').max).toBeGreaterThanOrEqual(120);
    expect(byName('波动率').max).toBeGreaterThanOrEqual(60);
    // floors still apply for low-value metrics (sharpe well below floor of 3)
    expect(byName('Sharpe').max).toBeGreaterThanOrEqual(2.0);
  });

  it('radar max_drawdown axis is inverted: smaller drawdown renders larger', async () => {
    // Fund A drawdown=5% (good), Fund B drawdown=40% (bad).
    // On a radar, "better" should reach further out. So A's radar value for the
    // max_drawdown spoke must be LARGER than B's.
    const ddResponse = {
      funds: [
        { code: '164906', name: '交银海外中国互联网', market: '', xirr: 10.0, volatility: 18.0, sharpe: 0.5, max_drawdown: 5.0, calmar: 0.4 },
        { code: '161128', name: '标普信息科技', market: '', xirr: 10.0, volatility: 18.0, sharpe: 0.5, max_drawdown: 40.0, calmar: 0.4 },
      ],
    };
    const funds: FundInfo[] = [
      { code: '164906', name: '交银海外中国互联网', type: 'QDII', security_type: 'fund', market: '', held_shares: 1000, current_value: 1500, unrealized_pnl: 500, pnl_pct: 50, latest_nav: 1.5 },
      { code: '161128', name: '标普信息科技', type: 'QDII', security_type: 'fund', market: '', held_shares: 2000, current_value: 3000, unrealized_pnl: 1000, pnl_pct: 50, latest_nav: 1.5 },
    ];
    const opt = await compareAndGetOption(funds, ddResponse);
    const series = opt.series[0].data as { name: string; value: number[] }[];
    // value order: [xirr, volatility, sharpe, max_drawdown, calmar] → index 3
    const a = series.find(s => s.name === '交银海外中国互联网')!.value[3];
    const b = series.find(s => s.name === '标普信息科技')!.value[3];
    // Fund A (5% dd) should plot further out than Fund B (40% dd)
    expect(a).toBeGreaterThan(b);
  });

  it('table best (lowest) drawdown is starred, not the worst', async () => {
    const user = userEvent.setup();
    render(<FundComparison funds={mockFunds} dark={false} />);
    await user.click(screen.getByText('交银海外中国互联网'));
    await user.click(screen.getByText('标普信息科技'));
    await user.click(screen.getByText('华安德国30(DAX)'));
    await user.click(screen.getByText('对比 (3)'));

    // 25.10 is the smallest drawdown → best → should carry the star
    await waitFor(() => {
      expect(screen.getByText('+25.10% ★')).toBeInTheDocument();
    });
    // the worst (35.20) must NOT be starred
    expect(screen.getByText('+35.20%')).toBeInTheDocument();
    expect(screen.queryByText('+35.20% ★')).not.toBeInTheDocument();
  });
});
