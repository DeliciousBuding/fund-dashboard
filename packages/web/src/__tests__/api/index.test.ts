import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
  fetchPortfolio,
  fetchFunds,
  fetchFundDetail,
  fetchNav,
  fetchXirr,
  fetchDrawdown,
  fetchPortfolioXirr,
  fetchSecurities,
  fetchIndices,
  fetchUSStock,
  fetchExchangeRate,
  fetchIndexHistory,
  fetchIndexLive,
  fetchPortfolioPenetration,
  fetchPortfolioAllocation,
  fetchInvestmentHarness,
  fetchInvestmentSourceBrief,
  transactionsToCsv,
  downloadCsv,
} from '../../api';

afterEach(() => {
  vi.restoreAllMocks();
});

// ── fetchFunds ──────────────────────────────────────────────────
describe('fetchFunds', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('calls /api/funds and returns typed array', async () => {
    const mockFunds = [
      { code: '019173', name: '广发纳斯达克100ETF', type: 'QDII', held_shares: 100, current_value: 150, unrealized_pnl: 50, pnl_pct: 0.5, latest_nav: 1.5 },
    ];
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockFunds),
    } as Response);

    const result = await fetchFunds();
    expect(result).toEqual(mockFunds);
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/funds', { signal: undefined });
  });

  it('passes AbortSignal', async () => {
    const ctrl = new AbortController();
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve([]),
    } as Response);

    await fetchFunds(ctrl.signal);
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/funds', { signal: ctrl.signal });
  });

  it('throws on error response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
    } as Response);

    await expect(fetchFunds()).rejects.toThrow('HTTP 500: Internal Server Error');
  });
});

// ── fetchPortfolio ──────────────────────────────────────────────
describe('fetchPortfolio', () => {
  it('calls /api/portfolio and returns shape', async () => {
    const mockPortfolio = {
      total_tx: 100, unique_funds: 10, unique_stocks: 8, held_funds: 5,
      total_buy: 50000, total_sell: 10000, total_fee: 100,
      unrealized_pnl: 5000,
      auto_tx: 50, manual_tx: 50,
      auto_amount: 25000, manual_amount: 25000,
      first_trade: '2023-01-01', last_trade: '2024-01-01',
      last_nav_date: '2024-01-15',
      settlement_distribution: {}, trade_type_breakdown: {},
      by_security_type: [],
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPortfolio),
    } as Response);

    const result = await fetchPortfolio();
    expect(result).toEqual(mockPortfolio);
    expect(result).toHaveProperty('total_tx');
    expect(result).toHaveProperty('unrealized_pnl');
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/portfolio', { signal: undefined });
  });
});

describe('fetchPortfolioAllocation', () => {
  it('calls /api/portfolio/allocation and returns dashboard buckets', async () => {
    const mockAllocation = {
      total_value: 830,
      by_security_type: [{ key: 'stock', label: '股票', value: 680, weight_pct: 81.93, count: 2 }],
      by_market: [{ key: 'us_stock', label: '美股', value: 380, weight_pct: 45.78, count: 1 }],
      by_fund_type: [{ key: 'QDII-股票', label: 'QDII-股票', value: 150, weight_pct: 18.07, count: 1 }],
      risk_flags: ['股票资产占比高于 80%'],
      agent_brief: '资产配置：股票 81.93%。风险提示：股票资产占比高于 80%。',
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockAllocation),
    } as Response);

    const result = await fetchPortfolioAllocation();
    expect(result.total_value).toBe(830);
    expect(result.risk_flags).toContain('股票资产占比高于 80%');
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/portfolio/allocation', { signal: undefined });
  });
});

