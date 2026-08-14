import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Grid } from '@cloudflare/kumo'
import { useAppStore } from '../stores/appStore'
import { getTheme, space, radius, fontSize, fontWeight, opacity } from '../styles/theme'
import { Card } from './ui/Card'
import { fetchInvestmentHarness, fetchInvestmentSourceBrief, fetchSourceEvents, markSourceEventApi, type InvestmentHarnessSnapshot, type InvestmentSourceBrief, type SourceEvent } from '../api'
import { sanitizeUserError } from '../services/userError'
import ChartFallback from './ChartFallback'

function tagLabel(tag: string, t: (key: string) => string): string {
  const labels: Record<string, string> = {
    price_drop_gt_5pct: 'harness.signals.price_drop_gt_5pct',
    price_rally_gt_5pct: 'harness.signals.price_rally_gt_5pct',
    price_range_bound: 'harness.signals.price_range_bound',
    below_cost_gt_10pct: 'harness.signals.below_cost_gt_10pct',
    above_cost_gt_10pct: 'harness.signals.above_cost_gt_10pct',
    near_cost_basis: 'harness.signals.near_cost_basis',
  };
  return t(labels[tag] || tag);
}

/**
 * Soft-scope source events to current holdings.
 * Limits (until source_events.portfolio_id exists — deferred):
 * - events without related_security_code pass through (global noise)
 * - code match only; same code across portfolios is not distinguished
 * - backend ignores portfolio_id query today; FE still sends it for future cutover
 */
function filterEventsForHoldings(events: SourceEvent[], heldCodes: Set<string>): SourceEvent[] {
  if (!heldCodes.size) return events;
  return events.filter((ev) => {
    const code = ev.related_security_code?.trim();
    if (!code) return true;
    return heldCodes.has(code);
  });
}

