/** AdminDashboard — 系统监控面板 (dev only, lazy loaded)
 *
 *  从 GET /api/dashboard 获取聚合指标:
 *  DB 大小 · 爬虫成功率 · API 延迟 · 内存 · uptime
 *
 *  v2.4 — 2026-06-19
 */

import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Text, Grid, Loader, Button } from '@cloudflare/kumo';
import StatCard from './StatCard';
import { ErrorBoundary } from './ErrorBoundary';

interface DashboardData {
  ok: boolean;
  timestamp: string;
  response_ms: number;
  system: {
    uptime_sec: number;
    uptime_human: string;
    memory: {
      rss_mb: number;
      heap_used_mb: number;
      heap_total_mb: number;
    };
    node_version: string;
    platform: string;
  };
  database: {
    size_bytes: number;
    size_mb: number;
  };
  crawler: {
    nav_total: number;
    nav_fresh_24h: number;
    success_rate_pct: number;
  };
  state: {
    transaction_count: number;
    last_transaction: string | null;
    last_nav_date: string | null;
    held_funds: number;
    nav_records: number;
    nav_funds: number;
    securities_total: number;
    anomaly_count: number;
    recent_anomalies: Array<{ seq: number; fund_code: string; anomaly: string }>;
  };
}

async function fetchDashboard(signal?: AbortSignal): Promise<DashboardData> {
  const res = await fetch('/api/dashboard', { signal });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

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
        <Text variant="secondary" as="span" size="xs" style={{ color: '#d63649' }}>{t('admin.loadFailed')}</Text>
      </div>
    </div>
  );
}

