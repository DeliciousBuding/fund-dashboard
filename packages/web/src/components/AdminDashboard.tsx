/** AdminDashboard — 系统监控面板 (dev only, lazy loaded)
 *
 *  从 GET /api/ops/dashboard 获取聚合指标:
 *  DB 大小 · 爬虫成功率 · API 延迟 · 内存 · uptime
 *
 *  Browser path: EdgeAuth via nginx-injected X-Fund-Edge-Key (not OIDC).
 *  Operator path remains GET /api/admin/dashboard with Bearer MCP_API_KEY.
 *  SPA never holds MCP_API_KEY.
 */

import { useState, useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Text, Grid, Loader, Button, Table } from '@cloudflare/kumo';
import StatCard from './StatCard';
import { ErrorBoundary } from './ErrorBoundary';
import { Card } from './ui/Card';
import { space, fontSize } from '../styles/theme';
import { useAppStore } from '../stores/appStore';
import { sanitizeUserError } from '../services/userError'

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
    /** Go runtime version. */
    go_version?: string;
    /** Release pin / FUND_VERSION (auth-gated ops dashboard only). */
    build_version?: string;
    platform: string;
  };
  database: {
    size_bytes: number;
    size_mb: number;
  };
  crawler: {
    nav_total: number;
    /** Fresh NAV fund_code count within server fresh_window_days (legacy name kept). */
    nav_fresh_24h: number;
    nav_fresh?: number;
    success_rate_pct: number;
    fresh_window_days?: number;
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
  // Browser path: EdgeAuth via nginx-injected key, not MCP_API_KEY.
  // Operator path remains /api/admin/dashboard with Bearer.
  const res = await fetch('/api/ops/dashboard', { signal });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

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
          <Text variant="secondary" as="span" size="xs" style={{ color: 'var(--fd-color-critical, var(--color-kumo-critical))' }}>{t('admin.loadFailed')}</Text>
        </div>
      </div>
    </Card>
  );
}

