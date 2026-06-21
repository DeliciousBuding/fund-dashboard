import { useState, useEffect, useMemo, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, LayerCard, Grid, Badge, Tabs, Button, Loader } from '@cloudflare/kumo'
import { DownloadIcon, UploadIcon } from '@phosphor-icons/react'
import { use as echartsUse, init as echartsInit, type ECharts, graphic } from 'echarts/core'
import { LineChart, ScatterChart } from 'echarts/charts'
import {
  GridComponent, TooltipComponent, LegendComponent,
  DataZoomComponent, MarkLineComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import {
  fetchFundDetail, fetchNav, fetchXirr, fetchDrawdown,
  fetchUSStock,
  transactionsToCsv, downloadCsv, downloadTransactionsXlsx,
  updateTransactionApi, deleteTransactionApi, addTransactionApi,
  type FundDetail, type NavPoint,
  type USStockInfo,
} from '../api'
import StatCard from './StatCard'
import FundChart from './FundChart'
import TransactionTable from './TransactionTable'
import TransactionForm from './TransactionForm'
import DcaPanel from './DcaPanel'
import { C, chartColors, fmt, sharedAxis, isUSStock, getCurrencySymbol } from '../utils'

echartsUse([
  LineChart, ScatterChart,
  GridComponent, TooltipComponent, LegendComponent,
  DataZoomComponent, MarkLineComponent,
  CanvasRenderer,
])

const DIR: Record<string, string> = { buy: '买入', sell: '卖出', dividend: '分红', convert_in: '转入', convert_out: '转出', forced_redeem: '强赎' };
const MARKET_LABELS: Record<string, string> = { SH: '沪', SZ: '深', HK: '港', US: '美' };
const MARKET_COLORS: Record<string, string> = { SH: 'red', SZ: 'green', HK: 'blue', US: 'purple' };

function marketBadge(market: string) {
  const label = MARKET_LABELS[market] || (market.toUpperCase && MARKET_LABELS[market.toUpperCase()]);
  if (!label) return null;
  const mkt = market.toUpperCase();
  return <Badge variant={MARKET_COLORS[mkt] as any || 'neutral'} style={{ marginLeft: 6, fontSize: 11 }}>{label}</Badge>;
}

function initChart(dom: HTMLDivElement | null, ref: React.MutableRefObject<ECharts[]>) {
  if (!dom) return null;
  const old = ref.current.find(c => { try { return c.getDom() === dom; } catch { return false; }});
  if (old) { old.dispose(); ref.current = ref.current.filter(c => c !== old); }
  const ch = echartsInit(dom); ref.current.push(ch);
  return ch;
}

export default function FundDetailView({ code, dark }: { code: string; dark: boolean }) {
  const { t } = useTranslation();
  const [data, setData] = useState<FundDetail | null>(null);
  const [navData, setNavData] = useState<NavPoint[]>([]);
  const [tab, setTab] = useState('chart');
  const [error, setError] = useState('');
  const [xirr, setXirr] = useState<number | null>(null);
  const [drawdown, setDrawdown] = useState<number | null>(null);
  const [showAddForm, setShowAddForm] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [usStock, setUsStock] = useState<USStockInfo | null>(null);
  const cumRef = useRef<HTMLDivElement>(null);
  const instRef = useRef<ECharts[]>([]);
  const abortRef = useRef<AbortController | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [toast, setToast] = useState('');
  const [refreshKey, setRefreshKey] = useState(0);
  const [exportOpen, setExportOpen] = useState(false);

  const disposeAll = useCallback(() => { instRef.current.forEach(c => { try { c.dispose() } catch {} }); instRef.current = []; }, []);

  useEffect(() => {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    const sig = ctrl.signal;

    setData(null); setNavData([]); setError(''); setXirr(null); setDrawdown(null); setUsStock(null); disposeAll();

    const usMarket = isUSStock(code);

    Promise.all([
      fetchFundDetail(code, sig),
      fetchNav(code, sig),
    ])
      .then(([d, n]) => { if (!sig.aborted) { setData(d); setNavData(n); } })
      .catch(e => { if (e.name !== 'AbortError' && !sig.aborted) setError(e.message || t('fundDetail.loadError')); });

    if (usMarket) {
      fetchUSStock(code, sig)
        .then(s => { if (!sig.aborted) setUsStock(s); })
        .catch(() => {});
    }

    fetchXirr(code, sig).then(r => { if (!sig.aborted) setXirr(r.xirr); }).catch(() => {});
    fetchDrawdown(code, sig).then(r => { if (!sig.aborted) setDrawdown(r.max_drawdown); }).catch(() => {});

    return () => { ctrl.abort(); disposeAll(); };
  }, [code, refreshKey, disposeAll]);

  // Cumulative chart effect
  useEffect(() => {
    if (tab !== 'overview' || !data) return;
    const rafId = requestAnimationFrame(() => {
      const dom = cumRef.current;
      if (!dom) return;
      const ch = initChart(dom, instRef); if (!ch) return;
      const cc = chartColors(dark);
      const ax = sharedAxis(dark);
      let cumCost = 0, cumShares = 0;
      const tl: { date: string; cost: number; value: number }[] = [];
      data.transactions.forEach(tx => {
        if (tx.direction === 'buy') { cumCost += tx.amount; cumShares += tx.shares; }
        else if (tx.direction === 'sell') {
          if (cumShares > 0.001) {
            const ratio = Math.abs(tx.shares || 0) / cumShares;
            cumCost -= cumCost * ratio;
          }
          cumShares -= Math.abs(tx.shares || 0);
        }
        else if (tx.direction === 'dividend') cumCost -= tx.amount;
        const nav = tx.nav || data.latest_nav;
        if (nav == null) return; // skip data point when NAV is unavailable
        tl.push({ date: tx.trade_time.substring(0, 10), cost: +cumCost.toFixed(2), value: +(cumShares * nav).toFixed(2) });
      });
      ch.setOption({
        tooltip: { trigger: 'axis' }, legend: { data: [t('fundDetail.totalCost'), t('fundDetail.currentValue')], top: 4, textStyle: { fontSize: 11, color: dark ? '#e5e7eb' : '#6b7280' } },
        grid: { left: 55, right: 20, top: 32, bottom: 24 },
        xAxis: { type: 'category', data: tl.map(t => t.date), ...ax },
        yAxis: { type: 'value', ...ax },
        series: [
          { name: t('fundDetail.totalCost'), type: 'line', data: tl.map(t => t.cost), lineStyle: { color: cc.amber, width: 2 }, symbol: 'none' },
          { name: t('fundDetail.currentValue'), type: 'line', data: tl.map(t => t.value), lineStyle: { color: cc.blue, width: 2 }, symbol: 'none',
            areaStyle: { color: new graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: cc.gridBg }, { offset: 1, color: cc.gridBgEnd }]) } },
        ],
      });
    });
    return () => cancelAnimationFrame(rafId);
  }, [data, tab, dark]);

  const handleToggleType = useCallback(async (seq: number, current: string) => {
    const newType = current === t('fundDetail.dcabuy') ? t('fundDetail.userbuy') : current === t('fundDetail.userbuy') ? t('fundDetail.dcabuy') : current;
    try {
      await updateTransactionApi(seq, { trade_type: newType });
      const d = await fetchFundDetail(code);
      setData(d);
    } catch (e: any) { alert(t('fundDetail.switchFail', { message: e.message })); }
  }, [code]);

  const handleDeleteTx = useCallback(async (seq: number) => {
    if (!confirm(t('fundDetail.deleteConfirm', { seq }))) return;
    setDeleting(seq);
    try {
      await deleteTransactionApi(seq);
      const d = await fetchFundDetail(code);
      setData(d);
    } catch (e: any) { alert(t('fundDetail.deleteFail', { message: e.message })); }
    setDeleting(null);
  }, [code]);

  const handleAddTx = useCallback(async (formData: { direction: string; trade_type: string; amount: string; shares: string; fee: string; date: string }) => {
    const amount = parseFloat(formData.amount);
    const shares = parseFloat(formData.shares);
    await addTransactionApi({
      fund_code: code,
      trade_time: formData.date + ':00',
      direction: formData.direction as 'buy' | 'sell',
      trade_type: formData.trade_type,
      confirm_amount: amount,
      confirm_share: formData.shares ? shares : undefined,
      fee: parseFloat(formData.fee) || 0,
    });
    setShowAddForm(false);
    const d = await fetchFundDetail(code);
    setData(d);
  }, [code]);

  const handleExportCsv = useCallback(() => {
    if (!data) return;
    const csv = transactionsToCsv(data.transactions, data.name);
    downloadCsv(csv, `${data.name}_${code}_transactions.csv`);
    setExportOpen(false);
  }, [data, code]);

  const handleExportXlsx = useCallback(async () => {
    if (!data) return;
    try {
      await downloadTransactionsXlsx(data.transactions, data.name);
    } catch (e: any) {
      alert(t('fundDetail.excelExportFail', { message: e.message }));
    }
    setExportOpen(false);
  }, [data]);

  if (error) return (
    <div style={{ padding: 60, textAlign: 'center' }}>
      <span style={{ fontSize: 16, color: '#d63649' }}>{t('fundDetail.loadError')}：{error}</span>
      <div style={{ marginTop: 16 }}><Button variant="primary" onClick={() => {
        setError(''); setUsStock(null);
        const ctrl = new AbortController();
        fetchFundDetail(code, ctrl.signal).then(setData).catch(e => setError(e.message));
        fetchNav(code, ctrl.signal).then(setNavData).catch(() => {});
        if (isUSStock(code)) fetchUSStock(code, ctrl.signal).then(setUsStock).catch(() => {});
      }}>{t('fundDetail.retry')}</Button></div>
    </div>
  );
  if (!data) return <div style={{ padding: 60, textAlign: 'center' }}><Loader /><div style={{ marginTop: 12 }}><Text variant="secondary" as="span">{t('fundDetail.loading')}</Text></div></div>;

  const pnl = data.unrealized_pnl ?? 0;
  const livePrice = usStock?.price;
  const liveChange = usStock?.change;
  const liveChangePct = usStock?.change_pct;
  const liveCurrency = usStock?.currency || 'USD';
  const usProfile = usStock?.profile;
  const isStock = data.security_type === 'stock';
  const isUS = isUSStock(code) || data.market === 'us';
  const chartTitle = isStock ? t('fundDetail.priceChart') : t('fundDetail.navChart');
  const priceLabel = isStock ? t('fundDetail.priceLabel') : t('fundDetail.navLabel');

  const tabs = [
    { value: 'chart', label: chartTitle },
    { value: 'dca', label: t('fundDetail.tabDca') },
    { value: 'overview', label: t('fundDetail.tabOverview') },
    { value: 'transactions', label: `${t('fundDetail.tabTransactions')} (${data.transactions.length})` },
  ];

  return (
    <div>
      {/* Header */}
      <div style={{ marginBottom: 20, display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', flexWrap: 'wrap', gap: 8 }}>
        <div>
          <Text variant="heading1" as="h1">{data.name}</Text>
          <div style={{ marginTop: 4, display: 'flex', alignItems: 'center', flexWrap: 'wrap' }}>
            <Text variant="secondary" as="span" size="xs">{code}</Text>
            {(isStock || isUS) && data.market && marketBadge(data.market.toUpperCase())}
            {isUS && livePrice != null && (
              <span style={{ marginLeft: 8, fontWeight: 600, fontSize: 13 }}>
                <span style={{ color: C.blue }}>${livePrice.toFixed(2)}</span>
                {liveChange != null && (
                  <span style={{ marginLeft: 6, fontSize: 12, color: liveChange >= 0 ? C.up : C.down }}>
                    {liveChange >= 0 ? '+' : ''}{liveChange.toFixed(2)} ({liveChange >= 0 ? '+' : ''}{liveChangePct?.toFixed(2) ?? '0.00'}%)
                  </span>
                )}
              </span>
            )}
            <Text variant="secondary" as="span" size="xs" style={{ marginLeft: 8 }}>· T+{data.median_settlement} · {data.buy_count} 买 / {data.sell_count} 卖</Text>
          </div>
          {usProfile && (
            <div style={{ marginTop: 4, display: 'flex', gap: 12, flexWrap: 'wrap' }}>
              {usProfile.sector && (
                <span style={{ fontSize: 12, color: 'var(--text-color-kumo-subtle)', display: 'flex', alignItems: 'center', gap: 4 }}>
                  <span style={{ width: 6, height: 6, borderRadius: '50%', background: C.blue, display: 'inline-block' }} />{usProfile.sector}
                </span>
              )}
              {usProfile.industry && (
                <span style={{ fontSize: 12, color: 'var(--text-color-kumo-subtle)' }}>{usProfile.industry}</span>
              )}
              {usProfile.market_cap != null && (
                <span style={{ fontSize: 12, color: 'var(--text-color-kumo-subtle)' }}>
                  市值 {usProfile.market_cap >= 1e12
                    ? `$${(usProfile.market_cap / 1e12).toFixed(2)}T`
                    : usProfile.market_cap >= 1e9
                      ? `$${(usProfile.market_cap / 1e9).toFixed(1)}B`
                      : `$${(usProfile.market_cap / 1e6).toFixed(0)}M`}
                </span>
              )}
              {usProfile.pe != null && (
                <span style={{ fontSize: 12, color: 'var(--text-color-kumo-subtle)' }}>PE {usProfile.pe.toFixed(1)}</span>
              )}
            </div>
          )}
        </div>
        <div style={{ position: 'relative' }}>
          <Button variant="secondary" size="sm" onClick={() => setExportOpen(v => !v)}>
            <DownloadIcon size={14} style={{ marginRight: 4 }} /> {t('common.export')}
          </Button>
          {exportOpen && (
            <div style={{
              position: 'absolute', top: '100%', right: 0, marginTop: 4,
              background: 'var(--color-kumo-surface)', border: '1px solid var(--color-kumo-border)',
              borderRadius: 8, boxShadow: '0 4px 12px rgba(0,0,0,0.15)', zIndex: 100,
              minWidth: 120, overflow: 'hidden',
            }}>
              <button onClick={handleExportCsv} style={{
                display: 'block', width: '100%', padding: '8px 16px', border: 'none',
                background: 'transparent', cursor: 'pointer', textAlign: 'left',
                fontSize: 13, color: 'var(--text-color-kumo-primary)',
              }}>CSV</button>
              <button onClick={handleExportXlsx} style={{
                display: 'block', width: '100%', padding: '8px 16px', border: 'none',
                background: 'transparent', cursor: 'pointer', textAlign: 'left',
                fontSize: 13, color: 'var(--text-color-kumo-primary)',
              }}>Excel (.xlsx)</button>
            </div>
          )}
        </div>
        <Button variant="secondary" size="sm" onClick={() => fileInputRef.current?.click()}>
          <UploadIcon size={14} style={{ marginRight: 4 }} /> 导入 CSV
        </Button>
        <input ref={fileInputRef} type="file" accept=".csv" style={{ display: 'none' }}
          onChange={async (e) => {
            const file = (e.target as HTMLInputElement).files?.[0];
            if (!file) return;
            const text = await file.text();
            try {
              const res = await fetch('/api/admin/import-csv', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ csv: text }),
              });
              const result = await res.json();
              if (result.ok) {
                setToast(t('fundDetail.importSuccess', { imported: result.imported, total: result.total }));
                setRefreshKey(k => k + 1); // refresh
              } else {
                setToast(t('fundDetail.importError', { error: result.error || '未知错误' }));
              }
            } catch (err: any) {
              setToast(t('fundDetail.importErr', { message: err.message }));
            }
            (e.target as HTMLInputElement).value = '';
          }}
        />
      </div>

      {/* Stat cards */}
      <Grid variant="4up" gap="base" style={{ marginBottom: 20 }}>
        <StatCard label={t('fundDetail.heldShares')} value={data.held_shares.toFixed(2)} />
        <StatCard label={t('fundDetail.totalCost')} value={`¥ ${Math.abs(data.total_cost).toFixed(2)}`} />
        <StatCard label={isStock ? t('fundDetail.latestPrice') : t('fundDetail.latestNav')} value={isUS && livePrice != null ? `$ ${livePrice.toFixed(2)}` : data.latest_nav?.toFixed(4) ?? '-'} />
        <StatCard label={t('fundDetail.currentValue')} value={`¥ ${(data.current_value ?? 0).toFixed(2)}`} />
        <StatCard label={t('fundDetail.unrealizedPnl')} value={fmt(pnl)} color={pnl > 0 ? 'up' : pnl < 0 ? 'down' : undefined} sub={data.pnl_pct ? `${pnl >= 0 ? '+' : ''}${data.pnl_pct.toFixed(2)}%` : undefined} />
        {isUS && livePrice != null && (
          <StatCard label={`${t('fundDetail.todayOpenPrevClose')} (${liveCurrency})`} value={`$${livePrice.toFixed(2)}`}
            sub={usStock ? `${t('common.price')} $${usStock.open.toFixed(2)} / ${t('stat.prevClose')} $${usStock.previous_close.toFixed(2)}` : undefined} />
        )}
        {isUS && usStock?.high != null && (
          <StatCard label={`${t('fundDetail.dayHighLow')} (${liveCurrency})`} value={`$${usStock.high.toFixed(2)}`}
            sub={`${t('common.price')} $${usStock.low.toFixed(2)}${usStock.volume ? ` · ${t('common.volume')} ${(usStock.volume / 1e6).toFixed(1)}M` : ''}`} />
        )}
        {!isUS && (<StatCard label={t('fundDetail.dcaManual')} value={`${data.auto_buy_count} / ${data.manual_buy_count} ${t('tx.trades')}`} />)}
        {isUS && (<StatCard label={t('fundDetail.tradeCount')} value={`${data.buy_count} ${t('fundDetail.dir.buy')} / ${data.sell_count} ${t('fundDetail.dir.sell')}`} />)}
        {xirr !== null && (
          <StatCard label={t('fundDetail.xirr')} value={`${xirr >= 0 ? '+' : ''}${xirr.toFixed(2)}%`}
            color={xirr > 0 ? 'up' : xirr < 0 ? 'down' : undefined} />
        )}
        {drawdown !== null && (
          <StatCard label={t('fundDetail.maxDrawdown')} value={`-${drawdown.toFixed(2)}%`} color="down" />
        )}
      </Grid>

      {/* Tabs */}
      <Tabs tabs={tabs} value={tab} onValueChange={setTab} variant="underline" style={{ marginBottom: 20 }} />

      {/* Tab content */}
      {tab === 'chart' && (
        <FundChart
          navData={navData}
          transactions={data.transactions}
          heldShares={data.held_shares}
          totalCost={data.total_cost}
          chartTitle={chartTitle}
          priceLabel={priceLabel}
          dark={dark}
        />
      )}

      {tab === 'dca' && (
        <DcaPanel
          fundCode={code}
          heldShares={data.held_shares}
          latestNav={data.latest_nav ?? 0}
          totalCost={data.total_cost}
          dark={dark}
        />
      )}

      {tab === 'overview' && (
        <Grid variant="2up" gap="base">
          <LayerCard><div style={{ padding: '16px 20px 0' }}><Text variant="heading3" as="h3">{t('fundDetail.costVsValue')}</Text></div><div ref={cumRef} style={{ height: 340 }} /></LayerCard>
          <LayerCard><div style={{ padding: '16px 20px' }}>
            <Text variant="heading3" as="h3">交易统计</Text>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px 24px', marginTop: 16 }}>
              {[['总买入次数',`${data.buy_count} 次`],['总卖出次数',`${data.sell_count} 次`],['定投买入',`${data.auto_buy_count} 笔 / ¥ ${data.auto_buy_amount.toFixed(0)}`],['手动买入',`${data.manual_buy_count} 笔 / ¥ ${data.manual_buy_amount.toFixed(0)}`],['平均买入金额',data.buy_count > 0 ? `¥ ${(Math.abs(data.total_cost)/data.buy_count).toFixed(0)}` : '-'],['结算周期',`T+${data.median_settlement}`]].map(([l,v]) => (<div key={l}><Text variant="secondary" as="span" size="xs">{l}</Text><div style={{ marginTop: 2 }}><Text variant="body" as="span" bold>{v}</Text></div></div>))}
            </div>
          </div></LayerCard>
        </Grid>
      )}

      {tab === 'transactions' && (
        <div>
          {showAddForm && (
            <TransactionForm
              onSubmit={handleAddTx}
              onCancel={() => setShowAddForm(false)}
            />
          )}
          <TransactionTable
            transactions={data.transactions}
            onToggleType={handleToggleType}
            onDelete={handleDeleteTx}
            onAdd={() => setShowAddForm(!showAddForm)}
            deleting={deleting}
          />
        </div>
      )}
    </div>
  );
}
