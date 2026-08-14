import { Suspense, lazy } from 'react';
import { useTranslation } from 'react-i18next';
import { Text } from '@cloudflare/kumo';
import { ErrorBoundary } from '../components/ErrorBoundary';
import ChartFallback from '../components/ChartFallback';

const AdminDashboard = lazy(() => import('../components/AdminDashboard'));


export default function AdminPage() {
  return (
    <Suspense fallback={<ChartFallback />}>
      <ErrorBoundary><AdminDashboard /></ErrorBoundary>
    </Suspense>
  );
}
