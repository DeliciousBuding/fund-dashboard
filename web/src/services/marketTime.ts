// ═══════ Market time helpers ═══════
// Extracted from utils.ts — date, timezone, market-hours, and Yahoo API helpers.

// ── DST detection ──────────────────────────────────────────────────

/** Detect whether a date falls within US Eastern Daylight Time (DST).
 *  US DST starts on the 2nd Sunday of March and ends on the 1st Sunday of November. */
export function isUSEasternDST(date: Date = new Date()): boolean {
  const year = date.getUTCFullYear();

  // 2nd Sunday of March
  const mar1 = new Date(Date.UTC(year, 2, 1)); // March = month 2
  const firstSundayMar = 1 + ((7 - mar1.getUTCDay()) % 7);
  const dstStart = new Date(Date.UTC(year, 2, firstSundayMar + 7));

  // 1st Sunday of November
  const nov1 = new Date(Date.UTC(year, 10, 1)); // November = month 10
  const firstSundayNov = 1 + ((7 - nov1.getUTCDay()) % 7);
  const dstEnd = new Date(Date.UTC(year, 10, firstSundayNov));

  return date >= dstStart && date < dstEnd;
}

// ── Holiday calendar ───────────────────────────────────────────────

/** Get the n-th occurrence of a weekday in a month (0=Sun, 1=Mon, ..., 6=Sat).
 *  n is 1-based (1st, 2nd, 3rd, ...). Negative n means "last" (-1 for last). */
function nthWeekdayOf(year: number, month: number, weekday: number, n: number): Date {
  const first = new Date(Date.UTC(year, month, 1));
  const firstDay = first.getUTCDay();
  // Days until first occurrence of target weekday in this month
  let dayOfMonth = 1 + ((weekday - firstDay + 7) % 7);
  if (n > 0) {
    dayOfMonth += (n - 1) * 7;
  } else {
    // Negative n: count from end of month
    const lastDay = new Date(Date.UTC(year, month + 1, 0)).getUTCDate();
    // Last occurrence
    let last = dayOfMonth;
    while (last + 7 <= lastDay) last += 7;
    dayOfMonth = last + (n + 1) * 7;
  }
  return new Date(Date.UTC(year, month, dayOfMonth));
}

/** Format a Date as YYYY-MM-DD in UTC (the holiday date is just a day marker). */
function fmtDate(d: Date): string {
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, "0");
  const day = String(d.getUTCDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

/** Adjust a fixed-date holiday to its observed date.
 *  If it falls on Saturday → observed Friday. If Sunday → observed Monday. */
function observedDate(d: Date): Date {
  const dow = d.getUTCDay();
  if (dow === 0) {
    // Sunday → observed Monday
    return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate() + 1));
  }
  if (dow === 6) {
    // Saturday → observed Friday
    return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate() - 1));
  }
  return d;
}

/** Calculate Easter Sunday using the Anonymous Gregorian (Butcher) algorithm. */
function getEasterSunday(year: number): Date {
  const a = year % 19;
  const b = Math.floor(year / 100);
  const c = year % 100;
  const d = Math.floor(b / 4);
  const e = b % 4;
  const f = Math.floor((b + 8) / 25);
  const g = Math.floor((b - f + 1) / 3);
  const h = (19 * a + b - d - g + 15) % 30;
  const i = Math.floor(c / 4);
  const k = c % 4;
  const l = (32 + 2 * e + 2 * i - h - k) % 7;
  const m = Math.floor((a + 11 * h + 22 * l) / 451);
  const month = Math.floor((h + l - 7 * m + 114) / 31);
  const day = ((h + l - 7 * m + 114) % 31) + 1;
  return new Date(Date.UTC(year, month - 1, day));
}

/** Compute the set of US market holiday dates (YYYY-MM-DD) for a given year. */
function getUSHolidays(year: number): Set<string> {
  const holidays = new Set<string>();

  // New Year's Day (Jan 1, observed)
  holidays.add(fmtDate(observedDate(new Date(Date.UTC(year, 0, 1)))));

  // Martin Luther King Jr. Day (3rd Monday of January)
  holidays.add(fmtDate(nthWeekdayOf(year, 0, 1, 3)));

  // Presidents' Day (3rd Monday of February)
  holidays.add(fmtDate(nthWeekdayOf(year, 1, 1, 3)));

  // Good Friday (Friday before Easter Sunday)
  const easter = getEasterSunday(year);
  const goodFriday = new Date(Date.UTC(year, easter.getUTCMonth(), easter.getUTCDate() - 2));
  holidays.add(fmtDate(goodFriday));

  // Memorial Day (last Monday of May)
  holidays.add(fmtDate(nthWeekdayOf(year, 4, 1, -1)));

  // Juneteenth (June 19, observed)
  holidays.add(fmtDate(observedDate(new Date(Date.UTC(year, 5, 19)))));

  // Independence Day (July 4, observed)
  holidays.add(fmtDate(observedDate(new Date(Date.UTC(year, 6, 4)))));

  // Labor Day (1st Monday of September)
  holidays.add(fmtDate(nthWeekdayOf(year, 8, 1, 1)));

  // Thanksgiving (4th Thursday of November)
  holidays.add(fmtDate(nthWeekdayOf(year, 10, 4, 4)));

  // Christmas (Dec 25, observed)
  holidays.add(fmtDate(observedDate(new Date(Date.UTC(year, 11, 25)))));

  return holidays;
}

// Cache holidays per year to avoid recomputation
const holidayCache = new Map<number, Set<string>>();

function isUSHoliday(date: Date): boolean {
  const year = date.getUTCFullYear();
  if (!holidayCache.has(year)) {
    holidayCache.set(year, getUSHolidays(year));
  }
  const dateStr = fmtDate(date);
  const holidays = holidayCache.get(year);
  return holidays ? holidays.has(dateStr) : false;
}

// ── Market open check ──────────────────────────────────────────────

/** Formatter to extract Eastern Time hours/minutes from a UTC date.
 *  Uses Intl.DateTimeFormat with America/New_York timezone — no manual offset arithmetic. */
const etTimeFormatter = new Intl.DateTimeFormat("en-US", {
  timeZone: "America/New_York",
  hour: "numeric",
  minute: "numeric",
  hour12: false,
});

/**
 * Check whether the US stock market is currently open.
 * Regular session: 9:30 AM – 4:00 PM ET, Monday–Friday, excluding holidays.
 */
export function isUSMarketOpen(now: Date = new Date()): boolean {
  const day = now.getUTCDay();
  // Weekend check
  if (day === 0 || day === 6) return false;

  // Holiday check
  if (isUSHoliday(now)) return false;

  // Get Eastern Time via Intl
  const parts = etTimeFormatter.formatToParts(now);
  let etHour = 0;
  let etMinute = 0;
  for (const part of parts) {
    if (part.type === "hour") etHour = parseInt(part.value, 10);
    if (part.type === "minute") etMinute = parseInt(part.value, 10);
  }
  const totalMin = etHour * 60 + etMinute;
  return totalMin >= 9 * 60 + 30 && totalMin < 16 * 60;
}
