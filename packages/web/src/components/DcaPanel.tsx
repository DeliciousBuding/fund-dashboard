import { useState, useEffect, Suspense, lazy } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Grid, Input, Button } from '@cloudflare/kumo'
import { fetchDcaPlan, type DcaPlan } from '../api'
import { getTheme, space, radius, fontSize, fontWeight } from '../styles/theme'
import { Card } from './ui/Card'
import { sanitizeUserError } from '../services/userError'
import { useAppStore } from '../stores/appStore'

const DcaBacktestChart = lazy(() => import('./DcaBacktestChart'))

interface DcaPanelProps {
  fundCode: string;
  heldShares: number;
  latestNav: number;
  totalCost: number;
  dark: boolean;
}


const DCA_SIGNAL_CODES = new Set([
  'increase', 'decrease', 'normal', 'dip_buy', 'rally_control', 'range_dca',
])

function signalLabel(t: (k: string) => string, code?: string): string {
  if (!code) return ''
  if (!DCA_SIGNAL_CODES.has(code)) return code // legacy/raw labels pass through
  return t(`dca.signal.${code}`)
}

function explanationLabel(
  t: (k: string, o?: Record<string, unknown>) => string,
  plan: {
    mode?: string
    signal?: string
    explanation?: string
    deviation_pct?: number | null
    change_pct?: number | null
    actual_amount?: number
  },
): string {
  // Structured i18n only for stable signal codes from Go (#201); else show API explanation.
  if (!plan.signal || !DCA_SIGNAL_CODES.has(plan.signal)) {
    return plan.explanation || plan.signal || ''
  }
  const amount = plan.actual_amount ?? 0
  const signal = signalLabel(t, plan.signal)
  if (plan.mode === 'change_pct' && plan.change_pct != null) {
    return t('dca.explanation.changePct', { change: plan.change_pct.toFixed(2), signal, amount: amount.toFixed(2) })
  }
  if (plan.deviation_pct != null) {
    return t('dca.explanation.deviation', { deviation: plan.deviation_pct.toFixed(2), signal, amount: amount.toFixed(2) })
  }
  return plan.explanation || signal
}

