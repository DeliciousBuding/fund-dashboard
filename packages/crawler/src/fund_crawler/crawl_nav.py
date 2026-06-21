"""Crawl historical NAV for each fund via AKShare."""

import argparse
import sys
import pandas as pd
from pathlib import Path

from fund_crawler.utils import (
    get_fund_codes,
    RateLimiter,
    retry,
    log,
    NAV_DIR,
    ensure_dirs,
)

# AKShare fund_open_fund_info_em with indicator="单位净值走势" returns:
#   净值日期(date), 单位净值(unit_nav), 日增长率(daily_change_pct)
#
# To also get accumulated NAV, use indicator="累计净值走势" (separate call)


def crawl_one_fund(code: str, limiter: RateLimiter) -> pd.DataFrame | None:
    """Fetch full NAV history for a single fund. Returns DataFrame or None on failure."""

    def _fetch():
        import akshare as ak

        limiter.wait()
        df = ak.fund_open_fund_info_em(symbol=code, indicator="单位净值走势")
        if df is None or df.empty:
            raise ValueError(f"empty response for {code}")
        return df

    result = retry(_fetch, times=3, backoff=2.0, label=f"NAV {code}")
    if result is None:
        return None

    # Normalize columns — AKShare returns Chinese column names
    col_map = {
        "净值日期": "date",
        "单位净值": "unit_nav",
        "累计净值": "accumulated_nav",
        "日增长率": "daily_change_pct",
    }
    result = result.rename(columns={k: v for k, v in col_map.items() if k in result.columns})
    result["date"] = pd.to_datetime(result["date"])
    result["fund_code"] = str(code).zfill(6)

    # Keep only columns that exist
    keep = ["fund_code", "date"]
    for col in ["unit_nav", "accumulated_nav", "daily_change_pct"]:
        if col in result.columns:
            keep.append(col)
    result = result[keep]
    return result


def crawl_nav(codes: list[str] | None = None, show_progress: bool = True) -> dict[str, pd.DataFrame]:
    """Crawl NAV history for given codes. Returns dict of code → DataFrame.

    Args:
        codes: List of fund codes. If None, uses all codes from transactions.
        show_progress: Print progress to stderr.
    """
    if codes is None:
        codes = get_fund_codes()

    ensure_dirs()
    limiter = RateLimiter(min_interval=1.5)
    results: dict[str, pd.DataFrame] = {}
    failed: list[str] = []
    total = len(codes)

    for i, code in enumerate(codes, 1):
        if show_progress:
            pct = i / total * 100
            print(f"\r  NAV [{i}/{total}] {pct:.0f}%  {code}  ", end="", file=sys.stderr, flush=True)

        df = crawl_one_fund(code, limiter)
        if df is not None:
            results[code] = df
            out_path = NAV_DIR / f"{code}.csv"
            df.to_csv(out_path, index=False)
        else:
            failed.append(code)

    if show_progress:
        print(file=sys.stderr)  # newline

    if failed:
        log.warning("Failed NAV crawls (%d): %s", len(failed), ", ".join(failed))

    log.info("NAV crawl complete: %d/%d funds", len(results), total)
    return results


def main():
    parser = argparse.ArgumentParser(description="Crawl fund NAV history")
    parser.add_argument("--codes", type=str, default=None, help="Comma-separated fund codes")
    parser.add_argument("--all", action="store_true", help="Crawl all funds from transactions")
    args = parser.parse_args()

    codes = None
    if args.codes:
        codes = [c.strip() for c in args.codes.split(",")]
    elif args.all:
        codes = None
    else:
        codes = get_fund_codes()[:5]

    crawl_nav(codes)


if __name__ == "__main__":
    main()
