"""Generate self-contained interactive HTML dashboard for fund analysis.

Builds the HTML in three clean segments: CSS, body, JavaScript.
No template escaping — pure string concatenation.
"""

import json
import pandas as pd
from pathlib import Path
from datetime import date

from fund_crawler.utils import (
    ENRICHED_CSV,
    FUND_DETAILS_CSV,
    NAV_DIR,
    DATA_OUTPUT,
    log,
    ensure_dirs,
)


def load_data():
    tx = pd.read_csv(ENRICHED_CSV, dtype={"fund_code": str})
    tx["trade_time"] = pd.to_datetime(tx["trade_time"])
    tx["confirm_date"] = pd.to_datetime(tx["confirm_date"])
    details = pd.read_csv(FUND_DETAILS_CSV, dtype={"fund_code": str})
    nav_data = {}
    for path in sorted(NAV_DIR.glob("*.csv")):
        code = path.stem
        df = pd.read_csv(path, parse_dates=["date"])
        cutoff = pd.Timestamp.now() - pd.Timedelta(days=3 * 365)
        df = df[df["date"] >= cutoff]
        if not df.empty:
            nav_data[code] = df
    return tx, details, nav_data


def classify_fund_type(ft: str) -> str:
    t = str(ft)
    if "QDII" in t.upper():
        return "QDII"
    if "债" in t:
        return "债券"
    if "货币" in t:
        return "货币"
    if "指数" in t:
        return "指数"
    if "混合" in t:
        return "混合"
    if "股票" in t:
        return "股票"
    if "黄金" in t:
        return "黄金"
    if "存单" in t:
        return "存单"
    return "其他"


def build_fund_groups(details):
    groups = {}
    for _, row in details.iterrows():
        cat = classify_fund_type(row.get("fund_type", ""))
        groups.setdefault(cat, []).append({
            "code": str(row["fund_code"]).zfill(6),
            "name": str(row["fund_name"]),
            "type": str(row.get("fund_type", "")),
        })
    for cat in ["QDII", "指数", "混合", "股票", "债券", "存单", "黄金", "货币", "其他"]:
        if cat not in groups:
            groups[cat] = []
    return groups


def build_portfolio(tx):
    total_buy = pd.to_numeric(tx[tx["direction"] == "buy"]["confirm_amount"], errors="coerce").sum()
    total_sell = pd.to_numeric(tx[tx["direction"] == "sell"]["confirm_amount"], errors="coerce").sum()
    total_fee = pd.to_numeric(tx["fee"], errors="coerce").sum()
    pnl = pd.to_numeric(tx["unrealized_pnl"], errors="coerce").sum()
    shares_by_fund = tx.groupby("fund_code")["signed_share_change"].apply(
        lambda x: pd.to_numeric(x, errors="coerce").sum()
    )
    held_codes = shares_by_fund[shares_by_fund > 0]
    auto = tx[tx["trade_type"].str.contains("定投", na=False)]
    manual = tx[tx["trade_type"].str.contains("用户", na=False)]
    return {
        "total_tx": len(tx),
        "unique_funds": int(tx["fund_code"].nunique()),
        "held_funds": int(len(held_codes)),
        "total_buy": round(total_buy, 2),
        "total_sell": round(total_sell, 2),
        "total_fee": round(total_fee, 2),
        "unrealized_pnl": round(pnl, 2) if pd.notna(pnl) else 0,
        "auto_tx": len(auto),
        "manual_tx": len(manual),
        "auto_amount": round(pd.to_numeric(auto["confirm_amount"], errors="coerce").sum(), 2),
        "manual_amount": round(pd.to_numeric(manual["confirm_amount"], errors="coerce").sum(), 2),
        "first_trade": str(tx["trade_time"].min().date()) if not tx.empty else "",
        "last_trade": str(tx["trade_time"].max().date()) if not tx.empty else "",
    }


