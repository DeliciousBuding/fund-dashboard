"""Crawl fund basic details: type, company via AKShare."""

import argparse
import sys
import pandas as pd

from fund_crawler.utils import (
    get_fund_codes,
    retry,
    log,
    FUND_DETAILS_CSV,
    ensure_dirs,
)


def crawl_details(codes: list[str] | None = None) -> pd.DataFrame:
    """Fetch fund master list and filter to target codes.

    AKShare fund_name_em() returns all ~20k funds with columns:
    基金代码, 基金简称, 基金类型, 拼音全称, 拼音简称
    """
    if codes is None:
        codes = get_fund_codes()

    ensure_dirs()

    def _fetch():
        import akshare as ak
        return ak.fund_name_em()

    log.info("Fetching fund master list from eastmoney...")
    df_all = retry(_fetch, times=3, backoff=2.0, label="fund master list")
    if df_all is None:
        log.error("Failed to fetch fund master list")
        return pd.DataFrame()

    # Normalize columns
    col_map = {
        "基金代码": "fund_code",
        "基金简称": "fund_name",
        "基金类型": "fund_type",
        "拼音全称": "pinyin_full",
        "拼音简称": "pinyin_short",
    }
    df_all = df_all.rename(columns={k: v for k, v in col_map.items() if k in df_all.columns})
    df_all["fund_code"] = df_all["fund_code"].astype(str).str.zfill(6)

    # Filter to our codes
    df = df_all[df_all["fund_code"].isin(codes)].copy()
    df = df.drop_duplicates(subset="fund_code", keep="first")
    df = df.sort_values("fund_code").reset_index(drop=True)

    missing = set(codes) - set(df["fund_code"])
    if missing:
        log.warning("Funds not found in master list (%d): %s", len(missing), ", ".join(sorted(missing)))

    # Keep only useful columns
    keep = ["fund_code", "fund_name", "fund_type"]
    for col in ["pinyin_full", "pinyin_short"]:
        if col in df.columns:
            keep.append(col)
    df = df[keep]

    df.to_csv(FUND_DETAILS_CSV, index=False)
    log.info("Fund details saved: %d funds → %s", len(df), FUND_DETAILS_CSV)
    return df


def main():
    parser = argparse.ArgumentParser(description="Crawl fund details")
    parser.add_argument("--codes", type=str, default=None, help="Comma-separated fund codes")
    parser.add_argument("--all", action="store_true", help="Crawl all funds from transactions")
    args = parser.parse_args()

    codes = None
    if args.codes:
        codes = [c.strip() for c in args.codes.split(",")]
    elif args.all:
        codes = None

    crawl_details(codes)


if __name__ == "__main__":
    main()
