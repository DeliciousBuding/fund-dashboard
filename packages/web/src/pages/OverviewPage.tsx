import { Suspense, lazy, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Text, Grid } from '@cloudflare/kumo';
import { TrendUp, Binoculars, Clock, ChartBar, CurrencyDollar } from '@phosphor-icons/react';
import { useQueryClient, useIsFetching } from '@tanstack/react-query';
import { useAppStore } from '../stores/appStore';
import { ErrorBoundary } from '../components/ErrorBoundary';
import ChartFallback from '../components/ChartFallback';
import PageFallback from '../components/PageFallback';
import StatCard from '../components/StatCard';
import { Card } from '../components/ui/Card';
import MarketTicker from '../components/MarketTicker';
import ExchangeRateBadge from '../components/ExchangeRateBadge';
import { fmt, formatNavDate } from '../services/format';
import { useQueryRange } from '../hooks/useQueryRange';
import { useToast } from '../components/feedback/Toast';
import DataFreshnessBar from '../components/DataFreshnessBar';
import { space, fontWeight, getTheme } from '../styles/theme';
import { sanitizeUserError } from '../services/userError';

const PortfolioChart = lazy(() => import('../components/PortfolioChart'));
const PortfolioAllocation = lazy(() => import('../components/PortfolioAllocation'));
const InvestmentHarnessPanel = lazy(() => import('../components/InvestmentHarnessPanel'));
const PortfolioPenetration = lazy(() => import('../components/PortfolioPenetration'));
const PnLDistributionChart = lazy(() => import('../components/PnLDistributionChart'));
const CorrelationHeatmap = lazy(() => import('../components/CorrelationHeatmap'));
const MonteCarloChart = lazy(() => import('../components/MonteCarloChart'));


function StatCardError({ label }: { label: string }) {
  const { t } = useTranslation();
  const dark = useAppStore((s) => s.dark);
  return (
    <Card dark={dark} glass padded={false}>
      <div style={{
        padding: `${space[4]}px ${space[5]}px`, minHeight: 80,
        display: 'flex', flexDirection: 'column', justifyContent: 'center',
      }}>
        <Text variant="secondary" as="span" size="xs">{label}</Text>
        <div style={{ marginTop: space[1] }}>
          <Text variant="secondary" as="span" size="xs" style={{ color: 'var(--fd-color-critical, var(--color-kumo-critical))' }}>{t('overview.loadError')}</Text>
        </div>
      </div>
    </Card>
  );
}


type OverviewTab = 'chart' | 'allocation' | 'harness' | 'penetration' | 'pnl_dist' | 'advanced';
const OVERVIEW_TABS = ['chart', 'allocation', 'harness', 'penetration', 'pnl_dist', 'advanced'] as const;

/** Core app query roots owned by App.tsx → Zustand; Overview only refetch/invalidate. */
const CORE_QUERY_ROOTS = ['funds', 'portfolio', 'securities', 'portfolioXirr', 'exchangeRate'] as const;