def build_fund_detail(code, tx, nav_df):
    ftx = tx[tx["fund_code"] == code].sort_values("trade_time")
    if ftx.empty:
        return None
    fund_name = str(ftx["fund_name"].iloc[0])
    total_shares = pd.to_numeric(ftx["signed_share_change"], errors="coerce").sum()
    total_cost = pd.to_numeric(ftx["signed_cash_flow"], errors="coerce").sum()
    latest_vals = ftx["latest_nav"].dropna()
    latest_nav = float(latest_vals.iloc[-1]) if len(latest_vals) > 0 else None
    current_value = total_shares * latest_nav if latest_nav and total_shares > 0 else None
    unrealized_pnl = (current_value + total_cost) if current_value is not None else None
    pnl_pct = (unrealized_pnl / abs(total_cost) * 100) if unrealized_pnl and total_cost else None
    sd = ftx["settlement_days"].dropna()
    median_settlement = int(sd.median()) if len(sd) > 0 else 0
    auto_buy = ftx[ftx["trade_type"] == "定投买入"]
    manual_buy = ftx[ftx["trade_type"] == "用户买入"]
    auto = ftx[ftx["trade_type"].str.contains("定投", na=False)]
    manual = ftx[ftx["trade_type"].str.contains("用户", na=False)]
    nav_chart = []
    if nav_df is not None and not nav_df.empty:
        for _, row in nav_df.tail(500).iterrows():
            pct = round(float(row["daily_change_pct"]), 2) if "daily_change_pct" in nav_df.columns and pd.notna(row.get("daily_change_pct")) else 0
            nav_chart.append([str(row["date"].date()), round(float(row["unit_nav"]), 4), pct])
    tx_list = []
    for _, row in ftx.iterrows():
        tx_list.append({
            "seq": int(row.get("seq", 0)) if pd.notna(row.get("seq")) else 0,
            "trade_time": str(row["trade_time"]),
            "confirm_date": str(row["confirm_date"].date()) if pd.notna(row["confirm_date"]) else "",
            "trade_type": str(row["trade_type"]),
            "direction": str(row["direction"]),
            "amount": round(float(row["confirm_amount"]), 2) if pd.notna(row.get("confirm_amount")) else 0,
            "shares": round(float(row["confirm_share"]), 2) if pd.notna(row.get("confirm_share")) else 0,
            "fee": round(float(row["fee"]), 2) if pd.notna(row.get("fee")) else 0,
            "nav": round(float(row["nav_on_effective_date"]), 4) if pd.notna(row.get("nav_on_effective_date")) else None,
            "pnl": round(float(row["unrealized_pnl"]), 2) if pd.notna(row.get("unrealized_pnl")) else None,
            "trade_day_type": str(row.get("trade_day_type", "")),
            "settlement_days": int(row["settlement_days"]) if pd.notna(row.get("settlement_days")) else None,
        })
    return {
        "code": code, "name": fund_name,
        "held_shares": round(total_shares, 2), "total_cost": round(total_cost, 2),
        "latest_nav": round(latest_nav, 4) if latest_nav else None,
        "current_value": round(current_value, 2) if current_value else None,
        "unrealized_pnl": round(unrealized_pnl, 2) if unrealized_pnl is not None else None,
        "pnl_pct": round(pnl_pct, 2) if pnl_pct is not None else None,
        "auto_buy_count": int(len(auto_buy)), "manual_buy_count": int(len(manual_buy)),
        "auto_buy_amount": round(pd.to_numeric(auto_buy["confirm_amount"], errors="coerce").sum(), 2),
        "manual_buy_amount": round(pd.to_numeric(manual_buy["confirm_amount"], errors="coerce").sum(), 2),
        "auto_tx": int(len(auto)), "manual_tx": int(len(manual)),
        "buy_count": int((ftx["direction"] == "buy").sum()),
        "sell_count": int((ftx["direction"] == "sell").sum()),
        "median_settlement": median_settlement,
        "nav_chart": nav_chart, "transactions": tx_list,
    }


# ═══════════════════════════════════════════════════════════════════════════
# CSS
# ═══════════════════════════════════════════════════════════════════════════

