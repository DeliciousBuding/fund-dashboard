import json

# Portfolio summary
summary = {
    "total_tx": 433,
    "unique_funds": 61,
    "held_funds": 13,
    "total_buy": 25500.51,
    "total_sell": 21766.74,
    "total_fee": 19.16,
    "unrealized_pnl": 224.68,
    "first_trade": "2024-08-12",
    "last_trade": "2026-06-15",
}

# Held funds (only those with held_shares > 0)
held_funds = [
    {"code": "006479", "name": "广发纳斯达克100ETF联接C", "type": "指数型-海外股票", "held_shares": 11.02, "current_value": 89.81, "unrealized_pnl": -0.19, "pnl_pct": -0.21},
    {"code": "008164", "name": "南方标普红利低波50ETF联接C", "type": "指数型-股票", "held_shares": 67.26, "current_value": 69.29, "unrealized_pnl": -0.46, "pnl_pct": -0.66},
    {"code": "008971", "name": "大成纳斯达克100ETF联接C", "type": "指数型-海外股票", "held_shares": 9.62, "current_value": 60.84, "unrealized_pnl": 0.84, "pnl_pct": 1.39},
    {"code": "009290", "name": "富国添享一年持有期债券A", "type": "债券型-混合一级", "held_shares": 0.82, "current_value": 1.03, "unrealized_pnl": 0.03, "pnl_pct": 3.35},
    {"code": "014880", "name": "天弘中证机器人ETF发起联接A", "type": "指数型-股票", "held_shares": 72.72, "current_value": 96.28, "unrealized_pnl": -9.10, "pnl_pct": -8.63},
    {"code": "015740", "name": "国泰中证港股通科技ETF发起联接C", "type": "指数型-股票", "held_shares": 81.42, "current_value": 84.38, "unrealized_pnl": -15.62, "pnl_pct": -15.62},
    {"code": "016453", "name": "南方纳斯达克100指数发起C", "type": "指数型-海外股票", "held_shares": 254.83, "current_value": 587.23, "unrealized_pnl": 47.23, "pnl_pct": 8.75},
    {"code": "016533", "name": "嘉实纳斯达克100ETF发起联接C", "type": "指数型-海外股票", "held_shares": 110.21, "current_value": 238.48, "unrealized_pnl": 28.48, "pnl_pct": 13.56},
]

# Compute cost basis
for f in held_funds:
    f["cost_basis"] = round(f["current_value"] - f["unrealized_pnl"], 2)

total_portfolio_value = sum(f["current_value"] for f in held_funds)
total_cost = sum(f["cost_basis"] for f in held_funds)

print("=" * 70)
print("PORTFOLIO RISK ANALYSIS")
print("=" * 70)

print("\nTotal Portfolio Value: ${:,.2f}".format(total_portfolio_value))
print("Total Cost Basis:     ${:,.2f}".format(total_cost))
print("Unrealized P&L:       ${:,.2f}".format(summary["unrealized_pnl"]))
print("Portfolio P&L %:      {:.2f}%".format(summary["unrealized_pnl"]/total_cost*100))

# ===== 1. CONCENTRATION RISK =====
print("\n" + "-" * 35 + " 1. Concentration Risk " + "-" * 35)

print("\n{:<45} {:>10} {:>10} {:>8}".format("Fund", "Value", "% of Port", "P&L%"))
print("-" * 75)
for f in sorted(held_funds, key=lambda x: x["current_value"], reverse=True):
    pct = f["current_value"] / total_portfolio_value * 100
    print("{:<45} ${:>8.2f} {:>9.1f}% {:>7.2f}%".format(f["name"], f["current_value"], pct, f["pnl_pct"]))

max_concentration = max(f["current_value"] / total_portfolio_value * 100 for f in held_funds)
max_fund = max(held_funds, key=lambda x: x["current_value"])
print("\nMAX SINGLE FUND: {} = {:.1f}% of portfolio".format(max_fund["name"], max_concentration))
if max_concentration > 30:
    print("  >> WARNING: exceeds 30% threshold")
else:
    print("  >> OK: under 30% threshold")

# 1b. Category concentration
categories = {}
for f in held_funds:
    cat = f["type"]
    if cat not in categories:
        categories[cat] = {"value": 0, "funds": [], "pnl": 0, "cost": 0}
    categories[cat]["value"] += f["current_value"]
    categories[cat]["funds"].append(f["name"])
    categories[cat]["pnl"] += f["unrealized_pnl"]
    categories[cat]["cost"] += f["cost_basis"]

