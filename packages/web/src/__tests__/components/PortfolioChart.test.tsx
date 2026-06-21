import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'

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
}))

vi.mock('echarts/charts', () => ({
  LineChart: {},
  BarChart: {},
}))

vi.mock('echarts/components', () => ({
  GridComponent: {},
  TooltipComponent: {},
  LegendComponent: {},
  DataZoomComponent: {},
}))

vi.mock('echarts/renderers', () => ({
  CanvasRenderer: {},
}))

// Import the mocked module to verify spy calls
import * as echartsCore from 'echarts/core'
import PortfolioChart from '../../components/PortfolioChart'

// ── Mock portfolio timeline data ──────────────────────────────

const mockTimeline = [
  { date: '2024-01-02', total_value: 10000, total_cost: 9000, pnl: 1000, pnl_pct: '11.11' },
  { date: '2024-01-03', total_value: 10200, total_cost: 9000, pnl: 1200, pnl_pct: '13.33' },
  { date: '2024-01-04', total_value: 10100, total_cost: 9000, pnl: 1100, pnl_pct: '12.22' },
  { date: '2024-01-05', total_value: 10500, total_cost: 9000, pnl: 1500, pnl_pct: '16.67' },
  { date: '2024-01-08', total_value: 10300, total_cost: 9200, pnl: 1100, pnl_pct: '11.96' },
  { date: '2024-01-09', total_value: 9800,  total_cost: 9200, pnl: 600,  pnl_pct: '6.52' },
  { date: '2024-01-10', total_value: 9700,  total_cost: 9200, pnl: 500,  pnl_pct: '5.43' },
]

describe('PortfolioChart', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders heading and subtitle after timeline fetch', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockTimeline),
    } as Response)

    render(<PortfolioChart dark={false} />)

    await waitFor(() => {
      expect(screen.getByText('组合净值走势')).toBeInTheDocument()
    })

    // Subtitle describing the lines
    const subtitle = screen.getByText(/蓝线:市值/)
    expect(subtitle).toBeInTheDocument()
    expect(subtitle.textContent).toContain('虚线:成本')
    expect(subtitle.textContent).toContain('柱状:每日盈亏')
    expect(subtitle.textContent).toContain('红涨绿跌')
  })

  it('renders chart container with 420px height', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockTimeline),
    } as Response)

    render(<PortfolioChart dark={false} />)

    await waitFor(() => {
      expect(screen.getByText('组合净值走势')).toBeInTheDocument()
    })

    const chartContainer = document.querySelector('[style*="height: 420px"]')
    expect(chartContainer).toBeInTheDocument()
  })

  it('verifies total value line + cost line + PnL bar series are set on echarts', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockTimeline),
    } as Response)

    render(<PortfolioChart dark={false} />)

    await waitFor(() => {
      expect(screen.getByText('组合净值走势')).toBeInTheDocument()
    })

    // echarts.init was called and setOption was invoked
    expect(echartsCore.init).toHaveBeenCalled()

    // Verify the series config contains three series: 市值, 成本, 盈亏
    const mockInstance = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value
    if (mockInstance) {
      expect(mockInstance.setOption).toHaveBeenCalled()
      const callArg = mockInstance.setOption.mock.calls[0]?.[0]
      expect(callArg).toBeDefined()
      expect(callArg.series).toHaveLength(3)
      expect(callArg.series[0].name).toBe('市值')
      expect(callArg.series[1].name).toBe('成本')
      expect(callArg.series[2].name).toBe('盈亏')
    }
  })

  it('renders with dark mode + dataZoom slider', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockTimeline),
    } as Response)

    render(<PortfolioChart dark={true} />)

    await waitFor(() => {
      expect(screen.getByText('组合净值走势')).toBeInTheDocument()
    })

    // Verify component rendered in dark mode without crashing
    expect(echartsCore.init).toHaveBeenCalled()
    const mockInstance = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value
    if (mockInstance) {
      expect(mockInstance.setOption).toHaveBeenCalled()
      // dataZoom should include both inside and slider types
      const callArg = mockInstance.setOption.mock.calls[0]?.[0]
      expect(callArg.dataZoom).toHaveLength(2)
      expect(callArg.dataZoom[0].type).toBe('inside')
      expect(callArg.dataZoom[1].type).toBe('slider')
    }
  })

  it('renders empty placeholder when timeline returns empty array', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve([]),
    } as Response)

    render(<PortfolioChart dark={false} />)

    // v3.0: empty state shows a placeholder instead of returning null (silent blank)
    await waitFor(() => {
      expect(screen.getByTestId('chart-empty')).toBeInTheDocument()
    })
  })

  it('renders error placeholder on fetch failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'))

    render(<PortfolioChart dark={false} />)

    // v3.0: fetch failure shows an error placeholder instead of silent null
    await waitFor(() => {
      expect(screen.getByTestId('chart-error')).toBeInTheDocument()
    })
  })
})
