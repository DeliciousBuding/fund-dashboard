"""Generate summary reports from enriched transaction data.

Includes: portfolio snapshot, NAV quality stats, settlement analysis,
trading suspension detection, and full JSON summary.
"""

import argparse
import json
import sys
import pandas as pd
from datetime import date

from fund_crawler.utils import (
    ENRICHED_CSV,
    FUND_DETAILS_CSV,
    DATA_OUTPUT,
    log,
    ensure_dirs,
)


def load_fund_status() -> pd.DataFrame:
    """Load current fund trading status, fallback to empty if unavailable."""
    status_path = DATA_OUTPUT / "fund_status.csv"
    if status_path.exists():
        return pd.read_csv(status_path, dtype={"fund_code": str})
    return pd.DataFrame()


def report() -> dict:
    """Generate portfolio snapshot, settlement analysis, and summary report."""
    ensure_dirs()

    tx = pd.read_csv(ENRICHED_CSV, dtype={"fund_code": str})
    tx["trade_time"] = pd.to_datetime(tx["trade_time"])
    tx["confirm_date"] = pd.to_datetime(tx["confirm_date"])

    anomalies = pd.read_csv(DATA_OUTPUT / "anomalies.csv") if (DATA_OUTPUT / "anomalies.csv").exists() else pd.DataFrame()

    # ── Fund status (current) ────────────────────────────────────────
    status = load_fund_status()

    # ── Portfolio snapshot (per fund) ─────────────────────────────────
    portfolio_rows = []
    for code, group in tx.groupby("fund_code"):
        fund_name = group["fund_name"].iloc[0]
        total_shares = pd.to_numeric(group["signed_share_change"], errors="coerce").sum()
        total_cost = pd.to_numeric(group["signed_cash_flow"], errors="coerce").sum()
        buy_count = (group["direction"] == "buy").sum()
        sell_count = (group["direction"] == "sell").sum()
        latest_nav_row = group["latest_nav"].dropna()
        latest_nav = float(latest_nav_row.iloc[-1]) if len(latest_nav_row) > 0 else None
        nav_date = group["latest_nav_date"].dropna().iloc[-1] if group["latest_nav_date"].notna().any() else None

        current_value = total_shares * latest_nav if latest_nav and total_shares > 0 else None
        unrealized_pnl = current_value + total_cost if current_value is not None else None

        # Fund status
        purchase_status = None
        redemption_status = None
        if not status.empty:
            s = status[status["fund_code"] == code]
            if not s.empty:
                purchase_status = s.iloc[0].get("purchase_status") or s.iloc[0].get("purchase_status_raw")
                redemption_status = s.iloc[0].get("redemption_status") or s.iloc[0].get("redemption_status_raw")

        # Settlement pattern
        settlement_vals = group["settlement_days"].dropna()
        median_settlement = int(settlement_vals.median()) if len(settlement_vals) > 0 else None

        # 定投 vs 手动分离
        auto_buy = group[group["trade_type"] == "定投买入"]
        manual_buy = group[group["trade_type"] == "用户买入"]
        auto_sell = group[group["trade_type"] == "定投卖出"]
        manual_sell = group[group["trade_type"] == "用户卖出"]

        auto_buy_amount = pd.to_numeric(auto_buy["confirm_amount"], errors="coerce").sum()
        manual_buy_amount = pd.to_numeric(manual_buy["confirm_amount"], errors="coerce").sum()
        auto_buy_shares = pd.to_numeric(auto_buy["signed_share_change"], errors="coerce").sum()
        manual_buy_shares = pd.to_numeric(manual_buy["signed_share_change"], errors="coerce").sum()

        portfolio_rows.append({
            "fund_code": code,
            "fund_name": fund_name,
            "held_shares": round(total_shares, 2),
            "total_cost": round(total_cost, 2),
            "latest_nav": round(latest_nav, 4) if latest_nav else None,
            "nav_date": str(nav_date)[:10] if pd.notna(nav_date) else None,
            "current_value": round(current_value, 2) if current_value else None,
            "unrealized_pnl": round(unrealized_pnl, 2) if unrealized_pnl is not None else None,
            "pnl_pct": round(unrealized_pnl / abs(total_cost) * 100, 2) if unrealized_pnl and total_cost else None,
            "buy_count": int(buy_count),
            "sell_count": int(sell_count),
            "auto_buy_count": int(len(auto_buy)),
            "manual_buy_count": int(len(manual_buy)),
            "auto_buy_amount": round(auto_buy_amount, 2),
            "manual_buy_amount": round(manual_buy_amount, 2),
            "median_settlement_days": median_settlement,
            "purchase_status": purchase_status,
            "redemption_status": redemption_status,
        })

    portfolio = pd.DataFrame(portfolio_rows)
    held = portfolio[portfolio["held_shares"] > 0].sort_values("current_value", ascending=False, na_position="last")
    not_held = portfolio[portfolio["held_shares"] <= 0].sort_values("total_cost")

    # ── Settlement Analysis ──────────────────────────────────────────
    settlement_by_code = (
        tx.groupby("fund_code")
        .agg(
            median_settlement=("settlement_days", "median"),
            max_settlement=("settlement_days", "max"),
            tx_count=("settlement_days", "count"),
        )
        .reset_index()
    )
    settlement_by_code["median_settlement"] = settlement_by_code["median_settlement"].round(0).fillna(0).astype(int)

    # ── NAV quality ──────────────────────────────────────────────────
    nav_matched = int(tx["nav_on_effective_date"].notna().sum())
    nav_total = len(tx)
    nav_verified = int(tx["nav_verified"].sum())
    nav_checked = int(tx["nav_verified"].notna().sum())

    # ── Trading suspension flags ─────────────────────────────────────
    suspended_purchase = []
    suspended_redeem = []
    if not status.empty:
        for _, row in status.iterrows():
            code = str(row.get("fund_code", "")).zfill(6)
            ps = row.get("purchase_status", "")
            rs = row.get("redemption_status", "")
            if ps and ("暂停" in str(ps) or "限" in str(ps)):
                suspended_purchase.append({
                    "fund_code": code,
                    "fund_name": row.get("fund_name", ""),
                    "status": str(ps),
                })
            if rs and "暂停" in str(rs):
                suspended_redeem.append({
                    "fund_code": code,
                    "fund_name": row.get("fund_name", ""),
                    "status": str(rs),
                })

    # ── Summary JSON ─────────────────────────────────────────────────
    total_buy = pd.to_numeric(tx[tx["direction"] == "buy"]["confirm_amount"], errors="coerce").sum()
    total_sell = pd.to_numeric(tx[tx["direction"] == "sell"]["confirm_amount"], errors="coerce").sum()
    total_dividend = pd.to_numeric(tx[tx["direction"] == "dividend"]["confirm_amount"], errors="coerce").sum()
    total_fee = pd.to_numeric(tx["fee"], errors="coerce").sum()

    first_trade = tx["trade_time"].min()
    last_trade = tx["trade_time"].max()

    # Trade day type breakdown
    day_type_counts = tx["trade_day_type"].value_counts().to_dict()

    # Settlement distribution
    settlement_dist = {
        str(int(k)): int(v)
        for k, v in tx["settlement_days"].value_counts().sort_index().items()
    }

    summary = {
        "generated": date.today().isoformat(),
        "overview": {
            "total_transactions": len(tx),
            "unique_funds": int(tx["fund_code"].nunique()),
            "first_trade": str(first_trade.date()),
            "last_trade": str(last_trade.date()),
            "total_buy": round(total_buy, 2),
            "total_sell": round(total_sell, 2),
            "total_dividend": round(total_dividend, 2),
            "total_fee": round(total_fee, 2),
        },
        "nav_quality": {
            "nav_matched": nav_matched,
            "nav_total": nav_total,
            "match_rate_pct": round(nav_matched / nav_total * 100, 1),
            "nav_verified": nav_verified,
            "nav_checked": nav_checked,
            "verify_rate_pct": round(nav_verified / nav_checked * 100, 1) if nav_checked else 0,
        },
        "trade_day_types": day_type_counts,
        "settlement_distribution": settlement_dist,
        "portfolio": {
            "held_funds": int(len(held)),
            "total_current_value": round(held["current_value"].sum(), 2) if not held.empty else 0,
            "total_cost": round(held["total_cost"].sum(), 2) if not held.empty else 0,
            "total_unrealized_pnl": round(held["unrealized_pnl"].sum(), 2) if not held.empty else 0,
            "top_5_holdings": held.head(5)[
                ["fund_code", "fund_name", "current_value", "unrealized_pnl", "pnl_pct"]
            ].to_dict("records"),
        },
        "trading_suspension": {
            "purchase_suspended_or_limited": suspended_purchase,
            "redemption_suspended": suspended_redeem,
        },
        "anomaly_count": len(anomalies),
        "trade_type_breakdown": tx["trade_type"].value_counts().to_dict(),
    }

    # ── Write outputs ─────────────────────────────────────────────────
    portfolio_path = DATA_OUTPUT / "portfolio_snapshot.csv"
    held.to_csv(portfolio_path, index=False)

    # Also write non-held positions for reference
    not_held.to_csv(DATA_OUTPUT / "closed_positions.csv", index=False)

    summary_path = DATA_OUTPUT / "summary_report.json"
    with open(summary_path, "w", encoding="utf-8") as f:
        json.dump(summary, f, ensure_ascii=False, indent=2)

    log.info("Portfolio snapshot: %d held + %d closed -> %s", len(held), len(not_held), portfolio_path)
    log.info("Summary report -> %s", summary_path)

    # ── Print report ─────────────────────────────────────────────────
    print("\n" + "=" * 60)
    print("  PORTFOLIO SUMMARY")
    print("=" * 60)
    print(f"  Held funds:         {len(held)}")
    if not held.empty:
        print(f"  Total cost:         {held['total_cost'].sum():,.2f} CNY")
        print(f"  Current value:      {held['current_value'].sum():,.2f} CNY")
        pnl = held["unrealized_pnl"].sum()
        pnl_pct = pnl / abs(held["total_cost"].sum()) * 100
        print(f"  Unrealized P&L:     {pnl:+,.2f} CNY ({pnl_pct:+.2f}%)")
    print(f"  Total fees paid:    {total_fee:,.2f} CNY")
    print()

    print("=" * 60)
    print("  NAV QUALITY")
    print("=" * 60)
    print(f"  NAV matched:        {nav_matched}/{nav_total} ({nav_matched/nav_total*100:.1f}%)")
    print(f"  NAV verified:       {nav_verified}/{nav_checked} ({nav_verified/nav_checked*100:.1f}%)" if nav_checked else "  NAV verified:       N/A")
    print(f"  Anomalies:          {len(anomalies)}")
    print()

    print("=" * 60)
    print("  TRADING SUSPENSION (current)")
    print("=" * 60)
    if suspended_purchase:
        print("  Purchase suspension/limit:")
        for s in suspended_purchase:
            print(f"    {s['fund_code']} {s['fund_name']}: {s['status']}")
    else:
        print("  (none detected from current snapshot)")
    print()

    print("=" * 60)
    print("  TOP 10 HOLDINGS")
    print("=" * 60)
    for _, row in held.head(10).iterrows():
        pnl_str = f"{row['unrealized_pnl']:+,.2f}" if pd.notna(row["unrealized_pnl"]) else "N/A"
        pct_str = f" ({row['pnl_pct']:+.2f}%)" if pd.notna(row["pnl_pct"]) else ""
        print(f"  {row['fund_code']} {row['fund_name'][:30]}")
        print(f"    Value: {row['current_value']:,.2f}  P&L: {pnl_str}{pct_str}  Settlement: T+{int(row['median_settlement_days'])}" if pd.notna(row.get('median_settlement_days')) else f"    Value: {row['current_value']:,.2f}  P&L: {pnl_str}{pct_str}")
    print("=" * 60 + "\n")

    return summary


def main():
    parser = argparse.ArgumentParser(description="Generate summary reports")
    args = parser.parse_args()
    report()


if __name__ == "__main__":
    main()
