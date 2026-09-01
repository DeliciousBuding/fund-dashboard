// 数据查询 hooks —— TanStack Query + contracts zod 边界校验。
// 每个 hook 对应一个后端端点；401 由 api client 统一跳登录。

import {
  CheckAlertsResponseSchema,
  CompareResultSchema,
  DcaComputeResultSchema,
  DcaPlansResponseSchema,
  DrawdownResultSchema,
  FreshnessReportSchema,
  FundDetailSchema,
  IndexHistorySchema,
  InvestmentHarnessSnapshotSchema,
  MarketIndexSchema,
  NavPointSchema,
  PenetrationResultSchema,
  PortfolioAllocationSchema,
  PortfolioDefinitionSchema,
  PortfolioSchema,
  SecurityInfoSchema,
  SourceEventsResponseSchema,
  TimelinePointSchema,
  type TransactionsListItemSchema,
  TransactionsResponseSchema,
  XirrResultSchema,
} from "@fund-dashboard/contracts";
import { queryOptions, useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { api } from "./api";

const FIVE_MIN = 5 * 60 * 1000;

function withPortfolio(path: string, portfolioId?: number): string {
  if (!portfolioId || portfolioId <= 1) return path;
  const sep = path.includes("?") ? "&" : "?";
  return `${path}${sep}portfolio_id=${portfolioId}`;
}

export async function fetchValidated<S extends z.ZodType>(
  path: string,
  schema: S,
  signal?: AbortSignal,
) {
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

export function usePortfolios() {
  return useQuery({
    queryKey: ["portfolio-definitions"],
    queryFn: ({ signal }) =>
      fetchValidated("/api/portfolio/portfolios", z.array(PortfolioDefinitionSchema), signal),
    staleTime: Infinity,
  });
}

export function useTimeline(portfolioId?: number) {
  return useQuery({
    queryKey: ["portfolio-timeline", portfolioId ?? 1],
    queryFn: ({ signal }) =>
      fetchValidated(
        withPortfolio("/api/portfolio/timeline", portfolioId),
        z.array(TimelinePointSchema),
        signal,
      ),
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

/** NAV 历史 queryOptions 工厂 —— useNavHistory 与 analysis useMultiNav 共用同一缓存键。 */
export function navHistoryOptions(code: string, limit = 2000) {
  return queryOptions({
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

export function useNavHistory(code: string, limit = 2000) {
  return useQuery(navHistoryOptions(code, limit));
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

export function useDcaCompute(code: string, baseAmount: number, mode: string) {
  return useQuery({
    queryKey: ["dca-compute", code, baseAmount, mode],
    queryFn: ({ signal }) =>
      fetchValidated(
        `/api/funds/${encodeURIComponent(code)}/dca?base=${baseAmount}&mode=${encodeURIComponent(mode)}`,
        DcaComputeResultSchema,
        signal,
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
    refetchInterval: 60 * 1000,
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

export function useSourceEvents(opts?: { unreadOnly?: boolean }) {
  const params = new URLSearchParams();
  // 后端读 show_read（false=仅未读）。unreadOnly → show_read=false。
  if (opts?.unreadOnly) params.set("show_read", "false");
  params.set("limit", "100");
  return useQuery({
    queryKey: ["source-events", opts?.unreadOnly ?? false],
    queryFn: ({ signal }) =>
      fetchValidated(`/api/portfolio/source-events?${params}`, SourceEventsResponseSchema, signal),
    staleTime: 60 * 1000,
  });
}

// ── 新鲜度（顶栏徽章 + 持仓陈旧度共用）────────────────────────────────

export function useFreshness() {
  return useQuery({
    queryKey: ["freshness"],
    queryFn: ({ signal }) => fetchValidated("/api/freshness", FreshnessReportSchema, signal),
    staleTime: FIVE_MIN,
  });
}

// ── W4 台账 / DCA ────────────────────────────────────────────────────

export type TransactionListItem = z.infer<typeof TransactionsListItemSchema>;

export interface TransactionsFilter {
  fundCode?: string;
  direction?: string;
  search?: string;
  limit?: number;
  offset?: number;
  portfolioId?: number;
  sortBy?: string;
  sortDesc?: boolean;
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
  if (filter.sortBy) params.set("sort", filter.sortBy);
  if (filter.sortBy) params.set("sort_dir", filter.sortDesc ? "desc" : "asc");
  const qs = params.toString();
  return useQuery({
    queryKey: ["transactions", qs],
    queryFn: ({ signal }) =>
      fetchValidated(`/api/transactions?${qs}`, TransactionsResponseSchema, signal),
    staleTime: 60 * 1000,
    placeholderData: (prev) => prev,
  });
}

export function useDcaPlans(activeOnly = false, portfolioId?: number) {
  const params = new URLSearchParams();
  if (activeOnly) params.set("active", "true");
  if (portfolioId && portfolioId > 1) params.set("portfolio_id", String(portfolioId));
  const qs = params.toString();
  return useQuery({
    queryKey: ["dca-plans", qs],
    queryFn: ({ signal }) =>
      fetchValidated(`/api/dca/plans${qs ? `?${qs}` : ""}`, DcaPlansResponseSchema, signal),
    staleTime: 60 * 1000,
  });
}

// ── W6 告警 ──────────────────────────────────────────────────────────

export function useAlerts() {
  return useQuery({
    queryKey: ["alerts"],
    queryFn: ({ signal }) => fetchValidated("/api/alerts", CheckAlertsResponseSchema, signal),
    staleTime: 5 * 60 * 1000,
  });
}
