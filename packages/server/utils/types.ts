/** Shared TypeScript interfaces for all database rows and service return types */

// ── Database row types ─────────────────────────────────────────────────

export interface TransactionRow {
  seq: number;
  order_id: string | null;
  trade_time: string;
  confirm_date: string | null;
  trade_type: string | null;
  direction: string;
  fund_code: string;
  fund_name: string | null;
  confirm_amount: number;
  confirm_share: number;
  fee: number;
  inferred_nav: number | null;
  nav_on_effective_date: number | null;
  nav_verified: number | boolean;
  signed_cash_flow: number | null;
  signed_share_change: number | null;
  trade_day_type: string | null;
  settlement_days: number | null;
  effective_nav_date: string | null;
  latest_nav: number | null;
  cost_basis: number | null;
  unrealized_pnl: number | null;
  anomaly: string | null;
}

export interface FundDetailRow {
  fund_code: string;
  fund_name: string;
  fund_type: string | null;
  security_type: string | null;
  market: string | null;
  currency: string | null;
  exchange: string | null;
}

export interface NavHistoryRow {
  date: string;
  fund_code: string;
  unit_nav: number;
  daily_change_pct: number;
  security_type: string | null;
}

export interface PortfolioSnapshotRow {
  fund_code: string;
  fund_name: string | null;
  held_shares: number;
  total_cost: number;
  latest_nav: number | null;
  current_value: number | null;
  unrealized_pnl: number | null;
  pnl_pct: number | null;
  security_type: string | null;
}

// ── Service return types ───────────────────────────────────────────────

export interface PortfolioSummary {
  total_tx: number;
  unique_funds: number;
  unique_stocks: number;
  held_funds: number;
  total_buy: number;
  total_sell: number;
  total_fee: number;
  unrealized_pnl: number;
  auto_tx: number;
  manual_tx: number;
  auto_amount: number;
  manual_amount: number;
  first_trade: string;
  last_trade: string;
  last_nav_date: string | null;
  settlement_distribution: Record<string, number>;
  trade_type_breakdown: Record<string, number>;
  by_security_type: { security_type: string; count: number; total_value: number; total_pnl: number }[];
}

export interface AllocationBucket {
  key: string;
  label: string;
  value: number;
  weight_pct: number;
  count: number;
}

export interface PortfolioAllocation {
  total_value: number;
  by_security_type: AllocationBucket[];
  by_market: AllocationBucket[];
  by_fund_type: AllocationBucket[];
  risk_flags: string[];
  agent_brief: string;
}

export interface InvestmentHarnessHoldingSignal {
  code: string;
  name: string;
  security_type: string;
  market: string;
  held_shares: number;
  current_value: number;
  weight_pct: number;
  latest_nav: number;
  cost_per_share: number | null;
  change_pct: number | null;
  deviation_pct: number | null;
  signal_tags: string[];
  data_points: {
    has_price: boolean;
    has_cost_basis: boolean;
    has_change_pct: boolean;
  };
}

export interface InvestmentHarnessSnapshot {
  generated_at: string;
  decision_boundary: "facts_only";
  total_value: number;
  holdings_count: number;
  allocation: PortfolioAllocation;
  holding_signals: InvestmentHarnessHoldingSignal[];
  data_quality: {
    stale_price_count: number;
    missing_cost_basis_count: number;
    missing_change_pct_count: number;
    holdings_coverage_pct: number;
  };
  available_agent_tools: string[];
  agent_brief: string;
}

export interface InvestmentSourceQuery {
  id: string;
  scope: "portfolio" | "holding" | "underlying";
  entity_code: string | null;
  entity_name: string;
  query: string;
  reason: string;
  freshness: "intraday" | "daily" | "weekly";
}

export interface InvestmentSourceTarget {
  kind: "web_search" | "market_data" | "official_disclosure" | "local_mcp";
  name: string;
  url_template: string | null;
  use_for: string;
}

export interface InvestmentSourceBrief {
  generated_at: string;
  decision_boundary: "source_queries_only";
  queries: InvestmentSourceQuery[];
  source_targets: InvestmentSourceTarget[];
  coverage: {
    holdings_scanned: number;
    underlying_scanned: number;
    max_queries: number;
  };
  agent_brief: string;
}

export interface FundTransaction {
  seq: number;
  trade_time: string;
  confirm_date: string;
  trade_type: string | null;
  direction: string;
  amount: number;
  shares: number;
  fee: number;
  nav: number | null;
  inferred_nav: number | null;
  nav_verified: boolean;
  trade_day_type: string;
  settlement_days: number | null;
  effective_nav_date: string;
  anomaly: string | null;
  order_id: string;
}

