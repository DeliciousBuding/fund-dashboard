"""Enrich transactions with NAV data — proper settlement logic.

Uses Chinese trading calendar + 15:00 cutoff to determine the correct NAV date
for each transaction. Computes T+N settlement lag per transaction and flags anomalies.
"""

import argparse
import sys
import pandas as pd
from datetime import date, timedelta

from fund_crawler.utils import (
    load_transactions,
    log,
    NAV_DIR,
    ENRICHED_CSV,
    DATA_OUTPUT,
    ensure_dirs,
)
from fund_crawler.trading_cal import (
    init_calendar,
    get_effective_nav_date,
    is_trading_day,
    trading_days_between,
    describe_date,
)


# ── NAV loading ────────────────────────────────────────────────────────

def load_all_nav() -> dict[str, pd.DataFrame]:
    """Load all cached NAV CSVs into a dict of code → DataFrame."""
    nav_data: dict[str, pd.DataFrame] = {}
    for path in sorted(NAV_DIR.glob("*.csv")):
        code = path.stem
        df = pd.read_csv(path, parse_dates=["date"])
        nav_data[code] = df.sort_values("date").reset_index(drop=True)
    return nav_data


def find_nav_on_date(nav_df: pd.DataFrame, target_date: date) -> dict | None:
    """Find NAV row closest to target_date. Returns {unit_nav, nav_date, delta_days} or None."""
    target_ts = pd.Timestamp(target_date)

    # Exact match first
    exact = nav_df[nav_df["date"] == target_ts]
    if not exact.empty:
        row = exact.iloc[0]
        return {"unit_nav": float(row["unit_nav"]), "nav_date": target_date, "delta_days": 0}

    # Search ±10 days (trading holidays can create multi-day gaps)
    window = nav_df[
        (nav_df["date"] >= target_ts - pd.Timedelta(days=10))
        & (nav_df["date"] <= target_ts + pd.Timedelta(days=10))
    ]
    if window.empty:
        return None

    window = window.copy()
    window["delta"] = (window["date"] - target_ts).abs()
    row = window.loc[window["delta"].idxmin()]
    return {
        "unit_nav": float(row["unit_nav"]),
        "nav_date": row["date"].date(),
        "delta_days": int(row["delta"].days),
    }


def find_latest_nav(nav_df: pd.DataFrame) -> dict:
    """Get most recent NAV row."""
    row = nav_df.iloc[-1]
    return {"latest_nav": float(row["unit_nav"]), "latest_nav_date": row["date"].date()}


def find_nav_range(nav_df: pd.DataFrame, start_date: date, end_date: date) -> pd.DataFrame:
    """Get NAV rows in a date range."""
    return nav_df[
        (nav_df["date"] >= pd.Timestamp(start_date))
        & (nav_df["date"] <= pd.Timestamp(end_date))
    ]


# ── Fund status ────────────────────────────────────────────────────────

def load_fund_status() -> pd.DataFrame | None:
    """Fetch current fund trading status (申购/赎回)."""
    from fund_crawler.utils import retry

    def _fetch():
        import akshare as ak
        return ak.fund_open_fund_daily_em()

    df = retry(_fetch, times=2, backoff=2.0, label="fund status")
    if df is None:
        return None

    # AKShare column names (may vary by version)
    col_map = {}
    for col in df.columns:
        if "代码" in col:
            col_map[col] = "fund_code"
        elif "简称" in col:
            col_map[col] = "fund_name"
        elif "申购状态" in col:
            col_map[col] = "purchase_status"
        elif "赎回状态" in col:
            col_map[col] = "redemption_status"
        elif "购买费率" in col or "费率" in col:
            col_map[col] = "fee_rate"

    df = df.rename(columns=col_map)
    if "fund_code" in df.columns:
        df["fund_code"] = df["fund_code"].astype(str).str.zfill(6)
    return df


# ── Main enrichment ────────────────────────────────────────────────────

