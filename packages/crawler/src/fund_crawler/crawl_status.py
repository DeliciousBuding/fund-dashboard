"""Fetch current fund trading status (purchase/redemption suspension)."""

import argparse
import sys
import pandas as pd

from fund_crawler.utils import (
    get_fund_codes,
    retry,
    log,
    DATA_OUTPUT,
    ensure_dirs,
)


def crawl_status(codes: list[str] | None = None) -> pd.DataFrame:
    """Fetch fund daily status snapshot and filter to target codes.

    AKShare fund_open_fund_daily_em() returns:
    col 0: fund_code, col 1: fund_name, col N: NAV columns,
    col -3: purchase_status, col -2: redemption_status, col -1: fee_rate

    Uses positional indexing to avoid encoding issues with Chinese column names.
    """
    if codes is None:
        codes = get_fund_codes()

    ensure_dirs()

    def _fetch():
        import akshare as ak
        return ak.fund_open_fund_daily_em()

    log.info("Fetching fund daily status...")
    df_all = retry(_fetch, times=2, backoff=2.0, label="fund status")
    if df_all is None:
        log.error("Failed to fetch fund status")
        return pd.DataFrame()

    # Use positional columns (avoids encoding issues)
    cols = list(df_all.columns)
    code_col = cols[0]       # fund code
    name_col = cols[1]       # fund name
    purchase_col = cols[-3]  # purchase status
    redeem_col = cols[-2]    # redemption status
    fee_col = cols[-1]       # fee rate

    df = df_all[[code_col, name_col, purchase_col, redeem_col, fee_col]].copy()
    df.columns = ["fund_code", "fund_name", "purchase_status", "redemption_status", "fee_rate"]
    df["fund_code"] = df["fund_code"].astype(str).str.zfill(6)

    # Filter to our codes
    df = df[df["fund_code"].isin(codes)].copy()
    df = df.drop_duplicates(subset="fund_code", keep="first")
    df = df.sort_values("fund_code").reset_index(drop=True)

    out_path = DATA_OUTPUT / "fund_status.csv"
    df.to_csv(out_path, index=False)

    log.info("Fund status saved: %d funds -> %s", len(df), out_path)

    # Print summary
    print_status_summary(df)
    return df


def print_status_summary(df: pd.DataFrame) -> None:
    """Print funds with unusual trading status."""
    suspended = df[df["purchase_status"].str.contains("暂停|限", na=False)]
    if not suspended.empty:
        print(f"\n  Purchase restricted/suspended ({len(suspended)}):")
        for _, row in suspended.iterrows():
            print(f"    {row.fund_code} {row.fund_name}: {row.purchase_status}")
    else:
        print("\n  [OK] All funds: purchase open")

    redeem_suspended = df[df["redemption_status"].str.contains("暂停", na=False)]
    if not redeem_suspended.empty:
        print(f"\n  Redemption suspended ({len(redeem_suspended)}):")
        for _, row in redeem_suspended.iterrows():
            print(f"    {row.fund_code} {row.fund_name}: {row.redemption_status}")
    else:
        print("  [OK] All funds: redemption open")


def main():
    parser = argparse.ArgumentParser(description="Crawl fund trading status")
    parser.add_argument("--codes", type=str, default=None, help="Comma-separated fund codes")
    parser.add_argument("--all", action="store_true", help="Crawl all funds from transactions")
    args = parser.parse_args()

    codes = None
    if args.codes:
        codes = [c.strip() for c in args.codes.split(",")]
    elif args.all:
        codes = None

    crawl_status(codes)


if __name__ == "__main__":
    main()
