// ═══════ Fund & stock classification ═══════
// Extracted from utils.ts — classification helpers.
import type { FundInfo, SecurityInfo } from '../api'
import { lightTheme } from '../styles/theme'

// --- Stock market configuration ---
// Colors from theme light tokens (semantic accents SSOT).
// labelKey resolves via i18n (market.label.*); display text is not hard-coded here.
export const STOCK_MARKETS: Record<string, { labelKey: string; color: string }> = {
  sh: { labelKey: 'market.label.sh', color: lightTheme.up },
  sz: { labelKey: 'market.label.sz', color: lightTheme.down },
  hk: { labelKey: 'market.label.hk', color: lightTheme.amber },
  us: { labelKey: 'market.label.us', color: lightTheme.blue },
};

/** Detect a stock code's market. Returns the market key (sh/sz/hk/us) or undefined. */
export function detectStockMarket(code: string): string | undefined {
  if (!code) return undefined;
  const c = code.trim();
  // 6xxxxx → Shanghai A-stock
  if (/^6\d{5}$/.test(c)) return 'sh';
  // 0xxxxx or 3xxxxx → Shenzhen A-stock
  if (/^[03]\d{5}$/.test(c)) return 'sz';
  // 5-digit → Hong Kong stock
  if (/^\d{5}$/.test(c)) return 'hk';
  // Alphabetic ticker (1-5 uppercase letters, e.g. NVDA, AAPL, META) → US stock
  if (/^[A-Za-z]{1,5}$/.test(c)) return 'us';
  return undefined;
}

// nameKey → i18n category.*; funds keywords stay Chinese for Eastmoney/DB name matching.
export const CATS: Record<string, { nameKey: string; funds: string[] }> = {
  nasdaq:  { nameKey: 'category.nasdaq', funds: ['纳斯达克', '纳指', 'NASDAQ', 'Nasdaq', 'NDX'] },
  tech:    { nameKey: 'category.tech', funds: ['科创', '科技', '半导体', '芯片', '人工智能', '机器人', '计算机', '信息产业', '高端装备', '新能源车'] },
  dividend:{ nameKey: 'category.dividend', funds: ['红利', '港股通红利', '主要消费', '金融'] },
  gold:    { nameKey: 'category.gold', funds: ['黄金', '白银', '有色金属', '电力'] },
  bond:    { nameKey: 'category.bond', funds: ['债', '存单', '稳利'] },
  qdii:    { nameKey: 'category.qdii', funds: [] },
  money:   { nameKey: 'category.money', funds: ['货币', '日日盈'] },
  ashare:  { nameKey: 'category.ashare', funds: [] },
  hkstock: { nameKey: 'category.hkstock', funds: [] },
  other:   { nameKey: 'category.other', funds: [] },
};

export const CAT_ORDER = ['nasdaq', 'dividend', 'tech', 'gold', 'bond', 'qdii', 'money', 'other'];

export function classify(f: FundInfo): string {
  // Stock detection: use security_type from backend, NOT code pattern guessing
  if (f.security_type === 'stock') {
    const mkt = f.market || detectStockMarket(f.code) || '';
    if (mkt === 'sh' || mkt === 'sz') return 'ashare';
    if (mkt === 'hk') return 'hkstock';
    if (mkt === 'us') return 'ashare'; // US stocks not in fund CATS — routed via stockGroups instead
    return 'ashare';
  }

  const n = f.name || '';
  const fundType = f.type || '';
  // Fund keyword classification — CN + EN name tokens (case-insensitive for Latin keywords) (#188)
  for (const [cat, cfg] of Object.entries(CATS)) {
    if (cfg.funds.some((kw) => nameMatchesKeyword(n, kw))) return cat;
  }
  const typeU = fundType.toUpperCase();
  if (typeU.includes('QDII')) return 'qdii';
  if (fundType.includes('债')) return 'bond';
  if (fundType.includes('货币')) return 'money';
  // Align with Go holdings_coverage / harness_quality fund_type filters (#188)
  if (typeU.includes('ETF') || fundType.includes('指数') || fundType.includes('混合')) return 'other';
  if (fundType.includes('股票')) return 'tech';
  return 'other';
}

/** Case-sensitive for CJK keywords; case-insensitive for Latin tokens. */
export function nameMatchesKeyword(name: string, keyword: string): boolean {
  if (!name || !keyword) return false;
  if (/[一-鿿]/.test(keyword)) return name.includes(keyword);
  return name.toUpperCase().includes(keyword.toUpperCase());
}

/** Nasdaq / NDX themed fund names (shared by Sidebar classify + NasdaqOverviewPage). */
export function isNasdaqFundName(name: string): boolean {
  return CATS.nasdaq.funds.some((kw) => nameMatchesKeyword(name || '', kw));
}

// ═══════ Stock-only category config (for SecurityInfo from /api/securities) ═══════

export const STOCK_CATS: Record<string, { nameKey: string }> = {
  'stock-a':  { nameKey: 'category.stockA' },
  'stock-hk': { nameKey: 'category.stockHk' },
  'stock-us': { nameKey: 'category.stockUs' },
};

export const STOCK_CAT_ORDER = ['stock-a', 'stock-hk', 'stock-us'];

/** Classify a SecurityInfo into a stock category based on its market field. */
export function classifySecurity(s: SecurityInfo): string {
  switch (s.market) {
    case 'sh':
    case 'sz': return 'stock-a';
    case 'hk': return 'stock-hk';
    case 'us': return 'stock-us';
    default:  return 'stock-a'; // fallback
  }
}

/**
 * Get the i18n key for a stock code's market label.
 * Resolve with t(key) at render. Returns e.g. "market.label.sh" for 6xxxxx.
 */
export function getMarketLabel(code: string): string {
  const mkt = detectStockMarket(code);
  if (!mkt) return '';
  return STOCK_MARKETS[mkt]?.labelKey ?? '';
}

/** Check if a security code represents a US stock (alphabetic ticker). */
export function isUSStock(code: string): boolean {
  return detectStockMarket(code) === 'us';
}

/** Get the currency symbol for a given market. */
export function getCurrencySymbol(market?: string): string {
  if (market === 'us') return '$';
  return '¥';
}
