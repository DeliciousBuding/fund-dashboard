import { useState, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Grid, Select, Input, Button } from '@cloudflare/kumo'
import { useAppStore } from '../stores/appStore'
import { getTheme, space } from '../styles/theme'
import {
  TRADE_TYPE_USER_BUY,
  TRADE_TYPE_DCA_BUY,
  TRADE_TYPE_USER_SELL,
  TRADE_TYPE_DCA_SELL,
} from '../services/tradeTypes'
import { Card } from './ui/Card'
import { sanitizeUserError } from '../services/userError'


interface TransactionFormData {
  direction: 'buy' | 'sell';
  trade_type: string;
  amount: string;
  shares: string;
  fee: string;
  date: string;
}

interface TransactionFormProps {
  onSubmit: (data: TransactionFormData) => Promise<void>;
  onCancel: () => void;
}

export default function TransactionForm({ onSubmit, onCancel }: TransactionFormProps) {
  const { t } = useTranslation();
  const dark = useAppStore((s) => s.dark);
  const theme = getTheme(dark);
  const [form, setForm] = useState<TransactionFormData>({
    direction: 'buy',
    trade_type: TRADE_TYPE_USER_BUY,
    amount: '',
    shares: '',
    fee: '0',
    date: new Date().toISOString().substring(0, 16),
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const update = useCallback((patch: Partial<TransactionFormData>) => {
    setForm(f => ({ ...f, ...patch }));
  }, []);

  const handleSubmit = useCallback(async () => {
    setError('');
    const amount = parseFloat(form.amount);
    if (!amount || amount <= 0) { setError(t('fundDetail.txForm.invalidAmount')); return; }
    const shares = parseFloat(form.shares);
    if (!shares || shares <= 0) { setError(t('fundDetail.txForm.invalidShares')); return; }
    if (!form.date || isNaN(Date.parse(form.date))) { setError(t('fundDetail.txForm.invalidDate')); return; }
    setSubmitting(true);
    try {
      await onSubmit(form);
      setForm({
        direction: 'buy',
        trade_type: TRADE_TYPE_USER_BUY,
        amount: '',
        shares: '',
        fee: '0',
        date: new Date().toISOString().substring(0, 16),
      });
      setError('');
    } catch (e: any) { setError(sanitizeUserError(e, t('common.loadError'))); }
    finally { setSubmitting(false); }
  }, [form, onSubmit, t]);

  return (
    <Card dark={dark} glass style={{ marginBottom: space[3] }} padded={false}>
      <div style={{ padding: space[5] }}>
        <Text variant="heading3" as="h4">{t('fundDetail.txForm.title')}</Text>
        <Grid variant="2up" gap="base" style={{ marginTop: space[3] }}>
          <Select
            label={t('fundDetail.txForm.direction')}
            value={form.direction}
            onValueChange={v => update({
              direction: v as 'buy' | 'sell',
              trade_type: v === 'buy' ? TRADE_TYPE_USER_BUY : TRADE_TYPE_USER_SELL,
            })}
          >
            <Select.Option value="buy">{t('fundDetail.dir.buy')}</Select.Option>
            <Select.Option value="sell">{t('fundDetail.dir.sell')}</Select.Option>
          </Select>
          <Select label={t('fundDetail.txForm.category')} value={form.trade_type} onValueChange={v => update({ trade_type: v })}>
            {/* Option values are DB trade_type codes (Chinese); labels are i18n UI copy */}
            <Select.Option value={TRADE_TYPE_USER_BUY}>{t('fundDetail.txForm.manualBuy')}</Select.Option>
            <Select.Option value={TRADE_TYPE_DCA_BUY}>{t('fundDetail.txForm.dcaBuy')}</Select.Option>
            <Select.Option value={TRADE_TYPE_USER_SELL}>{t('fundDetail.txForm.manualSell')}</Select.Option>
            <Select.Option value={TRADE_TYPE_DCA_SELL}>{t('fundDetail.txForm.dcaSell')}</Select.Option>
          </Select>
          <Input label={t('fundDetail.txForm.amount')} type="number" inputMode="decimal" placeholder="0.00" value={form.amount} onChange={e => update({ amount: (e.target as HTMLInputElement).value })} />
          <Input label={t('fundDetail.txForm.shares')} type="number" inputMode="decimal" placeholder="0" value={form.shares} onChange={e => update({ shares: (e.target as HTMLInputElement).value })} />
          <Input label={t('fundDetail.txForm.fee')} type="number" inputMode="decimal" placeholder="0" value={form.fee} onChange={e => update({ fee: (e.target as HTMLInputElement).value })} />
          <Input label={t('fundDetail.txForm.tradeTime')} type="datetime-local" value={form.date} onChange={e => update({ date: (e.target as HTMLInputElement).value })} />
        </Grid>
        <div aria-live="polite" role={error ? 'alert' : undefined}>
          {error && (
            <Text variant="body" size="xs" as="span" style={{ display: 'block', marginTop: 8, color: theme.critical }}>
              {error}
            </Text>
          )}
        </div>
        <div style={{ marginTop: space[4], display: 'flex', gap: 8 }}>
          <Button type="button" variant="primary" size="sm" onClick={handleSubmit} disabled={submitting} aria-busy={submitting}>
            {submitting ? t('fundDetail.txForm.submitting') : t('fundDetail.txForm.confirmAdd')}
          </Button>
          <Button type="button" variant="secondary" size="sm" onClick={onCancel} disabled={submitting}>{t('common.cancel')}</Button>
        </div>
      </div>
    </Card>
  );
}