describe('fetchInvestmentHarness', () => {
  it('calls /api/portfolio/harness and returns facts-only agent context', async () => {
    const mockHarness = {
      generated_at: '2026-06-19T00:00:00.000Z',
      decision_boundary: 'facts_only',
      total_value: 830,
      holdings_count: 1,
      allocation: {
        total_value: 830,
        by_security_type: [],
        by_market: [],
        by_fund_type: [],
        risk_flags: [],
        agent_brief: '资产配置',
      },
      holding_signals: [{
        code: '019173',
        name: '纳斯达克100',
        security_type: 'fund',
        market: 'CN',
        held_shares: 100,
        current_value: 150,
        weight_pct: 18.07,
        latest_nav: 1.5,
        cost_per_share: 1.2,
        change_pct: -4.2,
        deviation_pct: 25,
        signal_tags: ['above_cost_gt_10pct'],
        data_points: { has_price: true, has_cost_basis: true, has_change_pct: true },
      }],
      data_quality: { stale_price_count: 0, missing_cost_basis_count: 0, missing_change_pct_count: 0, holdings_coverage_pct: 100 },
      available_agent_tools: ['get_fund_detail'],
      agent_brief: 'Agent owns all investment decisions',
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockHarness),
    } as Response);

    const result = await fetchInvestmentHarness();
    expect(result.decision_boundary).toBe('facts_only');
    expect(result.holding_signals[0].signal_tags).toContain('above_cost_gt_10pct');
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/portfolio/harness', { signal: undefined });
  });
});

describe('fetchInvestmentSourceBrief', () => {
  it('calls /api/portfolio/source-brief and returns source queries', async () => {
    const mockBrief = {
      generated_at: '2026-06-19T00:00:00.000Z',
      decision_boundary: 'source_queries_only',
      queries: [{
        id: 'holding-AAPL',
        scope: 'holding',
        entity_code: 'AAPL',
        entity_name: 'Apple Inc.',
        query: 'Apple AAPL earnings news',
        reason: '持仓消息源',
        freshness: 'intraday',
      }],
      source_targets: [{
        kind: 'web_search',
        name: 'Hermes WebSearch',
        url_template: null,
        use_for: '新闻检索',
      }],
      coverage: { holdings_scanned: 1, underlying_scanned: 0, max_queries: 5 },
      agent_brief: 'Hermes source brief',
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockBrief),
    } as Response);

    const result = await fetchInvestmentSourceBrief(5);
    expect(result.decision_boundary).toBe('source_queries_only');
    expect(result.queries[0].query).toContain('Apple');
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/portfolio/source-brief?limit=5', { signal: undefined });
  });
});

// ── fetchFundDetail ─────────────────────────────────────────────
describe('fetchFundDetail', () => {
  it('calls /api/funds/019173', async () => {
    const mockDetail = {
      code: '019173', name: 'Test',
      held_shares: 100, total_cost: 15000, latest_nav: 1.5, current_value: 15000,
      unrealized_pnl: 0, pnl_pct: 0,
      auto_buy_count: 5, manual_buy_count: 3,
      auto_buy_amount: 5000, manual_buy_amount: 10000,
      auto_tx: 5, manual_tx: 3,
      buy_count: 8, sell_count: 0, median_settlement: 2,
      transactions: [],
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockDetail),
    } as Response);

    const result = await fetchFundDetail('019173');
    expect(result).toHaveProperty('code', '019173');
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/funds/019173', { signal: undefined });
  });
});

// ── fetchNav ────────────────────────────────────────────────────
describe('fetchNav', () => {
  it('calls /api/funds/019173/nav', async () => {
    const mockNav = [{ date: '2024-01-01', unit_nav: 1.5 }];
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockNav),
    } as Response);

    const result = await fetchNav('019173');
    expect(result).toEqual(mockNav);
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/funds/019173/nav', { signal: undefined });
  });
});

// ── fetchXirr ───────────────────────────────────────────────────
describe('fetchXirr', () => {
  it('calls /api/funds/019173/xirr', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ xirr: 0.15 }),
    } as Response);

    const result = await fetchXirr('019173');
    expect(result).toHaveProperty('xirr', 0.15);
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/funds/019173/xirr', { signal: undefined });
  });
});

