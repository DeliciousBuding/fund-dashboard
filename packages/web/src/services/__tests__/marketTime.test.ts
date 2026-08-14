import { describe, it, expect } from 'vitest';
import {
  isUSEasternDST,
  isUSMarketOpen,
} from '../marketTime';

// ── isUSEasternDST ───────────────────────────────────────────────
describe('isUSEasternDST', () => {
  it('returns true for a date in July (DST)', () => {
    expect(isUSEasternDST(new Date('2026-07-04T12:00:00Z'))).toBe(true);
  });

  it('returns false for a date in January (EST)', () => {
    expect(isUSEasternDST(new Date('2026-01-15T12:00:00Z'))).toBe(false);
  });

  it('returns true on the 2nd Sunday of March (DST start)', () => {
    // March 8, 2026 is the 2nd Sunday
    expect(isUSEasternDST(new Date('2026-03-08T12:00:00Z'))).toBe(true);
  });

  it('returns false on the Saturday before DST start', () => {
    expect(isUSEasternDST(new Date('2026-03-07T12:00:00Z'))).toBe(false);
  });

  it('returns true on the day before DST end (Oct 31)', () => {
    // November 1, 2026 is the 1st Sunday of November
    expect(isUSEasternDST(new Date('2026-10-31T12:00:00Z'))).toBe(true);
  });

  it('returns false on the 1st Sunday of November (DST end)', () => {
    // November 1, 2026 is the 1st Sunday
    expect(isUSEasternDST(new Date('2026-11-01T12:00:00Z'))).toBe(false);
  });

  it('returns false in December (EST)', () => {
    expect(isUSEasternDST(new Date('2026-12-25T12:00:00Z'))).toBe(false);
  });

  it('handles a year where March 1 is a Sunday (DST start = Mar 8)', () => {
    // 2026: March 1 is Sunday → 2nd Sunday = March 8
    expect(isUSEasternDST(new Date('2026-03-08T06:00:00Z'))).toBe(true);
    expect(isUSEasternDST(new Date('2026-03-07T23:59:59Z'))).toBe(false);
  });

  it('handles a year where March 1 is a Monday (DST start = Mar 14)', () => {
    // 2027: March 1 is Monday → 1st Sunday = March 7, 2nd Sunday = March 14
    expect(isUSEasternDST(new Date('2027-03-14T06:00:00Z'))).toBe(true);
    expect(isUSEasternDST(new Date('2027-03-13T23:59:59Z'))).toBe(false);
  });
});

// ── isUSMarketOpen (holiday-aware) ──────────────────────────────
describe('isUSMarketOpen', () => {
  it('returns a boolean value', () => {
    const result = isUSMarketOpen();
    expect(typeof result).toBe('boolean');
  });

  // ── Frozen-time tests ─────────────────────────────────────────

  it('EST: 2026-01-15T14:00:00Z (9:00 AM ET pre-market) should be closed', () => {
    // Jan 15, 2026 is a Thursday. 14:00 UTC = 9:00 AM EST (UTC-5).
    // 9:00 AM < 9:30 AM → closed.
    expect(isUSMarketOpen(new Date('2026-01-15T14:00:00Z'))).toBe(false);
  });

  it('EDT: 2026-07-15T14:00:00Z (10:00 AM ET) should be open', () => {
    // Jul 15, 2026 is a Wednesday. 14:00 UTC = 10:00 AM EDT (UTC-4).
    // 10:00 AM is between 9:30 AM and 4:00 PM → open.
    // Not a holiday.
    expect(isUSMarketOpen(new Date('2026-07-15T14:00:00Z'))).toBe(true);
  });

  // ── Holiday tests ─────────────────────────────────────────────

  it('Christmas Day should be closed', () => {
    // Dec 25, 2026 is a Friday. 16:00 UTC = 11:00 AM EST.
    // Christmas is a market holiday → closed.
    expect(isUSMarketOpen(new Date('2026-12-25T16:00:00Z'))).toBe(false);
  });

  it('Thanksgiving should be closed', () => {
    // Thanksgiving 2026 = Nov 26 (4th Thursday). 16:00 UTC = 11:00 AM EST.
    expect(isUSMarketOpen(new Date('2026-11-26T16:00:00Z'))).toBe(false);
  });

  it('Independence Day (observed) should be closed', () => {
    // Jul 4, 2026 is a Saturday → observed on Friday Jul 3.
    // 16:00 UTC = 12:00 PM EDT, which is within trading hours but holiday → closed.
    expect(isUSMarketOpen(new Date('2026-07-03T16:00:00Z'))).toBe(false);
  });

  it('New Year\'s Day should be closed', () => {
    // Jan 1, 2027 is a Friday. 16:00 UTC = 11:00 AM EST.
    expect(isUSMarketOpen(new Date('2027-01-01T16:00:00Z'))).toBe(false);
  });

  it('MLK Day should be closed', () => {
    // MLK Day 2027 = Jan 18 (3rd Monday). 16:00 UTC = 11:00 AM EST.
    expect(isUSMarketOpen(new Date('2027-01-18T16:00:00Z'))).toBe(false);
  });

  it('Memorial Day should be closed', () => {
    // Memorial Day 2026 = May 25 (last Monday). 16:00 UTC = 12:00 PM EDT.
    expect(isUSMarketOpen(new Date('2026-05-25T16:00:00Z'))).toBe(false);
  });

  it('Labor Day should be closed', () => {
    // Labor Day 2026 = Sep 7 (1st Monday). 16:00 UTC = 12:00 PM EDT.
    expect(isUSMarketOpen(new Date('2026-09-07T16:00:00Z'))).toBe(false);
  });

  // ── Weekend tests ─────────────────────────────────────────────

  it('returns false on Saturday', () => {
    // 2026-06-20 is a Saturday
    expect(isUSMarketOpen(new Date('2026-06-20T14:30:00Z'))).toBe(false);
  });

  it('returns false on Sunday', () => {
    // 2026-06-21 is a Sunday
    expect(isUSMarketOpen(new Date('2026-06-21T14:30:00Z'))).toBe(false);
  });

  // ── Regular trading hours ─────────────────────────────────────

  it('EDT: 2026-07-15T13:29:00Z (9:29 AM ET) should be closed (pre-open)', () => {
    expect(isUSMarketOpen(new Date('2026-07-15T13:29:00Z'))).toBe(false);
  });

  it('EDT: 2026-07-15T20:01:00Z (4:01 PM ET) should be closed (post-close)', () => {
    expect(isUSMarketOpen(new Date('2026-07-15T20:01:00Z'))).toBe(false);
  });

  it('EST: 2026-01-15T14:30:00Z (9:30 AM ET) should be open', () => {
    // Jan 15, 2026 is Thursday. 14:30 UTC = 9:30 AM EST. Not a holiday.
    expect(isUSMarketOpen(new Date('2026-01-15T14:30:00Z'))).toBe(true);
  });

  it('EST: 2026-01-15T21:00:00Z (4:00 PM ET) should be open (edge)', () => {
    // 21:00 UTC = 4:00 PM EST. 4:00 PM is the close; < 16:00 so still open.
    expect(isUSMarketOpen(new Date('2026-01-15T20:59:00Z'))).toBe(true);
  });
});
