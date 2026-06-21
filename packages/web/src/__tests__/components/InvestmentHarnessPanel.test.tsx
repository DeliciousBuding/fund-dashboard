import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import InvestmentHarnessPanel from '../../components/InvestmentHarnessPanel';

describe('InvestmentHarnessPanel', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders facts-only harness signals without suggested amounts', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({
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
      }),
    } as Response).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({
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
        coverage: { holdings_scanned: 1, underlying_scanned: 0, max_queries: 8 },
        agent_brief: 'Hermes source brief',
      }),
    } as Response).mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({
        count: 1,
        decision_boundary: 'facts_only',
        events: [{
          id: 1,
          title: '纳指100 ETF 资金流入创纪录',
          url: null,
          source: 'websearch',
          snippet: '纳斯达克100相关ETF本周资金净流入...',
          query: '纳斯达克 QDII 资金流向',
          related_security_code: '019173',
          related_security_name: '纳斯达克100指数(QDII)C',
          is_read: false,
          is_useful: false,
          fetched_at: '2026-06-19 12:00:00',
          created_at: '2026-06-19 12:00:00',
        }],
      }),
    } as Response);

    render(<InvestmentHarnessPanel />);

    await waitFor(() => expect(screen.getByText('Agent Harness')).toBeInTheDocument());
    expect(screen.getByText('纳斯达克100')).toBeInTheDocument();
    expect(screen.getByText('高于成本 >10%')).toBeInTheDocument();
    expect(screen.getByText('消息源查询')).toBeInTheDocument();
    expect(screen.getByText('Apple AAPL earnings news')).toBeInTheDocument();
    expect(screen.getByText('来源事件队列')).toBeInTheDocument();
    expect(screen.getByText('纳指100 ETF 资金流入创纪录')).toBeInTheDocument();
    expect(screen.queryByText(/建议扣款/)).not.toBeInTheDocument();
  });
});
