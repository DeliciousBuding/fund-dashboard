import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act, waitFor } from '@testing-library/react';
import AdminDashboard from '../../components/AdminDashboard';

/**
 * Regression test for the AbortController leak on the retry path.
 *
 * Previously `load()` created a fresh AbortController on every call but only
 * returned a cleanup consumed by useEffect on unmount — so clicking "retry"
 * orphaned the previous controller and its in-flight fetch. The fix aborts the
 * previous controller via a ref before starting a new request, and aborts on
 * unmount.
 */

const sampleDashboard = {
  ok: true,
  timestamp: '2026-06-21T00:00:00Z',
  response_ms: 12,
  system: {
    uptime_sec: 100,
    uptime_human: '1m',
    memory: { rss_mb: 50, heap_used_mb: 20, heap_total_mb: 40 },
    go_version: 'go1.25',
    platform: 'linux',
  },
  database: { size_bytes: 1024, size_mb: 1 },
  crawler: { nav_total: 1, nav_fresh_24h: 1, success_rate_pct: 100 },
  state: {
    transaction_count: 0, last_transaction: null, last_nav_date: null,
    held_funds: 0, nav_records: 0, nav_funds: 0, securities_total: 0,
    anomaly_count: 0, recent_anomalies: [],
  },
};

describe('AdminDashboard — AbortController cleanup', () => {
  let fetchMock: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    vi.useRealTimers();
    fetchMock = vi.fn();
    vi.stubGlobal('fetch', fetchMock);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  /** Helper: a fetch response that resolves to `body` (or rejects). */
  const resOk = (body: unknown) =>
    Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body) } as any);

  it('aborts the previous in-flight request when retry is clicked', async () => {
    let capturedSignals: AbortSignal[] = [];

    // First call rejects (triggers error UI), subsequent calls resolve.
    // IMPORTANT: mockImplementation must come BEFORE mockImplementationOnce,
    // otherwise mockImplementation wipes the one-shot queue.
    fetchMock.mockImplementation((_url: string, opts: any) => {
      capturedSignals.push(opts.signal);
      return resOk(sampleDashboard);
    });
    fetchMock.mockImplementationOnce((_url: string, opts: any) => {
      capturedSignals.push(opts.signal);
      return Promise.reject(new Error('HTTP 500'));
    });

    const { unmount } = render(<AdminDashboard />);

    // Wait for error UI / retry button.
    await waitFor(() => expect(screen.getByRole('button', { name: '重试' })).toBeInTheDocument(), { timeout: 3000 });
    expect(capturedSignals).toHaveLength(1);
    const firstSignal = capturedSignals[0];
    expect(firstSignal.aborted).toBe(false);

    // Click retry — must abort the previous controller before issuing a new fetch.
    act(() => {
      fireEvent.click(screen.getByRole('button', { name: '重试' }));
    });

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    // The previous (first) signal must now be aborted.
    expect(firstSignal.aborted).toBe(true);
    // A new, non-aborted signal was used for the second fetch.
    expect(capturedSignals[1].aborted).toBe(false);

    unmount();
    // After unmount, the latest controller is aborted too.
    expect(capturedSignals[1].aborted).toBe(true);
  });

  it('exposes section and anomaly table a11y labels', async () => {
    const withAnomalies = {
      ...sampleDashboard,
      state: {
        ...sampleDashboard.state,
        anomaly_count: 1,
        recent_anomalies: [{ seq: 1, fund_code: '000001', anomaly: 'stale nav' }],
      },
    };
    fetchMock.mockImplementation(() => resOk(withAnomalies));

    render(<AdminDashboard />);

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Admin 监控面板' })).toBeInTheDocument());

    expect(screen.getByRole('region', { name: '系统' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: '数据库' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: '爬虫' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: '最近异常' })).toBeInTheDocument();
    expect(screen.getByRole('table', { name: '最近异常' })).toBeInTheDocument();
    expect(screen.getByText('最近异常', { selector: 'caption' })).toBeInTheDocument();
  });
});
