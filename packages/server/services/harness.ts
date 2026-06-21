/**
 * Portfolio Service — allocation, harness snapshot, source brief.
 *
 * Extracted from services/portfolio.ts (2026-06-19).
 */

import { query, queryOne } from "../db";
import type {
  PortfolioAllocation, AllocationBucket,
  InvestmentHarnessSnapshot, InvestmentSourceBrief, InvestmentSourceQuery,
} from "../utils/types";

// ── Allocation (资产配置) ───────────────────────────────────────────

const TYPE_LABELS: Record<string, { key: string; label: string }> = {
  fund: { key: "fund", label: "基金" },
  stock: { key: "stock", label: "股票" },
  etf: { key: "etf", label: "ETF" },
  index: { key: "index", label: "指数" },
};
const MARKET_LABELS: Record<string, { key: string; label: string }> = {
  CN: { key: "cn_fund", label: "中国基金" },
  SH: { key: "a_share_sh", label: "A股沪市" },
  SZ: { key: "a_share_sz", label: "A股深市" },
  HK: { key: "hk_stock", label: "港股" },
  US: { key: "us_stock", label: "美股" },
  "": { key: "unclassified", label: "未分类" },
};

function bucketize(
  rows: { key: string | null; label?: string | null; value: number; count: number }[],
  totalValue: number,
  labels: Record<string, { key: string; label: string }> = {},
): AllocationBucket[] {
  return rows
    .map((r) => {
      const rawKey = r.key || "";
      const value = +(r.value || 0);
      const entry = labels[rawKey];
      return {
        key: entry?.key || rawKey,
        label: entry?.label || r.label || rawKey || "未分类",
        value: +value.toFixed(2),
        weight_pct: totalValue > 0 ? +((value / totalValue) * 100).toFixed(2) : 0,
        count: r.count || 0,
      };
    })
    .filter((r) => r.value > 0)
    .sort((a, b) => b.value - a.value);
}

export function getPortfolioAllocation(portfolioId: number = 1): PortfolioAllocation {
  const totalValue = queryOne<{ v: number }>(`
    SELECT COALESCE(SUM(current_value), 0) as v
    FROM portfolio_snapshot
    WHERE held_shares > 0.001 AND portfolio_id = ?
  `, [portfolioId])?.v ?? 0;

  const byType = bucketize(query<{ key: string; value: number; count: number }>(`
    SELECT COALESCE(ps.security_type, fd.security_type, 'fund') as key,
      COALESCE(SUM(ps.current_value), 0) as value,
      COUNT(*) as count
    FROM portfolio_snapshot ps
    LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
    WHERE ps.held_shares > 0.001 AND ps.portfolio_id = ?
    GROUP BY key
  `, [portfolioId]), totalValue, TYPE_LABELS);

  const byMarket = bucketize(query<{ key: string; value: number; count: number }>(`
    SELECT COALESCE(NULLIF(fd.market, ''), CASE WHEN COALESCE(ps.security_type, fd.security_type, 'fund') = 'fund' THEN 'CN' ELSE '' END) as key,
      COALESCE(SUM(ps.current_value), 0) as value,
      COUNT(*) as count
    FROM portfolio_snapshot ps
    LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
    WHERE ps.held_shares > 0.001 AND ps.portfolio_id = ?
    GROUP BY key
  `, [portfolioId]), totalValue, MARKET_LABELS);

  const byFundType = bucketize(query<{ key: string; value: number; count: number }>(`
    SELECT COALESCE(NULLIF(fd.fund_type, ''), '未分类') as key,
      COALESCE(SUM(ps.current_value), 0) as value,
      COUNT(*) as count
    FROM portfolio_snapshot ps
    LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
    WHERE ps.held_shares > 0.001 AND ps.portfolio_id = ?
    GROUP BY key
  `, [portfolioId]), totalValue);

  const riskFlags: string[] = [];
  const stockWeight = byType.find((r) => r.key === "stock")?.weight_pct ?? 0;
  const topMarket = byMarket[0];
  const topTheme = byFundType[0];
  if (stockWeight > 80) riskFlags.push("股票资产占比高于 80%");
  if (topMarket && topMarket.weight_pct > 70) riskFlags.push(`${topMarket.label}占比高于 70%`);
  if (topTheme && topTheme.weight_pct > 50) riskFlags.push(`${topTheme.label}主题占比高于 50%`);

  const typeBrief = byType.map((r) => `${r.label} ${r.weight_pct}%`).join("，") || "暂无持仓";
  const marketBrief = byMarket.slice(0, 3).map((r) => `${r.label} ${r.weight_pct}%`).join("，");

  return {
    total_value: +(totalValue || 0).toFixed(2),
    by_security_type: byType,
    by_market: byMarket,
    by_fund_type: byFundType,
    risk_flags: riskFlags,
    agent_brief: `资产配置：${typeBrief}${marketBrief ? `；市场：${marketBrief}` : ""}。${riskFlags.length ? `风险提示：${riskFlags.join("；")}。` : "风险提示：暂无集中度警报。"}`,
  };
}

