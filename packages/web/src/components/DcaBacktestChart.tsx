import { useState, useEffect, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Text } from '@cloudflare/kumo'
import { use as echartsUse } from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, DataZoomComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { fetchNav, type NavPoint } from '../api'
import { getTheme, chartAxis, chartTooltip, chartLegend, chartDataZoom, areaGradient } from '../styles/theme'
import { useEChart } from '../hooks/useEChart'
import { Card } from './ui/Card'

echartsUse([LineChart, GridComponent, TooltipComponent, LegendComponent, DataZoomComponent, CanvasRenderer])

// ── IRR via Newton's method ──────────────────────────────────────

function calcIRR(cashflows: number[], dates: Date[]): number | null {
  if (cashflows.length < 2) return null
  const allPos = cashflows.every(c => c >= 0)
  const allNeg = cashflows.every(c => c <= 0)
  if (allPos || allNeg) return null
  let rate = 0.1
  const msPerYear = 365.25 * 24 * 3600 * 1000
  const t0 = dates[0].getTime()
  for (let iter = 0; iter < 200; iter++) {
    let npv = 0, dnpv = 0
    for (let i = 0; i < cashflows.length; i++) {
      const yrs = (dates[i].getTime() - t0) / msPerYear
      const denom = Math.pow(1 + rate, yrs)
      npv += cashflows[i] / denom
      if (yrs > 0) dnpv += -yrs * cashflows[i] / (denom * (1 + rate))
    }
    if (Math.abs(dnpv) < 1e-15 || Math.abs(npv) < 1e-12) break
    const nr = rate - npv / dnpv
    if (Math.abs(nr - rate) < 1e-12) { rate = nr; break }
    rate = nr
    if (rate <= -1) rate = -0.999
    if (rate > 100) rate = 100
  }
  return isNaN(rate) || !isFinite(rate) || rate <= -0.999 ? null : rate
}

// ── Types ────────────────────────────────────────────────────────

interface BacktestResult {
  dates: string[]
  navs: number[]
  dcaValues: number[]
  lumpValues: number[]
  dcaInvested: number
  dcaFinalValue: number
  dcaPnl: number
  dcaPnlPct: number
  lumpFinalValue: number
  lumpPnl: number
  lumpPnlPct: number
  dcaIrr: number | null
  lumpIrr: number | null
}

interface DcaBacktestChartProps {
  fundCode: string
  dark: boolean
  baseAmount?: number
}

// ── Component ────────────────────────────────────────────────────

