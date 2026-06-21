import { z } from "zod";
import {
  FundInfoSchema,
  FundDetailSchema,
  NavPointSchema,
  PortfolioSchema,
  SecurityInfoSchema,
  XirrResultSchema,
  DrawdownResultSchema,
  MarketIndexSchema,
  USStockInfoSchema,
  ExchangeRateSchema,
  IndexHistorySchema,
  IndexHistoryPointSchema,
  PenetrationResultSchema,
  PenetrationStockSchema,
  PenetrationFundSchema,
  PortfolioAllocationSchema,
  DcaPlanSchema,
  InvestmentHarnessSnapshotSchema,
  InvestmentSourceBriefSchema,
  SourceEventsResponseSchema,
  SourceEventSchema,
  CompareResultSchema,
} from "./types";

// Re-export all types so existing imports don't break
export type {
  FundInfo,
  FundDetail,
  Transaction,
  NavPoint,
  Portfolio,
  Market,
  SecurityType,
  SecurityInfo,
  XirrResult,
  DrawdownResult,
  MarketIndex,
  USStockInfo,
  ExchangeRate,
  IndexHistoryPoint,
  IndexHistory,
  USSectorSummary,
  PenetrationFund,
  PenetrationStock,
  PenetrationResult,
  AllocationBucket,
  PortfolioAllocation,
  DcaPlan,
  InvestmentHarnessSnapshot,
  InvestmentHarnessHoldingSignal,
  InvestmentSourceBrief,
  InvestmentSourceQuery,
  InvestmentSourceTarget,
  SourceEvent,
  SourceEventsResponse,
  CompareFund,
  CompareResult,
} from "./types";

const BASE = '/api';

// ═══════ HTTP helpers ═══════

class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) { super(message); this.status = status; }
}

/** In-flight request dedup cache — avoids duplicate parallel fetches.
 *  v3.0 fix (F-INT-1): never hand a caller an in-flight promise that belongs to
 *  an already-aborted caller. Under React.StrictMode (double-invoke effect),
 *  the first fetch is aborted but its promise lingered in the cache, so the
 *  second invoke inherited a rejected (AbortError) promise → portfolio stuck
 *  on "loading" forever. */
const inflight = new Map<string, Promise<any>>();

async function fetchJson<T>(path: string, schema: z.ZodType<T>, signal?: AbortSignal): Promise<T> {
  // Don't reuse a cached promise if this caller is already aborted (it may be
  // the rejected promise of a previously-aborted caller).
  const cached = inflight.get(path);
  if (cached && !signal?.aborted) return cached;

  const promise = fetch(path, { signal })
    .then(async res => {
      if (!res.ok) throw new ApiError(`HTTP ${res.status}: ${res.statusText}`, res.status);
      const data = await res.json();
      return schema.parse(data);
    })
    .finally(() => { inflight.delete(path); });

  inflight.set(path, promise);
  // Drop the cache the instant this caller aborts, so the next caller starts
  // a fresh request instead of inheriting a rejected promise.
  signal?.addEventListener("abort", () => inflight.delete(path));
  return promise;
}

// ═══════ Fund endpoints ═══════

export async function fetchPortfolio(portfolioId?: number, signal?: AbortSignal): Promise<z.infer<typeof PortfolioSchema>> {
  const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : '';
  return fetchJson(`${BASE}/portfolio${qs}`, PortfolioSchema, signal);
}

export async function fetchPortfolioAllocation(portfolioId?: number, signal?: AbortSignal): Promise<z.infer<typeof PortfolioAllocationSchema>> {
  const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : '';
  return fetchJson(`${BASE}/portfolio/allocation${qs}`, PortfolioAllocationSchema, signal);
}

export async function fetchInvestmentHarness(portfolioId?: number, signal?: AbortSignal): Promise<z.infer<typeof InvestmentHarnessSnapshotSchema>> {
  const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : '';
  return fetchJson(`${BASE}/portfolio/harness${qs}`, InvestmentHarnessSnapshotSchema, signal);
}

export async function fetchInvestmentSourceBrief(limit = 20, portfolioId?: number, signal?: AbortSignal): Promise<z.infer<typeof InvestmentSourceBriefSchema>> {
  const pidParam = portfolioId != null ? `&portfolio_id=${portfolioId}` : '';
  return fetchJson(`${BASE}/portfolio/source-brief?limit=${limit}${pidParam}`, InvestmentSourceBriefSchema, signal);
}

export async function fetchFunds(signal?: AbortSignal): Promise<z.infer<typeof FundInfoSchema>[]> {
  return fetchJson(`${BASE}/funds`, z.array(FundInfoSchema), signal);
}

export async function fetchFundDetail(code: string, signal?: AbortSignal): Promise<z.infer<typeof FundDetailSchema>> {
  return fetchJson(`${BASE}/funds/${code}`, FundDetailSchema, signal);
}

