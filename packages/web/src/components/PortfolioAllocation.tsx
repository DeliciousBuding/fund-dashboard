import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Grid } from '@cloudflare/kumo'
import { getTheme, chartTooltip, hexToRgba, space, radius, fontSize, fontWeight, chartHeight } from '../styles/theme'
import { useEChart } from '../hooks/useEChart'
import { useChartData, useCoreCharts, ChartShell } from './charts'
import { useAppStore } from '../stores/appStore'
import { SECTOR_FALLBACK } from '../services/sector'
import {
  fetchPortfolioAllocation,
  fetchInvestmentHarness,
  type PortfolioAllocation as PortfolioAllocationData,
  type AllocationBucket,
  type InvestmentHarnessHoldingSignal,
} from '../api'

useCoreCharts()

interface PortfolioAllocationProps {
  dark: boolean;
}

const TYPE_LABEL_KEYS = new Set(['fund', 'stock', 'etf', 'index'])
const MARKET_LABEL_KEYS = new Set([
  'CN', 'US', 'HK', 'cn_fund', 'a_share_sh', 'a_share_sz', 'hk_stock', 'us_stock', 'unclassified',
])

function resolveAllocationLabel(
  row: AllocationBucket,
  t: (key: string) => string,
): string {
  // Prefer stable key → i18n; freeform fund_type keys fall back to API label (#180).
  if (TYPE_LABEL_KEYS.has(row.key)) return t(`allocation.typeLabels.${row.key}`)
  if (MARKET_LABEL_KEYS.has(row.key)) return t(`allocation.marketLabels.${row.key}`)
  return row.label || row.key
}

