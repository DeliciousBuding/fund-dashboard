import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'

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
}))

vi.mock('echarts/charts', () => ({
  LineChart: {},
  BarChart: {},
  ScatterChart: {},
  HeatmapChart: {},
  RadarChart: {},
  TreemapChart: {},
  SunburstChart: {},
}))
vi.mock('echarts/components', () => ({
  GridComponent: {},
  TooltipComponent: {},
  LegendComponent: {},
  DataZoomComponent: {},
  MarkLineComponent: {},
  MarkPointComponent: {},
  VisualMapComponent: {},
  RadarComponent: {},
}))
vi.mock('echarts/renderers', () => ({ CanvasRenderer: {} }))

import * as echartsCore from 'echarts/core'
import CorrelationHeatmap from '../../components/CorrelationHeatmap'

const mockHarness = {
  generated_at: '2026-06-19T00:00:00.000Z',
  decision_boundary: 'facts_only' as const,
  total_value: 50000,
  holdings_count: 5,
  allocation: {
    total_value: 50000,
    by_security_type: [],
    by_market: [],
    by_fund_type: [],
    risk_flags: [],
    agent_brief: '资产配置',
  },
  holding_signals: [
    { code: 'F01', name: '纳指100ETF', weight_pct: 30, security_type: 'fund', market: 'CN', held_shares: 1000, current_value: 15000, latest_nav: 1.5, cost_per_share: 1.2, change_pct: 4.2, deviation_pct: 25, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
    { code: 'F02', name: '沪深300', weight_pct: 25, security_type: 'fund', market: 'CN', held_shares: 800, current_value: 12000, latest_nav: 1.6, cost_per_share: 1.5, change_pct: 1.5, deviation_pct: 6.7, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
    { code: 'F03', name: '易方达蓝筹', weight_pct: 20, security_type: 'fund', market: 'CN', held_shares: 600, current_value: 10000, latest_nav: 1.2, cost_per_share: 1.1, change_pct: -2.1, deviation_pct: 9.1, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
    { code: 'F04', name: '科创50', weight_pct: 15, security_type: 'fund', market: 'CN', held_shares: 400, current_value: 8000, latest_nav: 0.9, cost_per_share: 0.85, change_pct: -1.2, deviation_pct: 5.9, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
    { code: 'F05', name: '恒生科技', weight_pct: 10, security_type: 'fund', market: 'CN', held_shares: 200, current_value: 5000, latest_nav: 1.1, cost_per_share: 1.0, change_pct: 0.8, deviation_pct: 10, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
  ],
  data_quality: { stale_price_count: 0, missing_cost_basis_count: 0, missing_change_pct_count: 0, holdings_coverage_pct: 100 },
  available_agent_tools: ['get_fund_detail'],
  agent_permissions: {
    decision_boundary: 'facts_only' as const,
    read_scope: ['portfolio'],
    write_scope: ['source_event_feedback'],
    requires_confirmation: ['add_transaction'],
    disabled_operations: ['broker_trade_execution', 'backup_producer'],
  },
  agent_capabilities: [],
  recommended_agent_actions: [],
  agent_brief: 'Portfolio overview',
}

function makeNav(base: number, noise: number): { date: string; unit_nav: number }[] {
  const nav: { date: string; unit_nav: number }[] = []
  let v = base
  for (let i = 0; i < 60; i++) {
    const d = new Date(2024, 0, 1)
    d.setDate(d.getDate() + i)
    v += (Math.random() - 0.5) * noise
    if (v < base * 0.5) v = base * 0.5
    nav.push({ date: d.toISOString().substring(0, 10), unit_nav: +v.toFixed(4) })
  }
  return nav
}

/** Create a URL-aware fetch mock that returns harness + NAV data */
function mockFetchWithNav() {
  vi.spyOn(globalThis, 'fetch').mockImplementation((input: any) => {
    const url = typeof input === 'string' ? input : input?.url || ''
    if (url.includes('/api/portfolio/harness')) {
      return Promise.resolve({ ok: true, json: () => Promise.resolve(mockHarness) } as Response)
    }
    if (url.includes('/api/funds/') && url.includes('/nav')) {
      const bases: Record<string, number> = { F01: 1.5, F02: 1.6, F03: 1.2, F04: 0.9, F05: 1.1 }
      const code = url.split('/api/funds/')[1]?.split('/')[0] || 'F01'
      return Promise.resolve({ ok: true, json: () => Promise.resolve(makeNav(bases[code] || 1.0, 0.03)) } as Response)
    }
    return Promise.reject(new Error('Unknown URL'))
  })
}

describe('CorrelationHeatmap', () => {
  afterEach(() => {
    vi.clearAllMocks()
    vi.restoreAllMocks()
  })

  it('renders heading and loading state initially', () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockHarness),
    } as Response)

    render(<CorrelationHeatmap dark={false} />)
    expect(screen.getByText('持仓相关性热力图')).toBeInTheDocument()
    expect(screen.getByText('正在计算相关性矩阵...')).toBeInTheDocument()
  })

  it('renders correlation matrix after NAV data loads', async () => {
    mockFetchWithNav()
    render(<CorrelationHeatmap dark={false} />)

    await waitFor(() => {
      expect(screen.getByText(/5 只基金/)).toBeInTheDocument()
    })
    expect(screen.getByText('持仓相关性热力图')).toBeInTheDocument()
  })

  it('renders with dark mode', async () => {
    mockFetchWithNav()
    render(<CorrelationHeatmap dark={true} />)

    await waitFor(() => {
      expect(screen.getByText(/Pearson 相关系数/)).toBeInTheDocument()
    })
    expect(screen.getByText('持仓相关性热力图')).toBeInTheDocument()
  })

  it('shows error when harness fetch fails', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'))
    render(<CorrelationHeatmap dark={false} />)

    await waitFor(() => {
      // useChartData sanitizes network dumps to common.loadError
      expect(screen.getByText('加载失败')).toBeInTheDocument()
    })
  })

  it('shows empty placeholder when holdings are empty (was an infinite-spinner bug)', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ ...mockHarness, holding_signals: [] }),
    } as Response)

    render(<CorrelationHeatmap dark={false} />)
    // Consolidated fetcher resolves empty holdings to {[], null} → empty state
    // (previously the second useEffect never fired and the chart spun forever).
    await waitFor(() => {
      expect(screen.getByTestId('chart-empty')).toBeInTheDocument()
    })
  })

  it('uses theme.blue for the visualMap inRange (stable color guard)', async () => {
    mockFetchWithNav()
    render(<CorrelationHeatmap dark={false} />)

    await waitFor(() => expect(screen.getByText(/5 只基金/)).toBeInTheDocument())
    const mockInstance = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value
    const callArg = mockInstance.setOption.mock.calls[0]?.[0]
    // visualMap inRange tops out at theme.blue (#3172d9 light)
    expect(callArg.visualMap.inRange.color).toContain('#3172d9')
  })
})
