import { Suspense, lazy, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { Text } from '@cloudflare/kumo';
import { useAppStore } from '../stores/appStore';
import { ErrorBoundary } from '../components/ErrorBoundary';
import ChartFallback from '../components/ChartFallback';
import { space } from '../styles/theme'
import MarketTicker from '../components/MarketTicker';
import ExchangeRateBadge from '../components/ExchangeRateBadge';
import { isNasdaqFundName } from '../services/classify';

const NasdaqOverview = lazy(() => import('../components/NasdaqOverview'));



export default function NasdaqOverviewPage() {
  const navigate = useNavigate();
  const funds = useAppStore((s) => s.funds);
  const exchangeRate = useAppStore((s) => s.exchangeRate);
  const dark = useAppStore((s) => s.dark);

  const nasdaqFunds = useMemo(() => {
    const g: Record<string, typeof funds> = {};
    for (const f of funds) {
      const isNasdaq = isNasdaqFundName(f.name);
      const cat = isNasdaq ? 'nasdaq' : 'other';
      if (!g[cat]) g[cat] = [];
      g[cat].push(f);
    }
    return g['nasdaq'] || [];
  }, [funds]);

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: space[3], flexWrap: 'wrap', marginBottom: space[2] }}>
        <MarketTicker />
        <ExchangeRateBadge exchangeRate={exchangeRate} />
      </div>
      <Suspense fallback={<ChartFallback />}>
        <ErrorBoundary><NasdaqOverview nasdaqFunds={nasdaqFunds} onSelect={(code) => navigate(`/fund/${encodeURIComponent(code)}`)} dark={dark} /></ErrorBoundary>
      </Suspense>
    </div>
  );
}