// ── Investment Harness snapshot (Agent 原生事实快照) ───────────────

function signalTags(changePct: number | null, deviationPct: number | null): string[] {
  const tags: string[] = [];
  if (changePct != null) {
    if (changePct <= -5) tags.push("price_drop_gt_5pct");
    else if (changePct >= 5) tags.push("price_rally_gt_5pct");
    else tags.push("price_range_bound");
  }
  if (deviationPct != null) {
    if (deviationPct <= -10) tags.push("below_cost_gt_10pct");
    else if (deviationPct >= 10) tags.push("above_cost_gt_10pct");
    else tags.push("near_cost_basis");
  }
  return tags;
}

export function getInvestmentHarnessSnapshot(portfolioId: number = 1): InvestmentHarnessSnapshot {
  const allocation = getPortfolioAllocation(portfolioId);
  const rows = query<{
    fund_code: string; fund_name: string | null; held_shares: number; total_cost: number; latest_nav: number;
    current_value: number; security_type: string | null; market: string | null; daily_change_pct: number | null;
  }>(`
    SELECT ps.fund_code, COALESCE(fd.fund_name, ps.fund_name) as fund_name,
      ps.held_shares, ps.total_cost, ps.latest_nav, ps.current_value,
      COALESCE(ps.security_type, fd.security_type, 'fund') as security_type,
      COALESCE(fd.market, '') as market,
      (
        SELECT daily_change_pct FROM nav_history
        WHERE fund_code = ps.fund_code
        ORDER BY date DESC
        LIMIT 1
      ) as daily_change_pct
    FROM portfolio_snapshot ps
    LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
    WHERE ps.held_shares > 0.001 AND ps.latest_nav IS NOT NULL AND ps.portfolio_id = ?
    ORDER BY ps.current_value DESC NULLS LAST
  `, [portfolioId]);

  const holdingSignals = rows.map((row) => {
    const costPerShare = row.total_cost && row.held_shares > 0 ? Math.abs(row.total_cost) / row.held_shares : null;
    const deviationPct = costPerShare && row.latest_nav > 0 ? +(((row.latest_nav - costPerShare) / costPerShare) * 100).toFixed(2) : null;
    const changePct = row.daily_change_pct != null ? +row.daily_change_pct.toFixed(2) : null;
    return {
      code: row.fund_code,
      name: row.fund_name || row.fund_code,
      security_type: row.security_type || "fund",
      market: row.market || "",
      held_shares: +(row.held_shares || 0).toFixed(2),
      current_value: +(row.current_value || 0).toFixed(2),
      weight_pct: allocation.total_value > 0 ? +(((row.current_value || 0) / allocation.total_value) * 100).toFixed(2) : 0,
      latest_nav: +(row.latest_nav || 0).toFixed(4),
      cost_per_share: costPerShare != null ? +costPerShare.toFixed(4) : null,
      change_pct: changePct,
      deviation_pct: deviationPct,
      signal_tags: signalTags(changePct, deviationPct),
      data_points: {
        has_price: !!row.latest_nav,
        has_cost_basis: costPerShare != null,
        has_change_pct: changePct != null,
      },
    };
  });

  const stalePriceCount = holdingSignals.filter((item) => !item.data_points.has_price).length;
  const missingCostBasisCount = holdingSignals.filter((item) => !item.data_points.has_cost_basis).length;
  const missingChangePctCount = holdingSignals.filter((item) => !item.data_points.has_change_pct).length;

  // Holdings coverage: what % of held funds have fund_holdings data
  const coverageRow = queryOne<{ total: number; with_holdings: number }>(`
    SELECT
      (SELECT COUNT(DISTINCT fund_code) FROM portfolio_snapshot WHERE held_shares > 0.001) as total,
      (SELECT COUNT(DISTINCT ps.fund_code)
       FROM portfolio_snapshot ps
       JOIN fund_holdings fh ON fh.fund_code = ps.fund_code
       WHERE ps.held_shares > 0.001) as with_holdings
  `);
  const holdingsCoveragePct = coverageRow && coverageRow.total > 0
    ? +((coverageRow.with_holdings / coverageRow.total) * 100).toFixed(1)
    : 0;

  return {
    generated_at: new Date().toISOString(),
    decision_boundary: "facts_only",
    total_value: allocation.total_value,
    holdings_count: holdingSignals.length,
    allocation,
    holding_signals: holdingSignals,
    data_quality: {
      stale_price_count: stalePriceCount,
      missing_cost_basis_count: missingCostBasisCount,
      missing_change_pct_count: missingChangePctCount,
      holdings_coverage_pct: holdingsCoveragePct,
    },
    available_agent_tools: [
      "get_portfolio_summary",
      "get_portfolio_allocation",
      "get_portfolio_penetration",
      "get_fund_detail",
      "get_nav_history",
      "compute_dca_amount",
      "crawl_nav",
      "crawl_fund_holdings",
    ],
    agent_brief: `Investment Harness facts only: ${holdingSignals.length} held assets, total value ${allocation.total_value.toFixed(2)}. Allocation: ${allocation.agent_brief} Data gaps: price ${stalePriceCount}, cost basis ${missingCostBasisCount}, change pct ${missingChangePctCount}. Agent owns all investment decisions and operations.`,
  };
}

