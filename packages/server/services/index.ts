/**
 * Portfolio Services — unified export.
 *
 * Re-exports all service functions from sub-modules.
 * Split from the original services/portfolio.ts (2026-06-19).
 */

export { getPortfolioSummary, getPortfolioXirr, getPortfolioTimeline, getPortfolioPenetration, getFundDetail, getFundXirr, getMaxDrawdown, recalculateAllSnapshots, adjustPosition, populateSummaryByFund, getSummaryByFund, recalcSnapshot } from "./summary";
export { getPortfolioAllocation, getInvestmentHarnessSnapshot, getInvestmentSourceBrief } from "./harness";
export { createSourceEvent, getSourceEvents, markSourceEventRead } from "./source-events";
export { getSystemStatus } from "./system";
export { runBacktest } from "./backtest";
export { checkAndNotify, getNotifyConfig } from "./notify";
export type { NotifyConfig, AlertItem } from "./notify";
export { sendFeishuMessage, handleFeishuMessage, handleFeishuEventCallback, getFeishuBotStatus } from "./feishu-bot";
export { generateWeeklyReport, generateMonthlyReport } from "./report";
export type { ReportResult } from "./report";