export default function OverviewPage() {
  const { t, i18n } = useTranslation();
  const portfolio = useAppStore((s) => s.portfolio);
  const portfolioXirr = useAppStore((s) => s.portfolioXirr);
  const exchangeRate = useAppStore((s) => s.exchangeRate);
  const dark = useAppStore((s) => s.dark);
  const portfolioId = useAppStore((s) => s.portfolioId);
  const theme = getTheme(dark);
  const toast = useToast();
  const queryClient = useQueryClient();
  // App.tsx already mounts useAppQueries; avoid dual subscription — only track fetch state.
  const isFetching = useIsFetching({
    predicate: (q) => CORE_QUERY_ROOTS.includes(q.queryKey[0] as typeof CORE_QUERY_ROOTS[number]),
  }) > 0;
  const [overviewTab, setOverviewTab] = useQueryRange('tab', 'chart', OVERVIEW_TABS);

  const handleRefresh = async () => {
    try {
      await Promise.all([
        queryClient.refetchQueries({ queryKey: ['funds', portfolioId] }),
        queryClient.refetchQueries({ queryKey: ['portfolio', portfolioId] }),
        queryClient.refetchQueries({ queryKey: ['securities', portfolioId] }),
        queryClient.refetchQueries({ queryKey: ['portfolioXirr', portfolioId] }),
        queryClient.refetchQueries({ queryKey: ['exchangeRate'] }),
      ]);
      toast.success(t('overview.refreshed'));
    } catch (e) {
      toast.error(t('overview.refreshFail', { message: sanitizeUserError(e, t('common.loadError')) }));
    }
  };

  if (!portfolio) {
    return <PageFallback />;
  }

  const invested = portfolio.invested_cost ?? 0;
  const currentValue = portfolio.current_value ?? 0;
  const pnlPct = portfolio.pnl_pct ?? (invested > 0 ? (portfolio.unrealized_pnl / invested) * 100 : 0);
  const heroValue = currentValue > 0 ? currentValue : Math.max(0, portfolio.total_buy - portfolio.total_sell);
  const pnlColor = portfolio.unrealized_pnl > 0 ? 'up' : portfolio.unrealized_pnl < 0 ? 'down' : undefined;
  const topGainer = portfolio.top_gainer ?? null;
  const topLoser = portfolio.top_loser ?? null;
  const staleDays = portfolio.stale_nav_days ?? null;

  return (
    <div>
      <Text variant="heading1" as="h1">{t('overview.title')}</Text>
      <div style={{ display: 'flex', alignItems: 'center', gap: space[3], flexWrap: 'wrap' }}>
        <MarketTicker />
        <ExchangeRateBadge exchangeRate={exchangeRate} />
        <DataFreshnessBar
          lastNavDate={portfolio.last_nav_date}
          staleDays={staleDays}
          isFetching={isFetching}
          onRefresh={() => { void handleRefresh(); }}
        />
      </div>
      <div style={{ marginTop: space[2], marginBottom: space[4] }}>
        <Text variant="secondary" as="span">{formatNavDate(portfolio.first_trade, i18n.language)} ~ {formatNavDate(portfolio.last_trade, i18n.language)} · {portfolio.total_tx} {t('tx.trades')} · {t('tx.auto')} {portfolio.auto_tx} / {t('tx.manual')} {portfolio.manual_tx}</Text>
        <div style={{ marginTop: space[1] }}>
          <Text variant="secondary" as="span" size="xs">{t('overview.heroHint')}</Text>
        </div>
      </div>

      {/* Hero KPI band — mark-to-market first (progressive disclosure of secondary stats below) */}
      <div className="fd-hero-grid">
        <div className="fd-hero-primary">
          <ErrorBoundary fallback={<StatCardError label={t('stat.currentValueHero')} />}>
            <StatCard
              emphasis
              accent={pnlColor === 'up' ? 'up' : pnlColor === 'down' ? 'down' : 'neutral'}
              label={t('stat.currentValueHero')}
              value={`¥ ${heroValue.toLocaleString(undefined, { maximumFractionDigits: 2 })}`}
              sub={invested > 0 ? `${t('stat.investedCost')} ¥ ${invested.toLocaleString(undefined, { maximumFractionDigits: 0 })}` : undefined}
            />
          </ErrorBoundary>
        </div>
        <ErrorBoundary fallback={<StatCardError label={t('stat.unrealizedPnl')} />}>
          <StatCard
            accent={pnlColor}
            label={t('stat.unrealizedPnl')}
            value={fmt(portfolio.unrealized_pnl)}
            color={pnlColor}
            sub={invested > 0 ? `${pnlPct >= 0 ? '+' : ''}${pnlPct.toFixed(2)}%` : undefined}
          />
        </ErrorBoundary>
        {portfolioXirr !== null ? (
          <ErrorBoundary fallback={<StatCardError label={t('stat.xirr')} />}>
            <StatCard
              accent={portfolioXirr > 0 ? 'up' : portfolioXirr < 0 ? 'down' : 'neutral'}
              label={t('stat.xirr')}
              value={`${portfolioXirr >= 0 ? '+' : ''}${portfolioXirr.toFixed(2)}%`}
              color={portfolioXirr > 0 ? 'up' : portfolioXirr < 0 ? 'down' : undefined}
              sub={`${t('stat.heldCount')} ${portfolio.held_funds}`}
            />
          </ErrorBoundary>
        ) : (
          <ErrorBoundary fallback={<StatCardError label={t('stat.heldCount')} />}>
            <StatCard
              accent="neutral"
              label={t('stat.heldCount')}
              value={String(portfolio.held_funds)}
              sub={`${t('stat.fee')} ¥ ${portfolio.total_fee.toFixed(2)}`}
            />
          </ErrorBoundary>
        )}
      </div>

      {/* Insight strip — top contributors only; freshness lives in DataFreshnessBar */}
      {(topGainer || topLoser) && (
        <div
          aria-label={t('stat.insightStrip')}
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            gap: space[2],
            marginBottom: space[5],
            alignItems: 'center',
          }}
        >
          {topGainer && (
            <span className="fd-insight-chip" style={{ borderColor: `${theme.up}44` }}>
              <TrendUp size={14} color={theme.up} aria-hidden />
              <span style={{ color: 'var(--text-color-kumo-subtle)' }}>{t('stat.topGainer')}</span>
              <strong className="fd-tabular-nums" style={{ fontWeight: fontWeight.semibold }}>{topGainer.name}</strong>
              <span className="fd-tabular-nums" style={{ color: theme.up }}>{fmt(topGainer.unrealized_pnl)}</span>
            </span>
          )}
          {topLoser ? (
            <span className="fd-insight-chip" style={{ borderColor: `${theme.down}44` }}>
              <TrendUp size={14} color={theme.down} style={{ transform: 'rotate(180deg)' }} aria-hidden />
              <span style={{ color: 'var(--text-color-kumo-subtle)' }}>{t('stat.topLoser')}</span>
              <strong className="fd-tabular-nums" style={{ fontWeight: fontWeight.semibold }}>{topLoser.name}</strong>
              <span className="fd-tabular-nums" style={{ color: theme.down }}>{fmt(topLoser.unrealized_pnl)}</span>
            </span>
          ) : (
            topGainer && (
              <span className="fd-insight-chip">
                <span style={{ color: 'var(--text-color-kumo-subtle)' }}>{t('overview.noLoser')}</span>
              </span>
            )
          )}
        </div>
      )}

      {/* Secondary stats — lower density, still scannable */}
      <Grid variant="4up" gap="base" style={{ marginBottom: space[5] }}>
        <ErrorBoundary fallback={<StatCardError label={t('stat.totalBuy')} />}>
          <StatCard label={t('stat.totalBuy')} value={`¥ ${portfolio.total_buy.toLocaleString()}`} />
        </ErrorBoundary>
        <ErrorBoundary fallback={<StatCardError label={t('stat.totalSell')} />}>
          <StatCard label={t('stat.totalSell')} value={`¥ ${portfolio.total_sell.toLocaleString()}`} />
        </ErrorBoundary>
        <ErrorBoundary fallback={<StatCardError label={t('stat.autoInvest')} />}>
          <StatCard label={t('stat.autoInvest')} value={`¥ ${portfolio.auto_amount.toLocaleString()}`} sub={`${portfolio.auto_tx} ${t('tx.trade')}`} />
        </ErrorBoundary>
        <ErrorBoundary fallback={<StatCardError label={t('stat.manualInvest')} />}>
          <StatCard label={t('stat.manualInvest')} value={`¥ ${portfolio.manual_amount.toLocaleString()}`} sub={`${portfolio.manual_tx} ${t('tx.trade')}`} />
        </ErrorBoundary>
      </Grid>
      <Suspense fallback={<ChartFallback />}>
        <ErrorBoundary><PortfolioChart dark={dark} portfolioId={portfolioId} /></ErrorBoundary>
      </Suspense>

      <div
        role="tablist"
        aria-label={t('overview.title')}
        className="fd-seg-tabs"
        onKeyDown={(e) => {
          const idx = OVERVIEW_TABS.indexOf(overviewTab);
          if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
            e.preventDefault();
            const dir = e.key === 'ArrowRight' ? 1 : -1;
            const next = OVERVIEW_TABS[(idx + dir + OVERVIEW_TABS.length) % OVERVIEW_TABS.length];
            setOverviewTab(next);
          } else if (e.key === 'Home') {
            e.preventDefault();
            setOverviewTab(OVERVIEW_TABS[0]);
          } else if (e.key === 'End') {
            e.preventDefault();
            setOverviewTab(OVERVIEW_TABS[OVERVIEW_TABS.length - 1]);
          }
        }}
      >
        <TabButton active={overviewTab === 'chart'} onClick={() => setOverviewTab('chart')} icon={<TrendUp size={16} aria-hidden />} label={t('overview.navTrend')} />
        <TabButton active={overviewTab === 'allocation'} onClick={() => setOverviewTab('allocation')} icon={<CurrencyDollar size={16} aria-hidden />} label={t('overview.allocation')} />
        <TabButton active={overviewTab === 'harness'} onClick={() => setOverviewTab('harness')} icon={<Clock size={16} aria-hidden />} label={t('overview.harness')} />
        <TabButton active={overviewTab === 'penetration'} onClick={() => setOverviewTab('penetration')} icon={<Binoculars size={16} aria-hidden />} label={t('overview.penetration')} />
        <TabButton active={overviewTab === 'pnl_dist'} onClick={() => setOverviewTab('pnl_dist')} icon={<TrendUp size={16} aria-hidden />} label={t('overview.pnlDist')} />
        <TabButton active={overviewTab === 'advanced'} onClick={() => setOverviewTab('advanced')} icon={<ChartBar size={16} aria-hidden />} label={t('overview.advanced')} />
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
            <div style={{ display: 'flex', flexDirection: 'column', gap: space[5] }}>
              <CorrelationHeatmap dark={dark} />
              <MonteCarloChart dark={dark} />
            </div>
          </ErrorBoundary>
        </Suspense>
      )}
    </div>
  );
}

/** Segmented glass tab for overview sections */
function TabButton({ active, onClick, icon, label }: { active: boolean; onClick: () => void; icon: ReactNode; label: string }) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={active}
      className="fd-seg-tab"
      tabIndex={active ? 0 : -1}
      onClick={onClick}
    >
      {icon}<span>{label}</span>
    </button>
  );
}