CSS = """
*{margin:0;padding:0;box-sizing:border-box}
:root{
  --bg:#0f1219;--sidebar-bg:#161b24;--card-bg:#1a1f2b;--border:#2a3040;
  --text:#c8d0da;--text-dim:#6b7385;--accent:#5b9bd5;--green:#4caf7d;
  --red:#e0556a;--orange:#e8a840;--header-bg:#1e2533;--hover:#222b3a
}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
  background:var(--bg);color:var(--text);display:flex;height:100vh;overflow:hidden}
.sidebar{width:280px;min-width:280px;background:var(--sidebar-bg);
  border-right:1px solid var(--border);display:flex;flex-direction:column;overflow:hidden}
.sidebar-header{padding:20px;border-bottom:1px solid var(--border)}
.sidebar-header h1{font-size:18px;font-weight:600;color:#e8ecf2}
.sidebar-header .sub{font-size:12px;color:var(--text-dim);margin-top:4px}
.portfolio-stats{padding:16px 20px;border-bottom:1px solid var(--border);
  display:grid;grid-template-columns:1fr 1fr;gap:8px}
.stat-item{text-align:center}
.stat-item .val{font-size:16px;font-weight:700;color:#e8ecf2}
.stat-item .label{font-size:10px;color:var(--text-dim);margin-top:2px}
.pnl-pos{color:#4caf7d!important}.pnl-neg{color:#e0556a!important}
.fund-nav{flex:1;overflow-y:auto;padding:8px 0}
.fund-group-title{font-size:11px;font-weight:600;color:var(--text-dim);
  text-transform:uppercase;letter-spacing:1px;padding:12px 16px 6px;
  border-bottom:1px solid var(--border);margin-bottom:2px}
.fund-item{display:flex;align-items:center;justify-content:space-between;
  padding:8px 16px;cursor:pointer;font-size:13px;transition:all .15s}
.fund-item:hover{background:var(--hover)}
.fund-item.active{background:var(--accent);color:#fff}
.fund-item .code{font-size:10px;color:inherit;opacity:.6;margin-left:4px}
.fund-item .mini-pnl{font-size:11px;font-weight:600}
.content{flex:1;overflow-y:auto;padding:24px 32px}
.fund-page{display:none}
.fund-page.active{display:block}
.fund-header{margin-bottom:24px}
.fund-header h2{font-size:22px;font-weight:600;color:#e8ecf2}
.fund-header .meta{font-size:13px;color:var(--text-dim);margin-top:4px}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px;
  margin-bottom:24px}
.card{background:var(--card-bg);border:1px solid var(--border);border-radius:10px;
  padding:14px 16px}
.card .card-label{font-size:11px;color:var(--text-dim);margin-bottom:4px}
.card .card-val{font-size:18px;font-weight:700}
.chart-row{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:24px}
.chart-box{background:var(--card-bg);border:1px solid var(--border);border-radius:10px;overflow:hidden}
.chart-box .chart-inner{height:400px}
.chart-full{grid-column:1/-1}
.section-title{font-size:14px;font-weight:600;padding:16px 20px 0;color:var(--text)}
.tx-table{width:100%;border-collapse:collapse;font-size:12px}
.tx-table th{background:var(--sidebar-bg);padding:10px 12px;text-align:left;
  font-weight:600;color:var(--text-dim);font-size:11px;border-bottom:2px solid var(--border)}
.tx-table td{padding:8px 12px;border-bottom:1px solid var(--border)}
.tx-table tr:hover td{background:var(--hover)}
.tag{display:inline-block;padding:2px 8px;border-radius:10px;font-size:10px;font-weight:600}
.tag-buy{background:rgba(76,175,125,.15);color:#4caf7d}
.tag-sell{background:rgba(224,85,106,.15);color:#e0556a}
.tag-dividend{background:rgba(232,168,64,.15);color:#e8a840}
.tag-auto{background:rgba(91,155,213,.15);color:#5b9bd5}
.tag-manual{background:rgba(107,115,133,.15);color:#6b7385}
.overview-table{width:100%;border-collapse:collapse;font-size:12px;margin-top:16px}
.overview-table th{background:var(--sidebar-bg);padding:10px 12px;text-align:left;
  font-weight:600;color:var(--text-dim);font-size:11px;border-bottom:2px solid var(--border)}
.overview-table td{padding:8px 12px;border-bottom:1px solid var(--border);cursor:pointer}
.overview-table tr:hover td{background:var(--hover)}
.no-data{text-align:center;padding:40px;color:var(--text-dim)}
@media(max-width:900px){.sidebar{display:none}.content{padding:12px}
  .cards{grid-template-columns:1fr 1fr}.chart-row{grid-template-columns:1fr}}
"""


# ═══════════════════════════════════════════════════════════════════════════
# HTML BODY
# ═══════════════════════════════════════════════════════════════════════════

def body_html(portfolio):
    pnl = portfolio["unrealized_pnl"]
    pnl_cls = "pnl-pos" if pnl >= 0 else "pnl-neg"
    pnl_sign = "+" if pnl >= 0 else ""
    return f"""
<div class="sidebar">
  <div class="sidebar-header">
    <h1>Fund Dashboard</h1>
    <div class="sub">Generated: {date.today().isoformat()}</div>
  </div>
  <div class="portfolio-stats">
    <div class="stat-item"><div class="val">{portfolio['held_funds']}</div><div class="label">Held</div></div>
    <div class="stat-item"><div class="val">{portfolio['unique_funds']}</div><div class="label">Total</div></div>
    <div class="stat-item"><div class="val {pnl_cls}">{pnl_sign}{pnl:,.0f}</div><div class="label">P&L (CNY)</div></div>
    <div class="stat-item"><div class="val">{portfolio['total_fee']:.0f}</div><div class="label">Fees</div></div>
  </div>
  <div class="fund-nav" id="fundNav"></div>
</div>
<div class="content" id="content"></div>
"""


# ═══════════════════════════════════════════════════════════════════════════
# JAVASCRIPT
# ═══════════════════════════════════════════════════════════════════════════

