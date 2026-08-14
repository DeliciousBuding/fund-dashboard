import { z } from "zod";
import i18n from "../i18n";
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
import type { Transaction } from "./types";

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


/** Accept (portfolioId?, signal?) or legacy (signal?) for list endpoints. */
function portfolioIdAndSignal(
  a?: number | AbortSignal,
  b?: AbortSignal,
): [number | undefined, AbortSignal | undefined] {
  if (typeof AbortSignal !== "undefined" && a instanceof AbortSignal) {
    return [undefined, a];
  }
  return [typeof a === "number" ? a : undefined, b];
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

export async function fetchFunds(portfolioIdOrSignal?: number | AbortSignal, signal?: AbortSignal): Promise<z.infer<typeof FundInfoSchema>[]> {
  const [portfolioId, sig] = portfolioIdAndSignal(portfolioIdOrSignal, signal);
  const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : '';
  return fetchJson(`${BASE}/funds${qs}`, z.array(FundInfoSchema), sig);
}

export async function fetchFundDetail(code: string, portfolioIdOrSignal?: number | AbortSignal, signal?: AbortSignal): Promise<z.infer<typeof FundDetailSchema>> {
  const [portfolioId, sig] = portfolioIdAndSignal(portfolioIdOrSignal, signal);
  const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : '';
  return fetchJson(`${BASE}/funds/${encodeURIComponent(code)}${qs}`, FundDetailSchema, sig);
}

export async function fetchNav(code: string, signal?: AbortSignal): Promise<z.infer<typeof NavPointSchema>[]> {
  return fetchJson(`${BASE}/funds/${encodeURIComponent(code)}/nav`, z.array(NavPointSchema), signal);
}

export async function fetchXirr(
  code: string,
  portfolioIdOrSignal?: number | AbortSignal,
  signal?: AbortSignal,
): Promise<z.infer<typeof XirrResultSchema>> {
  const [portfolioId, sig] = portfolioIdAndSignal(portfolioIdOrSignal, signal);
  const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : '';
  return fetchJson(`${BASE}/funds/${encodeURIComponent(code)}/xirr${qs}`, XirrResultSchema, sig);
}

export async function fetchDrawdown(code: string, signal?: AbortSignal): Promise<z.infer<typeof DrawdownResultSchema>> {
  return fetchJson(`${BASE}/funds/${encodeURIComponent(code)}/drawdown`, DrawdownResultSchema, signal);
}

export async function fetchDcaPlan(
  code: string,
  options: { base?: number; mode?: 'nav_deviation' | 'change_pct'; portfolioId?: number } = {},
  signal?: AbortSignal,
): Promise<z.infer<typeof DcaPlanSchema>> {
  const params = new URLSearchParams();
  if (options.base != null) params.set('base', String(options.base));
  if (options.mode) params.set('mode', options.mode);
  if (options.portfolioId != null) params.set('portfolio_id', String(options.portfolioId));
  const qs = params.toString();
  return fetchJson(`${BASE}/funds/${encodeURIComponent(code)}/dca${qs ? `?${qs}` : ''}`, DcaPlanSchema, signal);
}

export async function fetchPortfolioXirr(portfolioId?: number, signal?: AbortSignal): Promise<z.infer<typeof XirrResultSchema>> {
  const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : '';
  return fetchJson(`${BASE}/portfolio/xirr${qs}`, XirrResultSchema, signal);
}

/** Portfolio value timeline for Overview chart (facts only). */
export interface PortfolioTimelinePoint {
  date: string;
  total_value: number;
  total_cost: number;
  pnl: number;
  pnl_pct: number;
}

export async function fetchPortfolioTimeline(
  portfolioId?: number,
  signal?: AbortSignal,
): Promise<PortfolioTimelinePoint[]> {
  const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : '';
  // Backend returns raw points without envelope; pnl_pct is float64.
  return fetchJson(`${BASE}/portfolio/timeline${qs}`, z.array(z.object({
    date: z.string(),
    total_value: z.number(),
    total_cost: z.number(),
    pnl: z.number(),
    pnl_pct: z.number(),
  }).passthrough()), signal);
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
export async function fetchSecurities(portfolioIdOrSignal?: number | AbortSignal, signal?: AbortSignal): Promise<z.infer<typeof SecurityInfoSchema>[]> {
  const [portfolioId, sig] = portfolioIdAndSignal(portfolioIdOrSignal, signal);
  const qs = portfolioId != null ? `?portfolio_id=${portfolioId}` : '';
  return fetchJson(`${BASE}/securities${qs}`, z.array(SecurityInfoSchema), sig);
}

// ═══════ Transaction CRUD ═══════

/** Prefer short status-based ApiError; never surface raw JSON error bodies to UI. */
async function mutationError(res: Response): Promise<ApiError> {
  let detail = '';
  try {
    const text = await res.text();
    if (text) {
      try {
        const j = JSON.parse(text) as { error?: unknown; message?: unknown };
        const e = j?.error ?? j?.message;
        if (typeof e === 'string' && e.trim() && e.length <= 120 && !/^\s*[{\[]/.test(e)) {
          detail = e.trim();
        }
      } catch {
        // non-JSON body — ignore technical dump
      }
    }
  } catch {
    /* ignore */
  }
  const base = `HTTP ${res.status}`;
  return new ApiError(detail && !/expected\s|Zod|ECONN|at path/i.test(detail) ? `${base}: ${detail}` : base, res.status);
}

async function fetchPost(path: string, body: any, signal?: AbortSignal): Promise<any> {
  const res = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal });
  if (!res.ok) throw await mutationError(res);
  return res.json();
}

async function fetchPut(path: string, body: any, signal?: AbortSignal): Promise<any> {
  const res = await fetch(path, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal });
  if (!res.ok) throw await mutationError(res);
  return res.json();
}

async function fetchDelete(path: string): Promise<any> {
  const res = await fetch(path, { method: 'DELETE' });
  if (!res.ok) throw await mutationError(res);
  return res.json();
}

export async function addTransactionApi(tx: {
  fund_code: string; trade_time: string; direction: 'buy' | 'sell' | 'dividend';
  trade_type: string; confirm_amount: number; confirm_share?: number; fee?: number; order_id?: string;
}): Promise<{ ok: boolean; imported: number }> {
  // SPA path is edge-auth only; do not call /api/admin/* (requires MCP_API_KEY).
  return fetchPost(`${BASE}/transactions/import`, { transactions: [{ ...tx, order_id: tx.order_id || `web_${crypto.randomUUID()}` }] });
}

export async function updateTransactionApi(seq: number, fields: Record<string, any>): Promise<any> {
  return fetchPut(`${BASE}/transactions/${seq}`, fields);
}

export async function deleteTransactionApi(seq: number): Promise<any> {
  return fetchDelete(`${BASE}/transactions/${seq}`);
}

// ═══════ Transaction CSV ═══════

/** Export transactions as CSV string (headers / direction labels follow active locale) */
export function transactionsToCsv(transactions: Transaction[], _fundName: string): string {
  const headers = [
    i18n.t('fundDetail.csv.tradeTime'),
    i18n.t('fundDetail.csv.confirmDate'),
    i18n.t('fundDetail.csv.direction'),
    i18n.t('fundDetail.csv.amount'),
    i18n.t('fundDetail.csv.shares'),
    i18n.t('fundDetail.csv.dealNav'),
    i18n.t('fundDetail.csv.inferredNav'),
    i18n.t('fundDetail.csv.fee'),
    i18n.t('fundDetail.csv.settlement'),
    i18n.t('fundDetail.csv.tradeDay'),
  ];
  const dirMap: Record<string, string> = {
    buy: i18n.t('fundDetail.dir.buy'),
    sell: i18n.t('fundDetail.dir.sell'),
    dividend: i18n.t('fundDetail.dir.dividend'),
    convert_in: i18n.t('fundDetail.dir.convert_in'),
    convert_out: i18n.t('fundDetail.dir.convert_out'),
    forced_redeem: i18n.t('fundDetail.dir.forced_redeem'),
  };
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
  // Go returns { indices: { [code]: quote }, count, ... }; FE contract is MarketIndex[].
  const GoIndicesEnvelope = z.object({
    indices: z.record(z.string(), z.object({
      name: z.string().optional().default(''),
      market: z.string().optional().default(''),
      price: z.number().nullable().optional().default(null),
      change_pct: z.number().nullable().optional().default(null),
      change: z.number().nullable().optional().default(null),
      change_amt: z.number().nullable().optional().default(null),
      updated_at: z.string().optional().default(''),
    }).passthrough()).optional().default({}),
  }).passthrough();

  const raw = await fetchJson(`${BASE}/market/indices`, z.union([z.array(MarketIndexSchema), GoIndicesEnvelope]), signal);
  if (Array.isArray(raw)) return raw;
  return Object.entries(raw.indices ?? {}).map(([code, quote]) => ({
    code,
    name: quote.name || code,
    market: quote.market || '',
    price: quote.price ?? null,
    change_pct: quote.change_pct ?? null,
    change_amt: quote.change_amt ?? quote.change ?? null,
    updated_at: quote.updated_at || '',
  }));
}

/** Fetch live US stock data — price, profile, history */
export async function fetchUSStock(code: string, signal?: AbortSignal): Promise<z.infer<typeof USStockInfoSchema>> {
  return fetchJson(`${BASE}/stocks/${encodeURIComponent(code)}`, USStockInfoSchema, signal);
}

/** Fetch current USD/CNY exchange rate */
export async function fetchExchangeRate(signal?: AbortSignal): Promise<z.infer<typeof ExchangeRateSchema>> {
  return fetchJson(`${BASE}/market/exchange-rate`, ExchangeRateSchema, signal);
}

/** Fetch historical data for a market index */
export async function fetchIndexHistory(code: string, range?: string, interval?: string, signal?: AbortSignal): Promise<z.infer<typeof IndexHistorySchema>> {
  const r = range || '1y';
  const params = new URLSearchParams({ range: r });
  if (interval) params.set('interval', interval);
  return fetchJson(
    `${BASE}/market/index/${encodeURIComponent(code)}/history?${params.toString()}`,
    IndexHistorySchema,
    signal,
  );
}

/** Fetch a single live index quote (cached fallback on failure) */
export async function fetchIndexLive(code: string, signal?: AbortSignal): Promise<z.infer<typeof MarketIndexSchema> & {
  previous_close?: number; change?: number; high?: number; low?: number;
  open?: number; volume?: number; currency?: string; market_time?: string; source: string;
}> {
  return fetchJson(`${BASE}/market/index/${encodeURIComponent(code)}`, MarketIndexSchema.passthrough(), signal) as any;
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

/** Download transactions as Excel (.xlsx) from the server.
 *  Accept-Language mirrors SPA locale so headers/direction match CSV i18n (#154). */
export async function downloadTransactionsXlsx(transactions: Transaction[], fundName: string) {
  const lang = (i18n.language || 'zh').toLowerCase().startsWith('en') ? 'en' : 'zh';
  const res = await fetch('/api/export/transactions-xlsx', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Accept-Language': lang,
    },
    body: JSON.stringify({ transactions, fundName }),
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);
  const blob = await res.blob();
  downloadBlob(blob, `${fundName || 'transactions'}.xlsx`);
}

// ═══════ Source Events (V4) ═══════

export async function fetchCompare(
  codes: string[],
  portfolioIdOrSignal?: number | AbortSignal,
  signal?: AbortSignal,
): Promise<z.infer<typeof CompareResultSchema>> {
  const [portfolioId, sig] = portfolioIdAndSignal(portfolioIdOrSignal, signal);
  const params = new URLSearchParams();
  params.set('codes', codes.join(','));
  if (portfolioId != null) params.set('portfolio_id', String(portfolioId));
  return fetchJson(`${BASE}/analysis/compare?${params.toString()}`, CompareResultSchema, sig);
}

export async function fetchSourceEvents(
  opts: { code?: string; source?: string; show_read?: boolean; limit?: number; portfolioId?: number } = {},
  signal?: AbortSignal,
): Promise<z.infer<typeof SourceEventsResponseSchema>> {
  const params = new URLSearchParams();
  if (opts.code) params.set('code', opts.code);
  if (opts.source) params.set('source', opts.source);
  if (opts.show_read) params.set('show_read', '1');
  if (opts.limit != null) params.set('limit', String(opts.limit));
  // Reserved for multi-portfolio source-event scoping once schema gains portfolio_id.
  if (opts.portfolioId != null) params.set('portfolio_id', String(opts.portfolioId));
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