export async function fetchNav(code: string, signal?: AbortSignal): Promise<z.infer<typeof NavPointSchema>[]> {
  return fetchJson(`${BASE}/funds/${code}/nav`, z.array(NavPointSchema), signal);
}

export async function fetchXirr(code: string, signal?: AbortSignal): Promise<z.infer<typeof XirrResultSchema>> {
  return fetchJson(`${BASE}/funds/${code}/xirr`, XirrResultSchema, signal);
}

export async function fetchDrawdown(code: string, signal?: AbortSignal): Promise<z.infer<typeof DrawdownResultSchema>> {
  return fetchJson(`${BASE}/funds/${code}/drawdown`, DrawdownResultSchema, signal);
}

export async function fetchDcaPlan(
  code: string,
  options: { base?: number; mode?: 'nav_deviation' | 'change_pct' } = {},
  signal?: AbortSignal,
): Promise<z.infer<typeof DcaPlanSchema>> {
  const params = new URLSearchParams();
  if (options.base != null) params.set('base', String(options.base));
  if (options.mode) params.set('mode', options.mode);
  const qs = params.toString();
  return fetchJson(`${BASE}/funds/${code}/dca${qs ? `?${qs}` : ''}`, DcaPlanSchema, signal);
}

export async function fetchPortfolioXirr(portfolioId?: number, signal?: AbortSignal): Promise<z.infer<typeof XirrResultSchema>> {
  const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : '';
  return fetchJson(`${BASE}/portfolio/xirr${qs}`, XirrResultSchema, signal);
}

// ═══════ Portfolio Penetration ═══════

export async function fetchPortfolioPenetration(portfolioId?: number, signal?: AbortSignal): Promise<z.infer<typeof PenetrationResultSchema>> {
  const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : '';
  return fetchJson(`${BASE}/portfolio/penetration${qs}`, PenetrationResultSchema, signal);
}

// ═══════ Portfolio Definitions ═══════

export interface PortfolioDefinition {
  id: number;
  name: string;
  description: string;
}

/** List all available portfolio definitions */
export async function fetchPortfolios(signal?: AbortSignal): Promise<PortfolioDefinition[]> {
  const res = await fetch(`${BASE}/portfolio/portfolios`, { signal });
  if (!res.ok) throw new ApiError(`HTTP ${res.status}`, res.status);
  return res.json();
}

// ═══════ Security (stock + fund unified) ═══════

/** Fetch all securities — funds and stocks combined. */
export async function fetchSecurities(signal?: AbortSignal): Promise<z.infer<typeof SecurityInfoSchema>[]> {
  return fetchJson(`${BASE}/securities`, z.array(SecurityInfoSchema), signal);
}

// ═══════ Transaction CRUD ═══════

async function fetchPost(path: string, body: any, signal?: AbortSignal): Promise<any> {
  const res = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal });
  if (!res.ok) { const msg = await res.text().catch(() => ''); throw new ApiError(msg || `HTTP ${res.status}`, res.status); }
  return res.json();
}

async function fetchPut(path: string, body: any, signal?: AbortSignal): Promise<any> {
  const res = await fetch(path, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal });
  if (!res.ok) { const msg = await res.text().catch(() => ''); throw new ApiError(msg || `HTTP ${res.status}`, res.status); }
  return res.json();
}

async function fetchDelete(path: string): Promise<any> {
  const res = await fetch(path, { method: 'DELETE' });
  if (!res.ok) { const msg = await res.text().catch(() => ''); throw new ApiError(msg || `HTTP ${res.status}`, res.status); }
  return res.json();
}

export async function addTransactionApi(tx: {
  fund_code: string; trade_time: string; direction: 'buy' | 'sell' | 'dividend';
  trade_type: string; confirm_amount: number; confirm_share?: number; fee?: number; order_id?: string;
}): Promise<{ ok: boolean; imported: number }> {
  return fetchPost(`${BASE}/admin/import-transactions`, { transactions: [{ ...tx, order_id: tx.order_id || `web_${crypto.randomUUID()}` }] });
}

export async function updateTransactionApi(seq: number, fields: Record<string, any>): Promise<any> {
  return fetchPut(`${BASE}/admin/transactions/${seq}`, fields);
}

export async function deleteTransactionApi(seq: number): Promise<any> {
  return fetchDelete(`${BASE}/admin/transactions/${seq}`);
}

// ═══════ Transaction CSV ═══════

/** Transaction type for CSV export — imported from types for reuse */
import type { Transaction } from "./types";

