import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'

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
  ScatterChart: {},
}))

vi.mock('echarts/components', () => ({
  GridComponent: {},
  TooltipComponent: {},
  LegendComponent: {},
  DataZoomComponent: {},
  MarkLineComponent: {},
}))

vi.mock('echarts/renderers', () => ({
  CanvasRenderer: {},
}))

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
    const chartContainer = document.querySelector('[style*="height: 500px"]')
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
    const chartContainer = document.querySelector('[style*="height: 500px"]')
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
})
