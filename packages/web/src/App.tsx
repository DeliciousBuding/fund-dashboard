import "@cloudflare/kumo/styles/standalone";
import { useState, useEffect, Suspense, lazy } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useLocation } from 'react-router-dom'
import { Text, Button } from '@cloudflare/kumo'
import { Sun, Moon, CaretLeft, House } from '@phosphor-icons/react'
import { ErrorBoundary } from './components/ErrorBoundary'
import OfflineBanner from './components/OfflineBanner'
import PageFallback from './components/PageFallback'
import { useDarkMode } from './hooks/useDarkMode'
import { usePortfolioDeepLink } from './hooks/usePortfolioDeepLink'
import AppSidebar from './components/layout/Sidebar'
import AppLayout from './components/layout/AppLayout'
import { useAppStore } from './stores/appStore'
import { useAppQueries } from './hooks/queries'
import { getTheme, glassSurfaceStyle, space, radius, fontSize, fontWeight, zIndex, layout, hitTarget } from './styles/theme'

// Lazy-loaded page components
const OverviewPage = lazy(() => import('./pages/OverviewPage'));
const ComparePage = lazy(() => import('./pages/ComparePage'));
const NasdaqOverviewPage = lazy(() => import('./pages/NasdaqOverviewPage'));
const FundDetailPage = lazy(() => import('./pages/FundDetailPage'));
const AdminPage = lazy(() => import('./pages/AdminPage'));

