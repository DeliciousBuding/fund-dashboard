import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import DcaPanel from '../../components/DcaPanel';

describe('DcaPanel', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  const mockPlan = {
    fund_code: '019173',
    mode: 'nav_deviation' as const,
    base_amount: 100,
    latest_nav: 1.5,
    cost_per_share: 1.2,
    change_pct: 3.5,
    deviation_pct: 25,
    dca_rate: 0.8,
    actual_amount: 80,
    signal: 'decrease',
    range: '-20%~-10%',
    explanation: 'legacy explanation',
  };

  it('renders fallback plan with initial props and computes real plan on click', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPlan),
    } as Response);

    render(<DcaPanel fundCode="019173" heldShares={1000} latestNav={1.5} totalCost={-1200} dark={false} />);

    // Initial fallback plan renders immediately (cost_per_share = 1200/1000 = 1.2000)
    expect(screen.getByText('定投计算器')).toBeInTheDocument();
    expect(screen.getByText(/待计算/)).toBeInTheDocument();
    expect(screen.getByText('选择模式后计算模拟扣款。')).toBeInTheDocument();

    // Click compute → fetch triggers → real plan replaces fallback
    fireEvent.click(screen.getByRole('button', { name: '计算' }));

    await waitFor(() => {
      expect(screen.getByText(/80% · 减仓/)).toBeInTheDocument();
    });

    expect(screen.getByText(/当前价格相对成本偏离 25.00%，减仓，投入 80.00/)).toBeInTheDocument();
    expect(screen.getByText('¥ 80.00')).toBeInTheDocument();
    expect(screen.queryByText(/建议扣款/)).not.toBeInTheDocument();
  });

  it('switches between cost deviation and change pct mode tabs', async () => {
    const changePctPlan = { ...mockPlan, mode: 'change_pct' as const, signal: 'dip_buy', explanation: 'legacy change expl' };

    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockPlan),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(changePctPlan),
      } as Response);

    render(<DcaPanel fundCode="019173" heldShares={1000} latestNav={1.5} totalCost={-1200} dark={false} />);

    // Click cost deviation mode tab and compute
    fireEvent.click(screen.getByText('成本偏离'));
    fireEvent.click(screen.getByRole('button', { name: '计算' }));

    await waitFor(() => {
      expect(screen.getByText(/80% · 减仓/)).toBeInTheDocument();
    });

    // Switch to change pct mode — auto-recompute (#190) uses second mock
    fireEvent.click(screen.getByText('涨跌幅模式'));

    await waitFor(() => {
      expect(screen.getByText(/80% · 跌幅加仓/)).toBeInTheDocument();
    });
    expect(screen.getByText(/最近涨跌幅 3.50%，跌幅加仓，投入 80.00/)).toBeInTheDocument();
  });

  it('displays base_amount and signal in plan output', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPlan),
    } as Response);

    render(<DcaPanel fundCode="019173" heldShares={1000} latestNav={1.5} totalCost={-1200} dark={false} />);

    fireEvent.click(screen.getByRole('button', { name: '计算' }));

    await waitFor(() => {
      expect(screen.getByText('¥ 80.00')).toBeInTheDocument();
    });

    // base_amount (100) reflected in the actual_amount display
    expect(screen.getByText('模拟扣款')).toBeInTheDocument();
    // signal displayed alongside dca_rate
    expect(screen.getByText(/80% · 减仓/)).toBeInTheDocument();
    // explanation shown
    expect(screen.getByText(/当前价格相对成本偏离 25.00%，减仓，投入 80.00/)).toBeInTheDocument();
  });

  it('never renders "建议扣款" advisory text', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockPlan),
    } as Response);

    render(<DcaPanel fundCode="019173" heldShares={1000} latestNav={1.5} totalCost={-1200} dark={false} />);

    // Before compute
    expect(screen.queryByText(/建议扣款/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '计算' }));

    await waitFor(() => {
      expect(screen.getByText(/80% · 减仓/)).toBeInTheDocument();
    });

    // After compute
    expect(screen.queryByText(/建议扣款/)).not.toBeInTheDocument();
  });
});
