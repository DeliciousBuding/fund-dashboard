import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, LayerCard, Grid } from '@cloudflare/kumo'
import { fetchInvestmentHarness, fetchInvestmentSourceBrief, fetchSourceEvents, markSourceEventApi, type InvestmentHarnessSnapshot, type InvestmentSourceBrief, type SourceEvent } from '../api'

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

export default function InvestmentHarnessPanel() {
  const { t } = useTranslation();
  const [snapshot, setSnapshot] = useState<InvestmentHarnessSnapshot | null>(null);
  const [sourceBrief, setSourceBrief] = useState<InvestmentSourceBrief | null>(null);
  const [sourceEvents, setSourceEvents] = useState<SourceEvent[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    const ctrl = new AbortController();
    Promise.all([
      fetchInvestmentHarness(ctrl.signal),
      fetchInvestmentSourceBrief(8, ctrl.signal),
      fetchSourceEvents({ limit: 10 }, ctrl.signal),
    ])
      .then(([harness, sources, events]) => {
        setSnapshot(harness);
        setSourceBrief(sources);
        setSourceEvents(events.events);
      })
      .catch((e) => { if (e.name !== 'AbortError') setError(e.message || t('harness.error')); });
    return () => ctrl.abort();
  }, []);

  const handleMarkRead = (id: number) => {
    markSourceEventApi(id, { is_read: true }).then(() => {
      setSourceEvents(prev => prev.map(e => e.id === id ? { ...e, is_read: true } : e));
    }).catch(() => {});
  };

  return (
    <LayerCard>
      <div style={{ padding: 20 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'baseline', flexWrap: 'wrap' }}>
          <div>
            <Text variant="heading2" as="h2">{t('harness.title')}</Text>
            <Text variant="secondary" as="span" size="sm">{t('harness.subtitle')}</Text>
          </div>
          {snapshot && <Text variant="secondary" as="span">{t('harness.factsOnly')} · {snapshot.holdings_count} {t('tx.trades')}</Text>}
        </div>

        {error && <div style={{ marginTop: 12, color: '#d63649', fontSize: 12 }}>{error}</div>}
        {!snapshot && !error && <div style={{ marginTop: 12 }}><Text variant="secondary" as="span">{t('harness.loading')}</Text></div>}

        {snapshot && (
          <>
            <Grid variant="3up" gap="base" style={{ marginTop: 16 }}>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('harness.totalValue')}</Text>
                <div style={{ marginTop: 2, fontWeight: 700 }}>¥ {snapshot.total_value.toLocaleString()}</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('harness.dataGaps')}</Text>
                <div style={{ marginTop: 2, fontWeight: 700 }}>
                  {t('common.price')} {snapshot.data_quality.stale_price_count} · {t('portfolio.cost')} {snapshot.data_quality.missing_cost_basis_count}
                </div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('harness.availableTools')}</Text>
                <div style={{ marginTop: 2, fontWeight: 700 }}>{snapshot.available_agent_tools.length}</div>
              </div>
            </Grid>

            <div style={{ marginTop: 16, display: 'grid', gap: 10 }}>
              {snapshot.holding_signals.slice(0, 10).map((item) => (
                <div key={item.code} style={{ display: 'grid', gridTemplateColumns: '1fr auto', gap: 12, alignItems: 'center', padding: '10px 0', borderBottom: '1px solid var(--color-kumo-border)' }}>
                  <div>
                    <Text as="span" size="sm">{item.name}</Text>
                    <div style={{ fontSize: 12, color: 'var(--text-color-kumo-subtle)', marginTop: 2 }}>
                      {item.code} · {t('allocation.maxPosition')} {item.weight_pct.toFixed(2)}% · {t('common.change')} {item.change_pct != null ? `${item.change_pct.toFixed(2)}%` : '-'} · {t('portfolio.cost')}{t('common.change')} {item.deviation_pct != null ? `${item.deviation_pct.toFixed(2)}%` : '-'}
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', justifyContent: 'flex-end' }}>
                    {item.signal_tags.slice(0, 2).map((tag) => (
                      <span key={tag} style={{ fontSize: 12, border: '1px solid var(--color-kumo-border)', borderRadius: 6, padding: '3px 7px' }}>{tagLabel(tag, t)}</span>
                    ))}
                  </div>
                </div>
              ))}
            </div>

            <div style={{ marginTop: 14, padding: 12, borderRadius: 8, background: 'var(--color-kumo-canvas)' }}>
              <Text variant="secondary" as="span" size="sm">{snapshot.agent_brief}</Text>
            </div>

            {sourceBrief && (
              <div style={{ marginTop: 18 }}>
                <Text variant="heading3" as="h3">{t('harness.sourceQuery')}</Text>
                <div style={{ marginTop: 10, display: 'grid', gap: 8 }}>
                  {sourceBrief.queries.slice(0, 6).map((q) => (
                    <div key={q.id} style={{ padding: '9px 0', borderBottom: '1px solid var(--color-kumo-border)' }}>
                      <Text as="span" size="sm">{q.query}</Text>
                      <div style={{ marginTop: 2, fontSize: 12, color: 'var(--text-color-kumo-subtle)' }}>
                        {q.scope} · {q.freshness} · {q.reason}
                      </div>
                    </div>
                  ))}
                </div>
                <div style={{ marginTop: 12, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                  {sourceBrief.source_targets.slice(0, 5).map((target) => (
                    <span key={`${target.kind}-${target.name}`} style={{ fontSize: 12, border: '1px solid var(--color-kumo-border)', borderRadius: 6, padding: '4px 8px' }}>
                      {target.name}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {sourceEvents.length > 0 && (
              <div style={{ marginTop: 18 }}>
                <Text variant="heading3" as="h3">{t('harness.sourceEvents')}</Text>
                <div style={{ marginTop: 10, display: 'grid', gap: 6 }}>
                  {sourceEvents.map((ev) => (
                    <div key={ev.id} style={{ padding: '8px 10px', borderRadius: 6, background: 'var(--color-kumo-canvas)', border: '1px solid var(--color-kumo-border)', opacity: ev.is_read ? 0.5 : 1 }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, alignItems: 'flex-start' }}>
                        <div style={{ flex: 1 }}>
                          <Text as="span" size="sm">{ev.title}</Text>
                          {ev.snippet && <div style={{ fontSize: 12, color: 'var(--text-color-kumo-subtle)', marginTop: 3 }}>{ev.snippet.substring(0, 120)}{ev.snippet.length > 120 ? '...' : ''}</div>}
                          <div style={{ marginTop: 4, fontSize: 11, color: 'var(--text-color-kumo-subtle)', display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                            <span>{ev.source}</span>
                            {ev.related_security_code && <span>· {ev.related_security_code}</span>}
                            <span>· {ev.fetched_at?.substring(0, 16)}</span>
                          </div>
                        </div>
                        {!ev.is_read && (
                          <button onClick={() => handleMarkRead(ev.id)} style={{ fontSize: 11, padding: '3px 8px', borderRadius: 4, border: '1px solid var(--color-kumo-border)', background: 'transparent', cursor: 'pointer' }}>{t('harness.markRead')}</button>
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
    </LayerCard>
  );
}
