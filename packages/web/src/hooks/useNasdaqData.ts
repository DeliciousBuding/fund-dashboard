// useNasdaqData — fetch NDX index history + per-fund transaction data for NasdaqOverview.
// Extracted from NasdaqOverview.tsx (v3.0 refactor) to separate data fetching/computation
// from rendering. Handles AbortController lifecycle and derived stat computations.
//
// Index history refetches when `range` changes; fund details only when the fund-code
// list changes (range alone must not fan out N detail requests).
import { useState, useEffect, useMemo, useRef } from 'react';
import {
  fetchFundDetail, fetchIndexHistory,
  type FundInfo, type IndexHistory, type Transaction,
} from '../api';
import { mapRangeToYahoo, getIntervalForRange } from '../services/marketTime';
import { getDateRange } from '../components/charts';
import i18n from '../i18n';
import { sanitizeUserError } from '../services/userError';

export interface NasdqStats {
  totalBuyCount: number;
  totalSellCount: number;
  totalBuyAmt: number;
  totalSellAmt: number;
  heldFunds: FundInfo[];
  clearedFunds: FundInfo[];
  navPnl: number;
  navValue: number;
  latestReturn: number;
}

export interface FundTransactions {
  code: string;
  name: string;
  tx: Transaction[];
}

export interface UseNasdaDataResult {
  indexData: IndexHistory | null;
  allTx: FundTransactions[];
  /** All dates from index history, sorted */
  indexDates: string[];
  /** All closing prices from index history */
  indexCloses: number[];
  /** All unique transaction dates, sorted */
  allTxDates: string[];
  stats: NasdqStats;
  loading: boolean;
  error: string | null;
}

interface UseNasdaDataOptions {
  nasdaqFunds: FundInfo[];
  range: string;
  /** Scope fund detail / transactions to active portfolio when available. */
  portfolioId?: number;
}

export function useNasdaqData({ nasdaqFunds, range, portfolioId }: UseNasdaDataOptions): UseNasdaDataResult {
  const [indexData, setIndexData] = useState<IndexHistory | null>(null);
  const [allTx, setAllTx] = useState<FundTransactions[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const indexAbortRef = useRef<AbortController | null>(null);
  const fundsAbortRef = useRef<AbortController | null>(null);
  // Latest fund list for name lookup without re-running the detail effect on ref churn.
  const fundsRef = useRef(nasdaqFunds);
  fundsRef.current = nasdaqFunds;

  // Stable code-list key — detail fan-out only when membership changes, not on range.
  const fundCodesKey = nasdaqFunds.map(f => f.code).join(',');

  // Index history: depends on range (+ whether any funds exist).
  useEffect(() => {
    indexAbortRef.current?.abort();
    const ctrl = new AbortController();
    indexAbortRef.current = ctrl;
    const sig = ctrl.signal;

    if (!fundCodesKey) {
      setLoading(false);
      return;
    }

    setLoading(true);
    setError(null);

    const yahooRange = mapRangeToYahoo(range);
    const interval = getIntervalForRange(range);

    fetchIndexHistory('NDX', yahooRange, interval, sig)
      .then(d => {
        if (!sig.aborted) {
          setIndexData(d);
          setLoading(false);
        }
      })
      .catch(e => {
        if (e?.name !== 'AbortError') {
          console.warn('[nasdaqIndex]', e);
          // #194: sanitize raw fetch/Yahoo errors before ChartShell
          setError(sanitizeUserError(e, i18n.t('common.loadError')));
          setLoading(false);
        }
      });

    return () => { ctrl.abort(); };
  }, [range, fundCodesKey]);

  // Fund details / transactions: depend only on fund-code list (NOT range).
  useEffect(() => {
    fundsAbortRef.current?.abort();
    const ctrl = new AbortController();
    fundsAbortRef.current = ctrl;
    const sig = ctrl.signal;

    const funds = fundsRef.current;
    if (!fundCodesKey || !funds.length) {
      setAllTx([]);
      return;
    }

    Promise.all(funds.map(f =>
      fetchFundDetail(f.code, portfolioId, sig).then(d => ({
        code: f.code,
        name: f.name,
        tx: d.transactions.filter((t: Transaction) => t.direction === 'buy' || t.direction === 'sell'),
      }))
    ))
      .then(d => { if (!sig.aborted) setAllTx(d); })
      .catch(e => { if (e.name !== 'AbortError') console.warn('[nasdaqTx]', e); });

    return () => { ctrl.abort(); };
  }, [fundCodesKey, portfolioId]);

  // Derived data — dates from index + transactions
  const indexDates = useMemo(() =>
    indexData?.data.map(d => d.date) ?? [],
    [indexData]
  );
  const indexCloses = useMemo(() =>
    indexData?.data.map(d => d.close) ?? [],
    [indexData]
  );
  const allTxDates = useMemo(() => {
    const dates = new Set<string>();
    allTx.forEach(({ tx }) => tx.forEach(t => dates.add(t.trade_time.substring(0, 10))));
    return [...dates].sort();
  }, [allTx]);

  // Stats
  const stats = useMemo((): NasdqStats => {
    const totalBuyCount = allTx.reduce((s, { tx }) => s + tx.filter(t => t.direction === 'buy').length, 0);
    const totalSellCount = allTx.reduce((s, { tx }) => s + tx.filter(t => t.direction === 'sell').length, 0);
    const totalBuyAmt = allTx.reduce((s, { tx }) => s + tx.filter(t => t.direction === 'buy').reduce((a, t) => a + t.amount, 0), 0);
    const totalSellAmt = allTx.reduce((s, { tx }) => s + tx.filter(t => t.direction === 'sell').reduce((a, t) => a + t.amount, 0), 0);
    const heldFunds = nasdaqFunds.filter(f => (f.held_shares ?? 0) > 0.001);
    const navPnl = heldFunds.reduce((s, f) => s + (f.unrealized_pnl || 0), 0);
    const navValue = heldFunds.reduce((s, f) => s + (f.current_value || 0), 0);

    let latestReturn = 0;
    if (indexData?.data.length) {
      const [i0, i1] = getDateRange(range, indexDates, allTxDates);
      const base = indexCloses[i0] || 1;
      const latest = indexCloses[i1] || base;
      latestReturn = +(((latest - base) / base) * 100).toFixed(2);
    }

    return {
      totalBuyCount, totalSellCount, totalBuyAmt, totalSellAmt,
      heldFunds, clearedFunds: nasdaqFunds.filter(f => (f.held_shares ?? 0) <= 0.001),
      navPnl, navValue, latestReturn,
    };
  }, [allTx, nasdaqFunds, indexData, indexDates, indexCloses, allTxDates, range]);

  return { indexData, allTx, indexDates, indexCloses, allTxDates, stats, loading, error };
}
