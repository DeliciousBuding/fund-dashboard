"""Shared utilities: rate limiter, retry, logging, fund code extraction."""

import time
import logging
import pandas as pd
from pathlib import Path
from collections.abc import Callable
from typing import TypeVar

T = TypeVar("T")

# ── paths ──────────────────────────────────────────────────────────────
PROJECT_ROOT = Path(__file__).resolve().parent.parent.parent
DATA_INPUT = PROJECT_ROOT / "data" / "input"
DATA_OUTPUT = PROJECT_ROOT / "data" / "output"
NAV_DIR = DATA_OUTPUT / "nav"
TRANSACTIONS_CSV = DATA_INPUT / "transactions_clean.csv"
FUND_DETAILS_CSV = DATA_OUTPUT / "fund_details.csv"
FUND_DIVIDENDS_CSV = DATA_OUTPUT / "fund_dividends.csv"
ENRICHED_CSV = DATA_OUTPUT / "transactions_enriched.csv"
ERROR_LOG = DATA_OUTPUT / "errors.log"

# ── logging ────────────────────────────────────────────────────────────
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    handlers=[
        logging.StreamHandler(),
        logging.FileHandler(ERROR_LOG, mode="a"),
    ],
)
log = logging.getLogger("fund-crawler")


# ── rate limiter ───────────────────────────────────────────────────────
class RateLimiter:
    """Ensure minimum interval between calls."""

    def __init__(self, min_interval: float = 2.0):
        self.min_interval = min_interval
        self._last_call = 0.0

    def wait(self) -> None:
        now = time.monotonic()
        elapsed = now - self._last_call
        if elapsed < self.min_interval:
            time.sleep(self.min_interval - elapsed)
        self._last_call = time.monotonic()


# ── retry ──────────────────────────────────────────────────────────────
def retry(fn: Callable[[], T], times: int = 3, backoff: float = 2.0, label: str = "") -> T | None:
    """Call fn with retry on exception. Returns None if all retries fail."""
    for attempt in range(1, times + 1):
        try:
            return fn()
        except Exception as e:
            tag = f" [{label}]" if label else ""
            if attempt < times:
                wait = backoff**attempt
                log.warning("attempt %d/%d failed%s: %s — retrying in %.0fs", attempt, times, tag, e, wait)
                time.sleep(wait)
            else:
                log.error("all %d attempts failed%s: %s", times, tag, e)
    return None


# ── data helpers ───────────────────────────────────────────────────────
def load_transactions() -> pd.DataFrame:
    """Load cleaned transactions, return DataFrame."""
    df = pd.read_csv(TRANSACTIONS_CSV, dtype={"fund_code": str})
    df["fund_code"] = df["fund_code"].str.zfill(6)
    df["trade_time"] = pd.to_datetime(df["trade_time"])
    df["confirm_date"] = pd.to_datetime(df["confirm_date"])
    return df


def get_fund_codes() -> list[str]:
    """Extract sorted unique fund codes from transactions."""
    df = load_transactions()
    return sorted(df["fund_code"].unique().tolist())


def ensure_dirs() -> None:
    """Create all output directories."""
    NAV_DIR.mkdir(parents=True, exist_ok=True)
    DATA_OUTPUT.mkdir(parents=True, exist_ok=True)
