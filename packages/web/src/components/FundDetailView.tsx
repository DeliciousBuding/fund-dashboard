import { useToast } from './feedback/Toast';
import { useState, useEffect, useMemo, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Grid, Badge, Tabs, Button } from '@cloudflare/kumo'
import { Card } from './ui/Card'
import { DownloadIcon } from '@phosphor-icons/react'
import { graphic } from 'echarts/core'
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
import { fmt } from '../services/format'
import { isUSStock, getCurrencySymbol } from '../services/classify'
import { sanitizeUserError } from '../services/userError'
import { toggleBuyTradeType } from '../services/tradeTypes'
import { getTheme, chartAxis, glassSurfaceStyle, radius, space, fontSize, fontWeight, zIndex, chartHeight } from '../styles/theme'
import { useCoreCharts } from './charts'
import { useEChart } from '../hooks/useEChart'
import { useAppStore } from '../stores/appStore'
import { useQueryRange } from '../hooks/useQueryRange'
import ConfirmDialog from './ConfirmDialog'
import PageFallback from './PageFallback'

useCoreCharts()

const MARKET_LABEL_KEYS: Record<string, string> = { SH: 'category.沪', SZ: 'category.深', HK: 'category.港', US: 'category.美' };
const MARKET_COLORS: Record<string, string> = { SH: 'red', SZ: 'green', HK: 'blue', US: 'purple' };


function marketBadge(market: string, t: (key: string) => string) {
  const mkt = market.toUpperCase();
  const labelKey = MARKET_LABEL_KEYS[mkt] || MARKET_LABEL_KEYS[market];
  if (!labelKey) return null;
  return <Badge variant={MARKET_COLORS[mkt] as any || 'neutral'} style={{ marginLeft: 6, fontSize: fontSize.sm }}>{t(labelKey)}</Badge>;
}