def enrich(nav_data: dict[str, pd.DataFrame] | None = None) -> pd.DataFrame:
    """Enrich transactions with NAV data using proper settlement logic.

    For each transaction:
    1. Compute effective_nav_date = get_effective_nav_date(trade_time)
       (handles 15:00 cutoff + trading calendar)
    2. Look up NAV on effective_nav_date
    3. Verify inferred_nav from PDF extraction
    4. Compute settlement_days = (confirm_date - trade_time.date()).days
    5. Classify trade_date type (trading/non-trading, before/after cutoff)
    6. Compute current_value and unrealized_pnl using latest NAV
    """
    ensure_dirs()
    init_calendar()

    if nav_data is None:
        nav_data = load_all_nav()
        log.info("Loaded NAV data for %d funds", len(nav_data))

    tx = load_transactions()
    log.info("Loaded %d transactions across %d funds", len(tx), tx["fund_code"].nunique())

    # ── New enrichment columns ─────────────────────────────────────────
    tx["effective_nav_date"] = None       # The NAV date used for pricing
    tx["nav_on_effective_date"] = None    # NAV on that date (from API)
    tx["nav_date_delta"] = None           # Days between effective date and actual NAV found
    tx["nav_date_matched"] = None         # The actual NAV date used (may differ slightly)
    tx["nav_verified"] = None             # True/False/None
    tx["trade_day_type"] = None           # "trading_before_cutoff" | "trading_after_cutoff" | "non_trading"
    tx["settlement_days"] = None          # (confirm_date - trade_date).days
    tx["settlement_trading_days"] = None  # Trading days between trade and confirm
    tx["latest_nav"] = None
    tx["latest_nav_date"] = None
    tx["current_value"] = None
    tx["unrealized_pnl"] = None
    tx["anomaly"] = None                  # Anomaly flag

    anomalies: list[dict] = []

    for code in tx["fund_code"].unique():
        if code not in nav_data:
            log.warning("No NAV data for fund %s, skipping enrichment", code)
            continue

        nav_df = nav_data[code]
        latest = find_latest_nav(nav_df)
        mask = tx["fund_code"] == code

        # Set latest NAV for all rows
        tx.loc[mask, "latest_nav"] = latest["latest_nav"]
        tx.loc[mask, "latest_nav_date"] = pd.Timestamp(latest["latest_nav_date"])

        for idx in tx[mask].index:
            trade_time = tx.at[idx, "trade_time"]
            confirm_date = tx.at[idx, "confirm_date"]
            direction = tx.at[idx, "direction"]

            # 1. Compute effective NAV date (with trading calendar + 15:00 cutoff)
            effective_date = get_effective_nav_date(trade_time)
            tx.at[idx, "effective_nav_date"] = effective_date

            # 2. Classify trade day type
            if is_trading_day(trade_time.date()):
                if trade_time.time() < pd.Timestamp("15:00:00").time():
                    tx.at[idx, "trade_day_type"] = "trading_before_cutoff"
                else:
                    tx.at[idx, "trade_day_type"] = "trading_after_cutoff"
            else:
                tx.at[idx, "trade_day_type"] = "non_trading"

            # 3. Look up NAV on effective date
            nav_info = find_nav_on_date(nav_df, effective_date)
            if nav_info:
                tx.at[idx, "nav_on_effective_date"] = nav_info["unit_nav"]
                tx.at[idx, "nav_date_delta"] = nav_info["delta_days"]
                tx.at[idx, "nav_date_matched"] = pd.Timestamp(nav_info["nav_date"])

                # Verify against inferred_nav
                inferred = tx.at[idx, "inferred_nav"]
                api_nav = nav_info["unit_nav"]
                if pd.notna(inferred) and float(inferred) > 0 and api_nav > 0:
                    rel_error = abs(float(inferred) - api_nav) / api_nav
                    tx.at[idx, "nav_verified"] = rel_error < 0.005

            # 4. Settlement analysis
            if pd.notna(confirm_date) and pd.notna(trade_time):
                cal_days = (confirm_date.date() - trade_time.date()).days
                tx.at[idx, "settlement_days"] = cal_days
                tx.at[idx, "settlement_trading_days"] = trading_days_between(
                    trade_time.date(), confirm_date.date()
                )

            # 5. Anomaly detection
            anomaly_reasons = []

            # Trades on non-trading days are normal (system queues them), but flag if
            # the NAV date is more than 2 trading days away from trade date
            if nav_info and nav_info["delta_days"] > 2:
                anomaly_reasons.append(f"NAV delta={nav_info['delta_days']}d (effective={effective_date}, matched={nav_info['nav_date']})")

            # Unusual settlement: > 5 calendar days
            if pd.notna(tx.at[idx, "settlement_days"]) and tx.at[idx, "settlement_days"] > 5:
                anomaly_reasons.append(f"settlement={int(tx.at[idx, 'settlement_days'])}d")

            # Negative settlement (confirm before trade — data error?)
            if pd.notna(tx.at[idx, "settlement_days"]) and tx.at[idx, "settlement_days"] < 0:
                anomaly_reasons.append(f"negative settlement={int(tx.at[idx, 'settlement_days'])}d")

            if anomaly_reasons:
                tx.at[idx, "anomaly"] = "; ".join(anomaly_reasons)
                anomalies.append({
                    "seq": tx.at[idx, "seq"],
                    "fund_code": code,
                    "trade_time": str(trade_time),
                    "direction": direction,
                    "reasons": anomaly_reasons,
                })

    # 6. Compute current value & P&L
    tx["current_value"] = pd.to_numeric(tx["confirm_share"], errors="coerce") * pd.to_numeric(tx["latest_nav"], errors="coerce")
    pos_mask = pd.to_numeric(tx["signed_share_change"], errors="coerce") > 0
    tx.loc[pos_mask, "unrealized_pnl"] = (
        pd.to_numeric(tx.loc[pos_mask, "current_value"], errors="coerce")
        - pd.to_numeric(tx.loc[pos_mask, "confirm_amount"], errors="coerce")
    )

    # ── Output ─────────────────────────────────────────────────────────
    tx.to_csv(ENRICHED_CSV, index=False)
    log.info("Enriched transactions saved: %d rows → %s", len(tx), ENRICHED_CSV)

    # ── Stats ──────────────────────────────────────────────────────────
    print_stats(tx, anomalies)
    print_settlement_analysis(tx)
    print_anomalies(anomalies)

    # Save anomalies
    if anomalies:
        pd.DataFrame(anomalies).to_csv(DATA_OUTPUT / "anomalies.csv", index=False)

    return tx