JS_HEADER = """
// ---- DATA ----
var FUND_GROUPS = __FUND_GROUPS__;
var FUND_DETAILS = __FUND_DETAILS__;
var PORTFOLIO = __PORTFOLIO__;

var gCharts = {};
var gCurrentPage = 'overview';

// ---- HELPERS ----
function byId(id) { return document.getElementById(id); }
function el(tag, cls, html) {
  var e = document.createElement(tag);
  if (cls) e.className = cls;
  if (html !== undefined) e.innerHTML = html;
  return e;
}

function formatPnl(v) {
  if (v == null || isNaN(v)) return '-';
  return (v >= 0 ? '+' : '') + v.toFixed(2);
}

// ---- NAVIGATION ----
function showPage(id) {
  // update nav highlight
  var items = document.querySelectorAll('.fund-item');
  for (var i = 0; i < items.length; i++) items[i].classList.remove('active');
  var navEl = byId('nav-' + id);
  if (navEl) navEl.classList.add('active');
  else {
    var first = document.querySelector('.fund-item');
    if (first) first.classList.add('active');
  }

  // hide all pages, dispose old charts
  var pages = document.querySelectorAll('.fund-page');
  for (var i = 0; i < pages.length; i++) pages[i].classList.remove('active');
  for (var k in gCharts) { gCharts[k].dispose(); }
  gCharts = {};

  if (id === 'overview') {
    renderOverview();
  } else {
    var page = byId('page-' + id);
    if (!page) {
      page = el('div', 'fund-page', '');
      page.id = 'page-' + id;
      byId('content').appendChild(page);
      renderFundPage(id, page);
    }
    page.classList.add('active');
    setTimeout(function() {
      for (var k in gCharts) gCharts[k].resize();
    }, 150);
  }
  gCurrentPage = id;
}

// ---- BUILD SIDEBAR ----
function buildSidebar() {
  var nav = byId('fundNav');
  nav.appendChild(buildNavItem('overview', 'Portfolio Overview', '', ''));
  var order = ['QDII','指数','混合','股票','债券','存单','黄金','货币','其他'];
  for (var oi = 0; oi < order.length; oi++) {
    var cat = order[oi];
    var funds = FUND_GROUPS[cat];
    if (!funds || funds.length === 0) continue;
    nav.appendChild(el('div', 'fund-group-title', cat + ' (' + funds.length + ')'));
    for (var fi = 0; fi < funds.length; fi++) {
      var f = funds[fi];
      var d = FUND_DETAILS[f.code] || {};
      var pnl = d.unrealized_pnl || 0;
      var pnlCls = pnl >= 0 ? 'pnl-pos' : 'pnl-neg';
      var pnlStr = pnl ? formatPnl(pnl) + 'Y' : '-';
      var op = d.held_shares > 0.001 ? '' : ' style=\"opacity:0.4\"';
      nav.appendChild(buildNavItem(f.code, f.name, pnlStr, pnlCls, op));
    }
  }
}

function buildNavItem(code, name, pnl, pnlCls, extraStyle) {
  var item = el('div', 'fund-item', '');
  item.id = 'nav-' + code;
  item.setAttribute('data-code', code);
  if (extraStyle) item.setAttribute('style', extraStyle.replace(/"/g,''));
  item.innerHTML = '<span>' + name + '<span class=\"code\">' + code + '</span></span>' +
    (pnl ? '<span class=\"mini-pnl ' + pnlCls + '\">' + pnl + '</span>' : '');
  item.addEventListener('click', function() { showPage(code); });
  return item;
}

// ---- OVERVIEW PAGE ----
function renderOverview() {
  var page = byId('page-overview');
  if (!page) {
    page = el('div', 'fund-page overview-page', '');
    page.id = 'page-overview';
    byId('content').appendChild(page);
  }
  page.classList.add('active');
  page.innerHTML = '';

  // Header
  page.appendChild(el('div', 'fund-header',
    '<h2>Portfolio Overview</h2><div class=\"meta\">' +
    PORTFOLIO.first_trade + ' ~ ' + PORTFOLIO.last_trade +
    ' | ' + PORTFOLIO.total_tx + ' transactions | ' +
    PORTFOLIO.auto_tx + ' auto / ' + PORTFOLIO.manual_tx + ' manual</div>'));

  // Summary cards
  var cards = el('div', 'cards', '');
  var cd = [
    ['Total Buy', PORTFOLIO.total_buy.toLocaleString() + ' CNY', ''],
    ['Total Sell', PORTFOLIO.total_sell.toLocaleString() + ' CNY', ''],
    ['Unrealized P&L', formatPnl(PORTFOLIO.unrealized_pnl) + ' CNY', PORTFOLIO.unrealized_pnl >= 0 ? 'pnl-pos' : 'pnl-neg'],
    ['Fees', PORTFOLIO.total_fee.toFixed(2) + ' CNY', ''],
    ['Auto-invest', PORTFOLIO.auto_tx + ' tx / ' + PORTFOLIO.auto_amount.toLocaleString() + ' CNY', ''],
    ['Manual', PORTFOLIO.manual_tx + ' tx / ' + PORTFOLIO.manual_amount.toLocaleString() + ' CNY', ''],
  ];
  for (var ci = 0; ci < cd.length; ci++) {
    var c = el('div', 'card', '');
    c.innerHTML = '<div class=\"card-label\">' + cd[ci][0] + '</div><div class=\"card-val ' + cd[ci][2] + '\">' + cd[ci][1] + '</div>';
    cards.appendChild(c);
  }
  page.appendChild(cards);

  // Charts
  var chartBox1 = el('div', 'chart-box chart-full', '<div class=\"chart-inner\" id=\"chart-ov-held\"></div>');
  page.appendChild(chartBox1);

  var chartRow = el('div', 'chart-row', '');
  chartRow.innerHTML = '<div class=\"chart-box\"><div class=\"chart-inner\" id=\"chart-ov-pie\"></div></div>' +
    '<div class=\"chart-box\"><div class=\"chart-inner\" id=\"chart-ov-top\"></div></div>';
  page.appendChild(chartRow);

  // Held positions table
  var held = [], closed = [];
  for (var code in FUND_DETAILS) {
    var d = FUND_DETAILS[code];
    if (d.held_shares > 0.001) held.push(d);
    else closed.push(d);
  }
  held.sort(function(a,b) { return (b.current_value||0) - (a.current_value||0); });

  var tblDiv = el('div', '', '<div class=\"section-title\" style=\"padding-left:0\">Held Positions (' + held.length + ')</div>');
  var tblHtml = '<table class=\"overview-table\"><thead><tr><th>Fund</th><th>Shares</th><th>Cost</th><th>NAV</th><th>Value</th><th>P&L</th><th>A/M</th><th>T+</th></tr></thead><tbody>';
  for (var hi = 0; hi < held.length; hi++) {
    var h = held[hi];
    var pnl = h.unrealized_pnl || 0;
    var pnlCls = pnl >= 0 ? 'pnl-pos' : 'pnl-neg';
    tblHtml += '<tr onclick=\"showPage(\'' + h.code + '\')\"><td><b>' + h.name + '</b><br><span style=\"font-size:10px;color:var(--text-dim)\">' + h.code + '</span></td>' +
      '<td>' + h.held_shares.toFixed(2) + '</td>' +
      '<td>' + Math.abs(h.total_cost||0).toFixed(2) + '</td>' +
      '<td>' + (h.latest_nav || '-') + '</td>' +
      '<td>' + (h.current_value||0).toFixed(2) + '</td>' +
      '<td class=\"' + pnlCls + '\">' + formatPnl(pnl) + '</td>' +
      '<td><span style=\"color:#5b9bd5\">' + h.auto_buy_count + '</span>/<span style=\"color:#6b7385\">' + h.manual_buy_count + '</span></td>' +
      '<td>' + h.median_settlement + '</td></tr>';
  }
  tblHtml += '</tbody></table>';
  tblDiv.innerHTML += tblHtml;
  page.appendChild(tblDiv);

  // Init charts after DOM
  setTimeout(function() { initOverviewCharts(held); }, 100);
}

function initOverviewCharts(held) {
  // Held bar chart
  var ch1 = echarts.init(byId('chart-ov-held'));
  gCharts['ov-held'] = ch1;
  var topHeld = held.slice(0, 20);
  ch1.setOption({
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 170, right: 40, top: 16, bottom: 30 },
    xAxis: { type: 'value' },
    yAxis: { type: 'category', data: topHeld.map(function(f) { return f.name; }).reverse(),
      axisLabel: { fontSize: 11, width: 160, overflow: 'truncate' } },
    series: [{
      name: 'Cost', type: 'bar', stack: 't',
      data: topHeld.map(function(f) { return Math.abs(f.total_cost||0); }).reverse(),
      itemStyle: { color: '#5b9bd5' }
    }, {
      name: 'P&L', type: 'bar', stack: 't',
      data: topHeld.map(function(f) { var p = f.unrealized_pnl||0; return p > 0 ? p : 0; }).reverse(),
      itemStyle: { color: '#4caf7d' }
    }]
  });

  // Auto vs Manual pie
  var ch2 = echarts.init(byId('chart-ov-pie'));
  gCharts['ov-pie'] = ch2;
  ch2.setOption({
    title: { text: 'Auto vs Manual Investment', left: 'center', top: 14,
      textStyle: { fontSize: 13, color: '#c8d0da' } },
    tooltip: { trigger: 'item' },
    series: [{
      type: 'pie', radius: ['42%','70%'], center: ['50%','55%'],
      label: { formatter: '{b}\n{c} CNY\n({d}%)' },
      data: [
        { value: PORTFOLIO.auto_amount, name: 'Auto-invest (' + PORTFOLIO.auto_tx + ' tx)',
          itemStyle: { color: '#5b9bd5' } },
        { value: PORTFOLIO.manual_amount, name: 'Manual (' + PORTFOLIO.manual_tx + ' tx)',
          itemStyle: { color: '#e8a840' } }
      ]
    }]
  });

  // Top holdings value
  var ch3 = echarts.init(byId('chart-ov-top'));
  gCharts['ov-top'] = ch3;
  var th = held.slice(0, 10);
  ch3.setOption({
    title: { text: 'Top 10 by Market Value', left: 'center', top: 14,
      textStyle: { fontSize: 13, color: '#c8d0da' } },
    tooltip: { trigger: 'axis' },
    grid: { left: 140, right: 20, top: 48, bottom: 20 },
    xAxis: { type: 'value' },
    yAxis: { type: 'category', data: th.map(function(f) { return f.name; }).reverse(),
      axisLabel: { fontSize: 10, width: 130, overflow: 'truncate' } },
    series: [{
      type: 'bar',
      data: th.map(function(f) { return f.current_value||0; }).reverse(),
      itemStyle: { color: function(p) { return p.value > 0 ? '#4caf7d' : '#e0556a'; } }
    }]
  });
}

// ---- FUND DETAIL PAGE ----
function renderFundPage(code, container) {
  var d = FUND_DETAILS[code];
  if (!d) { container.innerHTML = '<div class=\"no-data\">No data for ' + code + '</div>'; return; }
  var pnlCls = (d.unrealized_pnl||0) >= 0 ? 'pnl-pos' : 'pnl-neg';
  var pnlSign = (d.unrealized_pnl||0) >= 0 ? '+' : '';

  container.innerHTML =
    '<div class=\"fund-header\"><h2>' + d.name + '</h2>' +
    '<div class=\"meta\">' + code + ' | T+' + d.median_settlement +
    ' | ' + d.buy_count + ' buys / ' + d.sell_count + ' sells</div></div>' +
    '<div class=\"cards\">' +
    '<div class=\"card\"><div class=\"card-label\">Held Shares</div><div class=\"card-val\">' + d.held_shares.toFixed(2) + '</div></div>' +
    '<div class=\"card\"><div class=\"card-label\">Total Cost</div><div class=\"card-val\">' + Math.abs(d.total_cost||0).toFixed(2) + ' CNY</div></div>' +
    '<div class=\"card\"><div class=\"card-label\">Latest NAV</div><div class=\"card-val\">' + (d.latest_nav||'-') + '</div></div>' +
    '<div class=\"card\"><div class=\"card-label\">Current Value</div><div class=\"card-val\">' + (d.current_value||0).toFixed(2) + ' CNY</div></div>' +
    '<div class=\"card\"><div class=\"card-label\">Unrealized P&L</div><div class=\"card-val ' + pnlCls + '\">' + pnlSign + (d.unrealized_pnl||0).toFixed(2) + ' CNY' +
    (d.pnl_pct ? ' (' + pnlSign + d.pnl_pct.toFixed(2) + '%)' : '') + '</div></div>' +
    '<div class=\"card\"><div class=\"card-label\">Auto / Manual</div><div class=\"card-val\" style=\"font-size:14px\">' +
    '<span style=\"color:#5b9bd5\">' + d.auto_buy_count + '</span> / <span style=\"color:#6b7385\">' + d.manual_buy_count + '</span> buys</div></div>' +
    '</div>';

  // NAV chart
  var navBox = el('div', 'chart-box chart-full',
    '<div class=\"chart-inner\" id=\"chart-' + code + '-nav\"></div>');
  container.appendChild(navBox);

  // Comparison row
  var cr = el('div', 'chart-row', '');
  cr.innerHTML = '<div class=\"chart-box\"><div class=\"chart-inner\" id=\"chart-' + code + '-cost\"></div></div>' +
    '<div class=\"chart-box\"><div class=\"chart-inner\" id=\"chart-' + code + '-pnl\"></div></div>';
  container.appendChild(cr);

  // Transaction table
  var txs = d.transactions || [];
  var txHtml = '<div style=\"margin-top:16px;background:var(--card-bg);border:1px solid var(--border);border-radius:10px;overflow:hidden\">' +
    '<div class=\"section-title\">Transactions (' + txs.length + ')</div>' +
    '<div style=\"overflow-x:auto;max-height:500px;overflow-y:auto\"><table class=\"tx-table\"><thead><tr>' +
    '<th>Date</th><th>Type</th><th>Amount</th><th>Shares</th><th>NAV</th><th>Fee</th><th>T+</th><th>Day</th></tr></thead><tbody>';

  for (var ti = 0; ti < txs.length; ti++) {
    var tx = txs[ti];
    var dirCls = tx.direction === 'buy' ? 'tag-buy' : (tx.direction === 'sell' ? 'tag-sell' : 'tag-dividend');
    var autoTag = '';
    if (tx.trade_type.indexOf('定投') >= 0) autoTag = ' <span class=\"tag tag-auto\">auto</span>';
    else if (tx.trade_type.indexOf('用户') >= 0) autoTag = ' <span class=\"tag tag-manual\">manual</span>';
    txHtml += '<tr><td>' + tx.trade_time.substring(0,16) + '</td>' +
      '<td><span class=\"tag ' + dirCls + '\">' + tx.direction + '</span>' + autoTag + '</td>' +
      '<td>' + tx.amount.toFixed(2) + '</td><td>' + tx.shares.toFixed(2) + '</td>' +
      '<td>' + (tx.nav||'-') + '</td><td>' + (tx.fee||0).toFixed(2) + '</td>' +
      '<td>' + (tx.settlement_days != null ? 'T+' + tx.settlement_days : '-') + '</td>' +
      '<td>' + tx.trade_day_type + '</td></tr>';
  }
  txHtml += '</tbody></table></div></div>';
  container.innerHTML += txHtml;

  // Init charts
  setTimeout(function() {
    initNavChart(code, d);
    initCostPie(code, d);
    initCumPnl(code, d);
  }, 100);
}

function initNavChart(code, d) {
  var el = byId('chart-' + code + '-nav');
  if (!el) return;
  var ch = echarts.init(el);
  gCharts[code + '-nav'] = ch;
  var navData = d.nav_chart || [];
  var dates = navData.map(function(r) { return r[0]; });
  var navs = navData.map(function(r) { return r[1]; });

  var buyMarkers = [], sellMarkers = [], divMarkers = [];
  var txs = d.transactions || [];
  for (var i = 0; i < txs.length; i++) {
    var tx = txs[i];
    var td = tx.trade_time.substring(0,10);
    var idx = dates.indexOf(td);
    if (idx >= 0) {
      if (tx.direction === 'buy') buyMarkers.push({ coord: [td, navs[idx]], value: tx.amount, type: tx.trade_type });
      else if (tx.direction === 'sell') sellMarkers.push({ coord: [td, navs[idx]], value: tx.amount });
      else if (tx.direction === 'dividend') divMarkers.push({ coord: [td, navs[idx]], value: tx.amount });
    }
  }

  var series = [{
    name: 'NAV', type: 'line', data: navs, smooth: true, symbol: 'none',
    lineStyle: { color: '#5b9bd5', width: 1.5 },
    areaStyle: { color: new echarts.graphic.LinearGradient(0,0,0,1,
      [{offset:0,color:'rgba(91,155,213,0.15)'},{offset:1,color:'rgba(91,155,213,0)'}]) }
  }];

  if (buyMarkers.length) {
    series.push({
      name: 'Buy', type: 'scatter',
      data: buyMarkers.map(function(m) { return { value: m.coord, amount: m.value, tradeType: m.type }; }),
      symbolSize: 8, itemStyle: { color: '#4caf7d' }
    });
  }
  if (sellMarkers.length) {
    series.push({
      name: 'Sell', type: 'scatter',
      data: sellMarkers.map(function(m) { return m.coord; }),
      symbolSize: 10, itemStyle: { color: '#e0556a' }
    });
  }
  if (divMarkers.length) {
    series.push({
      name: 'Div', type: 'scatter',
      data: divMarkers.map(function(m) { return m.coord; }),
      symbolSize: 8, symbol: 'diamond', itemStyle: { color: '#e8a840' }
    });
  }

  ch.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['NAV','Buy','Sell','Div'], top: 6, textStyle: { color: '#c8d0da', fontSize: 11 } },
    grid: { left: 60, right: 40, top: 36, bottom: 40 },
    xAxis: { type: 'category', data: dates, axisLabel: { fontSize: 10, color: '#6b7385' } },
    yAxis: { type: 'value', axisLabel: { fontSize: 10, color: '#6b7385' },
      splitLine: { lineStyle: { color: '#2a3040' } } },
    dataZoom: [{ type: 'inside', start: 0, end: 100 },
      { type: 'slider', bottom: 6, height: 20, borderColor: '#2a3040',
        backgroundColor: '#161b24', dataBackground: { lineStyle: { color: '#5b9bd5' },
        areaStyle: { color: 'rgba(91,155,213,0.1)' } } }],
    series: series
  });
}

function initCostPie(code, d) {
  var el = byId('chart-' + code + '-cost');
  if (!el) return;
  var ch = echarts.init(el);
  gCharts[code + '-cost'] = ch;
  ch.setOption({
    title: { text: 'Auto vs Manual Investment', left: 'center', top: 14,
      textStyle: { fontSize: 13, color: '#c8d0da' } },
    tooltip: { trigger: 'item' },
    series: [{
      type: 'pie', radius: ['40%','70%'], center: ['50%','55%'],
      label: { formatter: '{b}\n{c} CNY' },
      data: [
        { value: d.auto_buy_amount, name: 'Auto (' + d.auto_buy_count + ' tx)',
          itemStyle: { color: '#5b9bd5' } },
        { value: d.manual_buy_amount, name: 'Manual (' + d.manual_buy_count + ' tx)',
          itemStyle: { color: '#e8a840' } }
      ]
    }]
  });
}

function initCumPnl(code, d) {
  var el = byId('chart-' + code + '-pnl');
  if (!el) return;
  var ch = echarts.init(el);
  gCharts[code + '-pnl'] = ch;
  var txs = d.transactions || [];
  var cumCost = 0, cumShares = 0;
  var timeline = [];
  for (var i = 0; i < txs.length; i++) {
    var tx = txs[i];
    if (tx.direction === 'buy') { cumCost += tx.amount; cumShares += tx.shares; }
    else if (tx.direction === 'sell') { cumCost -= tx.amount; cumShares -= Math.abs(tx.shares||0); }
    else if (tx.direction === 'dividend') { cumCost -= tx.amount; }
    var nav = tx.nav || d.latest_nav || 0;
    timeline.push({
      date: tx.trade_time.substring(0,10),
      cost: parseFloat(cumCost.toFixed(2)),
      value: parseFloat((cumShares * nav).toFixed(2))
    });
  }
  if (timeline.length === 0) {
    ch.setOption({ title: { text: 'No data', left: 'center', top: 'center',
      textStyle: { color: '#6b7385' } } });
    return;
  }
  ch.setOption({
    title: { text: 'Cumulative Cost vs Market Value', left: 'center', top: 14,
      textStyle: { fontSize: 13, color: '#c8d0da' } },
    tooltip: { trigger: 'axis' },
    legend: { top: 36, textStyle: { color: '#c8d0da', fontSize: 11 } },
    grid: { left: 60, right: 20, top: 56, bottom: 30 },
    xAxis: { type: 'category', data: timeline.map(function(t) { return t.date; }),
      axisLabel: { fontSize: 10, color: '#6b7385' } },
    yAxis: { type: 'value', axisLabel: { fontSize: 10, color: '#6b7385' },
      splitLine: { lineStyle: { color: '#2a3040' } } },
    series: [
      { name: 'Cost', type: 'line', data: timeline.map(function(t) { return t.cost; }),
        lineStyle: { color: '#e8a840', width: 1.5 }, symbol: 'none' },
      { name: 'Value', type: 'line', data: timeline.map(function(t) { return t.value; }),
        lineStyle: { color: '#5b9bd5', width: 1.5 }, symbol: 'none',
        areaStyle: { color: new echarts.graphic.LinearGradient(0,0,0,1,
          [{offset:0,color:'rgba(91,155,213,0.1)'},{offset:1,color:'rgba(91,155,213,0)'}]) } }
    ]
  });
}

// ---- INIT ----
buildSidebar();
showPage('overview');
"""


