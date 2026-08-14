import { describe, it, expect } from 'vitest';
import {
  fmt,
  fmtShort,
} from '../format';

// ── fmt ─────────────────────────────────────────────────────────
describe('fmt', () => {
  it('formats positive values with + sign', () => {
    expect(fmt(1234.56)).toBe('+¥ 1234.56');
  });

  it('formats negative values with - sign', () => {
    expect(fmt(-500)).toBe('-¥ 500.00');
  });

  it('handles near-zero values', () => {
    expect(fmt(0)).toBe('¥ 0.00');
    expect(fmt(0.001)).toBe('¥ 0.00');
  });

  it('handles very small negative values as zero', () => {
    expect(fmt(-0.001)).toBe('¥ 0.00');
  });
});

// ── fmtShort ────────────────────────────────────────────────────
describe('fmtShort', () => {
  it('formats positive values with +', () => {
    expect(fmtShort(1234)).toBe('+1234');
    expect(fmtShort(56)).toBe('+56');
  });

  it('formats negative values naturally', () => {
    expect(fmtShort(-56)).toBe('-56');
  });

  it('returns "0" for zero', () => {
    expect(fmtShort(0)).toBe('0');
    expect(fmtShort(0.4)).toBe('0');
  });

  it('returns + for rounded-up near-zero', () => {
    expect(fmtShort(0.5)).toBe('+1');
  });
});