// ── Source brief for Hermes WebSearch / crawling ────────────────────

function dedupeQueries(queries: InvestmentSourceQuery[], limit: number): InvestmentSourceQuery[] {
  const seen = new Set<string>();
  const out: InvestmentSourceQuery[] = [];
  for (const q of queries) {
    const key = q.query.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(q);
    if (out.length >= limit) break;
  }
  return out;
}

export function getInvestmentSourceBrief(options: { limit?: number; portfolioId?: number } = {}): InvestmentSourceBrief {
  const limit = Math.max(1, Math.min(options.limit ?? 20, 50));
  const pid = options.portfolioId ?? 1;
  const holdings = query<{
    fund_code: string; fund_name: string | null; current_value: number; security_type: string | null; market: string | null; fund_type: string | null;
  }>(`
    SELECT ps.fund_code, COALESCE(fd.fund_name, ps.fund_name) as fund_name,
      ps.current_value, COALESCE(ps.security_type, fd.security_type, 'fund') as security_type,
      COALESCE(fd.market, '') as market, COALESCE(fd.fund_type, '') as fund_type
    FROM portfolio_snapshot ps
    LEFT JOIN fund_details fd ON fd.fund_code = ps.fund_code
    WHERE ps.held_shares > 0.001 AND ps.portfolio_id = ?
    ORDER BY ps.current_value DESC NULLS LAST
    LIMIT 20
  `, [pid]);
  const underlying = query<{ stock_code: string; stock_name: string; exposure: number }>(`
    SELECT fh.stock_code, fh.stock_name, SUM(COALESCE(ps.current_value, 0) * fh.weight_pct / 100.0) as exposure
    FROM fund_holdings fh
    JOIN portfolio_snapshot ps ON ps.fund_code = fh.fund_code
    WHERE ps.held_shares > 0.001 AND ps.portfolio_id = ?
    GROUP BY fh.stock_code, fh.stock_name
    ORDER BY exposure DESC
    LIMIT 20
  `, [pid]);

  const queries: InvestmentSourceQuery[] = [
    {
      id: "portfolio-global-market",
      scope: "portfolio",
      entity_code: null,
      entity_name: "portfolio",
      query: "今日 全球市场 纳斯达克 港股 A股 汇率 影响 QDII 基金",
      reason: "组合跨 CN/HK/US 市场，需要宏观和市场层消息作为背景。",
      freshness: "intraday",
    },
  ];

  for (const h of holdings) {
    const name = h.fund_name || h.fund_code;
    const market = h.market || (h.security_type === "stock" ? "stock" : "fund");
    queries.push({
      id: `holding-${h.fund_code}`,
      scope: "holding",
      entity_code: h.fund_code,
      entity_name: name,
      query: `${name} ${h.fund_code} ${market} 最新消息 公告 持仓 估值`,
      reason: `持仓级消息源，用于核对 ${name} 的公告、净值/股价和主题变化。`,
      freshness: h.security_type === "stock" ? "intraday" : "daily",
    });
  }

  for (const u of underlying) {
    queries.push({
      id: `underlying-${u.stock_code}`,
      scope: "underlying",
      entity_code: u.stock_code,
      entity_name: u.stock_name,
      query: `${u.stock_name} ${u.stock_code} earnings news guidance regulation`,
      reason: `穿透持仓底层股票消息源，估算组合间接暴露的新闻风险。`,
      freshness: "intraday",
    });
  }

  const finalQueries = dedupeQueries(queries, limit);

  return {
    generated_at: new Date().toISOString(),
    decision_boundary: "source_queries_only",
    queries: finalQueries,
    source_targets: [
      { kind: "web_search", name: "Hermes WebSearch", url_template: null, use_for: "新闻、公告、监管和宏观消息检索" },
      { kind: "market_data", name: "fund-dashboard MCP", url_template: "mcp:get_fund_detail({code})", use_for: "本地持仓、价格、成本、交易流水事实" },
      { kind: "official_disclosure", name: "Eastmoney / fundf10", url_template: "https://fundf10.eastmoney.com/ccmx_{code}.html", use_for: "基金季报持仓和披露核对" },
      { kind: "official_disclosure", name: "Yahoo Finance", url_template: "https://finance.yahoo.com/quote/{code}", use_for: "美股行情、财报和公司事件入口" },
      { kind: "local_mcp", name: "crawl_fund_holdings", url_template: "mcp:crawl_fund_holdings({fund_code})", use_for: "补全本地 fund_holdings 穿透数据" },
    ],
    coverage: {
      holdings_scanned: holdings.length,
      underlying_scanned: underlying.length,
      max_queries: limit,
    },
    agent_brief: `Hermes source brief: ${finalQueries.length} source queries generated from ${holdings.length} holdings and ${underlying.length} underlying stocks. This is search/crawl context only; investment decisions stay with the agent.`,
  };
}