// ── fetchUSStock ────────────────────────────────────────────────
describe('fetchUSStock', () => {
  it('calls /api/stocks/AAPL', async () => {
    const mockUSStock = {
      code: 'AAPL', name: 'Apple Inc.', market: 'us',
      price: 180, previous_close: 178, change: 2, change_pct: 1.12,
      high: 181, low: 177, open: 178.5, volume: 50000000,
      currency: 'USD', market_time: '2024-01-15T16:00:00Z',
      profile: { sector: 'Technology', industry: 'Consumer Electronics', market_cap: 3000000000000, pe: 30.5, description: 'Apple Inc.' },
      history: [{ date: '2024-01-15', close: 180, change_pct: 1.12 }],
      source: 'live',
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockUSStock),
    } as Response);

    const result = await fetchUSStock('AAPL');
    expect(result).toHaveProperty('code', 'AAPL');
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/stocks/AAPL', { signal: undefined });
  });
});

// ── fetchDrawdown ───────────────────────────────────────────────
describe('fetchDrawdown', () => {
  it('calls /api/funds/019173/drawdown', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ max_drawdown: 0.25, peak_date: '2024-01-01', trough_date: '2024-03-01' }),
    } as Response);

    const result = await fetchDrawdown('019173');
    expect(result).toHaveProperty('max_drawdown', 0.25);
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/funds/019173/drawdown', { signal: undefined });
  });
});

// ── fetchPortfolioXirr ──────────────────────────────────────────
describe('fetchPortfolioXirr', () => {
  it('calls /api/portfolio/xirr', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ xirr: 0.12 }),
    } as Response);

    const result = await fetchPortfolioXirr();
    expect(result).toHaveProperty('xirr', 0.12);
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/portfolio/xirr', { signal: undefined });
  });
});

// ── fetchSecurities ─────────────────────────────────────────────
describe('fetchSecurities', () => {
  it('calls /api/securities', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve([]),
    } as Response);

    const result = await fetchSecurities();
    expect(Array.isArray(result)).toBe(true);
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/securities', { signal: undefined });
  });
});

// ── fetchIndices ────────────────────────────────────────────────
describe('fetchIndices', () => {
  it('calls /api/market/indices', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve([]),
    } as Response);

    const result = await fetchIndices();
    expect(Array.isArray(result)).toBe(true);
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/market/indices', { signal: undefined });
  });
});

// ── fetchExchangeRate ───────────────────────────────────────────
describe('fetchExchangeRate', () => {
  it('calls /api/market/exchange-rate', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ from: 'USD', to: 'CNY', rate: 7.25, updated_at: '2024-01-01' }),
    } as Response);

    const result = await fetchExchangeRate();
    expect(result).toHaveProperty('rate', 7.25);
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/market/exchange-rate', { signal: undefined });
  });
});

// ── fetchIndexHistory ───────────────────────────────────────────
describe('fetchIndexHistory', () => {
  it('calls /api/market/index/IXIC/history?range=1y', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ symbol: 'IXIC', count: 250, range: '1y', data: [] }),
    } as Response);

    const result = await fetchIndexHistory('IXIC');
    expect(result).toHaveProperty('symbol', 'IXIC');
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/market/index/IXIC/history?range=1y', { signal: undefined });
  });
});

// ── fetchIndexLive ──────────────────────────────────────────────
describe('fetchIndexLive', () => {
  it('calls /api/market/index/IXIC', async () => {
    const mockLiveIndex = {
      code: '^IXIC', name: 'Nasdaq', market: 'us',
      price: 18000, change_pct: 1.5, change_amt: 270, updated_at: '2024-01-15',
      source: 'live',
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockLiveIndex),
    } as Response);

    const result = await fetchIndexLive('IXIC');
    expect(result).toHaveProperty('source', 'live');
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/market/index/IXIC', { signal: undefined });
  });
});

