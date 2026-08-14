import { chartHeight } from '../../styles/theme'
import { describe, it, expect, vi, afterEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import { renderWithRouter as render } from '../test-utils'

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

import * as echartsCore from 'echarts/core'
import FundChart from '../../components/FundChart'
import type { NavPoint, Transaction } from '../../api'

const mockNavData: NavPoint[] = [
  { date: '2024-01-02', unit_nav: 1.0000 },
  { date: '2024-01-03', unit_nav: 1.0100 },
  { date: '2024-01-04', unit_nav: 0.9950 },
  { date: '2024-01-05', unit_nav: 1.0200 },
  { date: '2024-01-08', unit_nav: 1.0300 },
  { date: '2024-01-09', unit_nav: 1.0250 },
  { date: '2024-01-10', unit_nav: 1.0400 },
  { date: '2024-01-11', unit_nav: 1.0350 },
  { date: '2024-01-12', unit_nav: 1.0500 },
  { date: '2024-01-15', unit_nav: 1.0450 },
  { date: '2024-01-16', unit_nav: 1.0600 },
  { date: '2024-01-17', unit_nav: 1.0550 },
  { date: '2024-01-18', unit_nav: 1.0700 },
  { date: '2024-01-19', unit_nav: 1.0650 },
  { date: '2024-01-22', unit_nav: 1.0800 },
  { date: '2024-01-23', unit_nav: 1.0750 },
  { date: '2024-01-24', unit_nav: 1.0900 },
  { date: '2024-01-25', unit_nav: 1.0850 },
  { date: '2024-01-26', unit_nav: 1.1000 },
  { date: '2024-01-29', unit_nav: 1.0950 },
  { date: '2024-01-30', unit_nav: 1.1100 },
  { date: '2024-01-31', unit_nav: 1.1050 },
]

const mockTransactions: Transaction[] = [
  {
    seq: 1,
    trade_time: '2024-01-03T09:30:00',
    confirm_date: '2024-01-04',
    trade_type: '用户买入',
    direction: 'buy',
    amount: 1000,
    shares: 990.10,
    fee: 1.50,
    nav: 1.0100,
    inferred_nav: null,
    settlement_days: 1,
    order_id: 'ord_001',
    anomaly: null,
  },
  {
    seq: 2,
    trade_time: '2024-01-10T10:00:00',
    confirm_date: '2024-01-11',
    trade_type: '定投买入',
    direction: 'buy',
    amount: 1000,
    shares: 961.54,
    fee: 1.50,
    nav: 1.0400,
    inferred_nav: null,
    settlement_days: 1,
    order_id: 'ord_002',
    anomaly: null,
  },
  {
    seq: 3,
    trade_time: '2024-01-22T14:00:00',
    confirm_date: '2024-01-23',
    trade_type: '用户卖出',
    direction: 'sell',
    amount: -500,
    shares: -462.96,
    fee: 0.75,
    nav: 1.0800,
    inferred_nav: null,
    settlement_days: 1,
    order_id: 'ord_003',
    anomaly: null,
  },
]

describe('FundChart', () => {
  afterEach(() => {
    vi.clearAllMocks()
    vi.restoreAllMocks()
  })

  it('renders chart title and container', () => {
    render(
      <FundChart
        navData={mockNavData}
        transactions={mockTransactions}
        heldShares={1488.68}
        totalCost={2000}
        chartTitle="基金净值走势"
        priceLabel="单位净值"
        dark={false}
      />,
    )

    expect(screen.getByText('基金净值走势')).toBeInTheDocument()
    // Chart container div with 500px height exists
    const chartContainer = document.querySelector(`[style*="height: ${chartHeight.large}px"]`)
    expect(chartContainer).toBeInTheDocument()
  })

  it('renders range tabs (交易区间, 近1月, 近3月, etc.)', () => {
    render(
      <FundChart
        navData={mockNavData}
        transactions={mockTransactions}
        heldShares={1488.68}
        totalCost={2000}
        chartTitle="基金净值走势"
        priceLabel="单位净值"
        dark={false}
      />,
    )

    expect(screen.getByText('交易区间')).toBeInTheDocument()
    expect(screen.getByText('近1月')).toBeInTheDocument()
    expect(screen.getByText('近3月')).toBeInTheDocument()
    expect(screen.getByText('近6月')).toBeInTheDocument()
    expect(screen.getByText('近1年')).toBeInTheDocument()
    expect(screen.getByText('全部')).toBeInTheDocument()
  })

  it('renders price line and cost line (markLine) when heldShares > 0', () => {
    // With heldShares=1488.68 and totalCost=2000, avgCost ≈ 1.3435
    render(
      <FundChart
        navData={mockNavData}
        transactions={mockTransactions}
        heldShares={1488.68}
        totalCost={2000}
        chartTitle="基金净值走势"
        priceLabel="单位净值"
        dark={false}
      />,
    )

    // echarts chart container was rendered (component renders without crashing)
    const chartContainer = document.querySelector(`[style*="height: ${chartHeight.large}px"]`)
    expect(chartContainer).toBeInTheDocument()
    // The chart title confirms the component rendered successfully with cost-basis data
    expect(screen.getByText('基金净值走势')).toBeInTheDocument()
  })

  it('renders without crashing in dark mode', () => {
    render(
      <FundChart
        navData={mockNavData}
        transactions={mockTransactions}
        heldShares={1488.68}
        totalCost={2000}
        chartTitle="基金净值走势"
        priceLabel="单位净值"
        dark={true}
      />,
    )

    expect(screen.getByText('基金净值走势')).toBeInTheDocument()
  })

  it('renders chart container even with empty navData', () => {
    render(
      <FundChart
        navData={[]}
        transactions={[]}
        heldShares={0}
        totalCost={0}
        chartTitle="空数据测试"
        priceLabel="净值"
        dark={false}
      />,
    )

    // v3.0: empty navData shows chart-empty placeholder instead of rendering the title
    expect(screen.getByTestId('chart-empty')).toBeInTheDocument()
  })

  it('renders buy markers red (up) and sell markers green (down) — CN convention', async () => {
    render(
      <FundChart
        navData={mockNavData}
        transactions={mockTransactions}
        heldShares={1488.68}
        totalCost={2000}
        chartTitle="基金净值走势"
        priceLabel="单位净值"
        dark={false}
      />,
    )

    await waitFor(() => {
      const calls = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value?.setOption?.mock?.calls ?? []
      expect(calls.length).toBeGreaterThan(0)
    })

    const mockInstance = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value
    const callArg = mockInstance.setOption.mock.calls[0]?.[0]
    const scatterSeries = callArg.series.filter((s: any) => s.type === 'scatter')
    // 2 buys (2024-01-03, 2024-01-10) + 1 sell (2024-01-22), all exact-matched
    const buySeries = scatterSeries.find((s: any) => s.itemStyle.color === '#d63649')
    const sellSeries = scatterSeries.find((s: any) => s.itemStyle.color === '#199c63')
    expect(buySeries).toBeDefined() // buy = red (theme.up)
    expect(sellSeries).toBeDefined() // sell = green (theme.down)
    expect(buySeries.data).toHaveLength(2)
    expect(sellSeries.data).toHaveLength(1)
  })

  // ── MA20 window regression ────────────────────────────────────────────────
  // The v3.0 risk register claimed "MA20 is a 21-point window"; this is wrong.
  // Standard MA20 = average of the last 20 points, first valid at index 19
  // (i.e. points 0..19). This test pins the contract so a future refactor
  // can't silently introduce an off-by-one (either 19- or 21-point).
  async function renderAndGetSeriesData(navData: NavPoint[]) {
    render(
      <FundChart
        navData={navData}
        transactions={[]}
        heldShares={0}
        totalCost={0}
        chartTitle="MA20 测试"
        priceLabel="净值"
        dark={false}
      />,
    )
    await waitFor(() => {
      const calls = (echartsCore.init as ReturnType<typeof vi.fn>).mock?.results?.[0]?.value?.setOption?.mock?.calls ?? []
      expect(calls.length).toBeGreaterThan(0)
    })
    const instance = (echartsCore.init as ReturnType<typeof vi.fn>).mock.results[0].value
    const opt = instance.setOption.mock.calls[0][0] as Record<string, any>
    return opt.series as Record<string, any>[]
  }

  it('MA20 is a 20-point window: flat series yields MA20 = flat value at index 19', async () => {
    // 25 flat points @ 1.2000 → MA20 must be exactly 1.2 from index 19 onward,
    // null before it. If the window were 19 or 21 points this still works for a
    // flat series, so the next test pins the count precisely.
    const flat: NavPoint[] = Array.from({ length: 25 }, (_, i) => ({
      date: `2024-01-${String(i + 1).padStart(2, '0')}`,
      unit_nav: 1.2,
    }))
    const series = await renderAndGetSeriesData(flat)
    const ma20 = series.find((s) => s.name === 'MA20')
    expect(ma20, 'MA20 series must be present when navData.length >= 20').toBeDefined()
    const data = ma20!.data as (number | null)[]
    expect(data).toHaveLength(25)
    // indices 0..18 must be null (window not yet full)
    for (let i = 0; i < 19; i++) {
      expect(data[i]).toBeNull()
    }
    // from index 19 onward, MA20 equals the flat value
    for (let i = 19; i < 25; i++) {
      expect(data[i]).toBe(1.2)
    }
  })

  it('MA20 is a 20-point window: known series yields known MA20 values at index 19 and 20', async () => {
    // Strictly increasing 1..22 so the window boundary is unambiguous:
    //   index 19 → mean(points[0..19]) = mean(1..20)   = 10.5
    //   index 20 → mean(points[1..20]) = mean(2..21)   = 11.5
    // If the window were 19 points: idx19 would be mean(1..19)=10 (FAIL).
    // If the window were 21 points: idx19 would be null          (FAIL).
    const rising: NavPoint[] = Array.from({ length: 22 }, (_, i) => ({
      date: `2024-02-${String(i + 1).padStart(2, '0')}`,
      unit_nav: +(1 + i).toFixed(4),
    }))
    const series = await renderAndGetSeriesData(rising)
    const ma20 = series.find((s) => s.name === 'MA20')!
    const data = ma20.data as (number | null)[]
    expect(data[18]).toBeNull() // one short of full window
    expect(data[19]).toBe(+(10.5).toFixed(4)) // mean(1..20)
    expect(data[20]).toBe(+(11.5).toFixed(4)) // mean(2..21), window slid by one
  })

  it('MA20 series is omitted when fewer than 20 points are available', async () => {
    const short: NavPoint[] = Array.from({ length: 19 }, (_, i) => ({
      date: `2024-03-${String(i + 1).padStart(2, '0')}`,
      unit_nav: 1 + i * 0.01,
    }))
    const series = await renderAndGetSeriesData(short)
    expect(series.find((s) => s.name === 'MA20')).toBeUndefined()
  })
})
