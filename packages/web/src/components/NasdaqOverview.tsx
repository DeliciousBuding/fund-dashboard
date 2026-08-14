import { useMemo } from 'react'
import { Text, Grid, Table } from '@cloudflare/kumo'
import { Card } from './ui/Card'
import { getTheme, chartTooltip, chartLegend, chartDataZoom, hexToRgba, chartShadowColor, space, radius, fontSize, fontWeight, opacity, chartHeight } from '../styles/theme'
import { useEChart } from '../hooks/useEChart'
import { useNasdaqData } from '../hooks/useNasdaqData'
import { useQueryRange } from '../hooks/useQueryRange'
import { ChartShell, useCoreCharts, type RangeOption, getDateRange, lineSeries, scatterSeries } from './charts'
import type { FundInfo } from '../api'
import StatCard from './StatCard'
import { fmt } from '../services/format'
import { useTranslation } from 'react-i18next'
import { useAppStore } from '../stores/appStore'

useCoreCharts();

interface NasdaqTradeMarker {
  value: [string, number];
  date: string;
  amt: number;
  count: number;
  close: string;
  returnPct: number;
  funds: { fund: string; code: string; amt: number; count: number }[];
}

function aggregateTradeMarkers(
  allTx: { code: string; name: string; tx: { trade_time: string; direction: string; amount: number }[] }[],
  direction: 'buy' | 'sell',
  dateToIdx: Record<string, number>,
  slicedDates: string[],
  slicedCloses: number[],
  returnPcts: number[],
): NasdaqTradeMarker[] {
  const byDate = new Map<string, NasdaqTradeMarker>();
  allTx.forEach(({ code, name, tx }) => {
    tx.forEach(t => {
      if (t.direction !== direction) return;
      const td = t.trade_time.substring(0, 10);
      const idx = dateToIdx[td];
      if (idx === undefined) return;
      let marker = byDate.get(td);
      if (!marker) {
        const ret = returnPcts[idx];
        marker = {
          value: [slicedDates[idx], ret],
          date: slicedDates[idx],
          amt: 0,
          count: 0,
          close: slicedCloses[idx].toFixed(2),
          returnPct: ret,
          funds: [],
        };
        byDate.set(td, marker);
      }
      marker.amt += t.amount || 0;
      marker.count += 1;
      let fund = marker.funds.find(item => item.code === code);
      if (!fund) {
        fund = { fund: name, code, amt: 0, count: 0 };
        marker.funds.push(fund);
      }
      fund.amt += t.amount || 0;
      fund.count += 1;
    });
  });
  return [...byDate.values()].sort((a, b) => a.date.localeCompare(b.date));
}

