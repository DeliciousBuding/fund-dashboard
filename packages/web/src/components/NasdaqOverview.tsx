import { useState, useEffect, useMemo, useRef } from 'react'
import { Text, LayerCard, Grid, Table, Tabs } from '@cloudflare/kumo'
import { use as echartsUse, graphic } from 'echarts/core'
import { LineChart, ScatterChart } from 'echarts/charts'
import {
  GridComponent, TooltipComponent, LegendComponent,
  DataZoomComponent, MarkLineComponent, MarkPointComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { getTheme, chartAxis, chartTooltip, chartLegend, chartDataZoom, hexToRgba } from '../styles/theme'
import { useEChart } from '../hooks/useEChart'
import { Card } from './ui/Card'
import {
  fetchFundDetail, fetchNav,
  type FundInfo, type NavPoint,
} from '../api'
import StatCard from './StatCard'
import { fmt, getDateRange } from '../utils'
import { useTranslation } from 'react-i18next'

echartsUse([
  LineChart, ScatterChart,
  GridComponent, TooltipComponent, LegendComponent,
  DataZoomComponent, MarkLineComponent, MarkPointComponent,
  CanvasRenderer,
])

/** Fill between daily NAV points with interpolated mid-points for smooth curves.
 *  Adds 3 sub-points per day so the line never looks flat between consecutive trading days. */
function fillDateGaps(dates: string[], values: number[]): { dates: string[], values: number[] } {
  if (dates.length < 2) return { dates, values };
  const SUB_STEPS = 3; // interpolated mid-points per day
  const filledDates: string[] = [];
  const filledValues: number[] = [];

  for (let i = 0; i < dates.length; i++) {
    if (i === dates.length - 1) {
      filledDates.push(dates[i]);
      filledValues.push(values[i]);
      break;
    }
    const prev = new Date(dates[i]);
    const curr = new Date(dates[i + 1]);
    const diffDays = Math.round((curr.getTime() - prev.getTime()) / 86400000);
    const totalSteps = Math.max(diffDays, 1) * SUB_STEPS;

    for (let step = 0; step < totalSteps; step++) {
      const frac = step / totalSteps;
      const mid = new Date(prev.getTime() + (curr.getTime() - prev.getTime()) * frac);
      const iso = mid.toISOString().substring(0, 10);
      const time = mid.toISOString().substring(11, 16);
      // Only show date label on the first sub-point of each actual date
      const label = step === 0 ? dates[i] : `${iso} ${time}`;
      filledDates.push(label);
      filledValues.push(+(values[i] + (values[i + 1] - values[i]) * frac).toFixed(4));
    }
  }
  return { dates: filledDates, values: filledValues };
}

export default function NasdaqOverview({ nasdaqFunds, onSelect, dark }: {
  nasdaqFunds: FundInfo[]; onSelect: (c: string) => void; dark: boolean
}) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const [proxyNav, setProxyNav] = useState<NavPoint[]>([]);
  const [allTx, setAllTx] = useState<{ code: string; name: string; tx: { trade_time: string; direction: string; amount: number }[] }[]>([]);
  const [range, setRange] = useState('tx');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const proxyFund = useMemo(() =>
    nasdaqFunds.find(f => f.code === '019173') || nasdaqFunds[0],
    [nasdaqFunds]
  );
  const proxyCode = proxyFund?.code || '019173';

  useEffect(() => {
    abortRef.current?.abort();
    const ctrl = new AbortController(); abortRef.current = ctrl; const sig = ctrl.signal;
    if (!proxyCode) return;
    setLoading(true);
    setError(null);
    fetchNav(proxyCode, sig)
      .then(d => {
        if (!sig.aborted) {
          setProxyNav(d);
          setLoading(false);
        }
      })
      .catch(e => {
        if (e.name !== 'AbortError') {
          console.warn('[nasdaqNav]', e);
          setError(e.message);
          setLoading(false);
        }
      });
    Promise.all(nasdaqFunds.map(f => fetchFundDetail(f.code, sig).then(d => ({
      code: f.code, name: f.name,
      tx: d.transactions.filter(t => t.direction === 'buy' || t.direction === 'sell')
    }))))
      .then(d => { if (!sig.aborted) setAllTx(d); })
      .catch(e => { if (e.name !== 'AbortError') console.warn('[nasdaqTx]', e); });
    return () => { ctrl.abort(); };
  }, [proxyCode, nasdaqFunds]);

  const allTxDates = useMemo(() => {
    const dates = new Set<string>();
    allTx.forEach(({ tx }) => tx.forEach(t => dates.add(t.trade_time.substring(0, 10))));
    return [...dates].sort();
  }, [allTx]);

  const dates10 = useMemo(() => proxyNav.map(d => d.date.substring(0, 10)), [proxyNav]);

  const scaleMap = useMemo(() => {
    const map: Record<string, number> = {};
    if (!proxyNav.length) return map;
    const proxyLatestNav = proxyNav[proxyNav.length - 1].unit_nav;
    if (!proxyLatestNav || proxyLatestNav <= 0) return map;
    for (const f of nasdaqFunds) map[f.code] = (f.latest_nav && f.latest_nav > 0) ? proxyLatestNav / f.latest_nav : 1;
    return map;
  }, [nasdaqFunds, proxyNav]);

  // ═══════ Chart option ═══════

  const chartOption = useMemo(() => {
    if (!proxyNav.length) return {} as Record<string, unknown>;

    const navs = proxyNav.map(d => d.unit_nav);
    const [i0, i1] = getDateRange(range, dates10, allTxDates);
    const slicedDates = dates10.slice(i0, i1 + 1);
    const slicedNavs = navs.slice(i0, i1 + 1);

    // Fill gaps between sparse dates with linear interpolation for smooth curves
    const filled = fillDateGaps(slicedDates, slicedNavs);
    const chartDates = filled.dates;
    const chartNavs = filled.values;
    const N = chartNavs.length;

    // Cumulative return % (first point = 0%)
    const baseNav = chartNavs[0] || 1;
    const returnPcts = chartNavs.map(n => +(((n - baseNav) / baseNav) * 100).toFixed(2));
    // Daily change % (on filled data shows smooth transitions)
    const dailyPcts = chartNavs.map((n, i) => i === 0 ? 0 : +(((n - chartNavs[i-1]) / chartNavs[i-1]) * 100).toFixed(2));

    // Map original dates for transaction markers
    const originalIndices: Record<string, number> = {};
    slicedDates.forEach((d, i) => {
      const idx = chartDates.indexOf(d);
      if (idx >= 0) originalIndices[d] = idx;
    });

    const buyPoints: any[] = [], sellPoints: any[] = [];
    allTx.forEach(({ code, name, tx }) => {
      const s = scaleMap[code] || 1;
      tx.forEach(t => {
        const td = t.trade_time.substring(0, 10);
        const idx = originalIndices[td];
        if (idx === undefined) return;
        const point = { value: [chartDates[idx], returnPcts[idx]], fund: name, code, amt: t.amount, normAmt: t.amount * s, nav: chartNavs[idx].toFixed(4) };
        if (t.direction === 'buy') buyPoints.push(point);
        else sellPoints.push(point);
      });
    });

    const totalBuyCount = allTx.reduce((s, { tx }) => s + tx.filter(t => t.direction === 'buy').length, 0);
    const totalSellCount = allTx.reduce((s, { tx }) => s + tx.filter(t => t.direction === 'sell').length, 0);
    const isUp = returnPcts[N-1] >= 0;
    const lineColor = isUp ? theme.down : theme.up; // CN convention: green=up, red=down for returns

    // Split area: green above zero, red below (CN convention)
    const aboveData = returnPcts.map(v => v >= 0 ? v : 0);
    const belowData = returnPcts.map(v => v < 0 ? v : 0);

    const series: any[] = [
      // Green area (above zero)
      { name: '收益(正)', type: 'line', data: aboveData, smooth: 0.6, symbol: 'none', yAxisIndex: 0, z: 2,
        lineStyle: { color: 'transparent', width: 0 },
        areaStyle: { color: new graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: hexToRgba(theme.down, 0.18) }, { offset: 1, color: hexToRgba(theme.down, 0) }]) },
        tooltip: { show: false }, legendHoverLink: false },
      // Red area (below zero)
      { name: '收益(负)', type: 'line', data: belowData, smooth: 0.6, symbol: 'none', yAxisIndex: 0, z: 2,
        lineStyle: { color: 'transparent', width: 0 },
        areaStyle: { color: new graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 1, color: hexToRgba(theme.up, 0.18) }, { offset: 0, color: hexToRgba(theme.up, 0) }]) },
        tooltip: { show: false }, legendHoverLink: false },
      // Main return curve
      { name: t('nasdaq.cumulativeReturn', '累计收益'), type: 'line', data: returnPcts, yAxisIndex: 0, z: 10,
        smooth: 0.6, symbol: 'none',
        lineStyle: { color: lineColor, width: 2.5, cap: 'round', shadowBlur: 6, shadowColor: hexToRgba(lineColor, 0.3) },
        markLine: { silent: true, symbol: 'none', lineStyle: { type: 'dashed', color: theme.border, width: 1 }, data: [{ yAxis: 0, label: { formatter: '0%', fontSize: 10 } }] },
        markPoint: { data: [
          { type: 'max', name: t('nasdaq.max', '最高'), symbol: 'pin', symbolSize: 32, itemStyle: { color: lineColor } },
          { type: 'min', name: t('nasdaq.min', '最低'), symbol: 'pin', symbolSize: 32, itemStyle: { color: theme.textMuted }, symbolRotate: 180 }],
          label: { fontSize: 10, fontWeight: 600 } },
      },
    ];

    // Buy scatter
    if (buyPoints.length) {
      series.push({ name: `${t('nasdaq.buy', '买入')} (${totalBuyCount})`, type: 'scatter', data: buyPoints, yAxisIndex: 0, z: 20,
        symbolSize: (val: any) => Math.min(16, Math.max(6, val.normAmt / 500)), symbol: 'circle',
        itemStyle: { color: theme.blue, borderColor: theme.surface, borderWidth: 1, opacity: 0.85 },
        emphasis: { scale: 1.4, itemStyle: { opacity: 1 } },
        tooltip: { formatter: (p: any) => { const d = p.data; return d?.fund ? `<b>${t('nasdaq.buy', '买入')}</b><br/>${d.fund}<br/>¥${(d.amt||0).toFixed(0)}<br/>${t('nasdaq.nav', '净值')}: ${d.nav}` : ''; } },
      });
    }
    if (sellPoints.length) {
      series.push({ name: `${t('nasdaq.sell', '卖出')} (${totalSellCount})`, type: 'scatter', data: sellPoints, yAxisIndex: 0, z: 20,
        symbolSize: (val: any) => Math.min(16, Math.max(6, val.normAmt / 500)), symbol: 'diamond',
        itemStyle: { color: theme.amber, borderColor: theme.surface, borderWidth: 1, opacity: 0.85 },
        emphasis: { scale: 1.4, itemStyle: { opacity: 1 } },
        tooltip: { formatter: (p: any) => { const d = p.data; return d?.fund ? `<b>${t('nasdaq.sell', '卖出')}</b><br/>${d.fund}<br/>¥${(d.amt||0).toFixed(0)}<br/>${t('nasdaq.nav', '净值')}: ${d.nav}` : ''; } },
      });
    }

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
          const ret = params.find(p => p.seriesName === t('nasdaq.cumulativeReturn', '累计收益'));
          const origIdx = originalIndices[date];
          let html = `<b>${date}</b>`;
          if (origIdx !== undefined) html += `<br/>净值: ${slicedNavs[origIdx]?.toFixed(4) || '-'}`;
          if (ret && ret.value !== undefined) {
            const v = Number(ret.value) || 0;
            html += `<br/>累计收益: <b style="color:${v >= 0 ? theme.down : theme.up}">${v >= 0 ? '+' : ''}${v}%</b>`;
          }
          const buys = params.filter(p => p.seriesName?.startsWith(t('nasdaq.buy', '买入')));
          const sells = params.filter(p => p.seriesName?.startsWith(t('nasdaq.sell', '卖出')));
          buys.forEach(p => { if (p.data?.fund) html += `<br/>🔵 ${p.data.fund} ¥${(p.data.amt||0).toFixed(0)}`; });
          sells.forEach(p => { if (p.data?.fund) html += `<br/>🟠 ${p.data.fund} ¥${(p.data.amt||0).toFixed(0)}`; });
          return html;
        },
      },
      legend: { selected: { '收益(正)': false, '收益(负)': false },
        data: series.filter(s => s.name && !s.name.startsWith('收益(')).map(s => s.name),
        top: 0, ...chartLegend(theme),
      },
      grid: { top: 40, right: 50, bottom: 60, left: 55 },
      xAxis: { type: 'category', data: chartDates,
        axisLabel: { fontSize: 9, rotate: N > 90 ? 45 : 0, color: theme.textMuted, interval: Math.max(1, Math.floor(N / 15)) },
        axisLine: { lineStyle: { color: theme.border } } },
      yAxis: [
        { type: 'value', name: '%', nameTextStyle: { fontSize: 10, color: theme.textMuted },
          axisLabel: { fontSize: 10, formatter: '{value}%', color: theme.textMuted },
          splitLine: { lineStyle: { color: theme.hairline } } },
      ],
      dataZoom: chartDataZoom(theme),
      series,
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [proxyNav, allTx, range, dates10, allTxDates, nasdaqFunds, scaleMap, proxyFund, dark]);

  const chartRef = useEChart(chartOption, [chartOption]);

  // ═══════ Stats (computed from data) ═══════
  const totalBuyCount = allTx.reduce((s, { tx }) => s + tx.filter(t => t.direction === 'buy').length, 0);
  const totalSellCount = allTx.reduce((s, { tx }) => s + tx.filter(t => t.direction === 'sell').length, 0);
  const totalBuyAmt = allTx.reduce((s, { tx }) => s + tx.filter(t => t.direction === 'buy').reduce((a, t) => a + t.amount, 0), 0);
  const totalSellAmt = allTx.reduce((s, { tx }) => s + tx.filter(t => t.direction === 'sell').reduce((a, t) => a + t.amount, 0), 0);
  const heldFunds = nasdaqFunds.filter(f => f.held_shares > 0.001);
  const navPnl = heldFunds.reduce((s, f) => s + (f.unrealized_pnl || 0), 0);
  const navValue = heldFunds.reduce((s, f) => s + (f.current_value || 0), 0);

  const latestReturn = (() => {
    if (!proxyNav.length) return 0;
    const navs = proxyNav.map(d => d.unit_nav);
    const [i0, i1] = getDateRange(range, dates10, allTxDates);
    const base = navs[i0] || 1;
    const latest = navs[i1] || base;
    return +(((latest - base) / base) * 100).toFixed(2);
  })();

  // ── Placeholder helper ──────────────────────────────────────────
  const placeholder = (msg: string, testid: string) => (
    <div data-testid={testid} style={{ height: 500, display: 'flex', alignItems: 'center', justifyContent: 'center', color: theme.textMuted, fontVariantNumeric: 'tabular-nums' }}>
      <Text variant="secondary" as="span" size="sm">{msg}</Text>
    </div>
  );

  return (
    <div>
      <Text variant="heading1" as="h1">{t('nasdaq.title', '纳斯达克总览')}</Text>
      <div style={{ marginTop: 8, marginBottom: 16 }}>
        <Text variant="secondary" as="span">
          {nasdaqFunds.length} {t('nasdaq.fundCount', '只纳指基金')} · {totalBuyCount} {t('nasdaq.buyCount', '笔买入')} / {totalSellCount} {t('nasdaq.sellCount', '笔卖出')} · {t('nasdaq.benchmark', '基准')}: {proxyFund?.name || ''}
        </Text>
      </div>
      <Grid variant="4up" gap="base" style={{ marginBottom: 20 }}>
        <StatCard label={t('nasdaq.funds', '纳指基金')} value={`${nasdaqFunds.length} ${t('common.units', '只')}`} />
        <StatCard label={t('nasdaq.totalBuy', '总买入')} value={`¥ ${totalBuyAmt.toLocaleString()}`} />
        {heldFunds.length > 0 && <StatCard label={t('nasdaq.holdValue', '持仓市值')} value={`¥ ${navValue.toFixed(0)}`} />}
        <StatCard label={t('nasdaq.pnl', '纳指盈亏')} value={fmt(navPnl)} color={navPnl > 0 ? 'up' : navPnl < 0 ? 'down' : undefined} />
        <StatCard label={t('nasdaq.periodReturn', '区间收益')} value={`${latestReturn >= 0 ? '+' : ''}${latestReturn}%`}
          color={latestReturn > 0 ? 'up' : latestReturn < 0 ? 'down' : undefined} />
      </Grid>
      <Card dark={dark} style={{ marginBottom: 20 }}>
        <div style={{ padding: '4px 0 16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 8 }}>
          <div>
            <Text variant="heading3" as="h3">{t('nasdaq.chartTitle', '纳指收益走势')}</Text>
            <div style={{ marginTop: 4 }}><Text variant="secondary" as="span" size="xs">{t('nasdaq.chartDesc', '累计收益率曲线 (线性插值平滑) + 买卖点标记')}</Text></div>
          </div>
          <Tabs tabs={RANGE_TABS} value={range} onValueChange={setRange} variant="segmented" size="sm" />
        </div>
        {loading
          ? placeholder(t('common.loading', '加载中…'), 'chart-loading')
          : error
            ? placeholder(error, 'chart-error')
            : !proxyNav.length
              ? placeholder(t('common.noData', '暂无数据'), 'chart-empty')
              : <div ref={chartRef} style={{ height: 500 }} />}
      </Card>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20, marginBottom: 20 }}>
        <LayerCard className="p-0">
          <div style={{ padding: '16px 20px 12px' }}><Text variant="heading3" as="h3">{t('nasdaq.held', '纳指持仓')}</Text></div>
          <Table>
            <Table.Header><Table.Row><Table.Head>{t('common.fund', '基金')}</Table.Head><Table.Head>{t('common.shares', '份额')}</Table.Head><Table.Head>{t('common.nav', '净值')}</Table.Head><Table.Head>{t('common.value', '市值')}</Table.Head><Table.Head>{t('common.pnl', '盈亏')}</Table.Head></Table.Row></Table.Header>
            <Table.Body>
              {heldFunds.sort((a, b) => (b.current_value ?? 0) - (a.current_value ?? 0)).map(f => {
                const pnl = f.unrealized_pnl ?? 0;
                return (
                  <Table.Row key={f.code} onClick={() => onSelect(f.code)} style={{ cursor: 'pointer' }}>
                    <Table.Cell><Text bold as="span">{f.name}</Text><br/><Text variant="secondary" as="span" size="xs">{f.code}</Text></Table.Cell>
                    <Table.Cell>{f.held_shares.toFixed(2)}</Table.Cell>
                    <Table.Cell>{f.latest_nav?.toFixed(4) ?? '-'}</Table.Cell>
                    <Table.Cell style={{ fontWeight: 500 }}>¥ {(f.current_value ?? 0).toFixed(2)}</Table.Cell>
                    <Table.Cell><span style={{ color: Number(pnl) > 0 ? theme.up : Number(pnl) < 0 ? theme.down : 'inherit' }}>{fmt(pnl)}</span></Table.Cell>
                  </Table.Row>
                );
              })}
            </Table.Body>
          </Table>
        </LayerCard>
        <LayerCard className="p-0">
          <div style={{ padding: '16px 20px 12px' }}><Text variant="heading3" as="h3">{t('nasdaq.cleared', '已清仓纳指')}</Text></div>
          <Table>
            <Table.Header><Table.Row><Table.Head>{t('common.fund', '基金')}</Table.Head><Table.Head>{t('nasdaq.historyTx', '历史交易')}</Table.Head></Table.Row></Table.Header>
            <Table.Body>
              {nasdaqFunds.filter(f => f.held_shares <= 0.001).map(f => (
                <Table.Row key={f.code} onClick={() => onSelect(f.code)} style={{ cursor: 'pointer' }}>
                  <Table.Cell><Text as="span">{f.name}</Text><br/><Text variant="secondary" as="span" size="xs">{f.code}</Text></Table.Cell>
                  <Table.Cell>{allTx.find(tx => tx.code === f.code)?.tx.length ?? 0} {t('nasdaq.txCount', '笔交易')}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table>
        </LayerCard>
      </div>
    </div>
  );
}

const RANGES = [
  { key: 'tx', label: '交易区间' }, { key: '1m', label: '近1月' }, { key: '3m', label: '近3月' },
  { key: '6m', label: '近6月' }, { key: '1y', label: '近1年' }, { key: 'all', label: '全部' },
];
const RANGE_TABS = RANGES.map(r => ({ value: r.key, label: r.label }));
