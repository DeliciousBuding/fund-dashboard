// 数据查询 hooks —— TanStack Query + contracts zod 边界校验。
// 每个 hook 对应一个后端端点；401 由 api client 统一跳登录。

import {
  CompareResultSchema,
  DrawdownResultSchema,
  ExchangeRateSchema,
  FundDetailSchema,
  FundInfoSchema,
  IndexHistorySchema,
  InvestmentHarnessSnapshotSchema,
  InvestmentSourceBriefSchema,
  MarketIndexSchema,
  NavPointSchema,
  PenetrationResultSchema,
  PortfolioAllocationSchema,
  PortfolioSchema,
  SecurityInfoSchema,
  USStockInfoSchema,
  XirrResultSchema,
} from "@fund-dashboard/contracts";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { api } from "./api";

const FIVE_MIN = 5 * 60 * 1000;

function withPortfolio(path: string, portfolioId?: number): string {
  if (!portfolioId || portfolioId <= 1) return path;
  const sep = path.includes("?") ? "&" : "?";
  return `${path}${sep}portfolio_id=${portfolioId}`;
}

async function fetchValidated<S extends z.ZodType>(path: string, schema: S, signal?: AbortSignal) {
  const data = await api<unknown>(path, { signal });
  return schema.parse(data);
}