function DcaBacktestChart({ fundCode, dark, baseAmount = 1000 }: DcaBacktestChartProps) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const [navData, setNavData] = useState<NavPoint[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // ── Fetch NAV history ──────────────────────────────────────────
  useEffect(() => {
    const ctrl = new AbortController()
    setLoading(true)
    setError('')
    fetchNav(fundCode, ctrl.signal)
      .then(d => { setNavData(d); setLoading(false) })
      .catch(e => {
        if (e.name !== 'AbortError') { setError(e.message || t('dca.error')); setLoading(false) }
      })
    return () => ctrl.abort()
  }, [fundCode])

  // ── Compute strategy simulations ───────────────────────────────
  const result = useMemo((): BacktestResult | null => {
    if (navData.length < 2) return null
    const sorted = [...navData].sort((a, b) => a.date.localeCompare(b.date))
    const firstNav = sorted[0].unit_nav

    // DCA: invest baseAmount at the first data point of each month
    const dcaValues: number[] = []
    const dates: string[] = []
    let dcaShares = 0
    let dcaInvested = 0
    let lastMonth = ''

    // IRR tracking
    const irrCashflows: number[] = []
    const irrDates: Date[] = []

    for (const pt of sorted) {
      const month = pt.date.substring(0, 7)
      if (month !== lastMonth) {
        const shares = baseAmount / pt.unit_nav
        dcaShares += shares
        dcaInvested += baseAmount
        irrCashflows.push(-baseAmount)
        irrDates.push(new Date(pt.date))
        lastMonth = month
      }
      dcaValues.push(dcaShares * pt.unit_nav)
      dates.push(pt.date)
    }

    // Lump sum: invest all at the start
    const lumpShares = dcaInvested / firstNav
    const lumpValues = sorted.map(p => lumpShares * p.unit_nav)

    const dcaFinal = dcaValues[dcaValues.length - 1]
    const lumpFinal = lumpValues[lumpValues.length - 1]

    // DCA IRR: add final value as positive cashflow
    irrCashflows.push(dcaFinal)
    irrDates.push(new Date(sorted[sorted.length - 1].date))
    const dcaIrr = calcIRR([...irrCashflows], [...irrDates])

    // Lump sum IRR
    const lumpIrr = calcIRR(
      [-dcaInvested, lumpFinal],
      [new Date(sorted[0].date), new Date(sorted[sorted.length - 1].date)],
    )

    return {
      dates,
      navs: sorted.map(p => p.unit_nav),
      dcaValues,
      lumpValues,
      dcaInvested,
      dcaFinalValue: dcaFinal,
      dcaPnl: dcaFinal - dcaInvested,
      dcaPnlPct: ((dcaFinal - dcaInvested) / dcaInvested) * 100,
      lumpFinalValue: lumpFinal,
      lumpPnl: lumpFinal - dcaInvested,
      lumpPnlPct: ((lumpFinal - dcaInvested) / dcaInvested) * 100,
      dcaIrr,
      lumpIrr,
    }
  }, [navData, baseAmount])

  const option = useMemo(() => {
    if (!result) return {} as Record<string, unknown>;
    return {
      tooltip: {
        trigger: 'axis',
        ...chartTooltip(theme),
        formatter: (params: any) => {
          const idx = params[0]?.dataIndex;
          if (idx == null) return '';
          const r = result;
          return t('dca.tooltip', { nav: r.navs[idx].toFixed(4), dca: r.dcaValues[idx].toFixed(0), lump: r.lumpValues[idx].toFixed(0) });
        },
      },
      legend: {
        data: [t('dca.dcaLegend'), t('dca.lumpLegend')], top: 4,
        ...chartLegend(theme),
      },
      grid: { left: 70, right: 30, top: 36, bottom: 44 },
      xAxis: { type: 'category', data: result.dates, boundaryGap: false, ...chartAxis(theme) },
      yAxis: {
        type: 'value', ...chartAxis(theme),
        axisLabel: { formatter: (v: number) => v >= 1e4 ? `¥${(v / 1e4).toFixed(1)}万` : `¥${v.toFixed(0)}`, color: theme.textMuted },
      },
      dataZoom: chartDataZoom(theme),
      series: [
        {
          name: t('dca.dcaLegend'), type: 'line', data: result.dcaValues, smooth: true, symbol: 'none',
          lineStyle: { color: theme.blue, width: 2 },
          areaStyle: { color: areaGradient(theme, theme.blue) },
        },
        {
          name: t('dca.lumpLegend'), type: 'line', data: result.lumpValues, smooth: true, symbol: 'none',
          lineStyle: { color: theme.amber, width: 1.5, type: 'dashed' },
        },
      ],
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [result, dark]);

  const chartRef = useEChart(option, [option]);

  // ── Loading / Error / Empty ─────────────────────────────────────
  const placeholder = (msg: string, testid: string) => (
    <div data-testid={testid} style={{ height: 420, display: 'flex', alignItems: 'center', justifyContent: 'center', color: theme.textMuted, fontVariantNumeric: 'tabular-nums' }}>
      {msg}
    </div>
  );

  if (loading) {
    return (
      <Card dark={dark} style={{ marginBottom: 20 }}>
        {placeholder(t('dca.loading'), 'chart-loading')}
      </Card>
    )
  }

  if (error) {
    return (
      <Card dark={dark} style={{ marginBottom: 20 }}>
        {placeholder(t('common.loadError', '加载失败'), 'chart-error')}
      </Card>
    )
  }

  if (!result) {
    return (
      <Card dark={dark} style={{ marginBottom: 20 }}>
        {placeholder(t('common.noData', '暂无数据'), 'chart-empty')}
      </Card>
    )
  }

  return (
    <Card dark={dark} style={{ marginBottom: 20 }}>
      <div style={{ padding: '4px 0 16px' }}>
        <Text variant="heading3" as="h3">{t('dca.backtestCompare')}</Text>
        <Text variant="secondary" as="span" size="xs" style={{ marginTop: 2 }}>
          {t('dca.backtestDesc', { amount: baseAmount })}
        </Text>
      </div>
      <div ref={chartRef} style={{ height: 380 }} />

      {/* Stats row */}
      <div style={{ padding: '8px 0 16px', display: 'flex', gap: 20, flexWrap: 'wrap' }}>
        <StatItem label={t('dca.dcaIrr')} value={result.dcaIrr != null ? `${(result.dcaIrr * 100).toFixed(2)}%` : '-'} />
        <StatItem label={t('dca.lumpIrr')} value={result.lumpIrr != null ? `${(result.lumpIrr * 100).toFixed(2)}%` : '-'} />
        <StatItem label={t('backtest.totalInvested')} value={`¥${result.dcaInvested.toFixed(0)}`} />
        <StatItem label={t('dca.dcaLegend')} value={`¥${result.dcaFinalValue.toFixed(0)}`} />
        <StatItem label={t('dca.lumpLegend')} value={`¥${result.lumpFinalValue.toFixed(0)}`} />
        <StatItem
          label={t('dca.dcaPnl')}
          value={`${result.dcaPnl >= 0 ? '+' : ''}¥${result.dcaPnl.toFixed(0)} (${result.dcaPnlPct >= 0 ? '+' : ''}${result.dcaPnlPct.toFixed(1)}%)`}
          color={result.dcaPnl >= 0 ? theme.up : theme.down}
        />
        <StatItem
          label={t('dca.lumpPnl')}
          value={`${result.lumpPnl >= 0 ? '+' : ''}¥${result.lumpPnl.toFixed(0)} (${result.lumpPnlPct >= 0 ? '+' : ''}${result.lumpPnlPct.toFixed(1)}%)`}
          color={result.lumpPnl >= 0 ? theme.up : theme.down}
        />
      </div>
    </Card>
  )
}

// ── Stat helper ──────────────────────────────────────────────────

function StatItem({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <div>
      <Text variant="secondary" as="span" size="xs">{label}</Text>
      <div style={{ marginTop: 2, fontWeight: 600, color, fontSize: 14 }}>{value}</div>
    </div>
  )
}

export default DcaBacktestChart
