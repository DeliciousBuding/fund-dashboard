import { useState, Suspense, lazy } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, LayerCard, Grid, Input, Button } from '@cloudflare/kumo'
import { fetchDcaPlan, type DcaPlan } from '../api'

const DcaBacktestChart = lazy(() => import('./DcaBacktestChart'))

interface DcaPanelProps {
  fundCode: string;
  heldShares: number;
  latestNav: number;
  totalCost: number;
  dark: boolean;
}

export default function DcaPanel({ fundCode, heldShares, latestNav, totalCost, dark }: DcaPanelProps) {
  const { t } = useTranslation()
  const [baseAmount, setBaseAmount] = useState('100');
  const [mode, setMode] = useState<'nav_deviation' | 'change_pct'>('nav_deviation');
  const [plan, setPlan] = useState<DcaPlan | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [showBacktest, setShowBacktest] = useState(false);

  const fallbackCostPerShare = totalCost && heldShares > 0 ? Math.abs(totalCost) / heldShares : null;

  const compute = () => {
    const amt = parseFloat(baseAmount);
    if (!amt || amt <= 0 || !latestNav || latestNav <= 0) return;
    setLoading(true);
    setError('');
    fetchDcaPlan(fundCode, { base: amt, mode })
      .then(setPlan)
      .catch((e) => setError(e.message || t('dca.computeError')))
      .finally(() => setLoading(false));
  };

  const displayedPlan = plan || (fallbackCostPerShare ? {
    actual_amount: parseFloat(baseAmount) || 0,
    dca_rate: 1,
    signal: t('dca.pendingCalc'),
    explanation: t('dca.pendingExplanation'),
    cost_per_share: fallbackCostPerShare,
    deviation_pct: ((latestNav - fallbackCostPerShare) / fallbackCostPerShare) * 100,
    change_pct: null,
    mode,
    base_amount: parseFloat(baseAmount) || 0,
    latest_nav: latestNav,
  } as DcaPlan : null);

  return (
    <>
      <LayerCard style={{ marginBottom: 20 }}>
      <div style={{ padding: '16px 20px' }}>
        <Text variant="heading3" as="h3">{t('dca.title')}</Text>
        <div style={{ marginTop: 12 }}>
          <Grid variant="2up" gap="base">
            <div>
              <Text variant="secondary" as="span" size="xs">{t('dca.heldShares')}</Text>
              <div style={{ marginTop: 2, fontWeight: 600 }}>{heldShares.toFixed(2)}</div>
            </div>
            <div>
              <Text variant="secondary" as="span" size="xs">{t('dca.latestNav')}</Text>
              <div style={{ marginTop: 2, fontWeight: 600 }}>{latestNav?.toFixed(4) ?? '-'}</div>
            </div>
            <div>
              <Text variant="secondary" as="span" size="xs">{t('dca.costPerShare')}</Text>
              <div style={{ marginTop: 2, fontWeight: 600 }}>{fallbackCostPerShare ? fallbackCostPerShare.toFixed(4) : '-'}</div>
            </div>
          </Grid>
        </div>
        <div style={{ marginTop: 14, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <Button variant={mode === 'nav_deviation' ? 'primary' : 'secondary'} size="sm" onClick={() => setMode('nav_deviation')}>
            {t('dca.modeDeviation')}
          </Button>
          <Button variant={mode === 'change_pct' ? 'primary' : 'secondary'} size="sm" onClick={() => setMode('change_pct')}>
            {t('dca.modeChangePct')}
          </Button>
        </div>
        <div style={{ marginTop: 16, display: 'flex', gap: 8, alignItems: 'flex-end' }}>
          <Input
            label={t('dca.baseAmountInput')}
            type="number"
            inputMode="decimal"
            placeholder="0.00"
            value={baseAmount}
            onChange={e => setBaseAmount((e.target as HTMLInputElement).value)}
          />
          <Button variant="primary" size="sm" onClick={compute} style={{ marginBottom: 0 }}>
            {loading ? t('dca.computing') : t('dca.compute')}
          </Button>
        </div>
        {error && <div style={{ marginTop: 10, color: '#d63649', fontSize: 12 }}>{error}</div>}
        {displayedPlan && (
          <div style={{ marginTop: 12 }}>
            <Grid variant="2up" gap="base">
              <div>
                <Text variant="secondary" as="span" size="xs">{t('dca.simulatedDeduction')}</Text>
                <div style={{ marginTop: 2, fontWeight: 700 }}>¥ {displayedPlan.actual_amount.toFixed(2)}</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('dca.deductionRate')}</Text>
                <div style={{ marginTop: 2, fontWeight: 600 }}>{(displayedPlan.dca_rate * 100).toFixed(0)}% · {displayedPlan.signal}</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('dca.deviationRate')}</Text>
                <div style={{ marginTop: 2, fontWeight: 600 }}>{displayedPlan.deviation_pct != null ? `${displayedPlan.deviation_pct.toFixed(2)}%` : '-'}</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('dca.recentChange')}</Text>
                <div style={{ marginTop: 2, fontWeight: 600 }}>{displayedPlan.change_pct != null ? `${displayedPlan.change_pct.toFixed(2)}%` : '-'}</div>
              </div>
            </Grid>
            <div style={{ marginTop: 10, fontSize: 12, color: 'var(--text-color-kumo-subtle)' }}>{displayedPlan.explanation}</div>
          </div>
        )}

        {/* ── Backtest toggle ────────────────────────────── */}
        <div style={{ marginTop: 16, borderTop: '1px solid var(--border-color-kumo-subtle)', paddingTop: 12 }}>
          <Button
            variant={showBacktest ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setShowBacktest(v => !v)}
          >
            {showBacktest ? t('dca.collapseBacktest') : t('dca.backtestSim')}
          </Button>
        </div>
      </div>
    </LayerCard>

    {showBacktest && (
      <Suspense fallback={
        <LayerCard style={{ marginBottom: 20 }}>
          <div style={{ padding: 20, textAlign: 'center' }}>
            <Text variant="secondary" as="span">{t('dca.loadingBacktest')}</Text>
          </div>
        </LayerCard>
      }>
        <DcaBacktestChart fundCode={fundCode} dark={dark} />
      </Suspense>
    )}
    </>
  );
}
