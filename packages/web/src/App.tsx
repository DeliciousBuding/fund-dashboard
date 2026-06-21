import "@cloudflare/kumo/styles/standalone";
import { useState, useEffect, useMemo, useRef, Suspense, lazy } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Grid, Button, Loader } from '@cloudflare/kumo'
import { TrendUp, Binoculars, Clock, CurrencyDollar, Sun, Moon, CaretLeft, House, ChartBar } from '@phosphor-icons/react'
import {
  fetchFunds, fetchPortfolio,
  fetchPortfolioXirr, fetchSecurities, fetchExchangeRate,
  type FundInfo, type Portfolio, type SecurityInfo, type ExchangeRate,
} from './api'
import StatCard from './components/StatCard'
import MarketTicker from './components/MarketTicker'
import { ErrorBoundary } from './components/ErrorBoundary'
import OfflineBanner from './components/OfflineBanner'
import { useDarkMode } from './hooks/useDarkMode'
import { fmt } from './utils'
import AppSidebar from './components/layout/Sidebar'
import AppLayout from './components/layout/AppLayout'

const NasdaqOverview = lazy(() => import('./components/NasdaqOverview'))
const FundDetailView = lazy(() => import('./components/FundDetailView'))
const PortfolioChart = lazy(() => import('./components/PortfolioChart'))
const PortfolioPenetration = lazy(() => import('./components/PortfolioPenetration'))
const PortfolioAllocation = lazy(() => import('./components/PortfolioAllocation'))
const InvestmentHarnessPanel = lazy(() => import('./components/InvestmentHarnessPanel'))
const PnLDistributionChart = lazy(() => import('./components/PnLDistributionChart'))
const CorrelationHeatmap = lazy(() => import('./components/CorrelationHeatmap'))
const MonteCarloChart = lazy(() => import('./components/MonteCarloChart'))
const AdminDashboard = lazy(() => import('./components/AdminDashboard'))
const FundComparison = lazy(() => import('./components/FundComparison'))
const PortfolioSwitcher = lazy(() => import('./components/PortfolioSwitcher'))

function ChartFallback() {
  const { t } = useTranslation();
  return <div style={{ padding: 60, textAlign: 'center' }}><Loader /><div style={{ marginTop: 12 }}><Text variant="secondary" as="span">{t('overview.loadingChart')}</Text></div></div>;
}

/** Compact error fallback for a single StatCard in the grid */
function StatCardError({ label }: { label: string }) {
  const { t } = useTranslation();
  return (
    <div style={{
      padding: '16px 20px', borderRadius: 8,
      background: 'var(--color-kumo-surface)',
      border: '1px solid var(--color-kumo-border)',
      minHeight: 80, display: 'flex', flexDirection: 'column', justifyContent: 'center',
    }}>
      <Text variant="secondary" as="span" size="xs">{label}</Text>
      <div style={{ marginTop: 4 }}>
        <Text variant="secondary" as="span" size="xs" style={{ color: '#d63649' }}>{t('overview.loadError')}</Text>
      </div>
    </div>
  );
}

/** Mobile detection hook */
function useIsMobile(breakpoint = 768) {
  const [isMobile, setIsMobile] = useState(window.innerWidth < breakpoint);
  useEffect(() => {
    const mq = window.matchMedia(`(max-width: ${breakpoint}px)`);
    const handler = (e: MediaQueryListEvent) => setIsMobile(e.matches);
    mq.addEventListener('change', handler);
    setIsMobile(mq.matches);
    return () => mq.removeEventListener('change', handler);
  }, [breakpoint]);
  return isMobile;
}