export default function NasdaqOverview({ nasdaqFunds, onSelect, dark }: {
  nasdaqFunds: FundInfo[]; onSelect: (c: string) => void; dark: boolean
}) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const portfolioId = useAppStore((s) => s.portfolioId);
  const NASDAQ_RANGES = ['tx', '1m', '3m', '6m', '1y', 'all'] as const;
  const [range, setRange] = useQueryRange('ndxRange', 'tx', NASDAQ_RANGES);

  const {
    indexData, allTx, indexDates, indexCloses, allTxDates,
    stats, loading, error,
  } = useNasdaqData({ nasdaqFunds, range, portfolioId });

  const RANGE_TABS: RangeOption[] = [
    { value: 'tx', label: t('chart.range.tx') },
    { value: '1m', label: t('chart.range.1m') },
    { value: '3m', label: t('chart.range.3m') },
    { value: '6m', label: t('chart.range.6m') },
    { value: '1y', label: t('chart.range.1y') },
    { value: 'all', label: t('chart.range.all') },
  ];

  const { totalBuyCount, totalSellCount, totalBuyAmt, heldFunds, clearedFunds, navPnl, navValue, latestReturn } = stats;

  // Build echarts option using chart library factories
  const chartOption = useMemo(() => {
    if (!indexData?.data.length) return {} as Record<string, unknown>;

    // Range slice
    const [i0, i1] = getDateRange(range, indexDates, allTxDates);
    const slicedDates = indexDates.slice(i0, i1 + 1);
    const slicedCloses = indexCloses.slice(i0, i1 + 1);
    const N = slicedCloses.length;
    if (N < 2) return {} as Record<string, unknown>;

    // Cumulative return %: (close - baseClose) / baseClose * 100
    const baseClose = slicedCloses[0] || 1;
    const returnPcts = slicedCloses.map(c => +(((c - baseClose) / baseClose) * 100).toFixed(2));

    // Date to index map for marker placement
    const dateToIdx: Record<string, number> = {};
    slicedDates.forEach((d, i) => { dateToIdx[d] = i; });

    const buyPoints = aggregateTradeMarkers(allTx, 'buy', dateToIdx, slicedDates, slicedCloses, returnPcts);
    const sellPoints = aggregateTradeMarkers(allTx, 'sell', dateToIdx, slicedDates, slicedCloses, returnPcts);
    const formatMarkerTooltip = (d: NasdaqTradeMarker | undefined, label: string, color: string) => {
      if (!d) return '';
      const rows = d.funds.slice(0, 5).map(item => {
        const count = item.count > 1 ? ` ×${item.count}` : '';
        return `${item.fund}${count}: ¥${item.amt.toFixed(0)}`;
      });
      const more = d.funds.length > 5 ? `<br/>+${d.funds.length - 5} ${t('common.units')}` : '';
      return `<b style="color:${color}">${label} ${d.count}${t('tx.trades')}</b><br/>${t('nasdaq.totalLine', { amount: d.amt.toFixed(0) })}<br/>NDX: ${d.close}<br/>${rows.join('<br/>')}${more}`;
    };

    // CN convention: red = up/profit, green = down/loss
    const isUp = returnPcts[N - 1] >= 0;
    const lineColor = isUp ? theme.up : theme.down;

    // Main cumulative return line (via library factory)
    const mainSeries = lineSeries({
      name: t('nasdaq.cumulativeReturn'),
      data: returnPcts,
      color: lineColor,
      smooth: true,
      area: true,
      areaAlpha: 0.18,
      width: 2.5,
      z: 10,
      markLine: {
        silent: true,
        symbol: 'none',
        lineStyle: { type: 'dashed', color: theme.border, width: 1 },
        data: [{ yAxis: 0, label: { formatter: '0%', fontSize: fontSize.xs } }],
      },
      markPoint: {
        data: [
          { type: 'max', name: t('nasdaq.max'), symbol: 'pin', symbolSize: 36, itemStyle: { color: lineColor } },
          { type: 'min', name: t('nasdaq.min'), symbol: 'pin', symbolSize: 36, itemStyle: { color: theme.textMuted }, symbolRotate: 180 },
        ],
        label: { fontSize: fontSize.xs, fontWeight: fontWeight.semibold },
      },
    });
    // Nasdaq-specific line aesthetic: shadow + rounded caps
    (mainSeries.lineStyle as any).cap = 'round';
    (mainSeries.lineStyle as any).shadowBlur = 8;
    (mainSeries.lineStyle as any).shadowColor = hexToRgba(lineColor, 0.3);

    const series: Record<string, unknown>[] = [mainSeries];

    // Buy scatter markers (rich per-fund data + dynamic symbolSize by amount)
    if (buyPoints.length) {
      const buySeries = scatterSeries({
        name: t('nasdaq.seriesBuy', { days: buyPoints.length, count: totalBuyCount }),
        data: buyPoints,
        color: theme.up,
        borderColor: theme.markerBorder,
        borderWidth: 2,
        symbolSize: (_value: unknown, params: any) => Math.min(24, Math.max(12, ((params?.data?.amt || 0) / 300))),
        symbol: 'circle',
        z: 100,
        opacity: opacity.seriesStrong,
      });
      (buySeries.itemStyle as any).shadowBlur = 4;
      (buySeries.itemStyle as any).shadowColor = chartShadowColor(theme);
      buySeries.emphasis = { scale: 1.6, itemStyle: { opacity: opacity.solid, shadowBlur: 8 } };
      buySeries.tooltip = {
        formatter: (p: any) => {
          return formatMarkerTooltip(p.data, t('nasdaq.buy'), theme.up);
        },
      };
      series.push(buySeries);
    }

    // Sell scatter markers
    if (sellPoints.length) {
      const sellSeries = scatterSeries({
        name: t('nasdaq.seriesSell', { days: sellPoints.length, count: totalSellCount }),
        data: sellPoints,
        color: theme.down,
        borderColor: theme.markerBorder,
        borderWidth: 2,
        symbolSize: (_value: unknown, params: any) => Math.min(24, Math.max(12, ((params?.data?.amt || 0) / 300))),
        symbol: 'diamond',
        z: 100,
        opacity: opacity.seriesStrong,
      });
      (sellSeries.itemStyle as any).shadowBlur = 4;
      (sellSeries.itemStyle as any).shadowColor = chartShadowColor(theme);
      sellSeries.emphasis = { scale: 1.6, itemStyle: { opacity: opacity.solid, shadowBlur: 8 } };
      sellSeries.tooltip = {
        formatter: (p: any) => {
          return formatMarkerTooltip(p.data, t('nasdaq.sell'), theme.down);
        },
      };
      series.push(sellSeries);
    }

    const totalPoints = slicedDates.length;

    return {
      tooltip: {
        trigger: 'axis',
        ...chartTooltip(theme),
        axisPointer: {
          type: 'cross',
          crossStyle: { color: theme.textMuted },
          label: { backgroundColor: theme.surface, color: theme.text },
        },
        formatter: (params: any[]) => {
          const date = params[0]?.axisValue || '';
          const retSeries = params.find((p: any) => p.seriesName === t('nasdaq.cumulativeReturn'));
          let html = `<b>${date}</b>`;
          if (retSeries && retSeries.value !== undefined) {
            const v = Number(retSeries.value) || 0;
            html += `<br/>${t('nasdaq.cumulativeReturn')}: <b style="color:${v >= 0 ? theme.up : theme.down}">${v >= 0 ? '+' : ''}${v.toFixed(2)}%</b>`;
          }
          const buys = params.filter((p: any) => (p.seriesName as string)?.startsWith(t('nasdaq.buy')));
          const sells = params.filter((p: any) => (p.seriesName as string)?.startsWith(t('nasdaq.sell')));
          buys.forEach((p: any) => {
            if (p.data?.count) html += `<br/><span style="color:${theme.up}">&#9679;</span> ${t('nasdaq.tooltipBuy', { count: p.data.count, amount: (p.data.amt || 0).toFixed(0) })}`;
          });
          sells.forEach((p: any) => {
            if (p.data?.count) html += `<br/><span style="color:${theme.down}">&#9670;</span> ${t('nasdaq.tooltipSell', { count: p.data.count, amount: (p.data.amt || 0).toFixed(0) })}`;
          });
          return html;
        },
      },
      legend: {
        data: series.filter(s => s.name).map(s => s.name),
        top: 0,
        ...chartLegend(theme),
      },
      grid: { top: 40, right: 55, bottom: 60, left: 55 },
      xAxis: {
        type: 'category',
        data: slicedDates,
        axisLabel: {
          fontSize: fontSize.xs,
          rotate: totalPoints > 90 ? 45 : 0,
          color: theme.textMuted,
          interval: Math.max(1, Math.floor(totalPoints / 15)),
        },
        axisLine: { lineStyle: { color: theme.border } },
      },
      yAxis: [
        {
          type: 'value',
          name: '%',
          nameTextStyle: { fontSize: fontSize.xs, color: theme.textMuted },
          axisLabel: { fontSize: fontSize.xs, formatter: '{value}%', color: theme.textMuted },
          splitLine: { lineStyle: { color: theme.hairline } },
        },
      ],
      dataZoom: chartDataZoom(theme),
      series,
    };
  }, [indexData, allTx, indexDates, indexCloses, allTxDates, range, dark, t, theme, totalBuyCount, totalSellCount]);

  const chartRef = useEChart(chartOption, [chartOption]);

  return (
    <div>
      <Text variant="heading1" as="h1">{t('nasdaq.title')}</Text>
      <div style={{ marginTop: 8, marginBottom: space[4] }}>
        <Text variant="secondary" as="span">
          {nasdaqFunds.length} {t('nasdaq.fundCount')} · {totalBuyCount} {t('nasdaq.buyCount')} / {totalSellCount} {t('nasdaq.sellCount')} · {t('nasdaq.benchmark')}: {t('nasdaq.benchmarkDesc')}
        </Text>
      </div>
      <Grid variant="4up" gap="base" style={{ marginBottom: space[5] }}>
        <StatCard label={t('nasdaq.funds')} value={`${nasdaqFunds.length} ${t('common.units')}`} />
        <StatCard label={t('nasdaq.totalBuy')} value={`¥ ${totalBuyAmt.toLocaleString()}`} />
        {heldFunds.length > 0 && <StatCard label={t('nasdaq.holdValue')} value={`¥ ${navValue.toFixed(0)}`} />}
        <StatCard label={t('nasdaq.pnl')} value={fmt(navPnl)} color={navPnl > 0 ? 'up' : navPnl < 0 ? 'down' : undefined} />
        <StatCard label={t('nasdaq.periodReturn')} value={`${latestReturn >= 0 ? '+' : ''}${latestReturn}%`}
          color={latestReturn > 0 ? 'up' : latestReturn < 0 ? 'down' : undefined} />
      </Grid>
      <ChartShell
        dark={dark}
        title={t('nasdaq.chartTitle')}
        subtitle={t('nasdaq.chartDesc')}
        ranges={RANGE_TABS} range={range} onRangeChange={setRange}
        loading={loading} error={error} empty={!indexData?.data.length}
        testidPrefix="chart" height={chartHeight.large}
        marginBottom={space[5]}
      >
        <div ref={chartRef} style={{ height: chartHeight.large }} />
      </ChartShell>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: space[5], marginBottom: space[5] }}>
        <Card dark={dark} glass padded={false}>
          <div style={{ padding: `${space[4]}px ${space[5]}px ${space[3]}px` }}><Text variant="heading3" as="h3">{t('nasdaq.held')}</Text></div>
          <Table>
            <Table.Header><Table.Row><Table.Head>{t('common.fund')}</Table.Head><Table.Head>{t('common.shares')}</Table.Head><Table.Head>{t('common.nav')}</Table.Head><Table.Head>{t('common.value')}</Table.Head><Table.Head>{t('common.pnl')}</Table.Head></Table.Row></Table.Header>
            <Table.Body>
              {[...heldFunds].sort((a, b) => (b.current_value ?? 0) - (a.current_value ?? 0)).map(f => {
                const pnl = f.unrealized_pnl ?? 0;
                return (
                  <Table.Row key={f.code} onClick={() => onSelect(f.code)} style={{ cursor: 'pointer' }}>
                    <Table.Cell><Text bold as="span">{f.name}</Text><br/><Text variant="secondary" as="span" size="xs">{f.code}</Text></Table.Cell>
                    <Table.Cell>{f.held_shares.toFixed(2)}</Table.Cell>
                    <Table.Cell>{f.latest_nav?.toFixed(4) ?? '-'}</Table.Cell>
                    <Table.Cell style={{ fontWeight: fontWeight.medium }}>¥ {(f.current_value ?? 0).toFixed(2)}</Table.Cell>
                    <Table.Cell><span style={{ color: Number(pnl) > 0 ? theme.up : Number(pnl) < 0 ? theme.down : 'inherit' }}>{fmt(pnl)}</span></Table.Cell>
                  </Table.Row>
                );
              })}
            </Table.Body>
          </Table>
        </Card>
        <Card dark={dark} glass padded={false}>
          <div style={{ padding: `${space[4]}px ${space[5]}px ${space[3]}px` }}><Text variant="heading3" as="h3">{t('nasdaq.cleared')}</Text></div>
          <Table>
            <Table.Header><Table.Row><Table.Head>{t('common.fund')}</Table.Head><Table.Head>{t('nasdaq.historyTx')}</Table.Head></Table.Row></Table.Header>
            <Table.Body>
              {clearedFunds.map(f => (
                <Table.Row key={f.code} onClick={() => onSelect(f.code)} style={{ cursor: 'pointer' }}>
                  <Table.Cell><Text as="span">{f.name}</Text><br/><Text variant="secondary" as="span" size="xs">{f.code}</Text></Table.Cell>
                  <Table.Cell>{allTx.find(tx => tx.code === f.code)?.tx.length ?? 0} {t('nasdaq.txCount')}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table>
        </Card>
      </div>
    </div>
  );
}