def generate():
    ensure_dirs()
    log.info("Loading data for dashboard...")
    tx, details, nav_data = load_data()
    fund_groups = build_fund_groups(details)
    portfolio = build_portfolio(tx)

    fund_details = {}
    for code in tx["fund_code"].unique():
        d = build_fund_detail(code, tx, nav_data.get(code))
        if d:
            fund_details[code] = d

    js = (JS_HEADER
          .replace("__FUND_GROUPS__", json.dumps(fund_groups, ensure_ascii=False))
          .replace("__FUND_DETAILS__", json.dumps(fund_details, ensure_ascii=False))
          .replace("__PORTFOLIO__", json.dumps(portfolio, ensure_ascii=False)))

    html = f"""<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Fund Portfolio Dashboard</title>
<script src="echarts.min.js"></script>
<style>{CSS}</style>
</head>
<body>
{body_html(portfolio)}
<script>{js}</script>
</body>
</html>"""

    out_path = DATA_OUTPUT / "fund_dashboard.html"
    out_path.write_text(html, encoding="utf-8")
    log.info("Dashboard generated: %s (%.0f KB)", out_path, len(html.encode("utf-8")) / 1024)
    print(f"\n  Dashboard -> {out_path}")
    return str(out_path)


if __name__ == "__main__":
    generate()