export default function App() {
  const { t } = useTranslation();
  const [funds, setFunds] = useState<FundInfo[]>([]);
  const [securities, setSecurities] = useState<SecurityInfo[]>([]);
  const [portfolio, setPortfolio] = useState<Portfolio | null>(null);
  const [activeCode, setActiveCode] = useState('overview');
  const [overviewTab, setOverviewTab] = useState<'chart' | 'allocation' | 'harness' | 'penetration' | 'pnl_dist' | 'advanced'>('chart');
  const [heldOnly, setHeldOnly] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [portfolioXirr, setPortfolioXirr] = useState<number | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [sidebarSearch, setSidebarSearch] = useState('');
  const [exchangeRate, setExchangeRate] = useState<ExchangeRate | null>(null);
  const [portfolioId, setPortfolioId] = useState(1);
  const isMobile = useIsMobile();
  const { dark, toggle: toggleDark } = useDarkMode();
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    abortRef.current?.abort();
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    const sig = ctrl.signal;

    Promise.all([fetchFunds(sig), fetchPortfolio(portfolioId, sig), fetchSecurities(sig)])
      .then(([f, p, s]) => { if (!sig.aborted) { setFunds(f); setPortfolio(p); setSecurities(s); } })
      .catch(e => { if (e.name !== 'AbortError' && !sig.aborted) setLoadError(e.message || '加载失败'); });

    fetchPortfolioXirr(portfolioId, sig).then(r => { if (!sig.aborted) setPortfolioXirr(r.xirr); }).catch(() => {});
    fetchExchangeRate(sig).then(r => { if (!sig.aborted) setExchangeRate(r); }).catch(() => {});

    return () => ctrl.abort();
  }, [portfolioId]);

  // Auto-collapse sidebar on mobile
  useEffect(() => {
    if (isMobile) setSidebarOpen(false);
  }, [isMobile, activeCode]);

  const nasdaqFunds = useMemo(() => {
    const g: Record<string, FundInfo[]> = {};
    for (const f of funds) {
      // Simple nasdaq classification inline to avoid importing utils just for nasdaq
      const isNasdaq = f.name.includes('纳斯达克') || f.name.includes('纳指');
      const cat = isNasdaq ? 'nasdaq' : 'other';
      if (!g[cat]) g[cat] = [];
      g[cat].push(f);
    }
    return g['nasdaq'] || [];
  }, [funds]);
  const pnl = portfolio?.unrealized_pnl ?? 0;

  const handleSelect = (code: string) => {
    setActiveCode(code);
    if (isMobile) setSidebarOpen(false);
  };

  /** Render exchange rate badge */
  const exchangeRateBadge = exchangeRate && (
    <span style={{
      fontSize: 11, color: 'var(--text-color-kumo-subtle)',
      display: 'inline-flex', alignItems: 'center', gap: 3,
      padding: '2px 8px', borderRadius: 4,
      background: 'var(--color-kumo-canvas)',
      whiteSpace: 'nowrap',
    }}>
      <CurrencyDollar size={12} />
      USD/CNY {exchangeRate.rate.toFixed(4)}
    </span>
  );

  const sidebar = (!isMobile || sidebarOpen) ? (
    <AppSidebar
      funds={funds}
      securities={securities}
      activeCode={activeCode}
      onSelect={handleSelect}
      heldOnly={heldOnly}
      onHeldToggle={() => setHeldOnly(v => !v)}
      searchQuery={sidebarSearch}
      onSearchChange={setSidebarSearch}
      dark={dark}
      onToggleDark={toggleDark}
      portfolio={portfolio}
      portfolioId={portfolioId}
      onPortfolioChange={setPortfolioId}
    />
  ) : null;

  return (
    <AppLayout sidebar={sidebar}>
      <OfflineBanner />
      {isMobile && (
        <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Button variant="secondary" size="sm" onClick={() => setSidebarOpen(!sidebarOpen)} aria-label={sidebarOpen ? t('mobile.closeMenu') : t('mobile.openMenu')}>
            {sidebarOpen ? t('mobile.closeMenu') : t('mobile.menu')}
          </Button>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            {portfolio?.last_nav_date && (
              <span style={{ fontSize: 11, color: 'var(--text-color-kumo-subtle)', display: 'flex', alignItems: 'center', gap: 4 }}>
                <Clock size={11} />{portfolio.last_nav_date}
              </span>
            )}
            <Button variant="secondary" size="sm" onClick={toggleDark} aria-label={dark ? t('nav.lightMode') : t('nav.darkMode')} style={{ padding: 6, minWidth: 32 }}>
              {dark ? <Sun size={16} weight="bold" /> : <Moon size={16} weight="bold" />}
            </Button>
          </div>
        </div>
      )}

      {loadError ? (
        <div style={{ padding: 60, textAlign: 'center' }}>
          <Text variant="body" as="span" style={{ display: 'block', fontSize: 16, color: '#d63649', marginBottom: 16 }}>{t('overview.loadErrorMsg', { error: loadError })}</Text>
          <Button variant="primary" onClick={() => {
            setLoadError(''); setFunds([]); setSecurities([]); setPortfolio(null);
            abortRef.current?.abort();
            const ctrl = new AbortController(); abortRef.current = ctrl;
            const sig = ctrl.signal;
            Promise.all([fetchFunds(sig), fetchPortfolio(portfolioId, sig), fetchSecurities(sig)])
              .then(([f, p, s]) => { if (!sig.aborted) { setFunds(f); setPortfolio(p); setSecurities(s); } })
              .catch(e => { if (!sig.aborted) setLoadError(e.message); });
          }}>{t('overview.retry')}</Button>
        </div>
      ) : activeCode === 'overview' ? (
        portfolio ? (
          <div>
            <Text variant="heading1" as="h1">{t('overview.title')}</Text>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
              <MarketTicker />
              {exchangeRateBadge}
              {import.meta.env.DEV && (
                <Button
                  variant={activeCode === 'admin' ? 'primary' : 'secondary'}
                  size="sm"
                  onClick={() => setActiveCode(activeCode === 'admin' ? 'overview' : 'admin')}
                  style={{ display: 'flex', alignItems: 'center', gap: 4 }}
                >
                  {activeCode === 'admin' ? t('overview.backToOverview') : t('overview.admin')}
                </Button>
              )}
            </div>
            <div style={{ marginTop: 8, marginBottom: 20 }}>
              <Text variant="secondary" as="span">{portfolio.first_trade} ~ {portfolio.last_trade} · {portfolio.total_tx} {t('tx.trades')} · {t('tx.auto')} {portfolio.auto_tx} / {t('tx.manual')} {portfolio.manual_tx}</Text>
              {portfolio.last_nav_date && (
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, marginLeft: 12, fontSize: 12, color: 'var(--text-color-kumo-subtle)' }}>
                  <Clock size={12} />
                  {t('tx.navUpdated')} {portfolio.last_nav_date}
                  {(() => { const now = new Date(); now.setHours(0, 0, 0, 0); const n = new Date(portfolio.last_nav_date!); n.setHours(0, 0, 0, 0); const days = Math.round((now.getTime() - n.getTime()) / 86400000); return days > 0 ? ` ${t('tx.daysAgo', { days })}` : ` ${t('tx.today')}`; })()}
                </span>
              )}
            </div>
            <Grid variant="4up" gap="base" style={{ marginBottom: 20 }}>
              <ErrorBoundary fallback={<StatCardError label={t('stat.totalBuy')} />}>
                <StatCard label={t('stat.totalBuy')} value={`¥ ${portfolio.total_buy.toLocaleString()}`} />
              </ErrorBoundary>
              <ErrorBoundary fallback={<StatCardError label={t('stat.totalSell')} />}>
                <StatCard label={t('stat.totalSell')} value={`¥ ${portfolio.total_sell.toLocaleString()}`} />
              </ErrorBoundary>
              <ErrorBoundary fallback={<StatCardError label={t('stat.unrealizedPnl')} />}>
                <StatCard label={t('stat.unrealizedPnl')} value={fmt(portfolio.unrealized_pnl)} color={portfolio.unrealized_pnl > 0 ? 'up' : portfolio.unrealized_pnl < 0 ? 'down' : undefined} />
              </ErrorBoundary>
              <ErrorBoundary fallback={<StatCardError label={t('stat.fee')} />}>
                <StatCard label={t('stat.fee')} value={`¥ ${portfolio.total_fee.toFixed(2)}`} />
              </ErrorBoundary>
              <ErrorBoundary fallback={<StatCardError label={t('stat.autoInvest')} />}>
                <StatCard label={t('stat.autoInvest')} value={`¥ ${portfolio.auto_amount.toLocaleString()}`} sub={`${portfolio.auto_tx} ${t('tx.trade')}`} />
              </ErrorBoundary>
              <ErrorBoundary fallback={<StatCardError label={t('stat.manualInvest')} />}>
                <StatCard label={t('stat.manualInvest')} value={`¥ ${portfolio.manual_amount.toLocaleString()}`} sub={`${portfolio.manual_tx} ${t('tx.trade')}`} />
              </ErrorBoundary>
              {portfolioXirr !== null && (
                <ErrorBoundary fallback={<StatCardError label={t('stat.xirr')} />}>
                  <StatCard label={t('stat.xirr')} value={`${portfolioXirr >= 0 ? '+' : ''}${portfolioXirr.toFixed(2)}%`}
                    color={portfolioXirr > 0 ? 'up' : portfolioXirr < 0 ? 'down' : undefined} />
                </ErrorBoundary>
              )}
            </Grid>
            <Suspense fallback={<ChartFallback />}>
              <ErrorBoundary><PortfolioChart dark={dark} portfolioId={portfolioId} /></ErrorBoundary>
            </Suspense>

            <div style={{ display: 'flex', gap: 8, marginBottom: 16, marginTop: 24 }}>
              <Button
                variant={overviewTab === 'chart' ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => setOverviewTab('chart')}
                style={{ display: 'flex', alignItems: 'center', gap: 6 }}
              >
                <TrendUp size={16} />{t('overview.navTrend')}
              </Button>
              <Button
                variant={overviewTab === 'allocation' ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => setOverviewTab('allocation')}
                style={{ display: 'flex', alignItems: 'center', gap: 6 }}
              >
                <CurrencyDollar size={16} />{t('overview.allocation')}
              </Button>
              <Button
                variant={overviewTab === 'harness' ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => setOverviewTab('harness')}
                style={{ display: 'flex', alignItems: 'center', gap: 6 }}
              >
                <Clock size={16} />{t('overview.harness')}
              </Button>
              <Button
                variant={overviewTab === 'penetration' ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => setOverviewTab('penetration')}
                style={{ display: 'flex', alignItems: 'center', gap: 6 }}
              >
                <Binoculars size={16} />{t('overview.penetration')}
              </Button>
              <Button
                variant={overviewTab === 'pnl_dist' ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => setOverviewTab('pnl_dist')}
                style={{ display: 'flex', alignItems: 'center', gap: 6 }}
              >
                <TrendUp size={16} />{t('overview.pnlDist')}
              </Button>
              <Button
                variant={overviewTab === 'advanced' ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => setOverviewTab('advanced')}
                style={{ display: 'flex', alignItems: 'center', gap: 6 }}
              >
                <ChartBar size={16} />{t('overview.advanced')}
              </Button>
            </div>

            {overviewTab === 'allocation' && (
              <Suspense fallback={<ChartFallback />}>
                <ErrorBoundary><PortfolioAllocation dark={dark} /></ErrorBoundary>
              </Suspense>
            )}

            {overviewTab === 'harness' && (
              <Suspense fallback={<ChartFallback />}>
                <ErrorBoundary><InvestmentHarnessPanel /></ErrorBoundary>
              </Suspense>
            )}

            {overviewTab === 'penetration' && (
              <Suspense fallback={<ChartFallback />}>
                <ErrorBoundary><PortfolioPenetration dark={dark} /></ErrorBoundary>
              </Suspense>
            )}

            {overviewTab === 'pnl_dist' && (
              <Suspense fallback={<ChartFallback />}>
                <ErrorBoundary><PnLDistributionChart dark={dark} /></ErrorBoundary>
              </Suspense>
            )}

            {overviewTab === 'advanced' && (
              <Suspense fallback={<ChartFallback />}>
                <ErrorBoundary>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
                    <CorrelationHeatmap dark={dark} />
                    <MonteCarloChart dark={dark} />
                  </div>
                </ErrorBoundary>
              </Suspense>
            )}
          </div>
        ) : (<div style={{ padding: 60, textAlign: 'center' }}><Loader /><div style={{ marginTop: 12 }}><Text variant="secondary" as="span">{t('overview.loading')}</Text></div></div>)
      ) : activeCode === 'compare' ? (
        <Suspense fallback={<ChartFallback />}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap', marginBottom: 8 }}>
            <MarketTicker />
            {exchangeRateBadge}
          </div>
          <ErrorBoundary><FundComparison funds={funds} dark={dark} /></ErrorBoundary>
        </Suspense>
      ) : activeCode === 'nasdaq-overview' ? (
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap', marginBottom: 8 }}>
            <MarketTicker />
            {exchangeRateBadge}
          </div>
          <Suspense fallback={<ChartFallback />}>
            <ErrorBoundary><NasdaqOverview nasdaqFunds={nasdaqFunds} onSelect={handleSelect} dark={dark} /></ErrorBoundary>
          </Suspense>
        </div>
      ) : activeCode === 'admin' && import.meta.env.DEV ? (
        <Suspense fallback={<ChartFallback />}>
          <ErrorBoundary><AdminDashboard /></ErrorBoundary>
        </Suspense>
      ) : (
        <Suspense fallback={<ChartFallback />}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap', marginBottom: 0 }}>
            <MarketTicker />
            {exchangeRateBadge}
          </div>
          <ErrorBoundary><FundDetailView code={activeCode} dark={dark} /></ErrorBoundary>
        </Suspense>
      )}

      {/* Mobile bottom nav */}
      {isMobile && (
        <nav style={{
          position: 'fixed', bottom: 0, left: 0, right: 0, height: 56,
          background: 'var(--color-kumo-surface)', borderTop: '1px solid var(--color-kumo-border)',
          display: 'flex', alignItems: 'center', justifyContent: 'space-around',
          zIndex: 1000, backdropFilter: 'blur(10px)', padding: '0 8px',
        }}>
          <Button variant="secondary" size="sm" onClick={() => handleSelect('overview')}
            style={{ flexDirection: 'column', gap: 2, height: 44, fontSize: 10, fontWeight: activeCode === 'overview' ? 600 : 400 }}>
            <House size={22} weight={activeCode === 'overview' ? 'fill' : 'regular'} />{t('mobile.overview')}
          </Button>
          <Button variant="secondary" size="sm" onClick={() => handleSelect('nasdaq-overview')}
            style={{ flexDirection: 'column', gap: 2, height: 44, fontSize: 10, fontWeight: activeCode === 'nasdaq-overview' ? 600 : 400 }}>
            <img src={dark ? "/ndaq-d.svg" : "/ndaq.svg"} width={22} height={22} style={{ borderRadius: 2 }} />{t('mobile.nasdaq')}
          </Button>
          {activeCode !== 'overview' && activeCode !== 'nasdaq-overview' && (
            <Button variant="secondary" size="sm" onClick={() => handleSelect('overview')}
              style={{ flexDirection: 'column', gap: 2, height: 44, fontSize: 10 }}>
              <CaretLeft size={22} />{t('mobile.back')}
            </Button>
          )}
        </nav>
      )}
    </AppLayout>
  );
}
