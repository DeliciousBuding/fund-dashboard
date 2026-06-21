import { useState, useCallback } from 'react'
import { Text, LayerCard, Grid, Select, Input, Button } from '@cloudflare/kumo'

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
  const [form, setForm] = useState<TransactionFormData>({
    direction: 'buy',
    trade_type: '用户买入',
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
    if (!amount || amount <= 0) { setError('请输入有效金额'); return; }
    const shares = parseFloat(form.shares);
    if (form.direction !== 'dividend' && (!shares || shares <= 0)) { setError('请输入有效份额'); return; }
    if (!form.date || isNaN(Date.parse(form.date))) { setError('请输入有效日期'); return; }
    setSubmitting(true);
    try {
      await onSubmit(form);
      setForm({
        direction: 'buy',
        trade_type: '用户买入',
        amount: '',
        shares: '',
        fee: '0',
        date: new Date().toISOString().substring(0, 16),
      });
      setError('');
    } catch (e: any) { setError(e.message); }
    finally { setSubmitting(false); }
  }, [form, onSubmit]);

  return (
    <LayerCard style={{ marginBottom: 12, padding: 20 }}>
      <Text variant="heading3" as="h4">添加交易</Text>
      <Grid variant="2up" gap="base" style={{ marginTop: 12 }}>
        <Select label="方向" value={form.direction} onValueChange={v => update({ direction: v as 'buy' | 'sell', trade_type: v === 'buy' ? '用户买入' : '用户卖出' })}>
          <Select.Option value="buy">买入</Select.Option>
          <Select.Option value="sell">卖出</Select.Option>
        </Select>
        <Select label="分类" value={form.trade_type} onValueChange={v => update({ trade_type: v })}>
          <Select.Option value="用户买入">手动买入</Select.Option>
          <Select.Option value="定投买入">定投买入</Select.Option>
          <Select.Option value="用户卖出">手动卖出</Select.Option>
          <Select.Option value="定投卖出">定投卖出</Select.Option>
        </Select>
        <Input label="金额 (元)" type="number" inputMode="decimal" placeholder="0.00" value={form.amount} onChange={e => update({ amount: (e.target as HTMLInputElement).value })} />
        <Input label="份额" type="number" inputMode="decimal" placeholder="可选" value={form.shares} onChange={e => update({ shares: (e.target as HTMLInputElement).value })} />
        <Input label="手续费" type="number" inputMode="decimal" placeholder="0" value={form.fee} onChange={e => update({ fee: (e.target as HTMLInputElement).value })} />
        <Input label="交易时间" type="datetime-local" value={form.date} onChange={e => update({ date: (e.target as HTMLInputElement).value })} />
      </Grid>
      {error && <Text variant="body" size="xs" as="span" style={{ display: 'block', marginTop: 8, color: '#d63649' }}>{error}</Text>}
      <div style={{ marginTop: 16, display: 'flex', gap: 8 }}>
        <Button variant="primary" size="sm" onClick={handleSubmit}>确认添加</Button>
        <Button variant="secondary" size="sm" onClick={onCancel}>取消</Button>
      </div>
    </LayerCard>
  );
}