function AllocationRows({ title, rows, theme, t }: {
  title: string
  rows: AllocationBucket[]
  theme: ReturnType<typeof getTheme>
  t: (key: string) => string
}) {
  return (
    <div>
      <Text variant="heading3" as="h3">{title}</Text>
      <div style={{ marginTop: space[3], display: 'grid', gap: space[2] + 2 }}>
        {rows.slice(0, 8).map((row, idx) => (
          <div key={`${title}-${row.key}`}>
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: space[3], alignItems: 'baseline' }}>
              <Text as="span" size="sm">{resolveAllocationLabel(row, t)}</Text>
              <Text variant="secondary" as="span" size="xs">{row.weight_pct.toFixed(2)}% · ¥ {row.value.toLocaleString()}</Text>
            </div>
            <div style={{ height: 8, borderRadius: radius.sm - 2, background: 'var(--color-kumo-canvas)', overflow: 'hidden', marginTop: space[1] + 1 }}>
              <div style={{
                width: `${Math.max(2, Math.min(100, row.weight_pct))}%`,
                height: '100%',
                borderRadius: radius.sm - 2,
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
function buildSunburstData(
  holdings: InvestmentHarnessHoldingSignal[],
  typeColors: Record<string, string>,
  marketColors: Record<string, string>,
  typeLabels: Record<string, string>,
  marketLabels: Record<string, string>,
): SunburstNode[] {
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

  return Object.entries(tree).map(([type, markets]) => ({
    name: typeLabels[type] || type,
    itemStyle: { color: typeColors[type] || SECTOR_FALLBACK },
    children: Object.entries(markets).map(([market, items]) => ({
      name: marketLabels[market] || market,
      itemStyle: { color: marketColors[market] || SECTOR_FALLBACK },
      children: items,
    })),
  }));
}

export default function PortfolioAllocation({ dark }: PortfolioAllocationProps) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const portfolioId = useAppStore((s) => s.portfolioId);

  const { data, loading, error } = useChartData<PortfolioAllocationData>(
    // AbortSignal is the 2nd arg — never pass it as portfolioId.
    (signal) => fetchPortfolioAllocation(portfolioId, signal),
    [portfolioId],
  );

  // Supplementary sunburst signals via shared chart fetcher (abort + no silent catch).
  const { data: harnessSnap } = useChartData(
    (signal) => fetchInvestmentHarness(portfolioId, signal),
    [portfolioId],
  );
  const holdingSignals: InvestmentHarnessHoldingSignal[] = harnessSnap?.holding_signals ?? [];

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

  const typeLabels = useMemo(
    () => ({
      stock: t('allocation.typeLabels.stock'),
      fund: t('allocation.typeLabels.fund'),
      etf: t('allocation.typeLabels.etf'),
      index: t('allocation.typeLabels.index'),
    }),
    [t],
  );

  const marketLabels = useMemo(
    () => ({
      CN: t('allocation.marketLabels.CN'),
      US: t('allocation.marketLabels.US'),
      HK: t('allocation.marketLabels.HK'),
    }),
    [t],
  );

  const sunburstData = useMemo<SunburstNode[] | null>(() => {
    if (!holdingSignals.length) return null;
    return buildSunburstData(holdingSignals, typeColors, marketColors, typeLabels, marketLabels);
  }, [holdingSignals, typeColors, marketColors, typeLabels, marketLabels]);

  // ── Sunburst option ─────────────────────────────────────────────
  const sunburstOption = useMemo(() => {
    if (!sunburstData) return {} as Record<string, unknown>;
    return {
      tooltip: {
        trigger: 'item',
        ...chartTooltip(theme),
        formatter: (params: any) => {
          const pct = params.percent ?? 0;
          return t('allocation.sunburstTooltip', {
            name: params.name,
            value: ((params.value ?? 0) as number).toLocaleString(),
            pct: pct.toFixed(2),
          });
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
            label: { fontSize: fontSize.lg, fontWeight: fontWeight.bold },
          },
          nodeClick: 'rootToNode',
          sort: 'desc',
          label: { show: true, rotate: 0, color: theme.text },
          itemStyle: { borderColor: theme.surface, borderWidth: 2 },
          levels: [
            {},
            { r0: '12%', r: '37%', label: { fontSize: fontSize.base, fontWeight: fontWeight.bold } },
            { r0: '37%', r: '62%', label: { fontSize: fontSize.sm } },
            { r0: '62%', r: '90%', label: { fontSize: fontSize.xs, minAngle: 8 } },
          ],
        },
      ],
    };
  }, [sunburstData, dark, t]);

  const sunburstRef = useEChart(sunburstOption, [sunburstOption]);

  const top = data?.by_security_type?.[0];

  return (
    <ChartShell
      dark={dark}
      title={t('allocation.title')}
      subtitle={data ? `${t('common.total')} ¥ ${data.total_value.toLocaleString()}` : undefined}
      loading={loading}
      error={error}
      empty={!loading && !error && !data}
      height={chartHeight.default}
      testidPrefix="chart"
    >
      {data && (
        <>

          {top && (
            <div style={{
              marginTop: space[4],
              minHeight: 112,
              display: 'grid',
              placeItems: 'center',
              borderRadius: radius.sm + 2,
              background: hexToRgba(theme.blue, dark ? 0.12 : 0.08),
              border: `1px solid ${theme.border}`,
            }}>
              <div style={{ textAlign: 'center' }}>
                <Text variant="secondary" as="span" size="xs">{t('allocation.maxPosition')}</Text>
                <div style={{ fontSize: fontSize["4xl"], fontWeight: fontWeight.bold, marginTop: space[1], color: theme.text }}>{resolveAllocationLabel(top, t)} {top.weight_pct.toFixed(2)}%</div>
              </div>
            </div>
          )}
          <Grid variant="3up" gap="base" style={{ marginTop: space[4] + 2 }}>
            <AllocationRows title={t('allocation.bySecurityType')} rows={data.by_security_type} theme={theme} t={t} />
            <AllocationRows title={t('allocation.byMarket')} rows={data.by_market} theme={theme} t={t} />
            <AllocationRows title={t('allocation.byTheme')} rows={data.by_fund_type} theme={theme} t={t} />
          </Grid>

          {/* Sunburst chart */}
          {sunburstData && (
            <div style={{ marginTop: space[5] }}>
              <Text variant="heading3" as="h3">{t('allocation.hierarchy')}</Text>
              <Text variant="secondary" as="span" size="xs" style={{ marginTop: space[1], display: 'block' }}>
                {t('allocation.hierarchyDesc')}
              </Text>
              <div
                ref={sunburstRef}
                data-testid="sunburst-chart"
                style={{ height: chartHeight.medium, marginTop: space[3] }}
              />
            </div>
          )}

          {!!data.risk_flags.length && (
            <div style={{ marginTop: space[4] + 2, display: 'flex', gap: space[2], flexWrap: 'wrap' }}>
              {data.risk_flags.map((flag) => (
                <span key={flag} style={{
                  fontSize: fontSize.md,
                  color: theme.critical,
                  border: `1px solid ${hexToRgba(theme.critical, 0.35)}`,
                  borderRadius: radius.sm,
                  padding: `${space[1]}px ${space[2]}px`,
                }}>{flag}</span>
              ))}
            </div>
          )}
          <div style={{ marginTop: space[4], padding: space[3], borderRadius: radius.sm + 2, background: 'var(--color-kumo-canvas)' }}>
            <Text variant="secondary" as="span" size="sm">{data.agent_brief}</Text>
          </div>
        
        </>
      )}
    </ChartShell>
  );
}