// ── fetchPortfolioPenetration ───────────────────────────────────
describe('fetchPortfolioPenetration', () => {
  it('calls /api/portfolio/penetration', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ penetration: [], total_portfolio_value: 100000, equity_fund_count: 5, unique_stocks: 20 }),
    } as Response);

    const result = await fetchPortfolioPenetration();
    expect(result).toHaveProperty('total_portfolio_value', 100000);
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/portfolio/penetration', { signal: undefined });
  });
});

// ── HTTP error propagation ─────────────────────────────────────
describe('error handling', () => {
  it('propagates non-ok responses as ApiError', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: false,
      status: 404,
      statusText: 'Not Found',
    } as Response);

    await expect(fetchFundDetail('999999')).rejects.toThrow('HTTP 404: Not Found');
  });

  it('propagates network errors', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'));

    await expect(fetchFunds()).rejects.toThrow('Network error');
  });
});

// ── transactionsToCsv ───────────────────────────────────────────
describe('transactionsToCsv', () => {
  it('generates CSV with headers', () => {
    const csv = transactionsToCsv([], '测试基金');
    expect(csv).toContain('交易时间');
    expect(csv).toContain('确认日期');
    expect(csv).toContain('类型');
    expect(csv).toContain('金额');
  });

  it('includes BOM prefix', () => {
    const csv = transactionsToCsv([], '测试');
    expect(csv.charCodeAt(0)).toBe(0xFEFF);
  });

  it('generates rows for transactions', () => {
    const txs = [{
      seq: 1,
      trade_time: '2024-01-15 10:30:00',
      confirm_date: '2024-01-16',
      trade_type: '用户买入',
      direction: 'buy',
      amount: 1000,
      shares: 666.67,
      fee: 1.5,
      nav: 1.5,
      inferred_nav: 1.4995,
      nav_verified: true,
      pnl: null,
      trade_day_type: 'trading_before_cutoff',
      settlement_days: 1,
      effective_nav_date: '2024-01-15',
      order_id: 'web_123',
    }];

    const csv = transactionsToCsv(txs, '测试');
    expect(csv).toContain('2024-01-15 10:30');
    expect(csv).toContain('买入');
    expect(csv).toContain('1000.00');
    expect(csv).toContain('666.67');
    expect(csv).toContain('1.5000');
    expect(csv).toContain('1.499500');
    expect(csv).toContain('1.50');
    expect(csv).toContain('T+1');
  });

  it('handles null values', () => {
    const txs = [{
      seq: 1,
      trade_time: '2024-01-15 10:30:00',
      confirm_date: '2024-01-16',
      trade_type: '用户买入',
      direction: 'buy',
      amount: 1000,
      shares: 666.67,
      fee: 0,
      nav: null,
      inferred_nav: null,
      nav_verified: null,
      pnl: null,
      trade_day_type: '',
      settlement_days: null,
      effective_nav_date: '2024-01-15',
      order_id: '',
    }];

    const csv = transactionsToCsv(txs, '测试');
    expect(csv).toContain('-');
  });
});

// ── downloadCsv ─────────────────────────────────────────────────
describe('downloadCsv', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('creates a download link and triggers click', () => {
    const createObjectURL = vi.fn().mockReturnValue('blob:test');
    const revokeObjectURL = vi.fn();
    URL.createObjectURL = createObjectURL;
    URL.revokeObjectURL = revokeObjectURL;

    const appendChild = vi.spyOn(document.body, 'appendChild').mockImplementation((node) => node);
    const removeChild = vi.spyOn(document.body, 'removeChild').mockImplementation((node) => node);

    // Mock anchor click
    const clickSpy = vi.fn();
    const origCreateElement = document.createElement.bind(document);
    vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = origCreateElement(tag);
      if (tag === 'a') {
        vi.spyOn(el, 'click').mockImplementation(clickSpy);
      }
      return el;
    });

    downloadCsv('test,csv', 'test.csv');

    expect(createObjectURL).toHaveBeenCalled();
    expect(clickSpy).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalled();
  });
});
