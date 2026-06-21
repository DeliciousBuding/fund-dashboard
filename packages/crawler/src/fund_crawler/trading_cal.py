"""Chinese A-share trading calendar — determines which days are trading days."""

from datetime import date, timedelta
import pandas as pd

from fund_crawler.utils import log, DATA_OUTPUT, retry

# Lazy-loaded cache
_trading_days: set[date] | None = None
_trading_days_sorted: list[date] | None = None


def _load_calendar() -> tuple[set[date], list[date]]:
    """Fetch Chinese trading calendar from AKShare (Sina source). Cached in memory."""
    global _trading_days, _trading_days_sorted

    if _trading_days is not None:
        return _trading_days, _trading_days_sorted

    # Try cached file first
    cache_path = DATA_OUTPUT / "trading_calendar.csv"
    if cache_path.exists():
        log.info("Loading trading calendar from cache")
        df = pd.read_csv(cache_path, parse_dates=["trade_date"])
    else:
        log.info("Fetching trading calendar from AKShare...")

        def _fetch():
            import akshare as ak
            return ak.tool_trade_date_hist_sina()

        df = retry(_fetch, times=3, backoff=2.0, label="trading calendar")
        if df is None:
            raise RuntimeError("Failed to fetch trading calendar")
        df.to_csv(cache_path, index=False)
        log.info("Trading calendar cached: %d days", len(df))

    _trading_days = set(d.date() for d in pd.to_datetime(df["trade_date"]))
    _trading_days_sorted = sorted(_trading_days)
    return _trading_days, _trading_days_sorted


def is_trading_day(d: date) -> bool:
    """Check if date is a Chinese A-share trading day."""
    td, _ = _load_calendar()
    return d in td


def next_trading_day(d: date) -> date:
    """Return the next trading day on or after d."""
    _, sorted_days = _load_calendar()
    for td in sorted_days:
        if td >= d:
            return td
    return sorted_days[-1]  # fallback


def prev_trading_day(d: date) -> date:
    """Return the most recent trading day on or before d."""
    _, sorted_days = _load_calendar()
    best = sorted_days[0]
    for td in sorted_days:
        if td <= d:
            best = td
        else:
            break
    return best


def trading_days_between(start: date, end: date) -> int:
    """Count trading days in [start, end] inclusive."""
    td, _ = _load_calendar()
    count = 0
    current = start
    while current <= end:
        if current in td:
            count += 1
        current += timedelta(days=1)
    return count


def get_effective_nav_date(trade_time: pd.Timestamp) -> date:
    """Determine the NAV date for a fund purchase/sell order.

    Rules:
    - Trade before 15:00 on trading day T → NAV date = T
    - Trade after 15:00 on trading day T → NAV date = next trading day after T
    - Trade on non-trading day → NAV date = next trading day

    Returns the date whose NAV should be used for pricing.
    """
    d = trade_time.date()
    t = trade_time.time()

    cutoff = pd.Timestamp("15:00:00").time()

    if is_trading_day(d):
        if t < cutoff:
            return d  # Same-day NAV
        else:
            return next_trading_day(d + timedelta(days=1))  # Next trading day's NAV
    else:
        return next_trading_day(d)  # Next trading day's NAV


def describe_date(d: date) -> str:
    """Human-readable description of a date's trading status."""
    if is_trading_day(d):
        return f"{d} (trading day)"
    else:
        dow = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"][d.weekday()]
        return f"{d} ({dow}, non-trading)"


def init_calendar() -> None:
    """Pre-load the trading calendar (call once at startup)."""
    _load_calendar()