export default function FundDetailView({ code, dark }: { code: string; dark: boolean }) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const portfolioId = useAppStore((s) => s.portfolioId);
  const [data, setData] = useState<FundDetail | null>(null);
  const [navData, setNavData] = useState<NavPoint[]>([]);
  const DETAIL_TABS = ['chart', 'dca', 'overview', 'transactions'] as const;
  const [tab, setTab] = useQueryRange('detailTab', 'chart', DETAIL_TABS);
  const [error, setError] = useState('');
  const [xirr, setXirr] = useState<number | null>(null);
  const [drawdown, setDrawdown] = useState<number | null>(null);
  const [showAddForm, setShowAddForm] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [pendingDeleteSeq, setPendingDeleteSeq] = useState<number | null>(null);
  const [usStock, setUsStock] = useState<USStockInfo | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const exportMenuRef = useRef<HTMLDivElement>(null);
  const toast = useToast();

  const [refreshKey, setRefreshKey] = useState(0);
  const [exportOpen, setExportOpen] = useState(false);

  useEffect(() => {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    const sig = ctrl.signal;

    setData(null); setNavData([]); setError(''); setXirr(null); setDrawdown(null); setUsStock(null);

    const usMarket = isUSStock(code);

    Promise.all([
      fetchFundDetail(code, portfolioId, sig),
      fetchNav(code, sig),
    ])
      .then(([d, n]) => { if (!sig.aborted) { setData(d); setNavData(n); } })
      .catch(e => { if (e.name !== 'AbortError' && !sig.aborted) setError(sanitizeUserError(e, t('fundDetail.loadError'))); });

    if (usMarket) {
      fetchUSStock(code, sig)
        .then(s => { if (!sig.aborted) setUsStock(s); })
        .catch((e: any) => { if (e?.name !== 'AbortError') console.warn('[fundDetail]', e); });
    }

    fetchXirr(code, portfolioId, sig).then(r => { if (!sig.aborted) setXirr(r.xirr); }).catch((e: any) => { if (e?.name !== 'AbortError') console.warn('[fundDetail]', e); });
    fetchDrawdown(code, sig).then(r => { if (!sig.aborted) setDrawdown(r.max_drawdown); }).catch((e: any) => { if (e?.name !== 'AbortError') console.warn('[fundDetail]', e); });

    return () => { ctrl.abort(); };
  }, [code, portfolioId, refreshKey, t]);

  // Cumulative cost/value chart — shared useEChart lifecycle (#109)
  const cumOption = useMemo(() => {
    if (tab !== 'overview' || !data) return {} as Record<string, unknown>;
    const cc = getTheme(dark);
    const ax = chartAxis(cc);
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
      if (nav == null) return;
      tl.push({ date: tx.trade_time.substring(0, 10), cost: +cumCost.toFixed(2), value: +(cumShares * nav).toFixed(2) });
    });
    if (!tl.length) return {} as Record<string, unknown>;
    return {
      tooltip: { trigger: 'axis' },
      legend: { data: [t('fundDetail.totalCost'), t('fundDetail.currentValue')], top: 4, textStyle: { fontSize: fontSize.sm, color: cc.textSubtle } },
      grid: { left: 55, right: 20, top: 32, bottom: 24 },
      xAxis: { type: 'category', data: tl.map(pt => pt.date), ...ax },
      yAxis: { type: 'value', ...ax },
      series: [
        { name: t('fundDetail.totalCost'), type: 'line', data: tl.map(pt => pt.cost), lineStyle: { color: cc.amber, width: 2 }, symbol: 'none' },
        {
          name: t('fundDetail.currentValue'), type: 'line', data: tl.map(pt => pt.value),
          lineStyle: { color: cc.blue, width: 2 }, symbol: 'none',
          areaStyle: { color: new graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: cc.gridBg }, { offset: 1, color: cc.gridBgEnd }]) },
        },
      ],
    } as Record<string, unknown>;
  }, [data, tab, dark, t]);
  const cumRef = useEChart(cumOption, [cumOption]);

  // Export menu: Escape + outside click (#109)
  useEffect(() => {
    if (!exportOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setExportOpen(false);
    };
    const onClick = (e: MouseEvent) => {
      if (!exportMenuRef.current?.contains(e.target as Node)) setExportOpen(false);
    };
    document.addEventListener('keydown', onKey);
    document.addEventListener('mousedown', onClick);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('mousedown', onClick);
    };
  }, [exportOpen]);

  const handleToggleType = useCallback(async (seq: number, current: string) => {
    // Compare/write Chinese DB trade_type codes only (#184). Never use t() display strings.
    const newType = toggleBuyTradeType(current)
    if (newType === current) return
    try {
      await updateTransactionApi(seq, { trade_type: newType });
      const d = await fetchFundDetail(code, portfolioId);
      setData(d);
    } catch (e: any) { toast.error(t('fundDetail.switchFail', { message: sanitizeUserError(e, t('common.loadError')) })); }
  }, [code, portfolioId, t, toast]);

  const requestDeleteTx = useCallback((seq: number) => {
    setPendingDeleteSeq(seq)
  }, [])

  const confirmDeleteTx = useCallback(async () => {
    const seq = pendingDeleteSeq
    if (seq == null) return
    setPendingDeleteSeq(null)
    setDeleting(seq)
    try {
      await deleteTransactionApi(seq)
      const d = await fetchFundDetail(code, portfolioId)
      setData(d)
      toast.success(t('fundDetail.deleteOk', { seq }))
    } catch (e: any) {
      toast.error(t('fundDetail.deleteFail', { message: sanitizeUserError(e, t('common.loadError')) }))
    }
    setDeleting(null)
  }, [pendingDeleteSeq, code, portfolioId, t, toast])

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
    const d = await fetchFundDetail(code, portfolioId);
    setData(d);
    toast.success(t('fundDetail.addOk'));
  }, [code, portfolioId, t, toast]);

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
      toast.error(t('fundDetail.excelExportFail', { message: sanitizeUserError(e, t('common.loadError')) }));
    }
    setExportOpen(false);
  }, [data]);

  if (error) return (
    <div style={{ padding: space[8], textAlign: 'center' }}>
      <span style={{ fontSize: fontSize.xl, color: theme.critical }}>{t('fundDetail.loadError')}: {error}</span>
      <div style={{ marginTop: space[4] }}><Button type="button" variant="primary" onClick={() => {
        setError(''); setUsStock(null);
        const ctrl = new AbortController();
        fetchFundDetail(code, portfolioId, ctrl.signal).then(setData).catch(e => setError(sanitizeUserError(e, t('fundDetail.loadError'))));
        fetchNav(code, ctrl.signal).then(setNavData).catch((e: any) => { if (e?.name !== 'AbortError') console.warn('[fundDetail]', e); });
        if (isUSStock(code)) fetchUSStock(code, ctrl.signal).then(setUsStock).catch((e: any) => { if (e?.name !== 'AbortError') console.warn('[fundDetail]', e); });
      }}>{t('fundDetail.retry')}</Button></div>
    </div>
  );
  if (!data) return <PageFallback labelKey="fundDetail.loading" />;

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
      <div style={{ marginBottom: space[5], display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', flexWrap: 'wrap', gap: space[2] }}>
        <div>
          <Text variant="heading1" as="h1">{data.name}</Text>
          <div style={{ marginTop: space[1], display: 'flex', alignItems: 'center', flexWrap: 'wrap' }}>
            <Text variant="secondary" as="span" size="xs">{code}</Text>
            {(isStock || isUS) && data.market && marketBadge(data.market.toUpperCase(), t)}
            {isUS && livePrice != null && (
              <span style={{ marginLeft: 8, fontWeight: fontWeight.semibold, fontSize: fontSize.base }}>
                <span style={{ color: theme.blue }}>${livePrice.toFixed(2)}</span>
                {liveChange != null && (
                  <span style={{ marginLeft: 6, fontSize: fontSize.md, color: liveChange >= 0 ? theme.up : theme.down }}>
                    {liveChange >= 0 ? '+' : ''}{liveChange.toFixed(2)} ({liveChange >= 0 ? '+' : ''}{liveChangePct?.toFixed(2) ?? '0.00'}%)
                  </span>
                )}
              </span>
            )}
            <Text variant="secondary" as="span" size="xs" style={{ marginLeft: 8 }}>· T+{data.median_settlement} · {t('fundDetail.buySellCounts', { buy: data.buy_count, sell: data.sell_count })}</Text>
          </div>
          {usProfile && (
            <div style={{ marginTop: space[1], display: 'flex', gap: space[3], flexWrap: 'wrap' }}>
              {usProfile.sector && (
                <span style={{ fontSize: fontSize.md, color: 'var(--text-color-kumo-subtle)', display: 'flex', alignItems: 'center', gap: space[1] }}>
                  <span style={{ width: space[2] - 2, height: space[2] - 2, borderRadius: '50%', background: theme.blue, display: 'inline-block' }} />{usProfile.sector}
                </span>
              )}
              {usProfile.industry && (
                <span style={{ fontSize: fontSize.md, color: 'var(--text-color-kumo-subtle)' }}>{usProfile.industry}</span>
              )}
              {usProfile.market_cap != null && (
                <span style={{ fontSize: fontSize.md, color: 'var(--text-color-kumo-subtle)' }}>
                  {t('fundDetail.marketCap')} {usProfile.market_cap >= 1e12
                    ? `$${(usProfile.market_cap / 1e12).toFixed(2)}T`
                    : usProfile.market_cap >= 1e9
                      ? `$${(usProfile.market_cap / 1e9).toFixed(1)}B`
                      : `$${(usProfile.market_cap / 1e6).toFixed(0)}M`}
                </span>
              )}
              {usProfile.pe != null && (
                <span style={{ fontSize: fontSize.md, color: 'var(--text-color-kumo-subtle)' }}>PE {usProfile.pe.toFixed(1)}</span>
              )}
            </div>
          )}
        </div>
        <div style={{ position: 'relative' }} ref={exportMenuRef}>
          <Button type="button"
            variant="secondary"
            size="sm"
            aria-haspopup="menu"
            aria-expanded={exportOpen}
            onClick={() => setExportOpen(v => !v)}
          >
            <DownloadIcon size={14} style={{ marginRight: 4 }} aria-hidden /> {t('common.export')}
          </Button>
          {exportOpen && (
            <div
              role="menu"
              aria-label={t('common.export')}
              style={{
                position: 'absolute', top: '100%', right: 0, marginTop: space[1],
                ...glassSurfaceStyle(theme, { borderRadius: radius.md }),
                zIndex: zIndex.dropdown, minWidth: 120, overflow: 'hidden',
              }}
            >
              <button
                type="button"
                role="menuitem"
                onClick={handleExportCsv}
                style={{
                  display: 'block', width: '100%', padding: `${space[2]}px ${space[4]}px`, border: 'none',
                  background: 'transparent', cursor: 'pointer', textAlign: 'left',
                  fontSize: fontSize.base, color: theme.text,
                }}
              >{t('fundDetail.exportCsv')}</button>
              <button
                type="button"
                role="menuitem"
                onClick={() => { void handleExportXlsx(); }}
                style={{
                  display: 'block', width: '100%', padding: `${space[2]}px ${space[4]}px`, border: 'none',
                  background: 'transparent', cursor: 'pointer', textAlign: 'left',
                  fontSize: fontSize.base, color: theme.text,
                }}
              >{t('fundDetail.exportXlsx')}</button>
            </div>
          )}
        </div>
      </div>

      {/* Stat cards */}
      <Grid variant="4up" gap="base" style={{ marginBottom: space[5] }}>
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
      <Tabs tabs={tabs} value={tab} onValueChange={setTab} variant="underline" style={{ marginBottom: space[5] }} />

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
          <Card dark={dark} glass padded={false}><div style={{ padding: `${space[4]}px ${space[5]}px 0` }}><Text variant="heading3" as="h3">{t('fundDetail.costVsValue')}</Text></div><div ref={cumRef} style={{ height: chartHeight.detail }} /></Card>
          <Card dark={dark} glass padded={false}><div style={{ padding: `${space[4]}px ${space[5]}px` }}>
            <Text variant="heading3" as="h3">{t('fundDetail.txStats')}</Text>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: `${space[3]}px ${space[5]}px`, marginTop: space[4] }}>
              {([
                [t('fundDetail.stats.totalBuyCount'), t('fundDetail.stats.countUnit', { count: data.buy_count })],
                [t('fundDetail.stats.totalSellCount'), t('fundDetail.stats.countUnit', { count: data.sell_count })],
                [t('fundDetail.stats.autoBuy'), t('fundDetail.stats.amountUnit', { count: data.auto_buy_count, amount: data.auto_buy_amount.toFixed(0) })],
                [t('fundDetail.stats.manualBuy'), t('fundDetail.stats.amountUnit', { count: data.manual_buy_count, amount: data.manual_buy_amount.toFixed(0) })],
                [t('fundDetail.stats.avgBuyAmount'), data.buy_count > 0 ? `¥ ${(Math.abs(data.total_cost)/data.buy_count).toFixed(0)}` : '-'],
                [t('fundDetail.stats.settlement'), `T+${data.median_settlement}`],
              ] as [string, string][]).map(([l, v]) => (
                <div key={l}>
                  <Text variant="secondary" as="span" size="xs">{l}</Text>
                  <div style={{ marginTop: space[1] / 2 }}><Text variant="body" as="span" bold>{v}</Text></div>
                </div>
              ))}
            </div>
          </div></Card>
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
            onDelete={requestDeleteTx}
            onAdd={() => setShowAddForm(!showAddForm)}
            deleting={deleting}
          />
        </div>
      )}

      <ConfirmDialog
        open={pendingDeleteSeq != null}
        title={t('fundDetail.deleteTitle')}
        message={t('fundDetail.deleteConfirm', { seq: pendingDeleteSeq ?? 0 })}
        confirmLabel={t('fundDetail.deleteThisTx')}
        cancelLabel={t('common.cancel')}
        destructive
        onConfirm={() => { void confirmDeleteTx() }}
        onCancel={() => setPendingDeleteSeq(null)}
      />
    </div>
  );
}
