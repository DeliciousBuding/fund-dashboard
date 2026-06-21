import { useState, useMemo, useRef, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Tabs } from '@cloudflare/kumo'
import { use as echartsUse } from 'echarts/core'
import { LineChart, ScatterChart } from 'echarts/charts'
import {
  GridComponent, TooltipComponent, LegendComponent,
  DataZoomComponent, MarkLineComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { NavPoint, Transaction } from '../api'
import { getTheme, chartAxis, chartTooltip, chartLegend, hexToRgba, areaGradient } from '../styles/theme'
import { useEChart } from '../hooks/useEChart'
import { Card } from './ui/Card'
import { getDateRange } from '../utils'

echartsUse([
  LineChart, ScatterChart,
  GridComponent, TooltipComponent, LegendComponent,
  DataZoomComponent, MarkLineComponent,
  CanvasRenderer,
])

function getBuySellMarkers(dates: string[], navs: number[], transactions: { trade_time: string; direction: string }[]) {
  const buys: [string, number][] = [], sells: [string, number][] = [];
  transactions.forEach(tx => {
    const td = tx.trade_time.substring(0, 10);
    let idx = dates.indexOf(td);
    if (idx < 0) {
      idx = dates.findIndex(d => d >= td);
    }
    if (idx >= 0) {
      if (tx.direction === 'buy') buys.push([dates[idx], navs[idx]]);
      else if (tx.direction === 'sell') sells.push([dates[idx], navs[idx]]);
    }
  });
  return { buys, sells };
}

interface FundChartProps {
  navData: NavPoint[];
  transactions: Transaction[];
  heldShares: number;
  totalCost: number;
  chartTitle: string;
  priceLabel: string;
  dark: boolean;
}

const RANGES = [
  { key: 'tx', label: '交易区间' }, { key: '1m', label: '近1月' }, { key: '3m', label: '近3月' },
  { key: '6m', label: '近6月' }, { key: '1y', label: '近1年' }, { key: 'all', label: '全部' },
];
const RANGE_TABS = RANGES.map(r => ({ value: r.key, label: r.label }));

export default function FundChart({ navData, transactions, heldShares, totalCost, chartTitle, priceLabel, dark }: FundChartProps) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const [range, setRange] = useState('tx');
  const mountedRef = useRef(false);

  const txDates = useMemo(() => {
    return [...new Set(transactions.map(tx => tx.trade_time.substring(0, 10)))].sort();
  }, [transactions]);

  const dates10 = useMemo(() => navData.map(d => d.date.substring(0, 10)), [navData]);

  const option = useMemo(() => {
    if (!navData.length) return {} as Record<string, unknown>;
    const navs = navData.map(d => d.unit_nav);
    const [i0, i1] = getDateRange(range, dates10, txDates);
    const slicedDates = dates10.slice(i0, i1 + 1);
    const slicedNavs = navs.slice(i0, i1 + 1);
    const { buys, sells } = getBuySellMarkers(slicedDates, slicedNavs, transactions);

    const series: any[] = [];
    const legendItems: string[] = [];

    // NAV line series
    const avgCost = heldShares > 0.001 ? Number(Math.abs(totalCost)) / heldShares : null;
    const navSeries: Record<string, any> = {
      name: priceLabel, type: 'line', data: slicedNavs, smooth: true, symbol: 'none',
      lineStyle: { color: theme.blue, width: 2 },
      areaStyle: { color: areaGradient(theme, theme.blue) },
    };
    if (avgCost && avgCost > 0) {
      navSeries.markLine = {
        silent: true, symbol: 'none',
        lineStyle: { color: hexToRgba(theme.amber, 0.4), type: 'dashed', width: 1 },
        data: [{ yAxis: +avgCost.toFixed(4), name: t('fund.costLabel', '成本'), label: { formatter: `${t('fund.costLabel', '成本')} ¥${avgCost.toFixed(4)}`, fontSize: 9, color: hexToRgba(theme.amber, 0.6) } }],
      };
      legendItems.push(t('fund.costLabel', '成本'));
    }
    series.push(navSeries);
    legendItems.unshift(priceLabel);

    // MA20
    if (slicedNavs.length >= 20) {
      const ma20: (number | null)[] = [];
      let sum = 0;
      for (let i = 0; i < slicedNavs.length; i++) {
        sum += slicedNavs[i];
        if (i >= 20) sum -= slicedNavs[i - 20];
        ma20.push(i >= 19 ? +(sum / 20).toFixed(4) : null);
      }
      series.push({
        name: 'MA20', type: 'line', data: ma20, smooth: true, symbol: 'none',
        lineStyle: { color: theme.amber, width: 1, type: 'dotted', opacity: 0.6 },
      });
      legendItems.push('MA20');
    }

    if (buys.length) {
      series.push({
        name: t('fund.buyLabel', '买入'), type: 'scatter', data: buys, symbolSize: 8,
        itemStyle: { color: theme.up, borderColor: theme.surface, borderWidth: 1.5 }, z: 10,
      });
      legendItems.push(t('fund.buyLabel', '买入'));
    }
    if (sells.length) {
      series.push({
        name: t('fund.sellLabel', '卖出'), type: 'scatter', data: sells, symbolSize: 10,
        itemStyle: { color: theme.down, borderColor: theme.surface, borderWidth: 1.5 }, z: 10,
      });
      legendItems.push(t('fund.sellLabel', '卖出'));
    }

    return {
      tooltip: {
        trigger: 'axis',
        confine: true,
        ...chartTooltip(theme),
      },
      legend: {
        data: legendItems, top: 4,
        ...chartLegend(theme),
      },
      grid: { left: 55, right: 30, top: 32, bottom: 36 },
      xAxis: { type: 'category', data: slicedDates, boundaryGap: false, ...chartAxis(theme) },
      yAxis: { type: 'value', scale: true, ...chartAxis(theme) },
      dataZoom: [{ type: 'inside' }],
      series,
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [navData, transactions, range, txDates, dates10, dark, heldShares, totalCost, priceLabel]);

  const chartRef = useEChart(option, [option]);

  // ── Empty placeholder ─────────────────────────────────────────
  if (!navData.length) {
    return (
      <Card dark={dark}>
        <div data-testid="chart-empty" style={{ height: 500, display: 'flex', alignItems: 'center', justifyContent: 'center', color: theme.textMuted, fontVariantNumeric: 'tabular-nums' }}>
          {t('common.noData', '暂无数据')}
        </div>
      </Card>
    );
  }

  return (
    <Card dark={dark}>
      <div style={{ padding: '4px 0 16px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 8 }}>
        <Text variant="heading3" as="h3">{chartTitle}</Text>
        <Tabs tabs={RANGE_TABS} value={range} onValueChange={setRange} variant="segmented" size="sm" />
      </div>
      <div ref={chartRef} style={{ height: 500 }} />
    </Card>
  );
}