// Keep route-based imports for Routes
import { Routes, Route } from 'react-router-dom';
import { sanitizeUserError } from './services/userError'
import { formatNavDate } from './services/format'

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
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();

  // Zustand store — read state with selectors
  const funds = useAppStore((s) => s.funds);
  const securities = useAppStore((s) => s.securities);
  const portfolio = useAppStore((s) => s.portfolio);
  const portfolioId = useAppStore((s) => s.portfolioId);
  const sidebarOpen = useAppStore((s) => s.sidebarOpen);
  const sidebarSearch = useAppStore((s) => s.sidebarSearch);
  const heldOnly = useAppStore((s) => s.heldOnly);
  const loadError = useAppStore((s) => s.loadError);
  const dark = useAppStore((s) => s.dark);

  // Store actions
  const setFunds = useAppStore((s) => s.setFunds);
  const setPortfolio = useAppStore((s) => s.setPortfolio);
  const setSecurities = useAppStore((s) => s.setSecurities);
  const setLoadError = useAppStore((s) => s.setLoadError);
  const setPortfolioXirr = useAppStore((s) => s.setPortfolioXirr);
  const setExchangeRate = useAppStore((s) => s.setExchangeRate);
  const setPortfolioId = useAppStore((s) => s.setPortfolioId);
  const setSidebarOpen = useAppStore((s) => s.setSidebarOpen);
  const setSidebarSearch = useAppStore((s) => s.setSidebarSearch);
  const toggleHeldOnly = useAppStore((s) => s.toggleHeldOnly);
  const resetForRetry = useAppStore((s) => s.resetForRetry);
  const setDark = useAppStore((s) => s.setDark);

  const isMobile = useIsMobile();
  // useDarkMode owns localStorage + data-theme; mirror into Zustand for consumers.
  const { dark: hookDark, toggle: toggleDark } = useDarkMode()
  const theme = getTheme(dark);
  const { handlePortfolioChange } = usePortfolioDeepLink(portfolioId, setPortfolioId);

  useEffect(() => {
    setDark(hookDark);
  }, [hookDark, setDark]);

  // TanStack Query — handles data fetching, caching, retry, and AbortController
  const { funds: fundsQ, portfolio: portfolioQ, securities: securitiesQ, xirr, fx, isLoading, error: queryError } = useAppQueries(portfolioId);

  // Sync query results to Zustand store
  useEffect(() => {
    if (fundsQ.data) setFunds(fundsQ.data);
  }, [fundsQ.data, setFunds]);

  useEffect(() => {
    if (portfolioQ.data) setPortfolio(portfolioQ.data);
  }, [portfolioQ.data, setPortfolio]);

  useEffect(() => {
    if (securitiesQ.data) setSecurities(securitiesQ.data);
  }, [securitiesQ.data, setSecurities]);

  useEffect(() => {
    if (xirr.data != null) setPortfolioXirr(xirr.data.xirr);
  }, [xirr.data, setPortfolioXirr]);

  useEffect(() => {
    if (fx.data) setExchangeRate(fx.data);
  }, [fx.data, setExchangeRate]);

  // Sync query errors to store loadError
  useEffect(() => {
    if (queryError) {
      setLoadError(sanitizeUserError(queryError, t('common.loadError')));
    } else {
      setLoadError('');
    }
  }, [queryError, setLoadError]);

  // Auto-collapse sidebar on mobile
  useEffect(() => {
    if (isMobile) setSidebarOpen(false);
  }, [isMobile, setSidebarOpen]);

  const retryLoad = () => {
    resetForRetry();
    fundsQ.refetch();
    portfolioQ.refetch();
    securitiesQ.refetch();
  };

  const sidebar = (!isMobile || sidebarOpen) ? (
    <AppSidebar
      funds={funds}
      securities={securities}
      heldOnly={heldOnly}
      onHeldToggle={toggleHeldOnly}
      searchQuery={sidebarSearch}
      onSearchChange={setSidebarSearch}
      dark={dark}
      onToggleDark={toggleDark}
      portfolio={portfolio}
      portfolioId={portfolioId}
      onPortfolioChange={handlePortfolioChange}
    />
  ) : null;

  return (
    <AppLayout
      sidebar={sidebar}
      contentPaddingBottom={isMobile ? `calc(${layout.mobileNavHeight}px + env(safe-area-inset-bottom, 0px) + 16px)` : undefined}
    >
      <a href="#main-content" className="fd-skip-link">
        {t('nav.skipToContent')}
      </a>
      <OfflineBanner />
      {isMobile && (
        <div style={{ marginBottom: space[3], display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Button type="button" variant="secondary" size="sm" onClick={() => setSidebarOpen(!sidebarOpen)} aria-label={sidebarOpen ? t('mobile.closeMenu') : t('mobile.openMenu')}>
            {sidebarOpen ? t('mobile.closeMenu') : t('mobile.menu')}
          </Button>
          <div style={{ display: 'flex', alignItems: 'center', gap: space[2] }}>
            {portfolio?.last_nav_date && (
              <span style={{ fontSize: fontSize.sm, color: 'var(--text-color-kumo-subtle)', display: 'flex', alignItems: 'center', gap: space[1] }} className="fd-tabular-nums">
                {formatNavDate(portfolio.last_nav_date, i18n.language)}
              </span>
            )}
            <Button type="button" variant="secondary" size="sm" onClick={toggleDark} aria-label={dark ? t('nav.lightMode') : t('nav.darkMode')} style={{ padding: space[2] - 2, minWidth: 32 }}>
              {dark ? <Sun size={16} weight="bold" /> : <Moon size={16} weight="bold" />}
            </Button>
          </div>
        </div>
      )}

      {loadError ? (
        <div style={{ padding: space[8], textAlign: 'center' }}>
          <Text variant="body" as="span" style={{ display: 'block', fontSize: fontSize.xl, color: 'var(--fd-color-critical, var(--color-kumo-critical))', marginBottom: space[4] }}>{t('overview.loadErrorMsg', { error: loadError })}</Text>
          <Button type="button" variant="primary" onClick={retryLoad}>{t('overview.retry')}</Button>
        </div>
      ) : isLoading ? (
        <PageFallback />
      ) : (
        <ErrorBoundary>
          <Suspense fallback={<PageFallback />}>
            <Routes>
              <Route path="/" element={<OverviewPage />} />
              <Route path="/compare" element={<ComparePage />} />
              <Route path="/nasdaq" element={<NasdaqOverviewPage />} />
              <Route path="/fund/:code" element={<FundDetailPage />} />
              {import.meta.env.DEV && (
                <Route path="/admin" element={<AdminPage />} />
              )}
            </Routes>
          </Suspense>
        </ErrorBoundary>
      )}

      {/* Mobile bottom nav */}
      {isMobile && (
        <nav aria-label={t('mobile.nav')} style={{
          position: 'fixed', bottom: 0, left: 0, right: 0, height: layout.mobileNavHeight,
          ...glassSurfaceStyle(theme),
          borderTop: `1px solid ${theme.border}`,
          borderLeft: 'none', borderRight: 'none', borderBottom: 'none',
          borderRadius: radius.none,
          display: 'flex', alignItems: 'center', justifyContent: 'space-around',
          zIndex: zIndex.modal, padding: `0 ${space[2]}px`,
          paddingBottom: 'env(safe-area-inset-bottom, 0px)',
        }}>
          <Button type="button" variant="secondary" size="sm" onClick={() => navigate('/')}
            style={{ flexDirection: 'column', gap: space[1]/2, height: hitTarget.mobile, fontSize: fontSize.xs, fontWeight: location.pathname === '/' ? fontWeight.semibold : fontWeight.regular }}>
            <House size={22} weight={location.pathname === '/' ? 'fill' : 'regular'} />{t('mobile.overview')}
          </Button>
          <Button type="button" variant="secondary" size="sm" onClick={() => navigate('/nasdaq')}
            style={{ flexDirection: 'column', gap: space[1]/2, height: hitTarget.mobile, fontSize: fontSize.xs, fontWeight: location.pathname === '/nasdaq' ? fontWeight.semibold : fontWeight.regular }}>
            <img src={dark ? "/ndaq-d.svg" : "/ndaq.svg"} width={22} height={22} style={{ borderRadius: radius.xs }} alt="" aria-hidden />{t('mobile.nasdaq')}
          </Button>
          {location.pathname !== '/' && location.pathname !== '/nasdaq' && (
            <Button type="button" variant="secondary" size="sm" onClick={() => navigate(-1)}
              style={{ flexDirection: 'column', gap: space[1]/2, height: hitTarget.mobile, fontSize: fontSize.xs }}>
              <CaretLeft size={22} />{t('mobile.back')}
            </Button>
          )}
        </nav>
      )}
    </AppLayout>
  );
}
