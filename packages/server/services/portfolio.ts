/**
 * Portfolio Service — backwards-compat re-export.
 *
 * @deprecated Prefer importing from services/index or the specific sub-module.
 * This file exists solely for backward compatibility. New code should use
 * `import { ... } from "../services/index"` directly.
 *
 * Split into sub-modules (2026-06-19):
 *   services/summary.ts, services/harness.ts,
 *   services/source-events.ts, services/system.ts
 */

export {
  getPortfolioSummary, getPortfolioXirr, getPortfolioTimeline,
  getPortfolioPenetration, getFundDetail, getFundXirr,
  getMaxDrawdown, recalculateAllSnapshots, adjustPosition,
  populateSummaryByFund, getSummaryByFund,
  getPortfolioAllocation, getInvestmentHarnessSnapshot, getInvestmentSourceBrief,
  createSourceEvent, getSourceEvents, markSourceEventRead,
  getSystemStatus,
} from "./index";
