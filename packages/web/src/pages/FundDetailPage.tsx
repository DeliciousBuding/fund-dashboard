import { Suspense, lazy } from 'react';
import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Text } from '@cloudflare/kumo';
import { useAppStore } from '../stores/appStore';
import { ErrorBoundary } from '../components/ErrorBoundary';
import ChartFallback from '../components/ChartFallback';
import { space, radius} from '../styles/theme';
import MarketTicker from '../components/MarketTicker';
import ExchangeRateBadge from '../components/ExchangeRateBadge';

const FundDetailView = lazy(() => import('../components/FundDetailView'));



export default function FundDetailPage() {
  const { code } = useParams<{ code: string }>();
  const { t } = useTranslation();
  const exchangeRate = useAppStore((s) => s.exchangeRate);
  const dark = useAppStore((s) => s.dark);

  if (!code) {
    return <div style={{ padding: space[8], textAlign: 'center' }}><Text variant="secondary" as="span">{t('fundDetail.noCode')}</Text></div>;
  }

  return (
    <Suspense fallback={<ChartFallback />}>
      <div style={{ display: 'flex', alignItems: 'center', gap: space[3], flexWrap: 'wrap', marginBottom: 0 }}>
        <MarketTicker />
        <ExchangeRateBadge exchangeRate={exchangeRate} />
      </div>
      <ErrorBoundary><FundDetailView code={code} dark={dark} /></ErrorBoundary>
    </Suspense>
  );
}
