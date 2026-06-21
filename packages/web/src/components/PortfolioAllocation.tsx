import { useEffect, useState, useRef, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Grid } from '@cloudflare/kumo'
import { use as echartsUse, graphic } from 'echarts/core'
import { SunburstChart } from 'echarts/charts'
import { TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { getTheme, chartTooltip, hexToRgba } from '../styles/theme'
import { useEChart } from '../hooks/useEChart'
import { Card } from './ui/Card'
import {
  fetchPortfolioAllocation,
  fetchInvestmentHarness,
  type PortfolioAllocation as PortfolioAllocationData,
  type AllocationBucket,
  type InvestmentHarnessHoldingSignal,
} from '../api'

echartsUse([SunburstChart, TooltipComponent, CanvasRenderer])

interface PortfolioAllocationProps {
  dark: boolean;
}

function AllocationRows({ title, rows, theme }: { title: string; rows: AllocationBucket[]; theme: ReturnType<typeof getTheme> }) {
  return (
    <div>
      <Text variant="heading3" as="h3">{title}</Text>
      <div style={{ marginTop: 12, display: 'grid', gap: 10 }}>
        {rows.slice(0, 8).map((row, idx) => (
          <div key={`${title}-${row.key}`}>
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'baseline' }}>
              <Text as="span" size="sm">{row.label}</Text>
              <Text variant="secondary" as="span" size="xs">{row.weight_pct.toFixed(2)}% · ¥ {row.value.toLocaleString()}</Text>
            </div>
            <div style={{ height: 8, borderRadius: 4, background: 'var(--color-kumo-canvas)', overflow: 'hidden', marginTop: 5 }}>
              <div style={{
                width: `${Math.max(2, Math.min(100, row.weight_pct))}%`,
                height: '100%',
                borderRadius: 4,
                background: theme.series[idx % theme.series.length],
              }} />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

interface SunburstNode {
  name: string;
  value?: number;
  weight_pct?: number;
  itemStyle?: { color: string };
  children?: SunburstNode[];
}

/** Build sunburst hierarchy from individual holding signals:
 *  L1: security_type (stock/fund), L2: market (CN/US/HK), L3: holding name */
function buildSunburstData(holdings: InvestmentHarnessHoldingSignal[], typeColors: Record<string, string>, marketColors: Record<string, string>): SunburstNode[] {
  const tree: Record<string, Record<string, SunburstNode[]>> = {};

  for (const h of holdings) {
    const type = h.security_type || 'other';
    const market = h.market || 'other';
    tree[type] ??= {};
    tree[type][market] ??= [];
    tree[type][market].push({
      name: h.name,
      value: h.current_value,
      weight_pct: h.weight_pct,
    });
  }

  const typeLabels: Record<string, string> = { stock: '股票', fund: '基金' };
  const marketLabels: Record<string, string> = { CN: 'A股', US: '美股', HK: '港股' };

  return Object.entries(tree).map(([type, markets]) => ({
    name: typeLabels[type] || type,
    itemStyle: { color: typeColors[type] || '#868e96' },
    children: Object.entries(markets).map(([market, items]) => ({
      name: marketLabels[market] || market,
      itemStyle: { color: marketColors[market] || '#868e96' },
      children: items,
    })),
  }));
}

export default function PortfolioAllocation({ dark }: PortfolioAllocationProps) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const [data, setData] = useState<PortfolioAllocationData | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [holdingSignals, setHoldingSignals] = useState<InvestmentHarnessHoldingSignal[]>([]);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    setLoading(true);
    setError('');
    fetchPortfolioAllocation(ctrl.signal)
      .then((res) => {
        setData(res);
        setLoading(false);
      })
      .catch((e) => {
        if (e.name !== 'AbortError') {
          console.warn('[allocation]', e);
          setError(e.message || t('allocation.error'));
          setLoading(false);
        }
      });
    return () => ctrl.abort();
  }, []);

  // Fetch individual holding signals for sunburst hierarchy
  useEffect(() => {
    const ctrl = new AbortController();
    fetchInvestmentHarness(ctrl.signal)
      .then((res) => setHoldingSignals(res.holding_signals))
      .catch(() => {}); // sunburst is supplementary; fail silently
    return () => ctrl.abort();
  }, []);

  // ── Build sunburst data ─────────────────────────────────────────
  const typeColors = useMemo(() => ({
    stock: theme.up,
    fund: theme.blue,
  }), [dark]);

  const marketColors = useMemo(() => ({
    CN: theme.down,
    US: theme.blue,
    HK: theme.amber,
  }), [dark]);

  const sunburstData = useMemo<SunburstNode[] | null>(() => {
    if (!holdingSignals.length) return null;
    return buildSunburstData(holdingSignals, typeColors, marketColors);
  }, [holdingSignals, typeColors, marketColors]);

  // ── Sunburst option ─────────────────────────────────────────────
  const sunburstOption = useMemo(() => {
    if (!sunburstData) return {} as Record<string, unknown>;
    return {
      tooltip: {
        trigger: 'item',
        ...chartTooltip(theme),
        formatter: (params: any) => {
          const pct = params.percent ?? 0;
          return `${params.name}<br/>市值: ¥${((params.value ?? 0) as number).toLocaleString()}<br/>占比: ${pct.toFixed(2)}%`;
        },
      },
      series: [
        {
          type: 'sunburst',
          data: sunburstData,
          radius: ['12%', '90%'],
          center: ['50%', '52%'],
          emphasis: {
            focus: 'ancestor',
            label: { fontSize: 14, fontWeight: 'bold' },
          },
          nodeClick: 'rootToNode',
          sort: 'desc',
          label: { show: true, rotate: 0, color: theme.text },
          itemStyle: { borderColor: theme.surface, borderWidth: 2 },
          levels: [
            {},
            { r0: '12%', r: '37%', label: { fontSize: 13, fontWeight: 'bold' } },
            { r0: '37%', r: '62%', label: { fontSize: 11 } },
            { r0: '62%', r: '90%', label: { fontSize: 10, minAngle: 8 } },
          ],
        },
      ],
    };
  }, [sunburstData, dark]);

  const sunburstRef = useEChart(sunburstOption, [sunburstOption]);

  // ── Placeholder helper ──────────────────────────────────────────
  const placeholder = (msg: string, testid: string) => (
    <div data-testid={testid} style={{ padding: '40px 0', display: 'flex', alignItems: 'center', justifyContent: 'center', color: theme.textMuted, fontVariantNumeric: 'tabular-nums' }}>
      <Text variant="secondary" as="span" size="sm">{msg}</Text>
    </div>
  );

  const top = data?.by_security_type?.[0];

  return (
    <Card dark={dark}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 12, flexWrap: 'wrap' }}>
        <Text variant="heading2" as="h2">{t('allocation.title')}</Text>
        {data && (
          <Text variant="secondary" as="span">{t('common.total')} ¥ {data.total_value.toLocaleString()}</Text>
        )}
      </div>

      {loading ? (
        placeholder(t('common.loading', '加载中…'), 'chart-loading')
      ) : error ? (
        placeholder(error, 'chart-error')
      ) : !data ? (
        placeholder(t('common.noData', '暂无数据'), 'chart-empty')
      ) : (
        <>
          {top && (
            <div style={{
              marginTop: 16,
              minHeight: 112,
              display: 'grid',
              placeItems: 'center',
              borderRadius: 8,
              background: hexToRgba(theme.blue, dark ? 0.12 : 0.08),
              border: `1px solid ${theme.border}`,
            }}>
              <div style={{ textAlign: 'center' }}>
                <Text variant="secondary" as="span" size="xs">{t('allocation.maxPosition')}</Text>
                <div style={{ fontSize: 30, fontWeight: 700, marginTop: 4, color: theme.text }}>{top.label} {top.weight_pct.toFixed(2)}%</div>
              </div>
            </div>
          )}
          <Grid variant="3up" gap="base" style={{ marginTop: 18 }}>
            <AllocationRows title={t('allocation.bySecurityType')} rows={data.by_security_type} theme={theme} />
            <AllocationRows title={t('allocation.byMarket')} rows={data.by_market} theme={theme} />
            <AllocationRows title={t('allocation.byTheme')} rows={data.by_fund_type} theme={theme} />
          </Grid>

          {/* Sunburst chart */}
          {sunburstData && (
            <div style={{ marginTop: 24 }}>
              <Text variant="heading3" as="h3">{t('allocation.hierarchy')}</Text>
              <Text variant="secondary" as="span" size="xs" style={{ marginTop: 4, display: 'block' }}>
                {t('allocation.hierarchyDesc')}
              </Text>
              <div
                ref={sunburstRef}
                data-testid="sunburst-chart"
                style={{ height: 400, marginTop: 12 }}
              />
            </div>
          )}

          {!!data.risk_flags.length && (
            <div style={{ marginTop: 18, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              {data.risk_flags.map((flag) => (
                <span key={flag} style={{
                  fontSize: 12,
                  color: theme.up,
                  border: `1px solid ${hexToRgba(theme.up, 0.35)}`,
                  borderRadius: 6,
                  padding: '4px 8px',
                }}>{flag}</span>
              ))}
            </div>
          )}
          <div style={{ marginTop: 16, padding: 12, borderRadius: 8, background: 'var(--color-kumo-canvas)' }}>
            <Text variant="secondary" as="span" size="sm">{data.agent_brief}</Text>
          </div>
        </>
      )}
    </Card>
  );
}
