import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Text } from '@cloudflare/kumo'
import { fetchNav, type NavPoint } from '../api'
import { getTheme, chartAxis, chartTooltip, chartLegend, chartDataZoom, space, radius, fontSize, fontWeight, chartHeight } from '../styles/theme'
import { useEChart } from '../hooks/useEChart'
import { ChartShell, useChartData, lineSeries, useCoreCharts } from './charts'
import { calcIRR } from '../services/irr'

useCoreCharts()

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

  const { data: navData, loading, error } = useChartData<NavPoint[]>(
    (signal) => fetchNav(fundCode, signal),
    [fundCode],
  );
  const nav = navData ?? [];

  // ── Compute strategy simulations ───────────────────────────────
  const result = useMemo((): BacktestResult | null => {
    if (nav.length < 2) return null
    const sorted = [...nav].sort((a, b) => a.date.localeCompare(b.date))
    const firstNav = sorted[0].unit_nav

    // DCA: invest baseAmount at the first data point of each month
    const dcaValues: number[] = []
    const dates: string[] = []
    let dcaShares = 0
    let dcaInvested = 0
    let lastMonth = ''

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
  }, [nav, baseAmount]);

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
        axisLabel: { formatter: (v: number) => v >= 1e4 ? `¥${(v / 1e4).toFixed(1)}${t('dca.valueUnit10k')}` : `¥${v.toFixed(0)}`, color: theme.textMuted },
      },
      dataZoom: chartDataZoom(theme),
      series: [
        lineSeries({ name: t('dca.dcaLegend'), data: result.dcaValues, color: theme.blue, area: true, width: 2 }),
        lineSeries({ name: t('dca.lumpLegend'), data: result.lumpValues, color: theme.amber, width: 1.5, dashed: true }),
      ],
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [result, dark]);

  const chartRef = useEChart(option, [option]);

  return (
    <ChartShell
      dark={dark}
      marginBottom={space[5]}
      height={chartHeight.backtest}
      title={t('dca.backtestCompare')}
      subtitle={t('dca.backtestDesc', { amount: baseAmount })}
      loading={loading}
      loadingText={t('dca.loading')}
      error={error}
      empty={!result}
    >
      <div ref={chartRef} style={{ height: chartHeight.backtest }} />
      {result && (
        <div style={{ padding: `${space[2]}px 0 ${space[4]}px`, display: 'flex', gap: space[5], flexWrap: 'wrap' }}>
          <StatItem label={t('dca.dcaIrr')} value={result.dcaIrr != null ? `${(result.dcaIrr * 100).toFixed(2)}%` : '-'} />
          <StatItem label={t('dca.lumpIrr')} value={result.lumpIrr != null ? `${(result.lumpIrr * 100).toFixed(2)}%` : '-'} />
          <StatItem label={t('dca.totalInvested')} value={`¥${result.dcaInvested.toFixed(0)}`} />
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
      )}
    </ChartShell>
  )
}

// ── Stat helper ──────────────────────────────────────────────────

function StatItem({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <div>
      <Text variant="secondary" as="span" size="xs">{label}</Text>
      <div style={{ marginTop: 2, fontWeight: fontWeight.semibold, color, fontSize: fontSize.lg }}>{value}</div>
    </div>
  )
}

export default DcaBacktestChart
