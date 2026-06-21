import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'

// Mock echarts before component import
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

vi.mock('echarts/charts', () => ({ LineChart: {} }))
vi.mock('echarts/components', () => ({
  GridComponent: {},
  TooltipComponent: {},
  LegendComponent: {},
  DataZoomComponent: {},
}))
vi.mock('echarts/renderers', () => ({ CanvasRenderer: {} }))

import DcaBacktestChart from '../../components/DcaBacktestChart'

// ── Mock NAV data spanning 6 months ──────────────────────────────

const mockNavData = [
  { date: '2024-01-02', unit_nav: 1.0000 },
  { date: '2024-01-15', unit_nav: 1.0200 },
  { date: '2024-01-30', unit_nav: 0.9800 },
  { date: '2024-02-05', unit_nav: 1.0500 },
  { date: '2024-02-20', unit_nav: 1.0300 },
  { date: '2024-03-04', unit_nav: 1.1000 },
  { date: '2024-03-18', unit_nav: 1.1200 },
  { date: '2024-04-02', unit_nav: 1.0800 },
  { date: '2024-04-16', unit_nav: 1.1500 },
  { date: '2024-05-06', unit_nav: 1.2000 },
  { date: '2024-05-20', unit_nav: 1.1800 },
  { date: '2024-06-03', unit_nav: 1.2500 },
]

describe('DcaBacktestChart', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders chart heading after NAV data loads', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockNavData),
    } as Response)

    render(<DcaBacktestChart fundCode="F01" dark={false} />)

    await waitFor(() => {
      expect(screen.getByText('DCA 回测对比')).toBeInTheDocument()
    })
    expect(screen.getByText(/蓝线:定投/)).toBeInTheDocument()
    expect(screen.getByText(/基础金额 ¥1000/)).toBeInTheDocument()
  })

  it('renders IRR, total invested, market value, and P&L stats', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockNavData),
    } as Response)

    render(<DcaBacktestChart fundCode="F02" dark={false} />)

    await waitFor(() => {
      expect(screen.getByText('DCA 回测对比')).toBeInTheDocument()
    })

    expect(screen.getByText('定投 IRR')).toBeInTheDocument()
    expect(screen.getByText('一次性 IRR')).toBeInTheDocument()
    expect(screen.getByText('总投入')).toBeInTheDocument()
    expect(screen.getByText('定投市值')).toBeInTheDocument()
    expect(screen.getByText('一次性市值')).toBeInTheDocument()
    expect(screen.getByText('定投盈亏')).toBeInTheDocument()
    expect(screen.getByText('一次性盈亏')).toBeInTheDocument()

    // 6 months x 1000 = 6000 total invested
    expect(screen.getByText('¥6000')).toBeInTheDocument()
  })

  it('renders without crashing in dark mode', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockNavData),
    } as Response)

    render(<DcaBacktestChart fundCode="F03" dark={true} />)

    await waitFor(() => {
      expect(screen.getByText('DCA 回测对比')).toBeInTheDocument()
    })
    expect(screen.getByText('总投入')).toBeInTheDocument()
  })

  it('shows loading text before NAV data resolves', () => {
    // Use a deferred promise so we can observe loading state without
    // polluting the inflight cache for other tests
    let resolvePromise!: (v: any) => void
    const deferred = new Promise<any>(r => { resolvePromise = r })
    vi.spyOn(globalThis, 'fetch').mockReturnValueOnce(deferred as any)

    render(<DcaBacktestChart fundCode="F04" dark={false} />)
    expect(screen.getByText('加载历史净值...')).toBeInTheDocument()

    // Cleanup: resolve the deferred promise so inflight cache clears
    resolvePromise({ ok: true, json: () => Promise.resolve(mockNavData) })
  })

  it('shows error message on fetch failure', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'))

    render(<DcaBacktestChart fundCode="F05" dark={false} />)

    // v3.0: fetch failure shows error placeholder with generic error text
    await waitFor(() => {
      expect(screen.getByTestId('chart-error')).toBeInTheDocument()
    })
  })

  it('renders nothing when NAV data has fewer than 2 points', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve([{ date: '2024-01-02', unit_nav: 1.0 }]),
    } as Response)

    render(<DcaBacktestChart fundCode="F06" dark={false} />)

    // v3.0: fewer than 2 points shows empty placeholder instead of returning null
    await waitFor(() => {
      expect(screen.getByTestId('chart-empty')).toBeInTheDocument()
    })
  })

  it('displays custom base amount in subtitle', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockNavData),
    } as Response)

    render(<DcaBacktestChart fundCode="F07" dark={false} baseAmount={500} />)

    await waitFor(() => {
      expect(screen.getByText(/基础金额 ¥500/)).toBeInTheDocument()
    })
  })
})