def print_stats(tx: pd.DataFrame, anomalies: list[dict]) -> None:
    """Print enrichment quality stats."""
    verified = tx["nav_verified"].sum()
    total_checked = tx["nav_verified"].notna().sum()
    nav_matched = tx["nav_on_effective_date"].notna().sum()

    print("\n" + "-" * 60)
    print("  ENRICHMENT QUALITY")
    print("-" * 60)
    print(f"  NAV matched:       {nav_matched}/{len(tx)} ({nav_matched/len(tx)*100:.1f}%)")
    if total_checked > 0:
        print(f"  NAV verified:      {int(verified)}/{int(total_checked)} ({verified/total_checked*100:.1f}%)")
    else:
        print("  NAV verified:      N/A")
    print(f"  Anomalies flagged: {len(anomalies)}")
    print(f"  Funds enriched:    {tx['nav_on_effective_date'].notna().groupby(tx['fund_code']).any().sum()}")

    # Trade day type breakdown
    print(f"\n  Trade day types:")
    for typ in ["trading_before_cutoff", "trading_after_cutoff", "non_trading"]:
        count = (tx["trade_day_type"] == typ).sum()
        print(f"    {typ}: {count} ({count/len(tx)*100:.1f}%)")

    # NAV match by trade day type
    for typ in ["trading_before_cutoff", "trading_after_cutoff", "non_trading"]:
        subset = tx[tx["trade_day_type"] == typ]
        matched = subset["nav_on_effective_date"].notna().sum()
        verified = subset["nav_verified"].sum()
        checked = subset["nav_verified"].notna().sum()
        print(f"    {typ}: matched={matched}/{len(subset)}, verified={int(verified)}/{int(checked)}")

    total_pnl = tx["unrealized_pnl"].sum()
    print(f"\n  Unrealized P&L:    {total_pnl:+,.2f} CNY")
    print("-" * 60)


def print_settlement_analysis(tx: pd.DataFrame) -> None:
    """Analyze T+N settlement patterns per fund type."""
    print("\n" + "-" * 60)
    print("  T+N SETTLEMENT ANALYSIS")
    print("-" * 60)

    # Group by fund to see patterns
    settlement_by_fund = tx.groupby("fund_code").agg(
        fund_name=("fund_name", "first"),
        tx_count=("settlement_days", "count"),
        min_settlement=("settlement_days", "min"),
        max_settlement=("settlement_days", "max"),
        median_settlement=("settlement_days", "median"),
    ).reset_index()

    # Highlight unusual patterns
    print(f"\n  Funds with settlement > 3 calendar days:")
    unusual = settlement_by_fund[settlement_by_fund["median_settlement"] > 3]
    if unusual.empty:
        print("    (none)")
    else:
        for _, row in unusual.iterrows():
            print(f"    {row.fund_code} {row.fund_name}: "
                  f"{int(row.min_settlement)}-{int(row.max_settlement)}d, "
                  f"median={int(row.median_settlement)}d ({int(row.tx_count)} tx)")

    # Overall distribution
    print(f"\n  Settlement (calendar days) distribution:")
    dist = tx["settlement_days"].value_counts().sort_index()
    for days, count in dist.items():
        bar = "#" * max(1, int(count / dist.max() * 40))
        print(f"    {int(days):>2}d: {bar} {count}")

    print("-" * 60)


def print_anomalies(anomalies: list[dict]) -> None:
    """Print anomaly summary."""
    if not anomalies:
        print("\n  [OK] No anomalies detected.")
        return

    print(f"\n  [WARN] {len(anomalies)} anomalies flagged -> data/output/anomalies.csv")
    for a in anomalies[:5]:
        print(f"    seq={a['seq']} {a['fund_code']} {a['direction']} {a['trade_time']}: {a['reasons']}")
    if len(anomalies) > 5:
        print(f"    ... and {len(anomalies) - 5} more")


def main():
    parser = argparse.ArgumentParser(description="Enrich transactions with NAV data")
    args = parser.parse_args()
    enrich()


if __name__ == "__main__":
    main()
