import { useState, useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Button, Table } from '@cloudflare/kumo'
import { CaretLeft } from '@phosphor-icons/react'
import { getInstanceByDom } from 'echarts/core'
import { getTheme, chartTooltip, chartShadowColor, space, radius, fontSize, fontWeight, chartHeight } from '../styles/theme'
import { useEChart } from '../hooks/useEChart'
import { Card } from './ui/Card'
import { useChartData, useCoreCharts, ChartShell } from './charts'
import { useAppStore } from '../stores/appStore'
import {
  fetchPortfolioPenetration,
  type PenetrationStock,
  type PenetrationFund,
  type PenetrationResult,
} from '../api'
import { classifySector, SECTOR_COLORS, SECTOR_NAMES } from '../services/sector'

useCoreCharts()

export default function PortfolioPenetration({ dark }: { dark: boolean }) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const portfolioId = useAppStore((s) => s.portfolioId);
  const [selectedStock, setSelectedStock] = useState<PenetrationStock | null>(null)

  // Resolve SECTOR_NAMES i18n keys → locale display labels (no Chinese hardcode fallback).
  const sectorName = useMemo<Record<string, string>>(() => {
    const labels: Record<string, string> = {}
    for (const [key, i18nKey] of Object.entries(SECTOR_NAMES)) {
      labels[key] = t(i18nKey)
    }
    return labels
  }, [t]);

  const { data, loading, error } = useChartData<PenetrationResult>(
    // AbortSignal is the 2nd arg — never pass it as portfolioId.
    (signal) => fetchPortfolioPenetration(portfolioId, signal),
    [portfolioId],
  );
  const stocks = data?.penetration ?? []
  const totalValue = data?.total_portfolio_value ?? 0
  const equityCount = data?.equity_fund_count ?? 0
  const uniqueStocks = data?.unique_stocks ?? 0

  // ── Treemap option ──────────────────────────────────────────────
  const treemapOption = useMemo(() => {
    if (!stocks.length) return {} as Record<string, unknown>

    type TreemapNode = {
      name: string
      value: number
      stock_code: string
      stock_name: string
      weight_pct: number
      held_by_funds: PenetrationFund[]
      itemStyle?: { color: string }
    }

    const nodes: TreemapNode[] = stocks.map(s => ({
      name: s.stock_name,
      value: s.total_exposure_cny,
      stock_code: s.stock_code,
      stock_name: s.stock_name,
      weight_pct: s.weight_pct,
      held_by_funds: s.held_by_funds,
      itemStyle: {
        color: SECTOR_COLORS[classifySector(s.stock_name).sectorKey] || SECTOR_COLORS.other,
      },
    }))

    return {
      tooltip: {
        trigger: 'item',
        ...chartTooltip(theme),
        formatter: (params: any) => {
          const d = params.data
          if (!d) return ''
          const funds = (d.held_by_funds as PenetrationFund[]) || []
          const fundList = funds
            .sort((a, b) => b.weight_pct - a.weight_pct)
            .map(f => `${f.fund_name}: ${f.weight_pct.toFixed(1)}%`)
            .join('<br/>  ')
          const sector = classifySector(d.stock_name || d.name).sectorKey
          return `<strong>${d.stock_name || d.name}</strong> (${d.stock_code || ''})<br/>
            ${t('penetration.sectorLabel')}: ${sectorName[sector] || sectorName.other || sector}<br/>
            ${t('penetration.totalExposure')}: ¥${(d.value as number).toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}<br/>
            ${t('penetration.portfolioWeight')}: ${(d.weight_pct as number).toFixed(2)}%<br/>
            ${t('penetration.holdingsDetail')}:<br/>  ${fundList || t('common.none')}`
        },
      },
      series: [
        {
          type: 'treemap',
          roam: false,
          nodeClick: 'link',
          breadcrumb: {
            show: true,
            height: 28,
            bottom: 0,
            itemStyle: {
              color: theme.surfaceHover,
              borderColor: theme.border,
              textStyle: { color: theme.text, fontSize: fontSize.sm },
            },
            emphasis: {
              itemStyle: { color: theme.surfaceHover },
            },
          },
          label: {
            show: true,
            formatter: (p: any) => {
              const name = p.name || ''
              const pct = p.data?.weight_pct ?? 0
              return `${name.length > 6 ? name.slice(0, 6) + '…' : name}\n${pct.toFixed(1)}%`
            },
            fontSize: fontSize.sm,
            color: theme.text,
          },
          upperLabel: {
            show: true,
            height: 20,
            fontSize: fontSize.xs,
            color: theme.textMuted,
          },
          itemStyle: {
            borderColor: theme.surface,
            borderWidth: 2,
            gapWidth: 2,
          },
          emphasis: {
            label: { fontSize: fontSize.base, fontWeight: fontWeight.bold },
            itemStyle: { shadowBlur: 10, shadowColor: chartShadowColor(theme) },
          },
          levels: [
            {
              colorMappingBy: 'value',
              itemStyle: { gapWidth: 2 },
            },
          ],
          data: nodes,
        },
      ],
    }
  }, [stocks, dark])

  const chartRef = useEChart(treemapOption, [treemapOption])

  // ── Click event binding for treemap drill-down ─────────────────
  useEffect(() => {
    const dom = chartRef.current
    if (!dom) return
    const inst = getInstanceByDom(dom)
    if (!inst) return

    const handler = (params: any) => {
      if (params.data && params.data.stock_code) {
        setSelectedStock(params.data as unknown as PenetrationStock & { stock_code: string; held_by_funds: PenetrationFund[] })
      }
    }

    inst.off('click')
    inst.on('click', handler)

    return () => { inst.off('click', handler) }
  }, [treemapOption])

  // ── Placeholder helper ──────────────────────────────────────────

  // ── Detail view: selected stock breakdown (Kumo Table — was raw <table>) ──
  if (selectedStock) {
    const sector = classifySector(selectedStock.stock_name).sectorKey
    const sectorColor = SECTOR_COLORS[sector] || SECTOR_COLORS.other
    return (
      <Card dark={dark} glass>
        <div style={{ display: 'flex', alignItems: 'center', gap: space[3], marginBottom: space[4] }}>
          <Button type="button" variant="secondary" size="sm" onClick={() => setSelectedStock(null)}
            style={{ padding: `${space[1]}px ${space[2]}px`, minWidth: 32 }}>
            <CaretLeft size={16} />
          </Button>
          <div>
            <Text variant="heading3" as="h3">{selectedStock.stock_name}</Text>
            <Text variant="secondary" as="span" size="xs">
              {selectedStock.stock_code}
              <span style={{
                marginLeft: 8, fontSize: fontSize.xs, fontWeight: fontWeight.semibold,
                padding: `1px ${space[2] - 2}px`, borderRadius: radius.xs,
                background: sectorColor, color: theme.onAccent,
              }}>
                {sectorName[sector] || sectorName.other || sector}
              </span>
            </Text>
          </div>
        </div>

        <Card dark={dark} glass style={{ marginBottom: space[4] }} padded={false}>
          <div style={{ padding: `${space[4]}px ${space[5]}px` }}>
            <div style={{ display: 'flex', gap: space[6], flexWrap: 'wrap' }}>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('penetration.totalExposure')}</Text>
                <div style={{ fontSize: fontSize["2xl"], fontWeight: fontWeight.semibold, color: theme.text }}>¥{selectedStock.total_exposure_cny.toLocaleString()}</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('penetration.portfolioWeight')}</Text>
                <div style={{ fontSize: fontSize["2xl"], fontWeight: fontWeight.semibold, color: sectorColor }}>{selectedStock.weight_pct.toFixed(2)}%</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('penetration.fundCount')}</Text>
                <div style={{ fontSize: fontSize["2xl"], fontWeight: fontWeight.semibold, color: theme.text }}>{selectedStock.held_by_funds.length}</div>
              </div>
            </div>
          </div>
        </Card>

        <Card dark={dark} glass padded={false}>
          <div style={{ padding: `${space[4]}px ${space[5]}px ${space[1]}px` }}>
            <div style={{ fontSize: fontSize.xl, fontWeight: fontWeight.semibold, marginBottom: space[3], color: theme.text }}>
              {t('penetration.holdingsDetail')}
            </div>
          </div>
          <Table>
            <Table.Header>
              <Table.Row>
                <Table.Head>{t('penetration.fundName')}</Table.Head>
                <Table.Head>{t('penetration.weight')}</Table.Head>
                <Table.Head>{t('penetration.fundValue')}</Table.Head>
                <Table.Head>{t('penetration.exposureAmount')}</Table.Head>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {[...selectedStock.held_by_funds]
                .sort((a, b) => b.weight_pct - a.weight_pct)
                .map(f => {
                  const exposure = f.fund_value_cny * (f.weight_pct / 100)
                  return (
                    <Table.Row key={f.fund_code}>
                      <Table.Cell>{f.fund_name}</Table.Cell>
                      <Table.Cell>{f.weight_pct.toFixed(2)}%</Table.Cell>
                      <Table.Cell>¥{f.fund_value_cny.toLocaleString()}</Table.Cell>
                      <Table.Cell>¥{exposure.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}</Table.Cell>
                    </Table.Row>
                  )
                })}
            </Table.Body>
          </Table>
        </Card>
      </Card>
    )
  }

  return (
    <ChartShell
      dark={dark}
      title={t('penetration.title')}
      subtitle={
        <>
          <span style={{ display: 'block' }}>
            {t('penetration.subtitle', {
              equityCount,
              uniqueStocks,
              value: totalValue.toLocaleString(),
            })}
          </span>
          <span style={{ display: 'block', marginTop: 2 }}>
            {t('penetration.desc')}
          </span>
        </>
      }
      loading={loading}
      error={error}
      empty={!loading && !error && !stocks.length}
      height={chartHeight.treemap}
      marginBottom={space[5]}
      testidPrefix="chart"
    >
      <div ref={chartRef} style={{ height: chartHeight.treemap }} data-testid="treemap-chart" />
    </ChartShell>
  )
}