/** Export transactions as CSV string */
export function transactionsToCsv(transactions: Transaction[], fundName: string): string {
  const headers = ['交易时间','确认日期','类型','金额','份额','成交净值','推算净值','手续费','结算','交易日'];
  const dirMap: Record<string, string> = { buy: '买入', sell: '卖出', dividend: '分红', convert_in: '转入', convert_out: '转出', forced_redeem: '强赎' };
  const rows = transactions.map(tx => [
    tx.trade_time.substring(0, 16),
    tx.confirm_date,
    dirMap[tx.direction] || tx.direction,
    tx.amount.toFixed(2),
    tx.shares.toFixed(2),
    tx.nav?.toFixed(4) ?? '',
    tx.inferred_nav?.toFixed(6) ?? '',
    tx.fee > 0 ? tx.fee.toFixed(2) : '',
    tx.settlement_days != null ? `T+${tx.settlement_days}` : '',
    tx.trade_day_type || '',
  ]);
  const bom = '﻿';
  return bom + [headers.join(','), ...rows.map(r => r.map(v => `"${v}"`).join(','))].join('\n');
}

// ═══════ Market index endpoints ═══════

export async function fetchIndices(signal?: AbortSignal): Promise<z.infer<typeof MarketIndexSchema>[]> {
  return fetchJson(`${BASE}/market/indices`, z.array(MarketIndexSchema), signal);
}

/** Fetch live US stock data — price, profile, history */
export async function fetchUSStock(code: string, signal?: AbortSignal): Promise<z.infer<typeof USStockInfoSchema>> {
  return fetchJson(`${BASE}/stocks/${code}`, USStockInfoSchema, signal);
}

/** Fetch current USD/CNY exchange rate */
export async function fetchExchangeRate(signal?: AbortSignal): Promise<z.infer<typeof ExchangeRateSchema>> {
  return fetchJson(`${BASE}/market/exchange-rate`, ExchangeRateSchema, signal);
}

/** Fetch historical data for a market index */
export async function fetchIndexHistory(code: string, range?: string, signal?: AbortSignal): Promise<z.infer<typeof IndexHistorySchema>> {
  const r = range || '1y';
  return fetchJson(`${BASE}/market/index/${code}/history?range=${r}`, IndexHistorySchema, signal);
}

/** Fetch a single live index quote (cached fallback on failure) */
export async function fetchIndexLive(code: string, signal?: AbortSignal): Promise<z.infer<typeof MarketIndexSchema> & {
  previous_close?: number; change?: number; high?: number; low?: number;
  open?: number; volume?: number; currency?: string; market_time?: string; source: string;
}> {
  return fetchJson(`${BASE}/market/index/${code}`, MarketIndexSchema.passthrough(), signal) as any;
}

export function downloadCsv(content: string, filename: string) {
  const blob = new Blob([content], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url; a.download = filename;
  document.body.appendChild(a); a.click();
  document.body.removeChild(a); URL.revokeObjectURL(url);
}

/** Generic blob download helper — used for xlsx, pdf, etc. */
export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url; a.download = filename;
  document.body.appendChild(a); a.click();
  document.body.removeChild(a); URL.revokeObjectURL(url);
}

/** Download transactions as Excel (.xlsx) from the server */
export async function downloadTransactionsXlsx(transactions: Transaction[], fundName: string) {
  const res = await fetch('/api/export/transactions-xlsx', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ transactions, fundName }),
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  const blob = await res.blob();
  downloadBlob(blob, `${fundName || 'transactions'}.xlsx`);
}

// ═══════ Source Events (V4) ═══════

export async function fetchCompare(codes: string[], signal?: AbortSignal): Promise<z.infer<typeof CompareResultSchema>> {
  return fetchJson(`${BASE}/analysis/compare?codes=${codes.join(',')}`, CompareResultSchema, signal);
}

export async function fetchSourceEvents(
  opts: { code?: string; source?: string; show_read?: boolean; limit?: number } = {},
  signal?: AbortSignal,
): Promise<z.infer<typeof SourceEventsResponseSchema>> {
  const params = new URLSearchParams();
  if (opts.code) params.set('code', opts.code);
  if (opts.source) params.set('source', opts.source);
  if (opts.show_read) params.set('show_read', '1');
  if (opts.limit != null) params.set('limit', String(opts.limit));
  const qs = params.toString();
  return fetchJson(`${BASE}/portfolio/source-events${qs ? `?${qs}` : ''}`, SourceEventsResponseSchema, signal);
}

export async function createSourceEventApi(
  event: { title: string; url?: string; source?: string; snippet?: string; query?: string; related_security_code?: string; related_security_name?: string },
  signal?: AbortSignal,
): Promise<z.infer<typeof SourceEventSchema>> {
  const res = await fetch(`${BASE}/portfolio/source-events`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(event),
    signal,
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const data = await res.json();
  return SourceEventSchema.parse(data);
}

export async function markSourceEventApi(
  id: number,
  fields: { is_read?: boolean; is_useful?: boolean },
  signal?: AbortSignal,
): Promise<{ ok: boolean; id: number }> {
  const res = await fetch(`${BASE}/portfolio/source-events/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(fields),
    signal,
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}
