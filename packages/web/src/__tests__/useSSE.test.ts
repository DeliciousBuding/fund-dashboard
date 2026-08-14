import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSSE } from '../hooks/useSSE';

/**
 * Local, controllable EventSource mock.
 *
 * The shared setup.ts stub is static (no way to drive readyState from a test).
 * For the backoff tests we need to fire onopen / onerror and control readyState,
 * so we install a richer mock via vi.stubGlobal per test.
 */
const CLOSED = 2;
const CONNECTING = 0;

class MockEventSource {
  // Static constants the real EventSource exposes; the hook reads these.
  static readonly CONNECTING = CONNECTING;
  static readonly OPEN = 1;
  static readonly CLOSED = CLOSED;

  static instances: MockEventSource[] = [];
  url: string;
  onopen: ((e?: unknown) => void) | null = null;
  onmessage: ((e: unknown) => void) | null = null;
  onerror: ((e?: unknown) => void) | null = null;
  readyState = CONNECTING;
  closed = false;

  constructor(url: string) {
    this.url = url;
    MockEventSource.instances.push(this);
  }
  addEventListener() {}
  removeEventListener() {}
  close() {
    this.closed = true;
    this.readyState = CLOSED;
  }
}

function installES() {
  MockEventSource.instances = [];
  vi.stubGlobal('EventSource', MockEventSource);
  return MockEventSource;
}

/**
 * Precisely measure the reconnect delay scheduled by the hook: advance fake time
 * in 1ms steps and return the number of ms elapsed when the next EventSource is
 * constructed (i.e. the pending reconnect setTimeout fired). Asserts a reconnect
 * actually happened (throws if not within the cap) so tests fail loudly otherwise.
 */
const measureReconnectDelayMs = (capMs = 120000): number => {
  const before = MockEventSource.instances.length;
  let advanced = 0;
  while (advanced < capMs) {
    act(() => { vi.advanceTimersByTime(1); });
    advanced += 1;
    if (MockEventSource.instances.length > before) return advanced;
  }
  throw new Error(`reconnect did not fire within ${capMs}ms`);
};

/** Drive a CLOSED failure on the latest EventSource. */
const failClosed = () => {
  const es = MockEventSource.instances[MockEventSource.instances.length - 1];
  es.readyState = CLOSED;
  act(() => { es.onerror && es.onerror(); });
};

describe('useSSE — exponential backoff', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    installES();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('grows the reconnect delay across consecutive CLOSED failures (5→10→20→40)', () => {
    const onMessage = vi.fn();
    renderHook(() => useSSE('https://x.test/sse', onMessage, { reconnectMs: 5000 }));

    failClosed();
    const d1 = measureReconnectDelayMs();
    failClosed();
    const d2 = measureReconnectDelayMs();
    failClosed();
    const d3 = measureReconnectDelayMs();
    failClosed();
    const d4 = measureReconnectDelayMs();

    expect(d1).toBe(5000);
    expect(d2).toBe(10000);
    expect(d3).toBe(20000);
    expect(d4).toBe(40000);

    // A new EventSource was created for each reconnect (4 reconnects + 1 init).
    expect(MockEventSource.instances).toHaveLength(5);
  });

  it('caps the backoff at 60s', () => {
    const onMessage = vi.fn();
    renderHook(() => useSSE('https://x.test/sse', onMessage, { reconnectMs: 5000 }));

    failClosed(); measureReconnectDelayMs(); // 5s
    failClosed(); measureReconnectDelayMs(); // 10s
    failClosed(); measureReconnectDelayMs(); // 20s
    failClosed(); measureReconnectDelayMs(); // 40s
    failClosed(); const d5 = measureReconnectDelayMs(); // capped
    failClosed(); const d6 = measureReconnectDelayMs(); // still capped

    expect(d5).toBe(60000);
    expect(d6).toBe(60000);
  });

  it('resets backoff to base on a successful onopen', () => {
    const onMessage = vi.fn();
    renderHook(() => useSSE('https://x.test/sse', onMessage, { reconnectMs: 5000 }));

    // Two failures grow the backoff (5s, 10s).
    failClosed(); measureReconnectDelayMs();
    failClosed(); measureReconnectDelayMs();

    // Now succeed: fire onopen on the latest EventSource.
    const openES = MockEventSource.instances[MockEventSource.instances.length - 1];
    act(() => { openES.onopen && openES.onopen(); });

    // The next failure should use the BASE delay again (~5s), not the grown one.
    failClosed();
    const dAfterOpen = measureReconnectDelayMs();
    expect(dAfterOpen).toBe(5000);
  });

  it('watchdog forces reconnect after reconnectMs when stuck in CONNECTING', () => {
    const onMessage = vi.fn();
    const { result } = renderHook(() => useSSE('https://x.test/sse', onMessage, { reconnectMs: 5000 }));

    const es = MockEventSource.instances[MockEventSource.instances.length - 1];
    const instancesBefore = MockEventSource.instances.length;
    es.readyState = CONNECTING;
    act(() => { es.onerror && es.onerror(); });

    // Error reflects the CONNECTING state.
    expect(result.current.error).toContain('Reconnect');

    // The watchdog fires after reconnectMs (5000ms) → es.close() → onerror sees
    // CLOSED and schedules backoff reconnect (10000ms for 1st failure).
    // After both timers, a new EventSource should exist.
    act(() => { vi.advanceTimersByTime(15000); });
    expect(MockEventSource.instances.length).toBe(instancesBefore + 1);
    // Error transitions to 'Connection lost' from the CLOSED path.
    expect(result.current.error).toContain('Connection lost');
  });

  it('watchdog does not fire when EventSource opens before timeout', () => {
    const onMessage = vi.fn();
    const { result } = renderHook(() => useSSE('https://x.test/sse', onMessage, { reconnectMs: 5000 }));

    const es = MockEventSource.instances[MockEventSource.instances.length - 1];
    const instancesBefore = MockEventSource.instances.length;
    es.readyState = CONNECTING;
    act(() => { es.onerror && es.onerror(); });

    expect(result.current.error).toContain('Reconnect');

    // Simulate the EventSource eventually opening (within the watchdog window).
    act(() => { vi.advanceTimersByTime(2000); });
    act(() => { es.onopen && es.onopen(); });

    expect(result.current.connected).toBe(true);
    expect(result.current.error).toBeNull();

    // Advance past where the watchdog would have fired — no reconnect should
    // happen because onopen cleared the watchdog.
    act(() => { vi.advanceTimersByTime(10000); });
    expect(MockEventSource.instances.length).toBe(instancesBefore);
  });

  it('honours a custom reconnectMs base', () => {
    const onMessage = vi.fn();
    renderHook(() => useSSE('https://x.test/sse', onMessage, { reconnectMs: 1000 }));

    failClosed(); const d1 = measureReconnectDelayMs();
    failClosed(); const d2 = measureReconnectDelayMs();
    failClosed(); const d3 = measureReconnectDelayMs();
    expect(d1).toBe(1000);
    expect(d2).toBe(2000);
    expect(d3).toBe(4000);
  });
});
