import { useState, useEffect, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Button, Loader, Table } from '@cloudflare/kumo'
import { use as echartsUse } from 'echarts/core'
import { RadarChart } from 'echarts/charts'
import { RadarComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { fetchCompare, type CompareFund, type FundInfo } from '../api'
import { getTheme, chartTooltip, chartLegend, hexToRgba } from '../styles/theme'
import { useEChart } from '../hooks/useEChart'
import { Card } from './ui/Card'

echartsUse([RadarChart, RadarComponent, TooltipComponent, LegendComponent, CanvasRenderer])

const METRICS = [
  { key: 'xirr', label: 'comparison.annualReturnLabel', max: 50 },
  { key: 'volatility', label: 'comparison.volatilityLabel', max: 40 },
  { key: 'sharpe', label: 'comparison.sharpeLabel', max: 3 },
  { key: 'max_drawdown', label: 'comparison.maxDrawdownLabel', max: 50 },
  { key: 'calmar', label: 'Calmar', max: 3 },
] as const;

interface FundComparisonProps {
  funds: FundInfo[];
  dark: boolean;
}

export default function FundComparison({ funds, dark }: FundComparisonProps) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const [selected, setSelected] = useState<string[]>([]);
  const [compareData, setCompareData] = useState<CompareFund[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const abortRef = useRef<AbortController | null>(null);

  const heldFunds = useMemo(() => funds.filter(f => f.held_shares > 0.001), [funds]);

  const option = useMemo(() => {
    if (!compareData.length) return {} as Record<string, unknown>;
    const indicator = METRICS.map(m => ({ name: t(m.label, m.label), max: m.max }));

    const seriesData = compareData.map((f, idx) => ({
      name: f.name,
      value: [
        f.xirr ?? 0,
        f.volatility ?? 0,
        f.sharpe ?? 0,
        f.max_drawdown ?? 0,
        f.calmar ?? 0,
      ],
      lineStyle: { color: theme.series[idx % theme.series.length], width: 2 },
      areaStyle: { color: hexToRgba(theme.series[idx % theme.series.length], 0.08) },
      symbol: 'circle', symbolSize: 4,
      itemStyle: { color: theme.series[idx % theme.series.length] },
    }));

    return {
      tooltip: { trigger: 'item', ...chartTooltip(theme) },
      legend: {
        data: compareData.map(f => f.name), bottom: 0,
        ...chartLegend(theme),
      },
      radar: {
        indicator,
        center: ['50%', '48%'], radius: '65%',
        axisName: { fontSize: 10, color: theme.textMuted },
        splitArea: {
          areaStyle: {
            color: [hexToRgba(theme.surface, 0.5), hexToRgba(theme.canvas, 0.5)],
          },
        },
        axisLine: { lineStyle: { color: theme.hairline } },
        splitLine: { lineStyle: { color: theme.hairline } },
      },
      series: [{ type: 'radar', data: seriesData }],
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [compareData, dark]);

  const chartRef = useEChart(option, [option]);

  const doCompare = async () => {
    if (selected.length < 2) { setError(t('comparison.selectMin2')); return; }
    setError(''); setLoading(true);
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    try {
      const result = await fetchCompare(selected, ctrl.signal);
      if (ctrl.signal.aborted) return;
      setCompareData(result.funds);
    } catch (e: any) {
      if (e.name !== 'AbortError') {
        setError(e.message || t('comparison.error'));
      }
    } finally {
      if (!ctrl.signal.aborted) {
        setLoading(false);
      }
    }
  };

  const toggleFund = (code: string) => {
    setSelected(prev => prev.includes(code) ? prev.filter(c => c !== code) : [...prev, code]);
  };

  const fmtVal = (v: number | null, suffix = '', d = 2): string => {
    if (v == null) return '-';
    return `${v > 0 ? '+' : ''}${v.toFixed(d)}${suffix}`;
  };

  // Compute best/worst for star markers
  const allXirr = compareData.map(d => d.xirr ?? -Infinity);
  const allSharpe = compareData.map(d => d.sharpe ?? -Infinity);
  const allCalmar = compareData.map(d => d.calmar ?? -Infinity);
  const allVol = compareData.map(d => d.volatility ?? Infinity);
  const allDd = compareData.map(d => d.max_drawdown ?? Infinity);
  const bestXirr = Math.max(...allXirr);
  const bestSharpe = Math.max(...allSharpe);
  const bestCalmar = Math.max(...allCalmar);
  const lowVol = Math.min(...allVol);
  const lowDd = Math.min(...allDd);

  const placeholder = (msg: string, testid: string) => (
    <div data-testid={testid} style={{ height: 450, display: 'flex', alignItems: 'center', justifyContent: 'center', color: theme.textMuted, fontVariantNumeric: 'tabular-nums' }}>
      {msg}
    </div>
  );

  return (
    <div>
      <Text variant="heading1" as="h1">{t('comparison.title')}</Text>

      <Card dark={dark} style={{ marginTop: 16 }}>
        <div style={{ padding: '4px 0 16px' }}>
          <Text variant="heading3" as="h3" style={{ marginBottom: 12 }}>{t('comparison.selectFunds')}</Text>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 12 }}>
            {heldFunds.map(f => (
              <Button
                key={f.code}
                variant={selected.includes(f.code) ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => toggleFund(f.code)}
              >
                {f.name}
              </Button>
            ))}
          </div>
          {heldFunds.length === 0 && (
            <Text variant="secondary" as="span">{t('comparison.noFunds')}</Text>
          )}
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Button variant="primary" onClick={doCompare} disabled={loading || selected.length < 2}>
              {loading ? t('comparison.comparing') : `${t('comparison.compareBtn')} (${selected.length})`}
            </Button>
            {error && <Text variant="secondary" as="span" style={{ color: theme.up }}>{error}</Text>}
          </div>
        </div>
      </Card>

      {compareData.length > 0 && (
        <>
          <Card dark={dark} style={{ marginTop: 16 }}>
            <div style={{ padding: '4px 0 16px' }}>
              <Text variant="heading3" as="h3">{t('comparison.radar')}</Text>
            </div>
            <div ref={chartRef} style={{ height: 450 }} />
          </Card>

          <Card dark={dark} style={{ marginTop: 16 }}>
            <div style={{ padding: '4px 0 16px' }}>
              <Text variant="heading3" as="h3" style={{ marginBottom: 12 }}>{t('comparison.metricTable')}</Text>
              <Table>
                <Table.Header>
                  <Table.Row>
                    <Table.Head>{t('common.name')}</Table.Head>
                    <Table.Head>{t('comparison.annualReturnLabel')}</Table.Head>
                    <Table.Head>{t('comparison.volatilityLabel')}</Table.Head>
                    <Table.Head>{t('comparison.sharpeLabel')}</Table.Head>
                    <Table.Head>{t('comparison.maxDrawdownLabel')}</Table.Head>
                    <Table.Head>Calmar</Table.Head>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {compareData.map((f, idx) => (
                    <Table.Row key={f.code}>
                      <Table.Cell>
                        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                          <span style={{
                            width: 10, height: 10, borderRadius: '50%',
                            background: theme.series[idx % theme.series.length], display: 'inline-block', flexShrink: 0,
                          }} />
                          {f.name}
                        </span>
                      </Table.Cell>
                      <Table.Cell>
                        <Text variant="body" as="span" size="sm" style={{
                          color: Number(f.xirr ?? 0) > 0 ? theme.up : Number(f.xirr ?? 0) < 0 ? theme.down : undefined,
                          fontWeight: f.xirr != null && f.xirr === bestXirr ? 600 : 400,
                        }}>
                          {fmtVal(f.xirr, '%')}
                          {f.xirr != null && f.xirr === bestXirr ? ' ★' : ''}
                        </Text>
                      </Table.Cell>
                      <Table.Cell>
                        <Text variant="body" as="span" size="sm" style={{
                          fontWeight: f.volatility != null && f.volatility === lowVol ? 600 : 400,
                        }}>
                          {fmtVal(f.volatility, '%')}
                          {f.volatility != null && f.volatility === lowVol ? ' ★' : ''}
                        </Text>
                      </Table.Cell>
                      <Table.Cell>
                        <Text variant="body" as="span" size="sm" style={{
                          fontWeight: f.sharpe != null && f.sharpe === bestSharpe ? 600 : 400,
                        }}>
                          {f.sharpe != null ? f.sharpe.toFixed(4) : '-'}
                          {f.sharpe != null && f.sharpe === bestSharpe ? ' ★' : ''}
                        </Text>
                      </Table.Cell>
                      <Table.Cell>
                        <Text variant="body" as="span" size="sm" style={{
                          color: theme.up,
                          fontWeight: f.max_drawdown != null && f.max_drawdown === lowDd ? 600 : 400,
                        }}>
                          {fmtVal(f.max_drawdown, '%')}
                          {f.max_drawdown != null && f.max_drawdown === lowDd ? ' ★' : ''}
                        </Text>
                      </Table.Cell>
                      <Table.Cell>
                        <Text variant="body" as="span" size="sm" style={{
                          fontWeight: f.calmar != null && f.calmar === bestCalmar ? 600 : 400,
                        }}>
                          {f.calmar != null ? f.calmar.toFixed(4) : '-'}
                          {f.calmar != null && f.calmar === bestCalmar ? ' ★' : ''}
                        </Text>
                      </Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table>
            </div>
          </Card>
        </>
      )}

      {loading && (
        <div data-testid="chart-loading" style={{ padding: 60, textAlign: 'center' }}>
          <Loader />
          <div style={{ marginTop: 12 }}><Text variant="secondary" as="span">{t('comparison.comparing')}</Text></div>
        </div>
      )}

      {!loading && !compareData.length && !error && selected.length >= 2 && (
        placeholder(t('common.noData', '暂无数据'), 'chart-empty')
      )}
    </div>
  );
}
