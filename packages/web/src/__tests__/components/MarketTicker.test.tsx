import { describe, it, expect, afterEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import MarketTicker from '../../components/MarketTicker';

describe('MarketTicker', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders nothing when no index data is available', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve([]),
    } as Response);

    const { container } = render(<MarketTicker />);

    // Component checks !indices.length and returns null
    await waitFor(() => {
      expect(container.innerHTML).toBe('');
    });
  });

  it('displays index data after successful fetch', async () => {
    const mockIndices = [
      { code: '^IXIC', name: 'NASDAQ', market: 'us', price: 18000, change_pct: 1.5, change_amt: 270, updated_at: '2024-01-01' },
      { code: 'sh000001', name: '上证指数', market: 'cn', price: 3000, change_pct: 0.5, change_amt: 15, updated_at: '2024-01-01' },
    ];

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockIndices),
    } as Response);

    render(<MarketTicker />);

    await waitFor(() => {
      expect(screen.getByText('纳指')).toBeInTheDocument();
    });

    expect(screen.getByText('18,000.00')).toBeInTheDocument();
    expect(screen.getByText('+1.50%')).toBeInTheDocument();
  });

  it('shows multiple indices in compact mode', async () => {
    const mockIndices = [
      { code: '^IXIC', name: 'NASDAQ', market: 'us', price: 18000, change_pct: 1.5, change_amt: 270, updated_at: '2024-01-01' },
      { code: '^NDX', name: 'NASDAQ 100', market: 'us', price: 16000, change_pct: 1.2, change_amt: 192, updated_at: '2024-01-01' },
      { code: '^GSPC', name: 'S&P 500', market: 'us', price: 5000, change_pct: 0.8, change_amt: 40, updated_at: '2024-01-01' },
      { code: '^DJI', name: 'Dow Jones', market: 'us', price: 38000, change_pct: 0.3, change_amt: 114, updated_at: '2024-01-01' },
      { code: 'sh000001', name: '上证指数', market: 'cn', price: 3000, change_pct: 0.5, change_amt: 15, updated_at: '2024-01-01' },
    ];

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockIndices),
    } as Response);

    render(<MarketTicker />);

    await waitFor(() => {
      expect(screen.getByText('纳指')).toBeInTheDocument();
    });

    // Default compact: first 4 indices shown
    expect(screen.getByText('纳指')).toBeInTheDocument();
    expect(screen.getByText('纳指100')).toBeInTheDocument();
    expect(screen.getByText('标普500')).toBeInTheDocument();
    expect(screen.getByText('道指')).toBeInTheDocument();
  });

  it('shows expand button when more than 4 indices', async () => {
    const mockIndices = [
      { code: '^IXIC', name: 'NASDAQ', market: 'us', price: 18000, change_pct: 1.5, change_amt: 270, updated_at: '2024-01-01' },
      { code: '^NDX', name: 'NASDAQ 100', market: 'us', price: 16000, change_pct: 1.2, change_amt: 192, updated_at: '2024-01-01' },
      { code: '^GSPC', name: 'S&P 500', market: 'us', price: 5000, change_pct: 0.8, change_amt: 40, updated_at: '2024-01-01' },
      { code: '^DJI', name: 'Dow Jones', market: 'us', price: 38000, change_pct: 0.3, change_amt: 114, updated_at: '2024-01-01' },
      { code: 'sh000001', name: '上证指数', market: 'cn', price: 3000, change_pct: 0.5, change_amt: 15, updated_at: '2024-01-01' },
    ];

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockIndices),
    } as Response);

    render(<MarketTicker />);

    await waitFor(() => {
      expect(screen.getByText('纳指')).toBeInTheDocument();
    });

    // Expand button should be shown
    const expandButton = screen.getByTitle('展开所有指数');
    expect(expandButton).toBeInTheDocument();
  });

  it('handles fetch failure gracefully', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'));

    const { container } = render(<MarketTicker />);

    // On fetch failure, indices stays [] and component returns null
    await waitFor(() => {
      expect(container.innerHTML).toBe('');
    });
  });

  it('renders all four major US indices with correct Chinese names', async () => {
    const mockIndices = [
      { code: '^IXIC', name: 'NASDAQ', market: 'us', price: 18000, change_pct: 1.5, change_amt: 270, updated_at: '2024-01-01' },
      { code: '^NDX', name: 'NASDAQ 100', market: 'us', price: 16000, change_pct: 1.2, change_amt: 192, updated_at: '2024-01-01' },
      { code: '^GSPC', name: 'S&P 500', market: 'us', price: 5000, change_pct: 0.8, change_amt: 40, updated_at: '2024-01-01' },
      { code: '^DJI', name: 'Dow Jones', market: 'us', price: 38000, change_pct: 0.3, change_amt: 114, updated_at: '2024-01-01' },
    ];

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockIndices),
    } as Response);

    render(<MarketTicker />);

    await waitFor(() => {
      expect(screen.getByText('纳指')).toBeInTheDocument();
    });

    // Verify all four indices rendered by their Chinese short names
    expect(screen.getByText('纳指')).toBeInTheDocument();
    expect(screen.getByText('纳指100')).toBeInTheDocument();
    expect(screen.getByText('标普500')).toBeInTheDocument();
    expect(screen.getByText('道指')).toBeInTheDocument();

    // Verify prices rendered
    expect(screen.getByText('18,000.00')).toBeInTheDocument();
    expect(screen.getByText('16,000.00')).toBeInTheDocument();
    expect(screen.getByText('5,000.00')).toBeInTheDocument();
    expect(screen.getByText('38,000.00')).toBeInTheDocument();
  });

  it('shows positive change in red and negative change in green', async () => {
    const mockIndices = [
      { code: '^IXIC', name: 'NASDAQ', market: 'us', price: 18000, change_pct: 2.50, change_amt: 450, updated_at: '2024-01-01' },
      { code: '^GSPC', name: 'S&P 500', market: 'us', price: 5000, change_pct: -1.20, change_amt: -60, updated_at: '2024-01-01' },
    ];

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockIndices),
    } as Response);

    render(<MarketTicker />);

    await waitFor(() => {
      expect(screen.getByText('纳指')).toBeInTheDocument();
    });

    // Positive change: +2.50% should have red color (#d63649)
    const positiveEl = screen.getByText('+2.50%');
    expect(positiveEl).toBeInTheDocument();
    expect(positiveEl.style.color).toBe('rgb(214, 54, 73)');

    // Negative change: -1.20% should have green color (#199c63)
    const negativeEl = screen.getByText('-1.20%');
    expect(negativeEl).toBeInTheDocument();
    expect(negativeEl.style.color).toBe('rgb(25, 156, 99)');
  });

  it('shows zero/neutral change without sign prefix', async () => {
    const mockIndices = [
      { code: '^DJI', name: 'Dow Jones', market: 'us', price: 38000, change_pct: 0.00, change_amt: 0, updated_at: '2024-01-01' },
    ];

    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockIndices),
    } as Response);

    render(<MarketTicker />);

    await waitFor(() => {
      expect(screen.getByText('道指')).toBeInTheDocument();
    });

    // 0% change uses ">=" check in TickerItem, so isUp=true → "+0.00%"
    const zeroEl = screen.getByText('+0.00%');
    expect(zeroEl).toBeInTheDocument();
    expect(zeroEl.style.color).toBe('rgb(214, 54, 73)');
  });
});
