import { Suspense, lazy } from 'react';
import { useTranslation } from 'react-i18next';
import { Text } from '@cloudflare/kumo';
import { useAppStore } from '../stores/appStore';
import { ErrorBoundary } from '../components/ErrorBoundary';
import ChartFallback from '../components/ChartFallback';
import { space } from '../styles/theme'
import MarketTicker from '../components/MarketTicker';
import ExchangeRateBadge from '../components/ExchangeRateBadge';

const FundComparison = lazy(() => import('../components/FundComparison'));



export default function ComparePage() {
  const funds = useAppStore((s) => s.funds);
  const exchangeRate = useAppStore((s) => s.exchangeRate);
  const dark = useAppStore((s) => s.dark);

  return (
    <Suspense fallback={<ChartFallback />}>
      <div style={{ display: 'flex', alignItems: 'center', gap: space[3], flexWrap: 'wrap', marginBottom: space[2] }}>
        <MarketTicker />
        <ExchangeRateBadge exchangeRate={exchangeRate} />
      </div>
      <ErrorBoundary><FundComparison funds={funds} dark={dark} /></ErrorBoundary>
    </Suspense>
  );
}
