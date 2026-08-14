import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, waitFor, act } from '@testing-library/react'

// Capture treemap click handlers so tests can trigger the drill-down detail view.
const clickHandlers: Record<string, (...args: any[]) => void> = {}

vi.mock('echarts/core', () => ({
  use: vi.fn(),
  init: vi.fn(() => ({
    setOption: vi.fn(),
    dispose: vi.fn(),
    resize: vi.fn(),
  })),
  getInstanceByDom: vi.fn(() => ({
    on: vi.fn((evt: string, cb: (...a: any[]) => void) => {
      clickHandlers[evt] = cb
    }),
    off: vi.fn(),
  })),
  graphic: { LinearGradient: function () { /* mock */ } },
}))

vi.mock('echarts/charts', () => ({
  LineChart: {},
  BarChart: {},
  ScatterChart: {},
  TreemapChart: {},
  RadarChart: {},
  SunburstChart: {},
  HeatmapChart: {},
}))
vi.mock('echarts/components', () => ({
  GridComponent: {},
  TooltipComponent: {},
  LegendComponent: {},
  DataZoomComponent: {},
  MarkLineComponent: {},
  MarkPointComponent: {},
  RadarComponent: {},
  VisualMapComponent: {},
}))
vi.mock('echarts/renderers', () => ({ CanvasRenderer: {} }))

import PortfolioPenetration from '../../components/PortfolioPenetration'

const mockPenetration = {
  penetration: [
    {
      stock_code: 'NVDA',
      stock_name: 'NVIDIA',
      total_exposure_cny: 3800,
      weight_pct: 45.78,
      held_by_funds: [
        { fund_code: 'F01', fund_name: '纳指100ETF', fund_value_cny: 5000, weight_pct: 76.0 },
      ],
    },
    {
      stock_code: 'AAPL',
      stock_name: 'Apple',
      total_exposure_cny: 2000,
      weight_pct: 24.09,
      held_by_funds: [
        { fund_code: 'F01', fund_name: '纳指100ETF', fund_value_cny: 5000, weight_pct: 40.0 },
      ],
    },
  ],
  total_portfolio_value: 8300,
  equity_fund_count: 1,
  unique_stocks: 2,
}

describe('PortfolioPenetration', () => {
  afterEach(() => {
    vi.clearAllMocks()
    vi.restoreAllMocks()
    Object.keys(clickHandlers).forEach((k) => delete clickHandlers[k])
  })

  it('renders title, subtitle counts, and treemap container after data loads', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPenetration),
    } as Response)

    render(<PortfolioPenetration dark={false} />)

    await waitFor(() => {
      expect(screen.getByTestId('treemap-chart')).toBeInTheDocument()
    })
    expect(screen.getByText('股权穿透')).toBeInTheDocument()
    // i18n subtitle interpolates equityCount / uniqueStocks / value into one sentence
    expect(screen.getByText(/1 只权益基金 · 2 只底层股票 · 组合总市值 ¥8,300/)).toBeInTheDocument()
  })

  it('renders without crashing in dark mode', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPenetration),
    } as Response)

    render(<PortfolioPenetration dark={true} />)
    await waitFor(() => {
      expect(screen.getByTestId('treemap-chart')).toBeInTheDocument()
    })
  })

  it('shows error placeholder on fetch failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'))
    render(<PortfolioPenetration dark={false} />)

    await waitFor(() => {
      expect(screen.getByTestId('chart-error')).toBeInTheDocument()
    })
  })

  it('shows empty placeholder when penetration is empty', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ ...mockPenetration, penetration: [] }),
    } as Response)

    render(<PortfolioPenetration dark={false} />)
    await waitFor(() => {
      expect(screen.getByTestId('chart-empty')).toBeInTheDocument()
    })
  })

  it('drill-down detail uses Kumo Table (no raw <table>) — P0 fix', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPenetration),
    } as Response)

    render(<PortfolioPenetration dark={false} />)
    await waitFor(() => {
      expect(screen.getByTestId('treemap-chart')).toBeInTheDocument()
    })

    // Simulate a treemap click → component sets selectedStock → detail view renders.
    await act(async () => {
      clickHandlers.click?.({ data: mockPenetration.penetration[0] })
    })

    // Detail view must render the Kumo Table (P0: was a raw <table>).
    await waitFor(() => {
      expect(screen.getByTestId('kumo-table')).toBeInTheDocument()
    })
    // And the holding detail heading.
    expect(screen.getByText('持有明细')).toBeInTheDocument()
  })
})