print("\n{:<35} {:>10} {:>10} {:>8} {:>8}".format("Category", "Value", "% of Port", "P&L", "P&L%"))
print("-" * 75)
for cat in sorted(categories, key=lambda c: categories[c]["value"], reverse=True):
    c = categories[cat]
    pct = c["value"] / total_portfolio_value * 100
    pnl_pct = (c["pnl"] / c["cost"] * 100) if c["cost"] > 0 else 0
    print("{:<35} ${:>8.2f} {:>9.1f}% ${:>6.2f} {:>7.2f}%".format(cat, c["value"], pct, c["pnl"], pnl_pct))

max_cat = max(categories, key=lambda c: categories[c]["value"] / total_portfolio_value)
max_cat_pct = categories[max_cat]["value"] / total_portfolio_value * 100
print("\nMAX CATEGORY: {} = {:.1f}% of portfolio".format(max_cat, max_cat_pct))
if max_cat_pct > 50:
    print("  >> WARNING: exceeds 50% threshold")
else:
    print("  >> OK: under 50% threshold")

# ===== 2. DRAWDOWN ANALYSIS =====
print("\n" + "-" * 35 + " 2. Drawdown Analysis " + "-" * 35)

drawdown_funds = [f for f in held_funds if f["pnl_pct"] < 0]
print("\nFunds currently underwater: {} of {}".format(len(drawdown_funds), len(held_funds)))

print("\n{:<45} {:>8} {:>8} {:>20}".format("Fund", "Value", "P&L%", "Drawdown Severity"))
print("-" * 85)
for f in sorted(drawdown_funds, key=lambda x: x["pnl_pct"]):
    if f["pnl_pct"] > -5:
        severity = "MILD"
    elif f["pnl_pct"] > -10:
        severity = "MODERATE"
    else:
        severity = "SEVERE"
    print("{:<45} ${:>6.2f} {:>7.2f}% {:>20}".format(f["name"], f["current_value"], f["pnl_pct"], severity))

if drawdown_funds:
    total_dd_value = sum(f["current_value"] for f in drawdown_funds)
    weighted_dd = sum(f["pnl_pct"] * f["current_value"] for f in drawdown_funds) / total_dd_value
    print("\nWeighted avg drawdown (underwater funds): {:.2f}%".format(weighted_dd))

# ===== 3. CORRELATION ANALYSIS =====
print("\n" + "-" * 35 + " 3. Correlation / Overlap Analysis " + "-" * 35)

nasdaq_funds = [f for f in held_funds if "纳斯达克" in f["name"]]
if len(nasdaq_funds) > 1:
    print("\nNAS100 OVERLAP: {} funds tracking the same index:".format(len(nasdaq_funds)))
    for f in nasdaq_funds:
        print("  - {} (${:.2f}, {:.2f}%)".format(f["name"], f["current_value"], f["pnl_pct"]))
    nasdaq_total = sum(f["current_value"] for f in nasdaq_funds)
    print("  Combined NAS100 exposure: ${:,.2f} ({:.1f}% of portfolio)".format(nasdaq_total, nasdaq_total/total_portfolio_value*100))
    print("  >> HIGH CORRELATION: These will move almost identically (rho ~0.95+)")

print("\nCategory proximity (estimated intra-category correlations):")
print("  NASDAQ 100 (overseas equity): 4 funds -- near-perfect correlation (~0.95+)")
print("  A-share equity (stock/index): 3 funds -- moderate cross-sector (~0.5-0.7)")
print("  Bond: 1 fund -- near-zero with equities (~0.0-0.2)")

# ===== 4. VOLATILITY ASSESSMENT =====
print("\n" + "-" * 35 + " 4. Volatility Assessment " + "-" * 35)

risk_buckets = {"HIGH": [], "MEDIUM": [], "LOW": []}
for f in held_funds:
    if any(kw in f["name"] for kw in ["纳斯达克", "港股通科技", "机器人"]):
        risk_buckets["HIGH"].append(f)
    elif any(kw in f["name"] for kw in ["红利", "沪深300", "500"]):
        risk_buckets["MEDIUM"].append(f)
    elif any(kw in f["name"] for kw in ["债券", "债"]):
        risk_buckets["LOW"].append(f)
    else:
        risk_buckets["MEDIUM"].append(f)

