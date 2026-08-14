import { chartHeight } from '../../styles/theme'
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
  ScatterChart: {},
  RadarChart: {},
  TreemapChart: {},
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

vi.mock('echarts/renderers', () => ({
  CanvasRenderer: {},
}))

// Import the mocked module to verify spy calls
import * as echartsCore from 'echarts/core'
import PortfolioChart from '../../components/PortfolioChart'

// ── Mock portfolio timeline data ──────────────────────────────

const mockTimeline = [
  { date: '2024-01-02', total_value: 10000, total_cost: 9000, pnl: 1000, pnl_pct: 11.11 },
  { date: '2024-01-03', total_value: 10200, total_cost: 9000, pnl: 1200, pnl_pct: 13.33 },
  { date: '2024-01-04', total_value: 10100, total_cost: 9000, pnl: 1100, pnl_pct: 12.22 },
  { date: '2024-01-05', total_value: 10500, total_cost: 9000, pnl: 1500, pnl_pct: 16.67 },
  { date: '2024-01-08', total_value: 10300, total_cost: 9200, pnl: 1100, pnl_pct: 11.96 },
  { date: '2024-01-09', total_value: 9800,  total_cost: 9200, pnl: 600,  pnl_pct: 6.52 },
  { date: '2024-01-10', total_value: 9700,  total_cost: 9200, pnl: 500,  pnl_pct: 5.43 },
]

describe('PortfolioChart', () => {
  afterEach(() => {
    // clearAllMocks resets the echarts `init` vi.fn mock.results so results[0]
    // is THIS test's instance (restoreAllMocks alone doesn't clear vi.mock fns,
    // so results[0] would otherwise stay the first test's stale instance).
    vi.clearAllMocks()
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

    const chartContainer = document.querySelector(`[style*="height: ${chartHeight.default}px"]`)
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

    // echarts.init was called and setOption was invoked — gate on init
    // being called to avoid CI race where title renders but echarts hasn't
    // initialised yet.
    await waitFor(() => {
      const calls = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value?.setOption?.mock?.calls ?? []
      expect(calls.length).toBeGreaterThan(0)
    })

    // Verify the series config contains three series: 市值, 成本, 盈亏
    const mockInstance = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value
    expect(mockInstance).toBeDefined()
    const callArg = mockInstance!.setOption.mock.calls[0]?.[0]
    expect(callArg).toBeDefined()
    expect(callArg.series).toHaveLength(3)
    expect(callArg.series[0].name).toBe('市值')
    expect(callArg.series[1].name).toBe('成本')
    expect(callArg.series[2].name).toBe('盈亏')
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

    // Gate on setOption to avoid CI race.
    await waitFor(() => {
      const calls = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value?.setOption?.mock?.calls ?? []
      expect(calls.length).toBeGreaterThan(0)
    })

    const mockInstance = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value
    // dataZoom should include both inside and slider types
    const callArg = mockInstance!.setOption.mock.calls[0]?.[0]
    expect(callArg.dataZoom).toHaveLength(2)
    expect(callArg.dataZoom[0].type).toBe('inside')
    expect(callArg.dataZoom[1].type).toBe('slider')
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

  // ── Red-up/green-down color guards (CN convention) ──────────────
  // Today there are zero color-value assertions anywhere; a red/green swap would
  // ship undetected. These lock the PnL bar coloring to theme tokens.

  it('colors daily PnL bars red-up/green-down in light mode', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockTimeline),
    } as Response)

    render(<PortfolioChart dark={false} />)
    // gate on setOption fired (heading renders during loading — would race otherwise).
    // Use a calls-length assertion so the matcher always runs on a number (calling
    // toHaveBeenCalled on a possibly-undefined mi throws a non-retryable error).
    await waitFor(() => {
      const calls = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value?.setOption?.mock?.calls ?? []
      expect(calls.length).toBeGreaterThan(0)
    })

    const mockInstance = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value
    const callArg = mockInstance.setOption.mock.calls[0]?.[0]
    // series[2] = daily PnL bar; itemStyle.color is the per-value up/down fn
    const colorFn = callArg.series[2].itemStyle.color
    expect(colorFn({ value: 100 })).toBe('#d63649') // light theme.up = red (profit)
    expect(colorFn({ value: -50 })).toBe('#199c63') // light theme.down = green (loss)
    // string values compared after Number()
    expect(colorFn({ value: '-5' })).toBe('#199c63')
  })

  it('uses dark-mode red/green tokens in dark mode', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockTimeline),
    } as Response)

    render(<PortfolioChart dark={true} />)
    await waitFor(() => {
      const calls = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value?.setOption?.mock?.calls ?? []
      expect(calls.length).toBeGreaterThan(0)
    })

    const mockInstance = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value
    const callArg = mockInstance.setOption.mock.calls[0]?.[0]
    const colorFn = callArg.series[2].itemStyle.color
    expect(colorFn({ value: 100 })).toBe('#f87171') // dark theme.up = brighter red
    expect(colorFn({ value: -50 })).toBe('#4ade80') // dark theme.down = brighter green
  })

  it('renders without crashing on a single-point timeline (boundary)', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve([mockTimeline[0]]),
    } as Response)

    render(<PortfolioChart dark={false} />)
    await waitFor(() => {
      expect(screen.getByText('组合净值走势')).toBeInTheDocument()
    })

    await waitFor(() => {
      const calls = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value?.setOption?.mock?.calls ?? []
      expect(calls.length).toBeGreaterThan(0)
    })
  })
})
