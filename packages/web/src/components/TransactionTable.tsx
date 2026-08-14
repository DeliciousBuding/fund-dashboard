import { useState, useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Table, Badge, Button, Input } from '@cloudflare/kumo'
import { MagnifyingGlassIcon, PlusIcon, TrashIcon, UserIcon, RobotIcon } from '@phosphor-icons/react'
import type { Transaction } from '../api'
import { useAppStore } from '../stores/appStore'
import { getTheme, space, radius, fontWeight, opacity } from '../styles/theme'
import { isAutoTradeType, isManualTradeType } from '../services/tradeTypes'
import { Card } from './ui/Card'

/** Direction keys stored on Transaction.direction (API codes, not UI copy). */
const DIR_KEYS = ['buy', 'sell', 'dividend', 'convert_in', 'convert_out', 'forced_redeem'] as const;

interface TransactionTableProps {
  transactions: Transaction[];
  onToggleType: (seq: number, current: string) => void;
  onDelete: (seq: number) => void;
  onAdd: () => void;
  deleting: number | null;
}

export default function TransactionTable({ transactions, onToggleType, onDelete, onAdd, deleting }: TransactionTableProps) {
  const { t } = useTranslation();
  const dark = useAppStore((s) => s.dark);
  const theme = getTheme(dark);
  const [txSearch, setTxSearch] = useState('');
  const [txSort, setTxSort] = useState<{ key: string; dir: 'asc' | 'desc' }>({ key: 'trade_time', dir: 'desc' });

  const dirLabel = useCallback((direction: string) => {
    if ((DIR_KEYS as readonly string[]).includes(direction)) {
      return t(`fundDetail.dir.${direction}`);
    }
    return direction;
  }, [t]);

  const toggleSort = useCallback((key: string) => {
    setTxSort(prev => prev.key === key ? { key, dir: prev.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: 'desc' });
  }, []);

  const filteredTxs = useMemo(() => {
    let txs = [...transactions];
    if (txSearch) {
      const q = txSearch.toLowerCase();
      txs = txs.filter(tx =>
        tx.trade_time.includes(q) || (tx.confirm_date?.includes(q)) || dirLabel(tx.direction).toLowerCase().includes(q) ||
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
  }, [transactions, txSearch, txSort, dirLabel]);

  const columns: [string, string][] = [
    ['trade_time', t('fundDetail.txTable.tradeTime')],
    ['confirm_date', t('fundDetail.txTable.confirm')],
    ['direction', t('fundDetail.txTable.direction')],
    ['amount', t('fundDetail.txTable.amount')],
    ['shares', t('fundDetail.txTable.shares')],
    ['nav', t('fundDetail.txTable.nav')],
    ['fee', t('fundDetail.txTable.fee')],
    ['settlement_days', t('fundDetail.txTable.settlement')],
    ['_actions', t('fundDetail.txTable.actions')],
  ];

  return (
    <Card dark={dark} glass padded={false} className="p-0">
      <div style={{ padding: `${space[3]}px ${space[4]}px`, borderBottom: `1px solid ${theme.border}`, display: 'flex', gap: space[2], alignItems: 'center' }}>
        <Input style={{ flex: 1 }} placeholder={t('fundDetail.txTable.searchPlaceholder')} value={txSearch} onChange={e => setTxSearch((e.target as HTMLInputElement).value)}
          prefix={<MagnifyingGlassIcon size={16} aria-hidden />} aria-label={t('fundDetail.txTable.searchAria')} />
        <Button type="button" variant="primary" size="sm" onClick={onAdd} aria-label={t('fundDetail.addTx')}>
          <PlusIcon size={14} style={{ marginRight: 4 }} aria-hidden /> {t('fundDetail.addTx')}
        </Button>
      </div>
      <div style={{ maxHeight: '55vh', overflow: 'auto' }}>
        <Table>
          <Table.Header><Table.Row>
            {columns.map(([k, l]) => {
              const sortable = k !== '_actions';
              const active = sortable && txSort.key === k;
              const ariaSort = !sortable ? undefined : active ? (txSort.dir === 'asc' ? 'ascending' : 'descending') : 'none';
              return (
                <Table.Head
                  key={k}
                  style={sortable ? { cursor: 'pointer', userSelect: 'none' } : {}}
                  onClick={() => sortable && toggleSort(k)}
                  aria-sort={ariaSort as 'ascending' | 'descending' | 'none' | undefined}
                >
                  {l}{active ? (txSort.dir === 'asc' ? ' ▲' : ' ▼') : ''}
                </Table.Head>
              );
            })}
          </Table.Row></Table.Header>
          <Table.Body>
            {filteredTxs.map(tx => {
              const isBuy=tx.direction==='buy', isSell=tx.direction==='sell';
              // Data-shape checks against stored trade_type DB values (Chinese literals required).
              const isAuto = isAutoTradeType(tx.trade_type);
              const isManual = isManualTradeType(tx.trade_type);
              return (<Table.Row key={tx.seq} style={{ opacity: deleting === tx.seq ? opacity.disabled : opacity.solid }}>
                <Table.Cell><Text variant="body" as="span" size="xs" className="fd-tabular-nums">{tx.trade_time.substring(0,16)}</Text></Table.Cell>
                <Table.Cell><Text variant="secondary" as="span" size="xs" className="fd-tabular-nums">{tx.confirm_date ?? '-'}</Text></Table.Cell>
                <Table.Cell>
                  <Badge variant={isBuy?'success':isSell?'error':'warning'}>{dirLabel(tx.direction)}</Badge>
                  {isAuto && <Badge variant="blue" style={{marginLeft:4}}>{t('tx.auto')}</Badge>}
                  {isManual && <Badge variant="neutral" style={{marginLeft:4}}>{t('tx.manual')}</Badge>}
                </Table.Cell>
                <Table.Cell className="fd-tabular-nums" style={{ fontWeight: fontWeight.medium }}>¥ {tx.amount.toFixed(2)}</Table.Cell>
                <Table.Cell className="fd-tabular-nums">{tx.shares.toFixed(2)}</Table.Cell>
                <Table.Cell><Text variant="mono" as="span" size="xs" className="fd-tabular-nums">{tx.nav?.toFixed(4)??'-'}</Text></Table.Cell>
                <Table.Cell className="fd-tabular-nums">{tx.fee>0?`¥ ${tx.fee.toFixed(2)}`:'-'}</Table.Cell>
                <Table.Cell className="fd-tabular-nums">{tx.settlement_days!=null?`T+${tx.settlement_days}`:'-'}</Table.Cell>
                <Table.Cell>
                  <div style={{ display: 'flex', gap: space[1] }}>
                    {(tx.direction === 'buy') && (
                      <Button type="button"
                        variant="secondary"
                        size="sm"
                        onClick={() => onToggleType(tx.seq, tx.trade_type)}
                        aria-label={isAuto ? t('fundDetail.switchToManual') : t('fundDetail.switchToDca')}
                        title={isAuto ? t('fundDetail.switchToManual') : t('fundDetail.switchToDca')}
                      >
                        {isAuto ? <UserIcon size={14} aria-hidden /> : <RobotIcon size={14} aria-hidden />}
                      </Button>
                    )}
                    <Button type="button"
                      variant="secondary"
                      size="sm"
                      onClick={() => onDelete(tx.seq)}
                      disabled={deleting === tx.seq}
                      aria-label={`${t('fundDetail.deleteThisTx')} ${tx.seq}`}
                      title={t('fundDetail.deleteThisTx')}
                    >
                      <TrashIcon size={14} style={{ color: theme.critical }} aria-hidden />
                    </Button>
                  </div>
                </Table.Cell>
              </Table.Row>);
            })}
            {filteredTxs.length === 0 && (
              <Table.Row><Table.Cell colSpan={9} style={{ textAlign: 'center', padding: space[6] }}><Text variant="secondary" as="span">{transactions.length === 0 ? t('fundDetail.txTable.noTx') : t('fundDetail.txTable.noMatch')}</Text></Table.Cell></Table.Row>
            )}
          </Table.Body>
        </Table>
      </div>
    </Card>
  );
}