export default function AdminDashboard() {
  const { t } = useTranslation();
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  const load = () => {
    setLoading(true);
    setError('');
    const ctrl = new AbortController();
    fetchDashboard(ctrl.signal)
      .then(d => { setData(d); setLoading(false); })
      .catch(e => {
        if (e.name !== 'AbortError') { setError(e.message || t('admin.loadFailed')); setLoading(false); }
      });
    return () => ctrl.abort();
  };

  useEffect(() => {
    return load();
  }, []);

  if (loading) {
    return <div style={{ padding: 60, textAlign: 'center' }}><Loader /><div style={{ marginTop: 12 }}><Text variant="secondary" as="span">{t('admin.loading')}</Text></div></div>;
  }

  if (error || !data) {
    return (
      <div style={{ padding: 60, textAlign: 'center' }}>
        <Text variant="body" as="span" style={{ display: 'block', fontSize: 16, color: '#d63649', marginBottom: 16 }}>{t('admin.loadError', { error })}</Text>
        <Button variant="primary" onClick={load}>{t('admin.retry')}</Button>
      </div>
    );
  }

  const { system, database, crawler, state, response_ms, timestamp } = data;

  return (
    <div>
      <Text variant="heading1" as="h1">{t('admin.title')}</Text>
      <Text variant="secondary" as="span" style={{ display: 'block', marginBottom: 20 }}>
        {t('admin.updatedAt', { timestamp: new Date(timestamp).toLocaleString(), latency: response_ms })}
      </Text>

      {/* System */}
      <Text variant="heading3" as="h3" style={{ marginBottom: 12, marginTop: 8 }}>{t('admin.system')}</Text>
      <Grid variant="4up" gap="base" style={{ marginBottom: 20 }}>
        <ErrorBoundary fallback={<StatCardError label={t('admin.uptimeLabel')} />}>
          <StatCard label={t('admin.uptimeLabel')} value={system.uptime_human} sub={`${system.uptime_sec}s`} />
        </ErrorBoundary>
        <ErrorBoundary fallback={<StatCardError label={t('admin.memoryRss')} />}>
          <StatCard label={t('admin.memoryRss')} value={`${system.memory.rss_mb} MB`}
            sub={t('admin.heapSub', { used: system.memory.heap_used_mb, total: system.memory.heap_total_mb })} />
        </ErrorBoundary>
        <ErrorBoundary fallback={<StatCardError label={t('admin.platform')} />}>
          <StatCard label={t('admin.platform')} value={system.platform}
            sub={`Node ${system.node_version}`} />
        </ErrorBoundary>
        <ErrorBoundary fallback={<StatCardError label={t('admin.apiLatency')} />}>
          <StatCard label={t('admin.apiLatency')} value={`${response_ms} ms`} />
        </ErrorBoundary>
      </Grid>

      {/* Database */}
      <Text variant="heading3" as="h3" style={{ marginBottom: 12 }}>{t('admin.database')}</Text>
      <Grid variant="4up" gap="base" style={{ marginBottom: 20 }}>
        <ErrorBoundary fallback={<StatCardError label={t('admin.dbSizeLabel')} />}>
          <StatCard label={t('admin.dbSizeLabel')} value={`${database.size_mb} MB`}
            sub={`${(database.size_bytes / 1024).toFixed(1)} KB`} />
        </ErrorBoundary>
        <ErrorBoundary fallback={<StatCardError label={t('admin.transactionCount')} />}>
          <StatCard label={t('admin.transactionCount')} value={`${state.transaction_count}`}
            sub={state.last_transaction ? t('admin.lastTxSub', { date: state.last_transaction.substring(0, 16) }) : t('admin.noTx')} />
        </ErrorBoundary>
        <ErrorBoundary fallback={<StatCardError label={t('admin.navRecords')} />}>
          <StatCard label={t('admin.navRecords')} value={`${state.nav_records}`}
            sub={t('admin.navFundCount', { count: state.nav_funds }) + (state.last_nav_date ? t('admin.lastNavSub', { date: state.last_nav_date }) : '')} />
        </ErrorBoundary>
        <ErrorBoundary fallback={<StatCardError label={t('admin.securitiesTotal')} />}>
          <StatCard label={t('admin.securitiesTotal')} value={`${state.securities_total}`}
            sub={t('admin.heldFundCount', { count: state.held_funds })} />
        </ErrorBoundary>
      </Grid>

      {/* Crawler */}
      <Text variant="heading3" as="h3" style={{ marginBottom: 12 }}>{t('admin.crawler')}</Text>
      <Grid variant="4up" gap="base" style={{ marginBottom: 20 }}>
        <ErrorBoundary fallback={<StatCardError label={t('admin.crawlerSuccessRate')} />}>
          <StatCard label={t('admin.crawlerSuccessRate')} value={`${crawler.success_rate_pct}%`}
            color={crawler.success_rate_pct >= 80 ? 'up' : crawler.success_rate_pct >= 50 ? undefined : 'down'} />
        </ErrorBoundary>
        <ErrorBoundary fallback={<StatCardError label={t('admin.navTotal')} />}>
          <StatCard label={t('admin.navTotal')} value={`${crawler.nav_total}`}
            sub={t('admin.fresh24h', { count: crawler.nav_fresh_24h })} />
        </ErrorBoundary>
        <ErrorBoundary fallback={<StatCardError label={t('admin.anomalyCount')} />}>
          <StatCard label={t('admin.anomalyCount')} value={`${state.anomaly_count}`}
            color={state.anomaly_count > 0 ? 'down' : 'up'} />
        </ErrorBoundary>
      </Grid>

      {/* Anomalies detail */}
      {state.recent_anomalies.length > 0 && (
        <div style={{
          marginTop: 8, padding: 16, borderRadius: 8,
          background: 'var(--color-kumo-surface)',
          border: '1px solid var(--color-kumo-border)',
        }}>
          <Text variant="heading3" as="h4" style={{ marginBottom: 8 }}>{t('admin.recentAnomalies')}</Text>
          <table style={{ width: '100%', fontSize: 13, borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--color-kumo-border)' }}>
                <th style={{ textAlign: 'left', padding: '4px 8px', color: 'var(--text-color-kumo-subtle)' }}>{t('admin.seq')}</th>
                <th style={{ textAlign: 'left', padding: '4px 8px', color: 'var(--text-color-kumo-subtle)' }}>{t('admin.fundCode')}</th>
                <th style={{ textAlign: 'left', padding: '4px 8px', color: 'var(--text-color-kumo-subtle)' }}>{t('admin.anomaly')}</th>
              </tr>
            </thead>
            <tbody>
              {state.recent_anomalies.map(a => (
                <tr key={a.seq} style={{ borderBottom: '1px solid var(--color-kumo-border)' }}>
                  <td style={{ padding: '4px 8px' }}>{a.seq}</td>
                  <td style={{ padding: '4px 8px' }}>{a.fund_code}</td>
                  <td style={{ padding: '4px 8px', color: '#d63649' }}>{a.anomaly}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