export interface FundDetail {
  code: string;
  name: string;
  held_shares: number;
  total_cost: number;
  latest_nav: number | null;
  current_value: number | null;
  unrealized_pnl: number | null;
  pnl_pct: number | null;
  auto_buy_count: number;
  manual_buy_count: number;
  auto_buy_amount: number;
  manual_buy_amount: number;
  auto_tx: number;
  manual_tx: number;
  buy_count: number;
  sell_count: number;
  median_settlement: number;
  transactions: FundTransaction[];
}

export interface TimelineEntry {
  date: string;
  total_value: number;
  total_cost: number;
  pnl: number;
  pnl_pct: number;
}

export interface PenetrationFund {
  fund_code: string;
  fund_name: string;
  weight_pct: number;
  fund_value_cny: number;
}

export interface PenetrationStock {
  stock_code: string;
  stock_name: string;
  total_exposure_cny: number;
  weight_pct: number;
  held_by_funds: PenetrationFund[];
}

export interface PenetrationResult {
  penetration: PenetrationStock[];
  total_portfolio_value: number;
  equity_fund_count: number;
  unique_stocks: number;
}

export interface SystemStatus {
  transactions: { count: number; last_trade: string | null };
  nav: { count: number; funds_covered: number; date_range: { first: string | null; last: string | null } };
  portfolio: { total_securities: number; held_securities: number; by_type: Record<string, number> };
  holdings_data: { funds_with_holdings: number; total_stock_positions: number };
  indices: { code: string; name: string; price: number; change_pct: number; updated: string }[];
  anomalies: { count: number; items: unknown[] };
}

export interface DrawdownResult {
  max_drawdown: number;
  peak_date: string;
  trough_date: string;
  code: string;
}

// ── DB Integrity types ─────────────────────────────────────────────────

export interface IntegrityReport {
  timestamp: string;
  overall: "ok" | "degraded" | "corrupted";
  checks: {
    integrity_check: { passed: boolean; detail: string };
    foreign_key_check: { passed: boolean; violations: number };
    quick_check: { passed: boolean; result: string };
    freelist_count: { passed: boolean; freelist: number; detail: string };
  };
  table_checksums: Record<string, string>;
  row_counts: Record<string, number>;
  recommendations: string[];
}

export interface RestoreResult {
  success: boolean;
  source: string;
  target: string;
  tables_restored: number;
  rows_restored: number;
  errors: string[];
}

// ── Source Events (V4: Hermes/Agent news & research context) ────────────

export interface SourceEventRow {
  id: number;
  title: string;
  url: string | null;
  source: string;
  snippet: string | null;
  query: string | null;
  related_security_code: string | null;
  related_security_name: string | null;
  is_read: number;
  is_useful: number;
  fetched_at: string;
  created_at: string;
}

export interface CreateSourceEventInput {
  title: string;
  url?: string;
  source?: string;
  snippet?: string;
  query?: string;
  related_security_code?: string;
  related_security_name?: string;
}

export interface GetSourceEventsOptions {
  limit?: number;
  offset?: number;
  related_security_code?: string;
  source?: string;
  is_read?: number;
  show_read?: boolean;
}

// ── Backtest types ──────────────────────────────────────────────────────

export type BacktestStrategy = "grid" | "momentum" | "rebalance" | "dca";

export interface BacktestParams {
  fund_code: string;
  strategy: BacktestStrategy;
  start_date: string;   // YYYY-MM-DD
  base_amount: number;
  /** Grid strategy: number of grid levels (default 5) */
  grid_levels?: number;
  /** Momentum strategy: lookback months (default 3) */
  momentum_months?: number;
  /** Rebalance strategy: target equity weight 0-1 (default 0.6) */
  target_weight?: number;
  /** Rebalance strategy: rebalance interval in months (default 3) */
  rebalance_interval?: number;
}

export interface BacktestTrade {
  date: string;
  action: "buy" | "sell";
  price: number;
  shares: number;
  amount: number;
  reason: string;
}

export interface BacktestTimelinePoint {
  date: string;
  nav: number;
  shares_held: number;
  cash: number;
  equity_value: number;
  total_value: number;
  total_invested: number;
}

export interface BacktestResult {
  fund_code: string;
  strategy: BacktestStrategy;
  start_date: string;
  end_date: string;
  base_amount: number;
  total_invested: number;
  final_value: number;
  total_return_pct: number;
  annual_return_pct: number;
  max_drawdown_pct: number;
  sharpe_ratio: number;
  trades: BacktestTrade[];
  timeline: BacktestTimelinePoint[];
  comparison: {
    lump_sum: { invested: number; final_value: number; return_pct: number };
    dca: { invested: number; final_value: number; return_pct: number };
  };
}