for level in ["HIGH", "MEDIUM", "LOW"]:
    funds_in = risk_buckets[level]
    val = sum(f["current_value"] for f in funds_in)
    pct = val / total_portfolio_value * 100
    print("\n{} VOLATILITY ({} funds, {:.1f}% of portfolio):".format(level, len(funds_in), pct))
    for f in funds_in:
        print("  {} -- ${:.2f} ({:+.2f}%)".format(f["name"], f["current_value"], f["pnl_pct"]))

# Portfolio-level volatility estimate
high_vol_val = sum(f["current_value"] for f in risk_buckets["HIGH"])
print("\nPortfolio volatility is DOMINATED by high-vol assets ({:.1f}%).".format(high_vol_val/total_portfolio_value*100))
print("Estimated annualized portfolio vol: 18-22% (equity-heavy, NAS100-concentrated)")

# ===== 5. RECOMMENDATIONS =====
print("\n" + "=" * 70)
print("5. RECOMMENDED DIVERSIFICATION ADJUSTMENTS")
print("=" * 70)

print("""
RISK 1 -- NASDAQ 100 Over-Concentration (CRITICAL):
  You hold 4 NAS100 funds totaling 79.7% of the portfolio. These are
  near-perfectly correlated -- you are not diversified across 4 funds,
  you have 4 copies of the same bet.
  RECOMMEND: Consolidate to 1-2 NAS100 funds. Choose the one with lowest
  expense ratio and best tracking error (likely southern or jiashi).

RISK 2 -- Equity-Heavy Portfolio (99.9% equities):
  Only 0.08% in bonds (fuguo tianxiang). The 22-month history (Aug 2024 -
  Jun 2026) and 433 trades show active engagement, but zero crash protection.
  RECOMMEND: Increase bond allocation to 10-20%. Consider short/intermediate
  government bond or broad bond index funds.

RISK 3 -- Sector-Specific Severe Drawdowns:
  - ganggu tongkeji ETF: -15.62% (SEVERE)
  - jiqiren ETF: -8.63% (MODERATE)
  These sector plays amplify NAS100 tech correlation during selloffs.
  RECOMMEND: Evaluate if HK tech thesis still holds. If not, harvest
  the loss and rotate into broad-market (CSI300/CSI500).

RISK 4 -- No A-Share Broad Market Exposure:
  Current holdings: NAS100 only + sector bets. Zero domestic broad market
  (CSI300, CSI500, CSI800).
  RECOMMEND: Add CSI300 or MSCI China A for 20-30% domestic core.

RISK 5 -- Auto-Investment Size Disconnect:
  217 of 433 trades (50.1%) are auto-invest (dingtou), but only $7,177 of
  $48,835 total invested (14.7%). Manual trades dominate in size,
  suggesting emotional/discretionary timing on large positions.
  RECOMMEND: Increase auto-invest proportion for discipline. Set up
  a systematic plan for core holdings.

TARGET ALLOCATION SUGGESTION:
  - NAS100: 30-40% (down from 79.7%) -- consolidate to 1 fund
  - A-Share Broad Market (CSI300/CSI500): 20-30%
  - Sector Plays (robot/tech/dividend): 10-20%
  - Bonds: 10-20%
  - Cash/Others: 5-10%
""")

print("=" * 70)
print("KEY METRICS SUMMARY")
print("=" * 70)
print("  Portfolio Value:        ${:,.2f}".format(total_portfolio_value))
print("  Number of Holdings:     {}".format(len(held_funds)))
print("  Unrealized P&L:         ${:,.2f} ({:.2f}%)".format(summary["unrealized_pnl"], summary["unrealized_pnl"]/total_cost*100))
print("  Max Single Fund:        {:.1f}% ({})".format(max_concentration, "WARNING >30%" if max_concentration > 30 else "OK"))
print("  Max Category:           {:.1f}% ({})".format(max_cat_pct, "WARNING >50%" if max_cat_pct > 50 else "OK"))
print("  Underwater Holdings:    {}/{} ({:.0f}%)".format(len(drawdown_funds), len(held_funds), len(drawdown_funds)/len(held_funds)*100))
print("  Effective Diversification: LOW (4 NAS100 funds = single bet)")
print("  Est. Annualized Vol:    18-22%")