export function usePortfolio(portfolioId?: number) {
  return useQuery({
    queryKey: ["portfolio", portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(withPortfolio("/api/portfolio/", portfolioId), PortfolioSchema, signal),
    staleTime: FIVE_MIN,
  });
}

export interface PortfolioDefinition {
  id: number;
  name: string;
  description?: string;
}

export function usePortfolios() {
  return useQuery({
    queryKey: ["portfolio-definitions"],
    queryFn: ({ signal }) => api<PortfolioDefinition[]>("/api/portfolio/portfolios", { signal }),
    staleTime: Infinity,
  });
}

export interface TimelinePoint {
  date: string;
  total_value: number;
  total_cost: number;
  pnl: number;
  pnl_pct: number;
}

export function useTimeline(portfolioId?: number) {
  return useQuery({
    queryKey: ["portfolio-timeline", portfolioId ?? 1],
    queryFn: ({ signal }) =>
      api<TimelinePoint[]>(withPortfolio("/api/portfolio/timeline", portfolioId), { signal }),
    staleTime: FIVE_MIN,
  });
}

export function useAllocation(portfolioId?: number) {
  return useQuery({
    queryKey: ["portfolio-allocation", portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(
        withPortfolio("/api/portfolio/allocation", portfolioId),
        PortfolioAllocationSchema,
        signal,
      ),
    staleTime: FIVE_MIN,
  });
}

export function usePortfolioXirr(portfolioId?: number) {
  return useQuery({
    queryKey: ["portfolio-xirr", portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(withPortfolio("/api/portfolio/xirr", portfolioId), XirrResultSchema, signal),
    staleTime: FIVE_MIN,
  });
}

export function useFunds(portfolioId?: number) {
  return useQuery({
    queryKey: ["funds", portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(withPortfolio("/api/funds/", portfolioId), z.array(FundInfoSchema), signal),
    staleTime: FIVE_MIN,
  });
}

export function useFundDetail(code: string, portfolioId?: number) {
  return useQuery({
    queryKey: ["fund-detail", code, portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(
        withPortfolio(`/api/funds/${encodeURIComponent(code)}`, portfolioId),
        FundDetailSchema,
        signal,
      ),
    staleTime: FIVE_MIN,
    enabled: code.length > 0,
  });
}

export function useNavHistory(code: string, limit = 2000) {
  return useQuery({
    queryKey: ["nav", code, limit],
    queryFn: ({ signal }) =>
      fetchValidated(
        `/api/funds/${encodeURIComponent(code)}/nav?limit=${limit}`,
        z.array(NavPointSchema),
        signal,
      ),
    staleTime: FIVE_MIN,
    enabled: code.length > 0,
  });
}

export function useFundXirr(code: string, portfolioId?: number) {
  return useQuery({
    queryKey: ["fund-xirr", code, portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(
        withPortfolio(`/api/funds/${encodeURIComponent(code)}/xirr`, portfolioId),
        XirrResultSchema,
        signal,
      ),
    staleTime: FIVE_MIN,
    enabled: code.length > 0,
  });
}

export function useDrawdown(code: string) {
  return useQuery({
    queryKey: ["drawdown", code],
    queryFn: ({ signal }) =>
      fetchValidated(
        `/api/funds/${encodeURIComponent(code)}/drawdown`,
        DrawdownResultSchema,
        signal,
      ),
    staleTime: FIVE_MIN,
    enabled: code.length > 0,
  });
}

export interface DcaComputeResult {
  code?: string;
  base_amount: number;
  multiplier: number;
  suggested_amount: number;
  signal: string;
  explanation: string;
  error?: string;
}

export function useDcaCompute(code: string, baseAmount: number, mode: string) {
  return useQuery({
    queryKey: ["dca-compute", code, baseAmount, mode],
    queryFn: ({ signal }) =>
      api<DcaComputeResult>(
        `/api/funds/${encodeURIComponent(code)}/dca?base_amount=${baseAmount}&mode=${encodeURIComponent(mode)}`,
        { signal },
      ),
    staleTime: FIVE_MIN,
    enabled: code.length > 0 && baseAmount > 0,
  });
}

export function useCompare(codes: string[], portfolioId?: number) {
  return useQuery({
    queryKey: ["compare", codes.join(","), portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(
        withPortfolio(
          `/api/analysis/compare?codes=${codes.map(encodeURIComponent).join(",")}`,
          portfolioId,
        ),
        CompareResultSchema,
        signal,
      ),
    staleTime: FIVE_MIN,
    enabled: codes.length > 0,
  });
}

export function useIndices() {
  return useQuery({
    queryKey: ["market-indices"],
    queryFn: ({ signal }) =>
      fetchValidated("/api/market/indices", z.array(MarketIndexSchema), signal),
    staleTime: 60 * 1000,
  });
}

export function useExchangeRate() {
  return useQuery({
    queryKey: ["exchange-rate"],
    queryFn: ({ signal }) =>
      fetchValidated("/api/market/exchange-rate", ExchangeRateSchema, signal),
    staleTime: 10 * 60 * 1000,
  });
}

export function useIndexHistory(code: string, range: string, interval?: string) {
  return useQuery({
    queryKey: ["index-history", code, range, interval ?? ""],
    queryFn: ({ signal }) =>
      fetchValidated(
        `/api/market/index/${encodeURIComponent(code)}/history?range=${encodeURIComponent(range)}${interval ? `&interval=${encodeURIComponent(interval)}` : ""}`,
        IndexHistorySchema,
        signal,
      ),
    staleTime: FIVE_MIN,
    enabled: code.length > 0,
  });
}

export function useUSStock(code: string) {
  return useQuery({
    queryKey: ["us-stock", code],
    queryFn: ({ signal }) =>
      fetchValidated(`/api/stocks/${encodeURIComponent(code)}`, USStockInfoSchema, signal),
    staleTime: FIVE_MIN,
    enabled: code.length > 0,
  });
}

export function usePenetration(portfolioId?: number) {
  return useQuery({
    queryKey: ["penetration", portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(
        withPortfolio("/api/portfolio/penetration", portfolioId),
        PenetrationResultSchema,
        signal,
      ),
    staleTime: FIVE_MIN,
  });
}

export function useSecurities(portfolioId?: number) {
  return useQuery({
    queryKey: ["securities", portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(
        withPortfolio("/api/securities", portfolioId),
        z.array(SecurityInfoSchema),
        signal,
      ),
    staleTime: FIVE_MIN,
  });
}

export function useHarness(portfolioId?: number) {
  return useQuery({
    queryKey: ["harness", portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(
        withPortfolio("/api/portfolio/harness", portfolioId),
        InvestmentHarnessSnapshotSchema,
        signal,
      ),
    staleTime: FIVE_MIN,
  });
}

export function useSourceBrief(portfolioId?: number) {
  return useQuery({
    queryKey: ["source-brief", portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(
        withPortfolio("/api/portfolio/source-brief", portfolioId),
        InvestmentSourceBriefSchema,
        signal,
      ),
    staleTime: FIVE_MIN,
  });
}

export interface SourceEvent {
  id: number;
  title: string;
  url?: string | null;
  source: string;
  snippet?: string | null;
  related_security_code?: string | null;
  related_security_name?: string | null;
  is_read: boolean;
  is_useful: boolean;
  fetched_at: string;
}

export function useSourceEvents(opts?: { unreadOnly?: boolean }) {
  const params = new URLSearchParams();
  if (opts?.unreadOnly) params.set("unread", "true");
  params.set("limit", "100");
  return useQuery({
    queryKey: ["source-events", opts?.unreadOnly ?? false],
    queryFn: ({ signal }) =>
      api<SourceEvent[]>(`/api/portfolio/source-events?${params}`, { signal }),
    staleTime: 60 * 1000,
  });
}

// ── W4 台账 / DCA ────────────────────────────────────────────────────

export interface TransactionListItem {
  seq: number;
  trade_time: string | null;
  confirm_date: string | null;
  direction: string | null;
  trade_type: string | null;
  fund_code: string;
  fund_name: string | null;
  amount: number | null;
  shares: number | null;
  fee: number | null;
  order_id: string | null;
  anomaly: string | null;
  settlement_days: number | null;
  portfolio_id: number | null;
}

export interface TransactionsPage {
  transactions: TransactionListItem[];
  total: number;
}

export interface TransactionsFilter {
  fundCode?: string;
  direction?: string;
  search?: string;
  limit?: number;
  offset?: number;
  portfolioId?: number;
}

export function useTransactions(filter: TransactionsFilter) {
  const params = new URLSearchParams();
  if (filter.fundCode) params.set("fund_code", filter.fundCode);
  if (filter.direction) params.set("direction", filter.direction);
  if (filter.search) params.set("search", filter.search);
  params.set("limit", String(filter.limit ?? 200));
  params.set("offset", String(filter.offset ?? 0));
  if (filter.portfolioId && filter.portfolioId > 1)
    params.set("portfolio_id", String(filter.portfolioId));
  const qs = params.toString();
  return useQuery({
    queryKey: ["transactions", qs],
    queryFn: ({ signal }) => api<TransactionsPage>(`/api/transactions?${qs}`, { signal }),
    staleTime: 60 * 1000,
    placeholderData: (prev) => prev,
  });
}

export interface DcaPlan {
  id: number;
  fund_code: string;
  fund_name: string | null;
  amount: number;
  frequency: string;
  weekday_mask: string;
  trade_type: string;
  portfolio_id: number;
  start_date: string;
  end_date: string | null;
  active: number;
  source: string;
  created_at: string;
  updated_at: string;
}

export function useDcaPlans(activeOnly = false, portfolioId?: number) {
  const params = new URLSearchParams();
  if (activeOnly) params.set("active", "true");
  if (portfolioId && portfolioId > 1) params.set("portfolio_id", String(portfolioId));
  const qs = params.toString();
  return useQuery({
    queryKey: ["dca-plans", qs],
    queryFn: ({ signal }) =>
      api<{ plans: DcaPlan[] }>(`/api/dca/plans${qs ? `?${qs}` : ""}`, { signal }),
    staleTime: 60 * 1000,
  });
}