export default function InvestmentHarnessPanel() {
  const { t } = useTranslation();
  const dark = useAppStore((s) => s.dark);
  const portfolioId = useAppStore((s) => s.portfolioId);
  const theme = getTheme(dark);
  const [snapshot, setSnapshot] = useState<InvestmentHarnessSnapshot | null>(null);
  const [sourceBrief, setSourceBrief] = useState<InvestmentSourceBrief | null>(null);
  const [sourceEvents, setSourceEvents] = useState<SourceEvent[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    const ctrl = new AbortController();
    setError('');
    // Isolate failures: harness is required; brief/events degrade to empty.
    Promise.allSettled([
      fetchInvestmentHarness(portfolioId, ctrl.signal),
      fetchInvestmentSourceBrief(8, portfolioId, ctrl.signal),
      fetchSourceEvents({ limit: 20, portfolioId }, ctrl.signal),
    ])
      .then(([harnessR, sourcesR, eventsR]) => {
        if (ctrl.signal.aborted) return;
        if (harnessR.status === 'rejected') {
          const e = harnessR.reason as { name?: string } | undefined;
          if (e?.name !== 'AbortError') setError(sanitizeUserError(e, t('harness.error')));
          setSnapshot(null);
          setSourceBrief(null);
          setSourceEvents([]);
          return;
        }
        const harness = harnessR.value;
        setSnapshot(harness);
        const held = new Set((harness.holding_signals || []).map((h) => h.code));
        if (sourcesR.status === 'fulfilled') setSourceBrief(sourcesR.value);
        else setSourceBrief(null);
        if (eventsR.status === 'fulfilled') {
          setSourceEvents(filterEventsForHoldings(eventsR.value.events || [], held));
        } else {
          setSourceEvents([]);
        }
      });
    return () => ctrl.abort();
  }, [portfolioId, t]);

  const handleMarkRead = (id: number) => {
    markSourceEventApi(id, { is_read: true }).then(() => {
      setSourceEvents(prev => prev.map(e => e.id === id ? { ...e, is_read: true } : e));
    }).catch((e: any) => { if (e?.name !== 'AbortError') console.warn('[harness/markRead]', e); });
  };

  return (
    <Card dark={dark} glass padded={false}>
      <div style={{ padding: space[5] }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: space[3], alignItems: 'baseline', flexWrap: 'wrap' }}>
          <div>
            <Text variant="heading2" as="h2">{t('harness.title')}</Text>
            <Text variant="secondary" as="span" size="sm">{t('harness.subtitle')}</Text>
          </div>
          {snapshot && <Text variant="secondary" as="span">{t('harness.factsOnly')} · {snapshot.holdings_count} {t('tx.trades')}</Text>}
        </div>

        {error && <div style={{ marginTop: space[3], color: theme.critical, fontSize: fontSize.md }}>{error}</div>}
        {!snapshot && !error && <div style={{ marginTop: space[3] }}><ChartFallback /></div>}

        {snapshot && (
          <>
            <Grid variant="3up" gap="base" style={{ marginTop: space[4] }}>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('harness.totalValue')}</Text>
                <div style={{ marginTop: space[1] / 2, fontWeight: fontWeight.bold }}>¥ {snapshot.total_value.toLocaleString()}</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('harness.dataGaps')}</Text>
                <div style={{ marginTop: space[1] / 2, fontWeight: fontWeight.bold }}>
                  {t('common.price')} {snapshot.data_quality.stale_price_count} · {t('portfolio.cost')} {snapshot.data_quality.missing_cost_basis_count}
                </div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('harness.availableTools')}</Text>
                <div style={{ marginTop: space[1] / 2, fontWeight: fontWeight.bold }}>{snapshot.available_agent_tools.length}</div>
              </div>
            </Grid>

            <div style={{ marginTop: space[4], display: 'grid', gap: space[2] + 2 }}>
              {snapshot.holding_signals.slice(0, 10).map((item) => (
                <div key={item.code} style={{ display: 'grid', gridTemplateColumns: '1fr auto', gap: space[3], alignItems: 'center', padding: `${space[2] + 2}px 0`, borderBottom: '1px solid var(--color-kumo-border)' }}>
                  <div>
                    <Text as="span" size="sm">{item.name}</Text>
                    <div style={{ fontSize: fontSize.md, color: 'var(--text-color-kumo-subtle)', marginTop: space[1] / 2 }}>
                      {item.code} · {t('harness.weight')} {item.weight_pct.toFixed(2)}% · {t('common.change')} {item.change_pct != null ? `${item.change_pct.toFixed(2)}%` : '-'} · {t('harness.vsCost')} {item.deviation_pct != null ? `${item.deviation_pct.toFixed(2)}%` : '-'}
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: space[2] - 2, flexWrap: 'wrap', justifyContent: 'flex-end' }}>
                    {item.signal_tags.slice(0, 2).map((tag) => (
                      <span key={tag} style={{ fontSize: fontSize.md, border: '1px solid var(--color-kumo-border)', borderRadius: radius.sm, padding: `3px ${space[2] - 1}px` }}>{tagLabel(tag, t)}</span>
                    ))}
                  </div>
                </div>
              ))}
            </div>

            <div style={{ marginTop: space[3] + 2, padding: space[3], borderRadius: radius.sm + 2, background: 'var(--color-kumo-canvas)' }}>
              <Text variant="secondary" as="span" size="sm">{snapshot.agent_brief}</Text>
            </div>

            {sourceBrief && (
              <div style={{ marginTop: space[4] + 2 }}>
                <Text variant="heading3" as="h3">{t('harness.sourceQuery')}</Text>
                <div style={{ marginTop: space[2] + 2, display: 'grid', gap: space[2] }}>
                  {sourceBrief.queries.slice(0, 6).map((q) => (
                    <div key={q.id} style={{ padding: `${space[2] + 1}px 0`, borderBottom: '1px solid var(--color-kumo-border)' }}>
                      <Text as="span" size="sm">{q.query}</Text>
                      <div style={{ marginTop: space[1] / 2, fontSize: fontSize.md, color: 'var(--text-color-kumo-subtle)' }}>
                        {q.scope} · {q.freshness} · {q.reason}
                      </div>
                    </div>
                  ))}
                </div>
                <div style={{ marginTop: space[3], display: 'flex', gap: space[2], flexWrap: 'wrap' }}>
                  {sourceBrief.source_targets.slice(0, 5).map((target) => (
                    <span key={`${target.kind}-${target.name}`} style={{ fontSize: fontSize.md, border: '1px solid var(--color-kumo-border)', borderRadius: radius.sm, padding: `${space[1]}px ${space[2]}px` }}>
                      {target.name}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {sourceEvents.length > 0 && (
              <div style={{ marginTop: space[4] + 2 }}>
                <Text variant="heading3" as="h3">{t('harness.sourceEvents')}</Text>
                <div style={{ marginTop: space[2] + 2, display: 'grid', gap: space[2] - 2 }}>
                  {sourceEvents.map((ev) => (
                    <div key={ev.id} style={{ padding: '8px 10px', borderRadius: radius.sm, background: 'var(--color-kumo-canvas)', border: '1px solid var(--color-kumo-border)', opacity: ev.is_read ? opacity.muted : opacity.solid }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', gap: space[2], alignItems: 'flex-start' }}>
                        <div style={{ flex: 1 }}>
                          <Text as="span" size="sm">{ev.title}</Text>
                          {ev.snippet && <div style={{ fontSize: fontSize.md, color: 'var(--text-color-kumo-subtle)', marginTop: 3 }}>{ev.snippet.substring(0, 120)}{ev.snippet.length > 120 ? '...' : ''}</div>}
                          <div style={{ marginTop: space[1], fontSize: fontSize.sm, color: 'var(--text-color-kumo-subtle)', display: 'flex', gap: space[2], flexWrap: 'wrap' }}>
                            <span>{ev.source}</span>
                            {ev.related_security_code && <span>· {ev.related_security_code}</span>}
                            <span>· {ev.fetched_at?.substring(0, 16)}</span>
                          </div>
                        </div>
                        {!ev.is_read && (
                          <button type="button" onClick={() => handleMarkRead(ev.id)} style={{ fontSize: fontSize.sm, padding: `3px ${space[2]}px`, borderRadius: radius.sm - 2, border: '1px solid var(--color-kumo-border)', background: 'transparent', cursor: 'pointer' }}>{t('harness.markRead')}</button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </Card>
  );
}
