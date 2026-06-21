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

vi.mock('echarts/charts', () => ({ HeatmapChart: {} }))
vi.mock('echarts/components', () => ({
  GridComponent: {},
  TooltipComponent: {},
  VisualMapComponent: {},
}))
vi.mock('echarts/renderers', () => ({ CanvasRenderer: {} }))

import CorrelationHeatmap from '../../components/CorrelationHeatmap'

const mockHarness = {
  holding_signals: [
    { code: 'F01', name: '纳指100ETF', weight_pct: 30, security_type: 'fund', market: 'CN', held_shares: 1000, current_value: 15000, latest_nav: 1.5, cost_per_share: 1.2, change_pct: 4.2, deviation_pct: 25, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
    { code: 'F02', name: '沪深300', weight_pct: 25, security_type: 'fund', market: 'CN', held_shares: 800, current_value: 12000, latest_nav: 1.6, cost_per_share: 1.5, change_pct: 1.5, deviation_pct: 6.7, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
    { code: 'F03', name: '易方达蓝筹', weight_pct: 20, security_type: 'fund', market: 'CN', held_shares: 600, current_value: 10000, latest_nav: 1.2, cost_per_share: 1.1, change_pct: -2.1, deviation_pct: 9.1, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
    { code: 'F04', name: '科创50', weight_pct: 15, security_type: 'fund', market: 'CN', held_shares: 400, current_value: 8000, latest_nav: 0.9, cost_per_share: 0.85, change_pct: -1.2, deviation_pct: 5.9, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
    { code: 'F05', name: '恒生科技', weight_pct: 10, security_type: 'fund', market: 'CN', held_shares: 200, current_value: 5000, latest_nav: 1.1, cost_per_share: 1.0, change_pct: 0.8, deviation_pct: 10, signal_tags: [], data_points: { has_price: true, has_cost_basis: true, has_change_pct: true } },
  ],
  total_value: 50000,
  holdings_count: 5,
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
      expect(screen.getByText('Network error')).toBeInTheDocument()
    })
  })

  it('stays loading when holdings are empty', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ ...mockHarness, holding_signals: [] }),
    } as Response)

    render(<CorrelationHeatmap dark={false} />)
    // With empty holdings, second useEffect does not trigger
    await waitFor(() => {
      expect(screen.getByText('正在计算相关性矩阵...')).toBeInTheDocument()
    })
  })
})