export default function AdminDashboard() {
  const { t } = useTranslation();
  const dark = useAppStore((s) => s.dark);
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  // Track the in-flight AbortController so each (re)load aborts the previous
  // one, and so unmount cleans it up. Without this, the retry button leaks a
  // new controller (and a concurrent fetch) on every click.
  const abortRef = useRef<AbortController | null>(null);

  const load = () => {
    // Abort any in-flight request before starting a new one.
    abortRef.current?.abort();
    setLoading(true);
    setError('');
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    fetchDashboard(ctrl.signal)
      .then(d => {
        setData(d);
        setLoading(false);
      })
      .catch(e => {
        if (e?.name !== 'AbortError') {
          setError(sanitizeUserError(e, t('admin.loadFailed')));
          setLoading(false);
        }
      });
  };

  useEffect(() => {
    load();
    return () => {
      abortRef.current?.abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (loading) {
    return <div style={{ padding: space[8], textAlign: 'center' }}><Loader /><div style={{ marginTop: space[3] }}><Text variant="secondary" as="span">{t('admin.loading')}</Text></div></div>;
  }

  if (error || !data) {
    return (
      <div style={{ padding: space[8], textAlign: 'center' }}>
        <Text variant="body" as="span" style={{ display: 'block', fontSize: fontSize.xl, color: 'var(--fd-color-critical, var(--color-kumo-critical))', marginBottom: 16 }}>{t('admin.loadError', { error })}</Text>
        <Button type="button" variant="primary" onClick={load} aria-label={t('admin.retry')}>{t('admin.retry')}</Button>
      </div>
    );
  }

  const { system, database, crawler, state, response_ms, timestamp } = data;

  return (
    <div>
      <Text variant="heading1" as="h1">{t('admin.title')}</Text>
      <Text variant="secondary" as="span" style={{ display: 'block', marginBottom: space[5] }}>
        {t('admin.updatedAt', { timestamp: new Date(timestamp).toLocaleString(), latency: response_ms })}
      </Text>

      {/* System */}
      <section aria-label={t('admin.system')}>
        <Text variant="heading3" as="h3" style={{ marginBottom: space[3], marginTop: space[2] }}>{t('admin.system')}</Text>
        <Grid variant="4up" gap="base" style={{ marginBottom: space[5] }}>
          <ErrorBoundary fallback={<StatCardError label={t('admin.uptimeLabel')} />}>
            <StatCard label={t('admin.uptimeLabel')} value={system.uptime_human} sub={`${system.uptime_sec}s`} />
          </ErrorBoundary>
          <ErrorBoundary fallback={<StatCardError label={t('admin.memoryRss')} />}>
            <StatCard label={t('admin.memoryRss')} value={`${system.memory.rss_mb} MB`}
              sub={t('admin.heapSub', { used: system.memory.heap_used_mb, total: system.memory.heap_total_mb })} />
          </ErrorBoundary>
          <ErrorBoundary fallback={<StatCardError label={t('admin.platform')} />}>
            <StatCard label={t('admin.platform')} value={system.platform}
              sub={[system.build_version && `build ${system.build_version}`, system.go_version && `Go ${system.go_version}`].filter(Boolean).join(' · ')} />
          </ErrorBoundary>
          <ErrorBoundary fallback={<StatCardError label={t('admin.apiLatency')} />}>
            <StatCard label={t('admin.apiLatency')} value={`${response_ms} ms`} />
          </ErrorBoundary>
        </Grid>
      </section>

      {/* Database */}
      <section aria-label={t('admin.database')}>
        <Text variant="heading3" as="h3" style={{ marginBottom: space[3] }}>{t('admin.database')}</Text>
        <Grid variant="4up" gap="base" style={{ marginBottom: space[5] }}>
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
      </section>

      {/* Crawler */}
      <section aria-label={t('admin.crawler')}>
        <Text variant="heading3" as="h3" style={{ marginBottom: space[3] }}>{t('admin.crawler')}</Text>
        <Grid variant="4up" gap="base" style={{ marginBottom: space[5] }}>
          <ErrorBoundary fallback={<StatCardError label={t('admin.crawlerSuccessRate')} />}>
            <StatCard label={t('admin.crawlerSuccessRate')} value={`${crawler.success_rate_pct}%`}
              color={crawler.success_rate_pct >= 80 ? 'up' : crawler.success_rate_pct >= 50 ? undefined : 'down'} />
          </ErrorBoundary>
          <ErrorBoundary fallback={<StatCardError label={t('admin.navTotal')} />}>
            <StatCard label={t('admin.navTotal')} value={`${crawler.nav_total}`}
              sub={t('admin.freshWindow', {
                count: crawler.nav_fresh ?? crawler.nav_fresh_24h,
                days: crawler.fresh_window_days ?? 4,
              })} />
          </ErrorBoundary>
          <ErrorBoundary fallback={<StatCardError label={t('admin.anomalyCount')} />}>
            <StatCard label={t('admin.anomalyCount')} value={`${state.anomaly_count}`}
              color={state.anomaly_count > 0 ? 'down' : 'up'} />
          </ErrorBoundary>
        </Grid>
      </section>

      {/* Anomalies detail */}
      {state.recent_anomalies.length > 0 && (
        <section aria-label={t('admin.recentAnomalies')}>
          <Card dark={dark} glass padded={false} style={{ marginTop: space[2] }}>
            <div style={{ padding: space[4] }}>
              <Text variant="heading3" as="h4" style={{ marginBottom: space[2] }}>{t('admin.recentAnomalies')}</Text>
              <Table aria-label={t('admin.recentAnomalies')}>
                <caption style={{ position: 'absolute', width: 1, height: 1, padding: 0, margin: -1, overflow: 'hidden', clip: 'rect(0, 0, 0, 0)', whiteSpace: 'nowrap', border: 0 }}>
                  {t('admin.recentAnomalies')}
                </caption>
                <Table.Header>
                  <Table.Row>
                    <Table.Head>{t('admin.seq')}</Table.Head>
                    <Table.Head>{t('admin.fundCode')}</Table.Head>
                    <Table.Head>{t('admin.anomaly')}</Table.Head>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {state.recent_anomalies.map(a => (
                    <Table.Row key={a.seq}>
                      <Table.Cell className="fd-tabular-nums">{a.seq}</Table.Cell>
                      <Table.Cell translate="no">{a.fund_code}</Table.Cell>
                      <Table.Cell style={{ color: 'var(--fd-color-critical, var(--color-kumo-critical))' }}>{a.anomaly}</Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table>
            </div>
          </Card>
        </section>
      )}
    </div>
  );
}
