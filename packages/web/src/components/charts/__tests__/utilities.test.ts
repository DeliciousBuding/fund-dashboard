import { describe, it, expect } from 'vitest';
import { getDateRange } from '../index';

// ── getDateRange ────────────────────────────────────────────────
describe('getDateRange', () => {
  const allDates = ['2024-01-01', '2024-02-01', '2024-03-01', '2024-04-01', '2024-05-01', '2024-06-01'];

  it('returns full range for "all"', () => {
    const [start, end] = getDateRange('all', allDates, []);
    expect(start).toBe(0);
    expect(end).toBe(5);
  });

  it('returns tx range when tx dates provided', () => {
    const [start, end] = getDateRange('tx', allDates, ['2024-02-01', '2024-03-01']);
    expect(start).toBe(0); // i0 - 10 clamped to 0
    expect(end).toBe(5);
  });

  it('returns specific range for known key', () => {
    // '1m' key uses 30-day lookback from last date
    const [start, end] = getDateRange('1m', allDates, []);
    expect(end).toBe(5);
  });

  it('returns [0, lastIdx] for unknown key', () => {
    const [start, end] = getDateRange('unknown', allDates, []);
    expect(start).toBe(0);
    expect(end).toBe(5);
  });

  it('handles empty dates array', () => {
    const [start, end] = getDateRange('1m', [], []);
    expect(start).toBe(0);
    expect(end).toBe(-1);
  });
});
