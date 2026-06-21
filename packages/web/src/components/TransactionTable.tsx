import { useState, useMemo, useCallback } from 'react'
import { Text, LayerCard, Table, Badge, Button, Input } from '@cloudflare/kumo'
import { MagnifyingGlassIcon, PlusIcon, TrashIcon, UserIcon, RobotIcon } from '@phosphor-icons/react'
import type { Transaction } from '../api'

const DIR: Record<string, string> = { buy: '买入', sell: '卖出', dividend: '分红', convert_in: '转入', convert_out: '转出', forced_redeem: '强赎' };

interface TransactionTableProps {
  transactions: Transaction[];
  onToggleType: (seq: number, current: string) => void;
  onDelete: (seq: number) => void;
  onAdd: () => void;
  deleting: number | null;
}

export default function TransactionTable({ transactions, onToggleType, onDelete, onAdd, deleting }: TransactionTableProps) {
  const [txSearch, setTxSearch] = useState('');
  const [txSort, setTxSort] = useState<{ key: string; dir: 'asc' | 'desc' }>({ key: 'trade_time', dir: 'desc' });

  const toggleSort = useCallback((key: string) => {
    setTxSort(prev => prev.key === key ? { key, dir: prev.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: 'desc' });
  }, []);

  const filteredTxs = useMemo(() => {
    let txs = [...transactions];
    if (txSearch) {
      const q = txSearch.toLowerCase();
      txs = txs.filter(tx =>
        tx.trade_time.includes(q) || tx.confirm_date.includes(q) || DIR[tx.direction]?.includes(q) ||
        tx.trade_type.includes(q) || tx.order_id?.includes(q) || String(tx.amount).includes(q)
      );
    }
    txs.sort((a, b) => {
      const aVal = txSort.key === 'trade_time' ? a.trade_time : txSort.key === 'amount' ? a.amount :
        txSort.key === 'shares' ? a.shares : txSort.key === 'nav' ? (a.nav ?? 0) : a.trade_time;
      const bVal = txSort.key === 'trade_time' ? b.trade_time : txSort.key === 'amount' ? b.amount :
        txSort.key === 'shares' ? b.shares : txSort.key === 'nav' ? (b.nav ?? 0) : b.trade_time;
      const cmp = typeof aVal === 'string' ? aVal.localeCompare(String(bVal)) : (aVal as number) - (bVal as number);
      return txSort.dir === 'asc' ? cmp : -cmp;
    });
    return txs;
  }, [transactions, txSearch, txSort]);

  return (
    <LayerCard className="p-0">
      <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--color-kumo-border)', display: 'flex', gap: 8, alignItems: 'center' }}>
        <Input style={{ flex: 1 }} placeholder="搜索交易记录（日期/类型/金额）..." value={txSearch} onChange={e => setTxSearch((e.target as HTMLInputElement).value)}
          prefix={<MagnifyingGlassIcon size={16} />} />
        <Button variant="primary" size="sm" onClick={onAdd}>
          <PlusIcon size={14} style={{ marginRight: 4 }} /> 添加交易
        </Button>
      </div>
      <div style={{ maxHeight: '55vh', overflow: 'auto' }}>
        <Table>
          <Table.Header><Table.Row>
            {[['trade_time','交易时间'],['confirm_date','确认'],['direction','类型'],['amount','金额'],['shares','份额'],['nav','净值'],['fee','手续费'],['settlement_days','结算'],['_actions','操作']].map(([k,l]) => (
              <Table.Head key={k} style={k !== '_actions' ? { cursor: 'pointer', userSelect: 'none' } : {}}
                onClick={() => k !== '_actions' && toggleSort(k)}>
                {l}{k !== '_actions' && txSort.key === k ? (txSort.dir === 'asc' ? ' ▲' : ' ▼') : ''}
              </Table.Head>
            ))}
          </Table.Row></Table.Header>
          <Table.Body>
            {filteredTxs.map(tx => {
              const isBuy=tx.direction==='buy', isSell=tx.direction==='sell';
              const isAuto = (tx.trade_type || '').includes('定投');
              const isManual = (tx.trade_type || '').includes('用户');
              return (<Table.Row key={tx.seq} style={{ opacity: deleting === tx.seq ? 0.4 : 1 }}>
                <Table.Cell><Text variant="body" as="span" size="xs">{tx.trade_time.substring(0,16)}</Text></Table.Cell>
                <Table.Cell><Text variant="secondary" as="span" size="xs">{tx.confirm_date}</Text></Table.Cell>
                <Table.Cell>
                  <Badge variant={isBuy?'success':isSell?'error':'warning'}>{DIR[tx.direction]||tx.direction}</Badge>
                  {isAuto && <Badge variant="blue" style={{marginLeft:4}}>定投</Badge>}
                  {isManual && <Badge variant="neutral" style={{marginLeft:4}}>手动</Badge>}
                </Table.Cell>
                <Table.Cell style={{ fontWeight: 500 }}>¥ {tx.amount.toFixed(2)}</Table.Cell>
                <Table.Cell>{tx.shares.toFixed(2)}</Table.Cell>
                <Table.Cell><Text variant="mono" as="span" size="xs">{tx.nav?.toFixed(4)??'-'}</Text></Table.Cell>
                <Table.Cell>{tx.fee>0?`¥ ${tx.fee.toFixed(2)}`:'-'}</Table.Cell>
                <Table.Cell>{tx.settlement_days!=null?`T+${tx.settlement_days}`:'-'}</Table.Cell>
                <Table.Cell>
                  <div style={{ display: 'flex', gap: 4 }}>
                    {(tx.direction === 'buy') && (
                      <Button variant="secondary" size="sm" onClick={() => onToggleType(tx.seq, tx.trade_type)} title={isAuto ? '切换为手动买入' : '切换为定投买入'}>
                        {isAuto ? <UserIcon size={14} /> : <RobotIcon size={14} />}
                      </Button>
                    )}
                    <Button variant="secondary" size="sm" onClick={() => onDelete(tx.seq)} title="删除此交易">
                      <TrashIcon size={14} style={{ color: '#d63649' }} />
                    </Button>
                  </div>
                </Table.Cell>
              </Table.Row>);
            })}
            {filteredTxs.length === 0 && (
              <Table.Row><Table.Cell colSpan={9} style={{ textAlign: 'center', padding: 32 }}><Text variant="secondary" as="span">无匹配交易</Text></Table.Cell></Table.Row>
            )}
          </Table.Body>
        </Table>
      </div>
    </LayerCard>
  );
}
