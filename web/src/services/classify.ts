// ═══════ Fund & stock classification ═══════
// Ported from the retired packages/web (fbeafd9^:services/classify.ts).
// Adaptations: types come from @fund-dashboard/contracts; display labels are
// direct zh-CN (single-tenant zh-only decision); colors are semantic tone
// names resolved by the chart/UI theme layer, not hex literals.
import type { FundInfo, SecurityInfo } from "@fund-dashboard/contracts";

// --- Stock market meta ---
export type ToneName = "up" | "down" | "accent" | "info" | "warn" | "muted";

export const STOCK_MARKETS: Record<string, { label: string; tone: ToneName }> = {
  sh: { label: "沪A", tone: "up" },
  sz: { label: "深A", tone: "down" },
  hk: { label: "港股", tone: "warn" },
  us: { label: "美股", tone: "info" },
};

/** Detect a stock code's market. Returns the market key (sh/sz/hk/us) or undefined. */
export function detectStockMarket(code: string): string | undefined {
  if (!code) return undefined;
  const c = code.trim();
  // 6xxxxx → Shanghai A-stock
  if (/^6\d{5}$/.test(c)) return "sh";
  // 0xxxxx or 3xxxxx → Shenzhen A-stock
  if (/^[03]\d{5}$/.test(c)) return "sz";
  // 5-digit → Hong Kong stock
  if (/^\d{5}$/.test(c)) return "hk";
  // Alphabetic ticker (1-5 uppercase letters, e.g. NVDA, AAPL, META) → US stock
  if (/^[A-Za-z]{1,5}$/.test(c)) return "us";
  return undefined;
}

// Fund categories — keywords stay Chinese for Eastmoney/DB name matching.
export const CATS: Record<string, { label: string; funds: string[] }> = {
  nasdaq: { label: "纳斯达克", funds: ["纳斯达克", "纳指", "NASDAQ", "Nasdaq", "NDX"] },
  tech: {
    label: "科技主题",
    funds: [
      "科创",
      "科技",
      "半导体",
      "芯片",
      "人工智能",
      "机器人",
      "计算机",
      "信息产业",
      "高端装备",
      "新能源车",
    ],
  },
  dividend: { label: "红利价值", funds: ["红利", "港股通红利", "主要消费", "金融"] },
  gold: { label: "黄金商品", funds: ["黄金", "白银", "有色金属", "电力"] },
  bond: { label: "债券存单", funds: ["债", "存单", "稳利"] },
  qdii: { label: "海外其他", funds: [] },
  money: { label: "货币基金", funds: ["货币", "日日盈"] },
  ashare: { label: "A股股票", funds: [] },
  hkstock: { label: "港股股票", funds: [] },
  other: { label: "其他", funds: [] },
};

export const CAT_ORDER = ["nasdaq", "dividend", "tech", "gold", "bond", "qdii", "money", "other"];

export function classify(f: FundInfo): string {
  // Stock detection: use security_type from backend, NOT code pattern guessing
  if (f.security_type === "stock") {
    const mkt = f.market || detectStockMarket(f.code) || "";
    if (mkt === "sh" || mkt === "sz") return "ashare";
    if (mkt === "hk") return "hkstock";
    return "ashare";
  }

  const n = f.name || "";
  const fundType = f.type || "";
  // Fund keyword classification — CN + EN name tokens (case-insensitive for Latin keywords) (#188)
  for (const [cat, cfg] of Object.entries(CATS)) {
    if (cfg.funds.some((kw) => nameMatchesKeyword(n, kw))) return cat;
  }
  const typeU = fundType.toUpperCase();
  if (typeU.includes("QDII")) return "qdii";
  if (fundType.includes("债")) return "bond";
  if (fundType.includes("货币")) return "money";
  // Align with Go holdings_coverage / harness_quality fund_type filters (#188)
  if (typeU.includes("ETF") || fundType.includes("指数") || fundType.includes("混合"))
    return "other";
  if (fundType.includes("股票")) return "tech";
  return "other";
}

/** Case-sensitive for CJK keywords; case-insensitive for Latin tokens. */
export function nameMatchesKeyword(name: string, keyword: string): boolean {
  if (!name || !keyword) return false;
  if (/[一-鿿]/.test(keyword)) return name.includes(keyword);
  return name.toUpperCase().includes(keyword.toUpperCase());
}

/** Nasdaq / NDX themed fund names (shared by sidebar classify + market page). */
export function isNasdaqFundName(name: string): boolean {
  return CATS.nasdaq.funds.some((kw) => nameMatchesKeyword(name || "", kw));
}

// ═══════ Stock-only category config (for SecurityInfo from /api/securities) ═══════

export const STOCK_CATS: Record<string, { label: string }> = {
  "stock-a": { label: "A股" },
  "stock-hk": { label: "港股通" },
  "stock-us": { label: "美股" },
};

export const STOCK_CAT_ORDER = ["stock-a", "stock-hk", "stock-us"];

/** Classify a SecurityInfo into a stock category based on its market field. */
export function classifySecurity(s: SecurityInfo): string {
  switch (s.market) {
    case "sh":
    case "sz":
      return "stock-a";
    case "hk":
      return "stock-hk";
    case "us":
      return "stock-us";
    default:
      return "stock-a"; // fallback
  }
}

/** Market label for a stock code (e.g. "沪A" for 6xxxxx), '' when unknown. */
export function getMarketLabel(code: string): string {
  const mkt = detectStockMarket(code);
  if (!mkt) return "";
  return STOCK_MARKETS[mkt]?.label ?? "";
}

/** Check if a security code represents a US stock (alphabetic ticker). */
export function isUSStock(code: string): boolean {
  return detectStockMarket(code) === "us";
}

/** Get the currency symbol for a given market. */
export function getCurrencySymbol(market?: string): string {
  if (market === "us") return "$";
  return "¥";
}
