"""Crawl fund dividend/split records via AKShare."""

import argparse
import sys
import pandas as pd

from fund_crawler.utils import (
    get_fund_codes,
    RateLimiter,
    retry,
    log,
    FUND_DIVIDENDS_CSV,
    ensure_dirs,
)


def crawl_one_fund_dividends(code: str, limiter: RateLimiter) -> pd.DataFrame | None:
    """Fetch dividend history for a single fund. Returns DataFrame, empty DataFrame, or None on error."""

    def _fetch():
        import akshare as ak
        limiter.wait()
        df = ak.fund_open_fund_info_em(symbol=code, indicator="分红送配详情")
        if df is None:
            return pd.DataFrame()
        return df

    result = retry(_fetch, times=3, backoff=2.0, label=f"dividend {code}")
    if result is None:
        return None
    if result.empty:
        return result

    # Normalize — AKShare column names may vary by version
    col_map = {
        "权益登记日": "registration_date",
        "除息日": "ex_dividend_date",
        "分红发放日": "payment_date",
        "每份分红": "dividend_per_unit",
        "分红金额": "dividend_amount",
        "拆分比例": "split_ratio",
    }
    result = result.rename(columns={k: v for k, v in col_map.items() if k in result.columns})
    result["fund_code"] = str(code).zfill(6)

    # Keep only consistent columns that exist
    keep = ["fund_code"]
    for col in ["registration_date", "ex_dividend_date", "payment_date", "dividend_per_unit"]:
        if col in result.columns:
            keep.append(col)
    for col in result.columns:
        if col not in keep and col != "fund_code":
            keep.append(col)  # catch any unexpected columns
    result = result[keep]
    return result


def crawl_dividends(codes: list[str] | None = None) -> pd.DataFrame:
    """Crawl dividend records for given codes. Returns combined DataFrame."""
    if codes is None:
        codes = get_fund_codes()

    ensure_dirs()
    limiter = RateLimiter(min_interval=1.5)
    all_dfs: list[pd.DataFrame] = []
    failed: list[str] = []
    no_dividend: list[str] = []
    total = len(codes)

    for i, code in enumerate(codes, 1):
        pct = i / total * 100
        print(f"\r  Dividends [{i}/{total}] {pct:.0f}%  {code}  ", end="", file=sys.stderr, flush=True)

        df = crawl_one_fund_dividends(code, limiter)
        if df is None:
            failed.append(code)
        elif df.empty:
            no_dividend.append(code)
        else:
            all_dfs.append(df)

    print(file=sys.stderr)

    if failed:
        log.warning("Failed dividend crawls (%d): %s", len(failed), ", ".join(failed))

    if all_dfs:
        combined = pd.concat(all_dfs, ignore_index=True)
        combined.to_csv(FUND_DIVIDENDS_CSV, index=False)
        log.info(
            "Dividends saved: %d records across %d funds → %s",
            len(combined), combined["fund_code"].nunique(), FUND_DIVIDENDS_CSV,
        )
        return combined
    else:
        log.info("No dividend records found for any fund (%d funds checked)", len(no_dividend))
        return pd.DataFrame()


def main():
    parser = argparse.ArgumentParser(description="Crawl fund dividend records")
    parser.add_argument("--codes", type=str, default=None, help="Comma-separated fund codes")
    parser.add_argument("--all", action="store_true", help="Crawl all funds from transactions")
    args = parser.parse_args()

    codes = None
    if args.codes:
        codes = [c.strip() for c in args.codes.split(",")]
    elif args.all:
        codes = None

    crawl_dividends(codes)


if __name__ == "__main__":
    main()
