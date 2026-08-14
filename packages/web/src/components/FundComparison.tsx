import { useState, useMemo, useRef, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router-dom'
import { Text, Button, Loader, Table } from '@cloudflare/kumo'
import { fetchCompare, type CompareFund, type FundInfo } from '../api'
import { getTheme, chartTooltip, chartLegend, hexToRgba, space, radius, fontSize, chartHeight } from '../styles/theme'
import { useEChart } from '../hooks/useEChart'
import { useCoreCharts, radarSeries } from './charts'
import { Card } from './ui/Card'
import { ChartShell } from './charts'
import { sanitizeUserError } from '../services/userError'
import { useAppStore } from '../stores/appStore'

useCoreCharts()

// Per-metric radar scale configuration.
// `max` here is a FLOOR — the on-screen indicator max grows beyond it when a
// fund's value would otherwise be clipped (so a fund returning 120% never
// spills outside the polygon), but never shrinks below it (keeping the visual
// scale stable for the common low-magnitude case and comparable across runs).
// `invert` marks "lower is better" metrics (max_drawdown is conventionally a
// positive magnitude where larger = worse). On a radar, "further out" reads as
// "better", so for inverted metrics we plot (max - value) instead of value —
// a 5% drawdown reaches near the rim while a 40% drawdown sits near the centre.
const METRICS = [
  { key: 'xirr', label: 'comparison.annualReturnLabel', max: 50, invert: false },
  { key: 'volatility', label: 'comparison.volatilityLabel', max: 40, invert: false },
  { key: 'sharpe', label: 'comparison.sharpeLabel', max: 3, invert: false },
  { key: 'max_drawdown', label: 'comparison.maxDrawdownLabel', max: 50, invert: true },
  { key: 'calmar', label: 'comparison.calmarLabel', max: 3, invert: false },
] as const;

interface FundComparisonProps {
  funds: FundInfo[];
  dark: boolean;
}

function parseCodesParam(raw: string | null): string[] {
  if (!raw) return [];
  return [...new Set(raw.split(',').map((s) => s.trim()).filter(Boolean))];
}

export default function FundComparison({ funds, dark }: FundComparisonProps) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const portfolioId = useAppStore((s) => s.portfolioId);
  const [searchParams, setSearchParams] = useSearchParams();
  const selected = useMemo(() => parseCodesParam(searchParams.get('codes')), [searchParams]);
  const [compareData, setCompareData] = useState<CompareFund[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const abortRef = useRef<AbortController | null>(null);
  const autoRanKey = useRef<string>('');

  const heldFunds = useMemo(() => funds.filter(f => f.held_shares > 0.001), [funds]);
  const heldCodes = useMemo(() => new Set(heldFunds.map((f) => f.code)), [heldFunds]);

  // Drop URL codes that are no longer held (stale links).
  useEffect(() => {
    if (!heldFunds.length || !selected.length) return;
    const filtered = selected.filter((c) => heldCodes.has(c));
    if (filtered.length !== selected.length) {
      setSearchParams((prev) => {
        const sp = new URLSearchParams(prev);
        if (filtered.length) sp.set('codes', filtered.join(','));
        else sp.delete('codes');
        return sp;
      }, { replace: true });
    }
  }, [heldFunds, heldCodes, selected, setSearchParams]);

  // Clear stale radar/table when portfolio membership changes.
  useEffect(() => {
    setCompareData([]);
    setError('');
    autoRanKey.current = '';
  }, [portfolioId]);

  const option = useMemo(() => {
    if (!compareData.length) return {} as Record<string, unknown>;

    // Per-metric indicator max = max(floor, ceil(largest observed value * 1.1)).
    // The 1.1 headroom keeps the outermost point off the rim; the floor keeps
    // the polygon stable when all values are small. For inverted metrics the
    // base max is still the indicator edge — values are mirrored below.
    const indicator = METRICS.map((m) => {
      const observed = compareData
        .map((f) => (f[m.key] as number | null) ?? 0)
        .filter((v) => Number.isFinite(v));
      const peak = observed.length ? Math.max(...observed) : 0;
      const dynamicMax = Math.max(m.max, Math.ceil(peak * 1.1));
      return { name: t(m.label, m.label), max: dynamicMax };
    });

    const radarData = compareData.map((f, idx) => {
      const color = theme.series[idx % theme.series.length];
      return {
        name: f.name,
        value: METRICS.map((m, mi) => {
          const raw = (f[m.key] as number | null) ?? 0;
          return m.invert ? Math.max(indicator[mi].max - raw, 0) : raw;
        }),
        color,
      };
    });

    const { radar, series } = radarSeries({ indicators: indicator, data: radarData });

    return {
      tooltip: { trigger: 'item', ...chartTooltip(theme) },
      legend: {
        data: compareData.map(f => f.name), bottom: 0,
        ...chartLegend(theme),
      },
      radar: {
        ...radar,
        axisName: { fontSize: fontSize.xs, color: theme.textMuted },
        splitArea: {
          areaStyle: {
            color: [hexToRgba(theme.surface, 0.5), hexToRgba(theme.canvas, 0.5)],
          },
        },
        axisLine: { lineStyle: { color: theme.hairline } },
        splitLine: { lineStyle: { color: theme.hairline } },
      },
      series,
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [compareData, dark]);

  const chartRef = useEChart(option, [option]);

  const doCompare = useCallback(async (codes: string[] = selected) => {
    if (codes.length < 2) { setError(t('comparison.selectMin2')); return; }
    setError(''); setLoading(true);
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    try {
      const result = await fetchCompare(codes, portfolioId, ctrl.signal);
      if (ctrl.signal.aborted) return;
      setCompareData(result.funds);
    } catch (e: any) {
      if (e.name !== 'AbortError') {
        setError(sanitizeUserError(e, t('comparison.error')));
      }
    } finally {
      if (!ctrl.signal.aborted) {
        setLoading(false);
      }
    }
  }, [selected, portfolioId, t]);

  // Auto-run only for initial deep-link (?codes=a,b), not for every user toggle.
  useEffect(() => {
    if (autoRanKey.current) return;
    const initial = parseCodesParam(searchParams.get('codes'));
    if (initial.length < 2) {
      autoRanKey.current = '__no_autoload__';
      return;
    }
    autoRanKey.current = initial.join(',');
    void doCompare(initial);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const toggleFund = (code: string) => {
    setSearchParams((prev) => {
      const sp = new URLSearchParams(prev);
      const cur = parseCodesParam(sp.get('codes'));
      const next = cur.includes(code) ? cur.filter((c) => c !== code) : [...cur, code];
      if (next.length) sp.set('codes', next.join(','));
      else sp.delete('codes');
      return sp;
    }, { replace: true });
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

  return (
    <div>
      <Text variant="heading1" as="h1">{t('comparison.title')}</Text>

      <Card dark={dark} glass style={{ marginTop: space[4] }}>
        <div style={{ padding: `${space[1]}px 0 ${space[4]}px` }}>
          <Text variant="heading3" as="h3" style={{ marginBottom: space[3] }}>{t('comparison.selectFunds')}</Text>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: space[2], marginBottom: space[3] }}>
            {heldFunds.map(f => (
              <Button type="button"
                key={f.code}
                variant={selected.includes(f.code) ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => toggleFund(f.code)}
                aria-pressed={selected.includes(f.code)}
              >
                {f.name}
              </Button>
            ))}
          </div>
          {heldFunds.length === 0 && (
            <Text variant="secondary" as="span">{t('comparison.noFunds')}</Text>
          )}
          <div style={{ display: 'flex', alignItems: 'center', gap: space[2] }}>
            <Button type="button" variant="primary" onClick={() => void doCompare()} disabled={loading || selected.length < 2} aria-busy={loading}>
              {loading ? t('comparison.comparing') : `${t('comparison.compareBtn')} (${selected.length})`}
            </Button>
            <div aria-live="polite">
              {error && <Text variant="secondary" as="span" style={{ color: theme.critical }}>{error}</Text>}
            </div>
          </div>
        </div>
      </Card>

      {compareData.length > 0 && (
        <>
          <ChartShell
            dark={dark}
            title={t('comparison.radar')}
            height={chartHeight.compare}
            style={{ marginTop: space[4] }}
            testidPrefix="chart"
          >
            <div ref={chartRef} style={{ height: chartHeight.compare }} />
          </ChartShell>

          <Card dark={dark} glass style={{ marginTop: space[4] }}>
            <div style={{ padding: `${space[1]}px 0 ${space[4]}px` }}>
              <Text variant="heading3" as="h3" style={{ marginBottom: space[3] }}>{t('comparison.metricTable')}</Text>
              <Table>
                <Table.Header>
                  <Table.Row>
                    <Table.Head>{t('common.name')}</Table.Head>
                    <Table.Head>{t('comparison.annualReturnLabel')}</Table.Head>
                    <Table.Head>{t('comparison.volatilityLabel')}</Table.Head>
                    <Table.Head>{t('comparison.sharpeLabel')}</Table.Head>
                    <Table.Head>{t('comparison.maxDrawdownLabel')}</Table.Head>
                    <Table.Head>{t('comparison.calmarLabel')}</Table.Head>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {compareData.map((f, idx) => (
                    <Table.Row key={f.code}>
                      <Table.Cell>
                        <span style={{ display: 'inline-flex', alignItems: 'center', gap: space[2] - 2 }}>
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
                          // max_drawdown is a positive magnitude of loss — larger is
                          // worse. CN convention: green (theme.down) = loss/bad, so a
                          // drawdown cell always reads as down regardless of magnitude;
                          // the lowest (best) drawdown is bolded + starred below.
                          color: theme.down,
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
        <div data-testid="chart-loading" style={{ padding: space[8], textAlign: 'center' }}>
          <Loader />
          <div style={{ marginTop: space[3] }}><Text variant="secondary" as="span">{t('comparison.comparing')}</Text></div>
        </div>
      )}

      {!loading && !compareData.length && !error && selected.length >= 2 && (
        <div data-testid="chart-empty" style={{ padding: space[8], textAlign: 'center', color: theme.textMuted, fontVariantNumeric: 'tabular-nums' }}>
          {t('common.noData')}
        </div>
      )}
    </div>
  );
}