export default function DcaPanel({ fundCode, heldShares, latestNav, totalCost, dark }: DcaPanelProps) {
  const { t } = useTranslation()
  const theme = getTheme(dark)
  const portfolioId = useAppStore((s) => s.portfolioId)
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
    fetchDcaPlan(fundCode, { base: amt, mode, portfolioId })
      .then(setPlan)
      .catch((e) => setError(sanitizeUserError(e, t('dca.computeError'))))
      .finally(() => setLoading(false));
  };

  // Recompute when mode / portfolio / fund switches if base amount is valid (#190 CX).
  useEffect(() => {
    const amt = parseFloat(baseAmount)
    if (!amt || amt <= 0 || !latestNav || latestNav <= 0) return
    // skip initial empty plan until user has interacted or we have prior plan
    if (!plan) return
    setLoading(true)
    setError('')
    fetchDcaPlan(fundCode, { base: amt, mode, portfolioId })
      .then(setPlan)
      .catch((e) => setError(sanitizeUserError(e, t('dca.computeError'))))
      .finally(() => setLoading(false))
    // Intentionally omit baseAmount/latestNav/plan: amount edits use the Compute
    // button (avoids fetch-on-keystroke); plan presence gates the first auto-run.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, portfolioId, fundCode])


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
      <Card dark={dark} glass style={{ marginBottom: space[5] }} padded={false}>
      <div style={{ padding: `${space[4]}px ${space[5]}px` }}>
        <Text variant="heading3" as="h3">{t('dca.title')}</Text>
        <div style={{ marginTop: space[3] }}>
          <Grid variant="2up" gap="base">
            <div>
              <Text variant="secondary" as="span" size="xs">{t('dca.heldShares')}</Text>
              <div className="fd-tabular-nums" style={{ marginTop: space[1] / 2, fontWeight: fontWeight.semibold }}>{heldShares.toFixed(2)}</div>
            </div>
            <div>
              <Text variant="secondary" as="span" size="xs">{t('dca.latestNav')}</Text>
              <div className="fd-tabular-nums" style={{ marginTop: space[1] / 2, fontWeight: fontWeight.semibold }}>{latestNav?.toFixed(4) ?? '-'}</div>
            </div>
            <div>
              <Text variant="secondary" as="span" size="xs">{t('dca.costPerShare')}</Text>
              <div className="fd-tabular-nums" style={{ marginTop: space[1] / 2, fontWeight: fontWeight.semibold }}>{fallbackCostPerShare ? fallbackCostPerShare.toFixed(4) : '-'}</div>
            </div>
          </Grid>
        </div>
        <div style={{ marginTop: space[3] + 2, display: 'flex', gap: space[2], flexWrap: 'wrap' }}>
          <Button type="button"
            variant={mode === 'nav_deviation' ? 'primary' : 'secondary'}
            size="sm"
            aria-pressed={mode === 'nav_deviation'}
            onClick={() => setMode('nav_deviation')}
          >
            {t('dca.modeDeviation')}
          </Button>
          <Button type="button"
            variant={mode === 'change_pct' ? 'primary' : 'secondary'}
            size="sm"
            aria-pressed={mode === 'change_pct'}
            onClick={() => setMode('change_pct')}
          >
            {t('dca.modeChangePct')}
          </Button>
        </div>
        <div style={{ marginTop: space[4], display: 'flex', gap: space[2], alignItems: 'flex-end' }}>
          <Input
            label={t('dca.baseAmountInput')}
            type="number"
            inputMode="decimal"
            placeholder="0.00"
            value={baseAmount}
            onChange={e => setBaseAmount((e.target as HTMLInputElement).value)}
          />
          <Button type="button" variant="primary" size="sm" onClick={compute} style={{ marginBottom: 0 }}>
            {loading ? t('dca.computing') : t('dca.compute')}
          </Button>
        </div>
        {error && <div role="alert" style={{ marginTop: space[3] - 2, color: theme.critical, fontSize: fontSize.md }}>{error}</div>}
        {displayedPlan && (
          <div style={{ marginTop: space[3] }}>
            <Grid variant="2up" gap="base">
              <div>
                <Text variant="secondary" as="span" size="xs">{t('dca.simulatedDeduction')}</Text>
                <div className="fd-tabular-nums" style={{ marginTop: space[1] / 2, fontWeight: fontWeight.bold }}>¥ {displayedPlan.actual_amount.toFixed(2)}</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('dca.deductionRate')}</Text>
                <div className="fd-tabular-nums" style={{ marginTop: space[1] / 2, fontWeight: fontWeight.semibold }}>{(displayedPlan.dca_rate * 100).toFixed(0)}% · {signalLabel(t, displayedPlan.signal)}</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('dca.deviationRate')}</Text>
                <div className="fd-tabular-nums" style={{ marginTop: space[1] / 2, fontWeight: fontWeight.semibold }}>{displayedPlan.deviation_pct != null ? `${displayedPlan.deviation_pct.toFixed(2)}%` : '-'}</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">{t('dca.recentChange')}</Text>
                <div className="fd-tabular-nums" style={{ marginTop: space[1] / 2, fontWeight: fontWeight.semibold }}>{displayedPlan.change_pct != null ? `${displayedPlan.change_pct.toFixed(2)}%` : '-'}</div>
              </div>
            </Grid>
            <div style={{ marginTop: space[3] - 2, fontSize: fontSize.md, color: 'var(--text-color-kumo-subtle)' }}>{explanationLabel(t, displayedPlan)}</div>
          </div>
        )}

        <div style={{ marginTop: space[4], borderTop: '1px solid var(--border-color-kumo-subtle)', paddingTop: space[3] }}>
          <Button type="button"
            variant={showBacktest ? 'primary' : 'secondary'}
            size="sm"
            onClick={() => setShowBacktest(v => !v)}
          >
            {showBacktest ? t('dca.collapseBacktest') : t('dca.backtestSim')}
          </Button>
        </div>
      </div>
    </Card>

    {showBacktest && (
      <Suspense fallback={
        <Card dark={dark} glass style={{ marginBottom: space[5] }}>
          <div style={{ padding: space[5], textAlign: 'center' }}>
            <Text variant="secondary" as="span">{t('dca.loadingBacktest')}</Text>
          </div>
        </Card>
      }>
        <DcaBacktestChart fundCode={fundCode} dark={dark} />
      </Suspense>
    )}
    </>
  );
}
