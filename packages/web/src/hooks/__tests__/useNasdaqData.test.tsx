import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useNasdaqData } from '../useNasdaqData';
import * as api from '../../api';
import type { FundInfo, IndexHistory } from '../../api';

vi.mock('../../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api')>();
  return {
    ...actual,
    fetchFundDetail: vi.fn(),
    fetchIndexHistory: vi.fn(),
  };
});

const mockFunds: FundInfo[] = [
  {
    code: '019173',
    name: '纳斯达克100指数(QDII)C',
    type: 'QDII',
    security_type: 'fund',
    market: 'CN',
    held_shares: 1000,
    current_value: 15000,
    unrealized_pnl: 3000,
    pnl_pct: 25,
    latest_nav: 1.5,
  } as FundInfo,
  {
    code: '016533',
    name: '嘉实纳斯达克100ETF联接(QDII)C',
    type: 'QDII',
    security_type: 'fund',
    market: 'CN',
    held_shares: 500,
    current_value: 7500,
    unrealized_pnl: 500,
    pnl_pct: 7.14,
    latest_nav: 1.5,
  } as FundInfo,
];

const mockIndex = (range: string): IndexHistory => ({
  symbol: 'NDX',
  count: 2,
  range,
  data: [
    { date: '2026-01-01', close: 19000, change_pct: 0 },
    { date: '2026-06-01', close: 20000, change_pct: 1 },
  ],
});

const mockDetail = (code: string) => ({
  code,
  name: code,
  security_type: 'fund' as const,
  market: 'CN' as const,
  held_shares: 100,
  total_cost: -1000,
  latest_nav: 1.5,
  current_value: 1500,
  unrealized_pnl: 500,
  pnl_pct: 50,
  auto_buy_count: 1,
  manual_buy_count: 0,
  auto_buy_amount: 1000,
  manual_buy_amount: 0,
  auto_tx: 1,
  manual_tx: 0,
  buy_count: 1,
  sell_count: 0,
  median_settlement: 2,
  transactions: [
    {
      seq: 1,
      trade_time: '2026-03-01 10:00:00',
      confirm_date: '2026-03-02',
      trade_type: '定投买入',
      direction: 'buy' as const,
      amount: 800,
      shares: 500,
      fee: 0,
      nav: 1.4,
      inferred_nav: null,
      anomaly: null,
    },
  ],
});

describe('useNasdaqData', () => {
  beforeEach(() => {
    vi.mocked(api.fetchIndexHistory).mockImplementation(async (_sym, range) =>
      mockIndex(String(range ?? 'max')),
    );
    vi.mocked(api.fetchFundDetail).mockImplementation(async (code) => mockDetail(code) as any);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('fetches index history and fund details on mount', async () => {
    const { result } = renderHook(() =>
      useNasdaqData({ nasdaqFunds: mockFunds, range: 'tx' }),
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
      expect(result.current.allTx).toHaveLength(2);
    });

    expect(api.fetchIndexHistory).toHaveBeenCalledTimes(1);
    expect(api.fetchFundDetail).toHaveBeenCalledTimes(2);
    expect(api.fetchFundDetail).toHaveBeenCalledWith('019173', undefined, expect.any(AbortSignal));
    expect(api.fetchFundDetail).toHaveBeenCalledWith('016533', undefined, expect.any(AbortSignal));
  });

  it('passes portfolioId into fund detail fetches', async () => {
    const { result } = renderHook(() =>
      useNasdaqData({ nasdaqFunds: mockFunds, range: 'tx', portfolioId: 2 }),
    );

    await waitFor(() => {
      expect(result.current.allTx).toHaveLength(2);
    });

    expect(api.fetchFundDetail).toHaveBeenCalledWith('019173', 2, expect.any(AbortSignal));
    expect(api.fetchFundDetail).toHaveBeenCalledWith('016533', 2, expect.any(AbortSignal));
  });

  it('does not re-hit fund details when only range changes', async () => {
    const { result, rerender } = renderHook(
      ({ range }: { range: string }) =>
        useNasdaqData({ nasdaqFunds: mockFunds, range }),
      { initialProps: { range: 'tx' } },
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
      expect(result.current.allTx).toHaveLength(2);
    });

    expect(api.fetchIndexHistory).toHaveBeenCalledTimes(1);
    expect(api.fetchFundDetail).toHaveBeenCalledTimes(2);

    await act(async () => {
      rerender({ range: '1m' });
    });

    await waitFor(() => {
      expect(api.fetchIndexHistory).toHaveBeenCalledTimes(2);
    });

    // Range change must refetch index only — no detail fan-out.
    expect(api.fetchFundDetail).toHaveBeenCalledTimes(2);
    expect(result.current.allTx).toHaveLength(2);
  });

  it('refetches fund details when fund-code list changes', async () => {
    const { result, rerender } = renderHook(
      ({ funds }: { funds: FundInfo[] }) =>
        useNasdaqData({ nasdaqFunds: funds, range: 'tx' }),
      { initialProps: { funds: mockFunds } },
    );

    await waitFor(() => {
      expect(result.current.allTx).toHaveLength(2);
    });
    expect(api.fetchFundDetail).toHaveBeenCalledTimes(2);

    await act(async () => {
      rerender({ funds: [mockFunds[0]] });
    });

    await waitFor(() => {
      expect(api.fetchFundDetail).toHaveBeenCalledTimes(3);
    });
    expect(api.fetchFundDetail).toHaveBeenLastCalledWith('019173', undefined, expect.any(AbortSignal));
  });
});
